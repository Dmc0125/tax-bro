import { sql, type Generated, type Kysely } from 'kysely';
import type { Database } from '../utils';

export type DbWallet = {
	id: Generated<number>;
	address: string;
	status: 'in_queue' | 'processing' | 'updating' | 'processed';
};

export type DbSignature = {
	signature: string;
};

export async function up(db: Kysely<Database>) {
	await db.schema
		.createType('wallet_status')
		.asEnum(['in_queue', 'processing', 'updating', 'processed'])
		.execute();

	await db.schema
		.createTable('signature')
		.addColumn('signature', 'varchar(88)', (c) => c.notNull().primaryKey())
		.execute();

	await db.schema
		.createTable('wallet')
		.addColumn('id', 'serial', (c) => c.notNull().primaryKey())
		.addColumn('address', 'varchar(44)', (c) => c.notNull().unique())
		.addColumn('status', sql`wallet_status`, (c) => c.defaultTo('in_queue'))
		.execute();
}

export async function down(db: Kysely<Database>) {
	await db.schema.dropTable('wallet').ifExists().execute();
	await db.schema.dropTable('signature').ifExists().execute();
	await db.schema.dropType('wallet_status').ifExists().execute();
}
