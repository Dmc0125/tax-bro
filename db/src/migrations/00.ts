import { sql, type Generated, type Kysely } from 'kysely';

import type { Database } from '../utils';

export type DbUser = {
	id: Generated<number>;
	username: string;
	github_id: number;
	avatar_url: string;
};

export type DbSesssion = {
	id: string;
	user_id: number;
	expires_at: Date;
};

export type DbWallet = {
	id: Generated<number>;
	user_id: number;
	address: string;
	signatures_status: 'init' | 'init_processing' | 'processed';
	transactions_status: 'init' | 'init_processing' | 'processed';
	parsing_status: 'init' | 'init_processing' | 'processed';
	last_signature?: string;
	signatures_count: number;
};

export type DbSignature = {
	id: Generated<number>;
	signature: string;
};

export type DbSignaterWalletIntermediary = {
	wallet_id: number;
	signature_id: number;
};

export type DbTransaction = {
	id: Generated<number>;
	signature_id: number;
	accounts: string[];
	timestamp: Date;
	version: 'legacy' | '0';
	fee: number;
	fee_payer: string;
};

export type DbTransactionIx = {
	id: Generated<number>;
	signature_id: number;
	accounts: string[];
	program_id: string;
	/** hex encoded */
	data: string;
};

export type DbTransactionInnerIx = DbTransactionIx & {
	index: number;
};

export async function up(db: Kysely<Database>) {
	await db.schema
		.createTable('user')
		.addColumn('id', 'serial', (c) => c.notNull().primaryKey())
		.addColumn('username', 'varchar(44)', (c) => c.notNull())
		.addColumn('github_id', 'integer', (c) => c.notNull())
		.addColumn('avatar_url', 'varchar', (c) => c.notNull())
		.execute();

	await db.schema
		.createTable('session')
		.addColumn('id', 'varchar', (c) => c.notNull().primaryKey())
		.addColumn('user_id', 'integer', (c) =>
			c.notNull().references('user.id').onDelete('cascade').onUpdate('cascade'),
		)
		.addColumn('expires_at', 'timestamp', (c) => c.notNull())
		.execute();

	await db.schema
		.createType('processing_status')
		.asEnum(['init', 'init_processing', 'processed'])
		.execute();

	await db.schema
		.createTable('signature')
		.addColumn('id', 'serial', (c) => c.notNull().primaryKey())
		.addColumn('signature', 'varchar(88)', (c) => c.notNull().unique())
		.execute();

	await db.schema
		.createTable('wallet')
		.addColumn('id', 'serial', (c) => c.notNull().primaryKey())
		.addColumn('user_id', 'integer', (c) =>
			c.notNull().references('user.id').onDelete('cascade').onUpdate('cascade'),
		)
		.addColumn('address', 'varchar(44)', (c) => c.notNull().unique())
		.addColumn('signatures_status', sql`processing_status`, (c) => c.defaultTo('init'))
		.addColumn('transactions_status', sql`processing_status`, (c) => c.defaultTo('init'))
		.addColumn('parsing_status', sql`processing_status`, (c) => c.defaultTo('init'))
		.addColumn('last_signature', 'varchar(88)')
		.addColumn('signatures_count', 'integer', (c) => c.defaultTo(0))
		.execute();

	await db.schema
		.createTable('signature_wallet_intermediary')
		.addColumn('signature_id', 'integer', (c) =>
			c.references('signature.id').onDelete('cascade').onUpdate('cascade').notNull(),
		)
		.addColumn('wallet_id', 'integer', (c) =>
			c.references('wallet.id').onDelete('cascade').onUpdate('cascade').notNull(),
		)
		.addPrimaryKeyConstraint('swi_pk', ['signature_id', 'wallet_id'])
		.execute();

	await db.schema.createType('tx_version').asEnum(['legacy', '0']).execute();

	await db.schema
		.createTable('transaction')
		.addColumn('id', 'serial', (c) => c.notNull().primaryKey())
		.addColumn('signature_id', 'integer', (c) =>
			c.references('signature.id').onDelete('cascade').onUpdate('cascade').notNull(),
		)
		.addColumn('accounts', sql`varchar(44)[]`, (c) => c.notNull())
		.addColumn('timestamp', 'timestamp', (c) => c.notNull())
		.addColumn('version', sql`tx_version`, (c) => c.notNull())
		.addColumn('fee', 'numeric', (c) => c.notNull())
		.addColumn('fee_payer', 'varchar(44)', (c) => c.notNull())
		.execute();

	await db.schema
		.createTable('transaction_ix')
		.addColumn('id', 'serial', (c) => c.notNull().primaryKey())
		.addColumn('signature_id', 'integer', (c) =>
			c.notNull().references('signature.id').onDelete('cascade').onUpdate('cascade'),
		)
		.addColumn('accounts', sql`varchar(44)[]`, (c) => c.notNull())
		.addColumn('program_id', 'varchar(44)', (c) => c.notNull())
		.addColumn('data', 'varchar')
		.execute();

	await db.schema
		.createTable('transaction_inner_ix')
		.addColumn('id', 'serial', (c) => c.notNull().primaryKey())
		.addColumn('signature_id', 'integer', (c) =>
			c.notNull().references('signature.id').onDelete('cascade').onUpdate('cascade'),
		)
		.addColumn('accounts', sql`varchar(44)[]`, (c) => c.notNull())
		.addColumn('program_id', 'varchar(44)', (c) => c.notNull())
		.addColumn('data', 'varchar')
		.addColumn('index', 'int4', (c) => c.notNull())
		.execute();
}

export async function down(db: Kysely<Database>) {
	await db.schema.dropTable('transaction_ix').ifExists().execute();
	await db.schema.dropTable('transaction_inner_ix').ifExists().execute();
	await db.schema.dropTable('transaction').ifExists().execute();

	await db.schema.dropTable('signature_wallet_intermediary').ifExists().execute();
	await db.schema.dropTable('wallet').ifExists().execute();
	await db.schema.dropTable('signature').ifExists().execute();

	await db.schema.dropTable('session').ifExists().execute();
	await db.schema.dropTable('user').ifExists().execute();

	await db.schema.dropType('processing_status').ifExists().execute();
	await db.schema.dropType('tx_version').ifExists().execute();
}
