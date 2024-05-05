import { ASSOCIATED_TOKEN_PROGRAM_ID, TOKEN_PROGRAM_ID } from '@solana/spl-token';
import { PublicKey } from '@solana/web3.js';
import bs58 from 'bs58';

import { connection, env } from './config';
import { watchSignatures, watchTransactions, watchWallets } from './watchers';

watchWallets();
watchSignatures();
watchTransactions();

// Bun.serve({
// 	port: env.PORT,
// 	async fetch(req) {
// 		if (req.method !== 'POST' || req.headers.get('content-type') !== 'application/json') {
// 			return new Response(null, { status: 400 });
// 		}

// 		const apiSecret = req.headers.get('x-api-secret');

// 		if (!apiSecret || apiSecret !== env.API_SECRET) {
// 			return new Response(null, {
// 				status: 401,
// 			});
// 		}

// 		const body = await req.json();
// 		const [urlPath, _] = req.url.split('?');

// 		try {
// 			// if (urlPath.endsWith('fetch_signatures')) {
// 			// 	addToFetchSignaturesQueue(body);
// 			// }
// 		} catch (error) {
// 			return error as Response;
// 		}

// 		return new Response(null, { status: 200 });
// 	},
// });

console.log(`Server running on http://localhost:${env.PORT}`);
