import { ASSOCIATED_TOKEN_PROGRAM_ID, TOKEN_PROGRAM_ID } from '@solana/spl-token';
import type { DbTransactionInnerIx, DbTransactionIx } from 'db';
import type { DbEvent, DbTokenAccount } from 'db/src/migrations/01';
import { sql, type Insertable } from 'kysely';

import { parseAssociatedTokenInstruction } from './associated_token';
import { db } from '../config';

export class Parser {
	tokenAccounts: (DbTokenAccount & { inserted: boolean })[] = [];
	events: Insertable<DbEvent>[] = [];

	constructor(
		public userId: number,
		public internalWallets: string[],
		public internalWalletsWithIds: { id: number; address: string }[],
	) {}

	async fetchTokenAccounts() {
		const tas = await db
			.selectFrom('token_account')
			.where(
				'token_account.address',
				'=',
				// @ts-ignore
				sql`ANY(ARRAY[${this.internalWallets.join(',')}]::varchar(44)[])`,
			)
			.selectAll()
			.execute();

		tas.forEach((ta) => {
			this.tokenAccounts.push({
				...ta,
				inserted: true,
			});
		});
	}

	async save() {
		const insertableTokenAccounts: Insertable<DbTokenAccount>[] = [];
		for (const { inserted, ...taData } of this.tokenAccounts) {
			if (!inserted) {
				insertableTokenAccounts.push(taData);
			}
		}

		await db.insertInto('token_account').values(insertableTokenAccounts).execute();
	}

	parse(
		accounts: string[],
		signature_id: number,
		ixs: Omit<DbTransactionIx, 'id' | 'signature_id'>[],
		inner_ixs: Omit<DbTransactionInnerIx, 'id' | 'signature_id'>[],
	) {
		console.log(this.tokenAccounts);
		for (let i = 0; i < ixs.length; i++) {
			const ix = ixs[i];
			const data = Buffer.from(ix.data, 'base64');

			switch (ix.program_id) {
				case ASSOCIATED_TOKEN_PROGRAM_ID.toString(): {
					const { owner, ata, mint, event } = parseAssociatedTokenInstruction(
						ix.accounts,
						ix.data,
						i,
						inner_ixs,
					);
					const existingTokenAccount = this.tokenAccounts.find((ta) => ta.address === ata);

					if (owner && ata && !existingTokenAccount) {
						const ownerWallet = this.internalWalletsWithIds.find((w) => w.address === owner);
						if (ownerWallet) {
							this.tokenAccounts.push({
								mint,
								address: ata,
								wallet_id: ownerWallet.id,
								inserted: false,
							});
						}
					}

					if (event) {
						this.events.push({
							type: 'transfer',
							data: event,
							user_id: this.userId,
							signature_id,
						});
					}

					break;
				}
				case TOKEN_PROGRAM_ID.toString(): {
					break;
				}
			}
		}
		console.log(this.tokenAccounts);
	}
}
