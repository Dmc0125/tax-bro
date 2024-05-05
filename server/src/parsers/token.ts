import {
	TOKEN_PROGRAM_ID,
	TokenInstruction,
	decodeInstruction,
	type DecodedTransferInstruction,
} from '@solana/spl-token';
import { PublicKey, TransactionInstruction } from '@solana/web3.js';

export function parseTokenInstruction(accounts: string[], data: Buffer, internalWallets: string[]) {
	const parsedIx = decodeInstruction(
		new TransactionInstruction({
			programId: TOKEN_PROGRAM_ID,
			keys: accounts.map((a) => ({ isSigner: false, isWritable: false, pubkey: new PublicKey(a) })),
			data,
		}),
	);

	if (parsedIx.data.instruction === TokenInstruction.Transfer) {
		const { source, destination, owner } = parsedIx.keys as DecodedTransferInstruction['keys'];
	}

	console.log(parsedIx);
}
