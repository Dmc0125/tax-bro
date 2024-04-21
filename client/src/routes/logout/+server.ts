import type { RequestEvent } from './$types';
import { lucia } from '../auth';

export async function GET(event: RequestEvent): Promise<Response> {
	if (!event.locals.session) {
		return new Response(null, {
			status: 401,
		});
	}
	await lucia.invalidateSession(event.locals.session.id);
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
