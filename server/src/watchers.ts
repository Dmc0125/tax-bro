import { jsonArrayFrom } from 'kysely/helpers/postgres';
import { Message, MessageV0, PublicKey } from '@solana/web3.js';
import { setTimeout } from 'timers/promises';
import type { DbTransaction, DbTransactionIx, DbTransactionInnerIx } from 'db';
import type { Insertable } from 'kysely';
import bs58 from 'bs58';
import { sql } from 'kysely';

import { connection, db } from './config';
import { Parser } from './parsers';

export async function watchTransactions() {
	while (true) {
		const walletsRes = await db
			.selectFrom('wallet as w1')
			.where('w1.parsing_status', '=', 'init_processing')
			.where('w1.transactions_status', '=', 'processed')
			.orderBy('w1.id asc')
			.select((eb) => [
				'w1.user_id as userId',
				jsonArrayFrom(
					eb
						.selectFrom('wallet')
						.whereRef('w1.user_id', '=', 'wallet.user_id')
						.select(['wallet.address', 'wallet.id', 'parsing_status', 'transactions_status']),
				).as('wallets'),
			])
			.execute();

		let userId: null | number = null;
		let walletsWithFullTxHistory: { id: number; address: string }[] | null = null;

		for (const { wallets, userId: _userId } of walletsRes) {
			if (
				wallets.every(
					(w) => w.transactions_status === 'processed' && w.parsing_status === 'init_processing',
				)
			) {
				userId = _userId;
				walletsWithFullTxHistory = wallets.map((w) => ({ address: w.address, id: w.id }));
				break;
			}
		}

		if (walletsWithFullTxHistory && userId) {
			const walletsAddresses = walletsWithFullTxHistory.map((w) => w.address);
			const txs = await db
				.selectFrom('transaction')
				.where(
					'transaction.accounts',
					'&&',
					// @ts-ignore
					sql`ARRAY[${walletsAddresses.join(',')}]::varchar(44)[]`,
				)
				.orderBy(['timestamp asc', 'transaction.id desc'])
				.leftJoin('signature', 'signature.id', 'transaction.signature_id')
				.select((eb) => [
					'signature.signature',
					'transaction.signature_id as signature_id',
					'transaction.id',
					'transaction.accounts',
					jsonArrayFrom(
						eb
							.selectFrom('transaction_ix')
							.whereRef('transaction_ix.signature_id', '=', 'signature.id')
							.select([
								'transaction_ix.accounts',
								'transaction_ix.data',
								'transaction_ix.program_id',
							]),
					).as('ixs'),
					jsonArrayFrom(
						eb
							.selectFrom('transaction_inner_ix')
							.whereRef('transaction_inner_ix.signature_id', '=', 'signature.id')
							.select([
								'transaction_inner_ix.accounts',
								'transaction_inner_ix.data',
								'transaction_inner_ix.program_id',
								'transaction_inner_ix.index',
							]),
					).as('inner_ixs'),
				])
				.limit(1)
				.execute();

			const parser = new Parser(userId, walletsAddresses, walletsWithFullTxHistory);
			await parser.fetchTokenAccounts();

			for (const tx of txs) {
				console.log(`https://solscan.io/tx/${tx.signature}`);
				parser.parse(tx.accounts, tx.signature_id, tx.ixs, tx.inner_ixs);
			}
		}

		await setTimeout(3000);
	}
}

