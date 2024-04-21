import { z } from 'zod';
import { setTimeout } from 'timers/promises';
import { jsonArrayFrom } from 'kysely/helpers/postgres';
import { Connection, Message, MessageV0, PublicKey } from '@solana/web3.js';
import type { Insertable } from 'kysely';
import bs58 from 'bs58';

import { createDb, requireEnvVar } from './db/utils';
import type { DbTransaction, DbTransactionInnerIx, DbTransactionIx } from './db/migrations/00';

const dbHost = requireEnvVar('DATABASE_HOST');
const dbUsername = requireEnvVar('DATABASE_USERNAME');
const dbPassword = requireEnvVar('DATABASE_PASSWORD');
const dbPort = requireEnvVar('DATABASE_PORT');
const dbDatabase = requireEnvVar('DATABASE');
const rpcUrl = requireEnvVar('RPC_URL');

const { db } = createDb(dbHost, dbUsername, dbPassword, dbPort, dbDatabase);
const connection = new Connection(rpcUrl);

const reqBodySchema = z.array(z.string().min(1)).min(1);

async function watchSignatures() {
	while (true) {
		const unfetched = await db
			.selectFrom('signature')
			.orderBy('id asc')
			.select(['signature', 'signature.id'])
			.leftJoin('transaction', 'transaction.signature_id', 'signature.id')
			.where('transaction.id', 'is', null)
			.limit(10)
			.execute();

		console.log(`Fetching transactions: ${unfetched.length}`);

		if (unfetched.length) {
			const txs = await connection.getTransactions(
				unfetched.map((u) => u.signature),
				{ maxSupportedTransactionVersion: 0 },
			);

			const insertableMetas: Insertable<DbTransaction>[] = [];
			const insertableIxs: Insertable<DbTransactionIx>[] = [];
			const insertableInnerIxs: Insertable<DbTransactionInnerIx>[] = [];

			for (let i = 0; i < unfetched.length; i++) {
				const signature = unfetched[i];
				const tx = txs[i];

				if (tx) {
					const timestamp = new Date(tx.blockTime! * 1000);
					let accounts: string[] = [];

					if (tx.version === 0) {
						const m = tx.transaction.message as MessageV0;
						accounts = m.staticAccountKeys.map((a) => a.toString());
						tx.meta?.loadedAddresses?.writable.forEach((a) => {
							accounts.push(a.toString());
						});
						tx.meta?.loadedAddresses?.readonly.forEach((a) => {
							accounts.push(a.toString());
						});

						m.compiledInstructions.forEach((cix) => {
							const d = Buffer.from(cix.data).toString('base64');
							insertableIxs.push({
								signature_id: signature.id,
								data: d,
								program_id: accounts[cix.programIdIndex],
								accounts: cix.accountKeyIndexes.map((a) => accounts[a]),
							});
						});
					} else {
						const m = tx.transaction.message as Message;
						accounts = m.accountKeys.map((a) => a.toString());

						m.instructions.forEach((ix) => {
							const d = Buffer.from(bs58.decode(ix.data)).toString('base64');
							insertableIxs.push({
								signature_id: signature.id,
								data: d,
								program_id: accounts[ix.programIdIndex],
								accounts: ix.accounts.map((a) => accounts[a]),
							});
						});
					}

					if (tx.meta?.innerInstructions) {
						tx.meta.innerInstructions.forEach((ii) => {
							ii.instructions.forEach((iix) => {
								const d = Buffer.from(bs58.decode(iix.data)).toString('base64');

								insertableInnerIxs.push({
									signature_id: signature.id,
									data: d,
									index: ii.index,
									program_id: accounts[iix.programIdIndex],
									accounts: iix.accounts.map((a) => accounts[a]),
								});
							});
						});
					}

					insertableMetas.push({
						signature_id: signature.id,
						timestamp,
						accounts,
						version: tx.version === 0 ? '0' : 'legacy',
						fee: tx.meta?.fee || 0,
						fee_payer: accounts[0],
					});
				}
			}

			await db.transaction().execute(async (tx) => {
				if (insertableMetas.length) {
					await tx.insertInto('transaction').values(insertableMetas).execute();
				}
				if (insertableIxs.length) {
					await tx.insertInto('transaction_ix').values(insertableIxs).execute();
				}
				if (insertableInnerIxs.length) {
					await tx.insertInto('transaction_inner_ix').values(insertableInnerIxs).execute();
				}
			});
		}

		await setTimeout(3000);
	}
}

async function fetchSignatures(address: string, id: number) {
	let beforeSig: string | undefined = undefined;

	while (true) {
		const sigs = await connection.getSignaturesForAddress(new PublicKey(address), {
			limit: 1000,
			before: beforeSig,
		});

		if (sigs.length) {
			await db
				.insertInto('signature')
				.values(sigs.map((s) => ({ signature: s.signature })))
				.onConflict((oc) => oc.column('signature').doNothing())
				.execute();
		}

		if (sigs.length < 1000) {
			await db
				.updateTable('wallet')
				.set({
					status: 'processed',
				})
				.where('id', '=', id)
				.execute();

			console.log(`Finished fetching signatures for: ${address}`);
			break;
		} else {
			beforeSig = sigs.at(-1)?.signature;
		}

		await setTimeout(5000);
	}
}

async function watchQueue() {
	while (true) {
		try {
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

				console.log(`Fetching signatures for: ${address}`);
				fetchSignatures(address, id);

				await db
					.updateTable('wallet')
					.set({
						status: 'processing',
					})
					.where('id', '=', id)
					.execute();
			}
		} catch (error) {}

		await setTimeout(10000);
	}
}

watchQueue();
watchSignatures();

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
