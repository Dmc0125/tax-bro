import { z } from 'zod';
import { setTimeout } from 'timers/promises';
import { jsonArrayFrom } from 'kysely/helpers/postgres';
import { Connection, PublicKey } from '@solana/web3.js';

import { createDb, requireEnvVar } from './db/utils';

const dbHost = requireEnvVar('DATABASE_HOST');
const dbUsername = requireEnvVar('DATABASE_USERNAME');
const dbPassword = requireEnvVar('DATABASE_PASSWORD');
const dbPort = requireEnvVar('DATABASE_PORT');
const dbDatabase = requireEnvVar('DATABASE');
const rpcUrl = requireEnvVar('RPC_URL');

const db = createDb(dbHost, dbUsername, dbPassword, dbPort, dbDatabase);
const connection = new Connection(rpcUrl);

const reqBodySchema = z.array(z.string().min(1)).min(1);

async function fetchSignatures(address: string) {
	const sigs = await connection.getSignaturesForAddress(new PublicKey(address), { limit: 1000 });
	console.log(sigs);
}

async function watchQueue() {
	while (true) {
		const { processing, in_queue } = await db
			.selectFrom('wallet')
			.select((qb) => [
				jsonArrayFrom(
					qb
						.selectFrom('wallet')
						.select(['address', 'id'])
						.orderBy('id')
						.where('status', '=', 'processing'),
				).as('processing'),
				jsonArrayFrom(
					qb
						.selectFrom('wallet')
						.select(['address', 'id'])
						.orderBy('id')
						.where('status', '=', 'in_queue'),
				).as('in_queue'),
			])
			.executeTakeFirstOrThrow();

		if (processing.length < 2) {
			const { address, id } = in_queue[0];

			fetchSignatures(address);

			await db
				.updateTable('wallet')
				.set({
					status: 'processing',
				})
				.where('id', '=', id)
				.execute();
		}

		await setTimeout(10000);
	}
}

watchQueue();

const server = Bun.serve({
	port: 3000,
	async fetch(req) {
		if (req.method === 'POST') {
			const b = await req.json();
			const parseResponse = reqBodySchema.safeParse(b);

			if (parseResponse.success) {
				await db
					.insertInto('wallet')
					.values(parseResponse.data.map((a) => ({ address: a, status: 'in_queue' })))
					.execute();
			}
		}

		return new Response('Bun!');
	},
});

console.log('Listening at http://localhost:3000');
