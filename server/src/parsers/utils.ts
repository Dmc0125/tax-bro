import { PublicKey, TransactionInstruction } from '@solana/web3.js';
import type { DbTransactionIx } from 'db';

export function dbIxIntoTransactionInstruction(dbIx: Omit<DbTransactionIx, 'id' | 'signature_id'>) {
	const data = Buffer.from(dbIx.data, 'hex');
	return new TransactionInstruction({
		data,
		keys: dbIx.accounts.map((a) => ({
			isSigner: false,
			isWritable: false,
			pubkey: new PublicKey(a),
		})),
		programId: new PublicKey(dbIx.program_id),
	});
}

export function rawToUi(rawAmount: number, decimals: number) {
	return Number((rawAmount / 10 ** decimals).toFixed(decimals));
}
