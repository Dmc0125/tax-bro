import { Kysely, PostgresDialect } from 'kysely';
import pg from 'pg';

import type {
	DbSesssion,
	DbSignature,
	DbTransaction,
	DbTransactionInnerIx,
	DbTransactionIx,
	DbUser,
	DbWallet,
} from './migrations/00';

export type Database = {
	user: DbUser;
	session: DbSesssion;

	wallet: DbWallet;
	signature: DbSignature;
	transaction: DbTransaction;
	transaction_ix: DbTransactionIx;
	transaction_inner_ix: DbTransactionInnerIx;
};

export function createDb(
	dbHost: string = '0.0.0.0',
	dbUsername: string = 'postgres',
	dbPassword: string = 'super_secret',
	port: string = '6543',
	database: string = 'postgres',
) {
	const pool = new pg.Pool({
		user: dbUsername,
		password: dbPassword,
		host: dbHost,
		port: Number(port),
		database,
	});
	const db = new Kysely<Database>({
		dialect: new PostgresDialect({
			pool,
		}),
	});

	return {
		pool,
		db,
	};
}

export function requireEnvVar(name: string): string {
	if (!process.env[name]) {
		throw Error(`Missing ENV variable: ${name}`);
	}
	return process.env[name]!;
}
