import { Kysely, PostgresDialect } from 'kysely';
import { Pool as PgPool } from 'pg';
import type { DbSignature, DbWallet } from './migrations/00';

export type Database = {
	wallet: DbWallet;
	signature: DbSignature;
};

export function createDb(
	dbHost: string = '0.0.0.0',
	dbUsername: string = 'postgres',
	dbPassword: string = 'super_secret',
	port: string = '6543',
	database: string = 'postgres',
): Kysely<Database> {
	return new Kysely<Database>({
		dialect: new PostgresDialect({
			pool: new PgPool({
				user: dbUsername,
				password: dbPassword,
				host: dbHost,
				port: Number(port),
				database,
			}),
		}),
	});
}

export function requireEnvVar(name: string): string {
	if (!process.env[name]) {
		throw Error(`Missing ENV variable: ${name}`);
	}
	return process.env[name]!;
}
