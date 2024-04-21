import { GITHUB_CLIENT_ID, GITHUB_CLIENT_SECRET } from '$env/static/private';
import { OAuth2RequestError } from 'arctic';
import { z } from 'zod';

import { db, lucia } from '../../../auth';
import type { RequestEvent } from './$types';

const githubTokenResponseSchema = z.object({
	access_token: z.string().min(1),
	expires_in: z.number().min(0),
	refresh_token: z.string().min(1),
	refresh_token_expires_in: z.number().min(0),
	token_type: z.string().min(1),
	scope: z.string(),
});

export async function GET(event: RequestEvent) {
	const code = event.url.searchParams.get('code');
	const state = event.url.searchParams.get('state');
	const storedState = event.cookies.get('github_oauth_state') ?? null;

	if (!code || !state || !storedState || state !== storedState) {
		return new Response(null, {
			status: 400,
		});
	}

	try {
		const res = await fetch('https://github.com/login/oauth/access_token', {
			method: 'POST',
			headers: {
				'Content-Type': 'application/json',
				Accept: 'application/json',
			},
			body: JSON.stringify({
				code,
				client_id: GITHUB_CLIENT_ID,
				client_secret: GITHUB_CLIENT_SECRET,
			}),
		});
		const tokens = await res.json();
		const parseResult = githubTokenResponseSchema.safeParse(tokens);

		if (!parseResult.success) {
			// Invalid response
			return new Response(null, {
				status: 400,
			});
		}

		const { access_token } = parseResult.data;
		const githubUserResponse = await fetch('https://api.github.com/user', {
			headers: {
				Authorization: `Bearer ${access_token}`,
			},
		});
		const githubUser: GitHubUser = await githubUserResponse.json();
		const existingUser = await db
			.selectFrom('user')
			.where('github_id', '=', githubUser.id)
			.select('id')
			.executeTakeFirst();

		if (existingUser) {
			const session = await lucia.createSession(existingUser.id.toString(), {});
			const sessionCookie = lucia.createSessionCookie(session.id);
			event.cookies.set(sessionCookie.name, sessionCookie.value, {
				path: '.',
				...sessionCookie.attributes,
			});
		} else {
			const insertedUser = await db
				.insertInto('user')
				.values({
					github_id: githubUser.id,
					username: githubUser.login,
				})
				.returning('id')
				.executeTakeFirst();

			if (!insertedUser) {
				// User not inserted
				return new Response(null, {
					status: 400,
				});
			}

			const session = await lucia.createSession(insertedUser.id.toString(), {});
			const sessionCookie = lucia.createSessionCookie(session.id);
			event.cookies.set(sessionCookie.name, sessionCookie.value, {
				path: '.',
				...sessionCookie.attributes,
			});
		}
		return new Response(null, {
			status: 302,
			headers: {
				Location: '/dashboard',
			},
		});
	} catch (e) {
		if (e instanceof OAuth2RequestError) {
			return new Response(null, {
				status: 400,
			});
		}
		// Invalid code
		return new Response(null, {
			status: 500,
		});
	}
}

interface GitHubUser {
	id: number;
	login: string;
}
