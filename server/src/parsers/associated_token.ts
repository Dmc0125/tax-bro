import { SystemInstruction } from '@solana/web3.js';
import type { DbTransactionInnerIx } from 'db';

import { dbIxIntoTransactionInstruction, rawToUi } from './utils';
import type { DbTransferEvent } from 'db/src/migrations/01';

export function parseAssociatedTokenInstruction(
	accounts: string[],
	data: string,
	ixIdx: number,
	innerIxs: Omit<DbTransactionInnerIx, 'id' | 'signature_id'>[],
) {
	if (!data.length || data.startsWith('00') || data.startsWith('01')) {
		const iixs = innerIxs.filter((iix) => iix.index === ixIdx);

		if (!iixs.length) {
			// already exists
			return {};
		}

		const [_, ata, owner, mint] = accounts;

		const rentIxDecoded = SystemInstruction.decodeCreateAccount(
			dbIxIntoTransactionInstruction(iixs[1]),
		);
		const event: DbTransferEvent['data'] = {
			// needs to be checked
			type: 'internal_transfer',
			amount_ui: rawToUi(rentIxDecoded.lamports, 9),
			from: rentIxDecoded.fromPubkey.toString(),
			to: rentIxDecoded.newAccountPubkey.toString(),
			mint,
		};

		return {
			event,
			owner,
			ata,
			mint,
		};
	}

	return {};
}