export async function watchSignatures() {
	while (true) {
		const unfetched = await db
			.selectFrom('wallet')
			.where((eb) =>
				eb.or([
					eb('signatures_status', '=', 'init_processing'),
					eb('signatures_status', '=', 'processed'),
				]),
			)
			.where('transactions_status', '=', 'init_processing')
			.orderBy('id asc')
			.select((qb) => [
				'wallet.id as walletId',
				jsonArrayFrom(
					qb
						.selectFrom('signature_wallet_intermediary')
						.whereRef('signature_wallet_intermediary.wallet_id', '=', 'wallet.id')
						.limit(10)
						.orderBy('signature_wallet_intermediary.signature_id asc')
						.leftJoin('signature', 'signature.id', 'signature_wallet_intermediary.signature_id')
						.leftJoin('transaction', 'transaction.signature_id', 'signature.id')
						.where('transaction.id', 'is', null)
						.select(['signature.signature', 'signature.id']),
				).as('signatures'),
			])
			.executeTakeFirst();

		if (unfetched && unfetched.signatures.length) {
			const signatures = unfetched.signatures;
			console.log(`Fetching transactions: ${signatures.length}`);

			const txs = await connection.getTransactions(
				signatures.map((u) => u.signature!),
				{ maxSupportedTransactionVersion: 0 },
			);

			const insertableMetas: Insertable<DbTransaction>[] = [];
			const insertableIxs: Insertable<DbTransactionIx>[] = [];
			const insertableInnerIxs: Insertable<DbTransactionInnerIx>[] = [];

			for (let i = 0; i < signatures.length; i++) {
				const signature = signatures[i];
				const tx = txs[i];

				const ixs: Insertable<DbTransactionIx>[] = [];
				const innerIxs: Insertable<DbTransactionInnerIx>[] = [];
				let accounts: string[] = [];

				if (tx) {
					const timestamp = new Date(tx.blockTime! * 1000);

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
							const d = Buffer.from(cix.data).toString('hex');
							ixs.push({
								signature_id: signature.id!,
								data: d,
								program_id: accounts[cix.programIdIndex],
								accounts: cix.accountKeyIndexes.map((a) => accounts[a]),
							});
						});
					} else {
						const m = tx.transaction.message as Message;
						accounts = m.accountKeys.map((a) => a.toString());

						m.instructions.forEach((ix) => {
							const d = Buffer.from(bs58.decode(ix.data)).toString('hex');
							ixs.push({
								signature_id: signature.id!,
								data: d,
								program_id: accounts[ix.programIdIndex],
								accounts: ix.accounts.map((a) => accounts[a]),
							});
						});
					}

					if (tx.meta?.innerInstructions) {
						tx.meta.innerInstructions.forEach((ii) => {
							ii.instructions.forEach((iix) => {
								const d = Buffer.from(bs58.decode(iix.data)).toString('hex');

								innerIxs.push({
									signature_id: signature.id!,
									data: d,
									index: ii.index,
									program_id: accounts[iix.programIdIndex],
									accounts: iix.accounts.map((a) => accounts[a]),
								});
							});
						});
					}

					insertableMetas.push({
						signature_id: signature.id!,
						timestamp,
						accounts,
						version: tx.version === 0 ? '0' : 'legacy',
						fee: tx.meta?.fee || 0,
						fee_payer: accounts[0],
					});
				}

				insertableIxs.push(...ixs);
				insertableInnerIxs.push(...innerIxs);
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
		} else if (unfetched && !unfetched.signatures.length) {
			await db
				.updateTable('wallet')
				.where('wallet.id', '=', unfetched.walletId)
				.set({
					transactions_status: 'processed',
					parsing_status: 'init_processing',
				})
				.execute();
		}

		await setTimeout(3000);
	}
}

async function fetchSignaturesForAddress(address: string, walletId: number) {
	let beforeSig: string | undefined = undefined;
	let firstSet = false;

	while (true) {
		const sigs = await connection.getSignaturesForAddress(new PublicKey(address), {
			limit: 1000,
			before: beforeSig,
		});

		if (sigs.length) {
			await db.transaction().execute(async (tx) => {
				if (!firstSet) {
					await db
						.updateTable('wallet')
						.set({
							last_signature: sigs[0].signature,
							signatures_status: 'init_processing',
						})
						.where('id', '=', walletId)
						.execute();
					firstSet = true;
				}
				await db
					.updateTable('wallet')
					.set((eb) => ({
						signatures_count: eb('signatures_count', '+', sigs.length),
					}))
					.where('id', '=', walletId)
					.execute();
				const insertedIds = await tx
					.insertInto('signature')
					.values(sigs.map((s) => ({ signature: s.signature })))
					.onConflict((oc) => oc.column('signature').doNothing())
					.returning('signature.id')
					.execute();
				await tx
					.insertInto('signature_wallet_intermediary')
					.values(insertedIds.map((sig) => ({ signature_id: sig.id, wallet_id: walletId })))
					.execute();
			});
		}

		if (sigs.length < 1000) {
			await db
				.updateTable('wallet')
				.set({
					signatures_status: 'processed',
					transactions_status: 'init_processing',
				})
				.where('id', '=', walletId)
				.execute();

			console.log(`Finished fetching signatures for: ${address}`);
			break;
		} else {
			beforeSig = sigs.at(-1)?.signature;
		}

		await setTimeout(5000);
	}
}

export async function watchWallets() {
	while (true) {
		try {
			const { initializing, in_queue } = await db
				.selectFrom('wallet')
				.select((qb) => [
					jsonArrayFrom(
						qb
							.selectFrom('wallet')
							.select(['address', 'id'])
							.orderBy('id')
							.where('signatures_status', '=', 'init_processing'),
					).as('initializing'),
					jsonArrayFrom(
						qb
							.selectFrom('wallet')
							.select(['address', 'id'])
							.orderBy('id')
							.where('signatures_status', '=', 'init'),
					).as('in_queue'),
				])
				.executeTakeFirstOrThrow();

			if (initializing.length < 2) {
				const { address, id } = in_queue[0];

				console.log(`Fetching signatures for: ${address}`);
				fetchSignaturesForAddress(address, id);

				await db
					.updateTable('wallet')
					.set({
						signatures_status: 'init_processing',
					})
					.where('id', '=', id)
					.execute();
			}
		} catch (error) {}

		await setTimeout(10000);
	}
}
