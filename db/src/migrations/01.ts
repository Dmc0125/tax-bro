import { sql, type Generated, type Kysely } from 'kysely';

import type { Database } from '../utils';

export type DbTokenAccount = {
	wallet_id: number;
	address: string;
	mint?: string;
};

export type DbEventBase = {
	id: Generated<number>;
	user_id: number;
	signature_id: number;
};

export type DbTransferEvent = {
	type: 'transfer';
	data: {
		type: 'transfer_out' | 'transfer_in' | 'internal_transfer';
		from: string;
		to: string;
		mint: string;
		amount_ui: number;
	};
} & DbEventBase;

export type DbEvent = DbTransferEvent;

export async function up(db: Kysely<Database>) {
	await db.schema
		.createTable('token_account')
		.addColumn('wallet_id', 'integer', (c) =>
			c.references('wallet.id').onDelete('cascade').onUpdate('cascade').notNull(),
		)
		.addColumn('address', 'varchar(44)', (c) => c.primaryKey().notNull())
		.addColumn('mint', 'varchar(44)')
		.execute();

	await db.schema.createType('event_type').asEnum(['transfer']).execute();

	await db.schema
		.createTable('event')
		.addColumn('user_id', 'integer', (c) =>
			c.notNull().references('user.id').onDelete('cascade').onUpdate('cascade'),
		)
		.addColumn('signature_id', 'integer', (c) =>
			c.notNull().references('signature.id').onDelete('cascade').onUpdate('cascade'),
		)
		.addColumn('type', sql`event_type`, (c) => c.notNull())
		.addColumn('data', 'jsonb', (c) => c.notNull())
		.execute();
}

export async function down(db: Kysely<Database>) {
	await db.schema.dropTable('token_account').ifExists().execute();
	await db.schema.dropTable('event').ifExists().execute();
	await db.schema.dropType('event_type').ifExists().execute();
}
