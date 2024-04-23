import { Lucia } from 'lucia';
import { NodePostgresAdapter } from '@lucia-auth/adapter-postgresql';
import { GitHub } from 'arctic';
import { createDb } from 'db';

import { dev } from '$app/environment';
import {
	DATABASE,
	DATABASE_HOST,
	DATABASE_PASSWORD,
	DATABASE_PORT,
	DATABASE_USERNAME,
	GITHUB_CLIENT_ID,
	GITHUB_CLIENT_SECRET,
} from '$env/static/private';

export const github = new GitHub(GITHUB_CLIENT_ID, GITHUB_CLIENT_SECRET);

const { db, pool } = createDb(
	DATABASE_HOST,
	DATABASE_USERNAME,
	DATABASE_PASSWORD,
	DATABASE_PORT,
	DATABASE,
);

export { db };

const adapter = new NodePostgresAdapter(pool, { user: 'user', session: 'session' });

export const lucia = new Lucia(adapter, {
	sessionCookie: {
		attributes: {
			secure: !dev,
		},
	},
	getUserAttributes(attributes) {
		return {
			githubId: attributes.github_id,
			username: attributes.username,
		};
	},
});

declare module 'lucia' {
	interface Register {
		Lucia: typeof lucia;
		DatabaseUserAttributes: DatabaseUserAttributes;
	}
}

interface DatabaseUserAttributes {
	github_id: number;
	username: string;
}
