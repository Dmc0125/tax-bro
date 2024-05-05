import type { Handle } from '@sveltejs/kit';

import { db, lucia } from './routes/auth';

export const handle: Handle = async ({ event, resolve }) => {
	if (event.url.pathname.startsWith('/dashboard') || event.url.pathname.startsWith('/logout')) {
		const sessionId = event.cookies.get('auth_session');
		if (!sessionId) {
			return new Response(null, {
				status: 302,
				headers: {
					Location: '/login',
				},
			});
		}
		const session = await db
			.selectFrom('session')
			.where('id', '=', sessionId)
			.selectAll()
			.executeTakeFirst();
		if (!session) {
			return new Response(null, {
				status: 302,
				headers: {
					Location: '/login',
				},
			});
		} else if (session.expires_at.getTime() < new Date().getTime()) {
			await lucia.invalidateSession(session.id);
			const sessionCookie = lucia.createBlankSessionCookie();
			event.cookies.set(sessionCookie.name, sessionCookie.value, {
				path: '.',
				...sessionCookie.attributes,
			});

			return new Response(null, {
				status: 302,
				headers: {
					Location: '/login',
				},
			});
		}
		const user = await db
			.selectFrom('user')
			.where('id', '=', session.user_id)
			.selectAll()
			.executeTakeFirst();
		if (!user) {
			return new Response(null, {
				status: 302,
				headers: {
					Location: '/login',
				},
			});
		}
		event.locals.user = {
			id: user.id,
			githubId: user.github_id,
			username: user.username,
			avatarUrl: user.avatar_url,
		};
		event.locals.session = {
			id: session.id,
			expiresAt: session.expires_at,
		};
	}

	const r = await resolve(event);
	return r;
};
