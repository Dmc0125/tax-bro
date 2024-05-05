import { z } from 'zod';
import { PublicKey } from '@solana/web3.js';
import { fail } from '@sveltejs/kit';

import type { Actions, PageServerLoad } from './$types';
import { db } from '../auth';
import type { DbWallet } from 'db';

export const load: PageServerLoad = async ({ locals }) => {
	const userId = locals.user.id!;
	const wallets = await db.selectFrom('wallet').where('user_id', '=', userId).selectAll().execute();
	console.log('Rerunning load');
	return {
		wallets,
	};
};

const addressSchema = z
	.string()
	.min(1)
	.refine(
		(s) => {
			try {
				new PublicKey(s);
				return true;
			} catch (error) {
				return false;
			}
		},
		{ message: 'Not a valid public key' },
	);

export const actions: Actions = {
	add: async ({ request, locals }) => {
		if (!locals.user || !locals.session) {
			return fail(401);
		}

		const data = await request.formData();
		const address = data.get('address');
		const parseResult = addressSchema.safeParse(address);

		if (parseResult.success) {
			try {
				new PublicKey(parseResult.data);
			} catch (error) {
				return fail(400);
			}

			let insertedWallet: Omit<DbWallet, 'id'> & { id: number };
			try {
				insertedWallet = await db
					.insertInto('wallet')
					.values({
						address: parseResult.data,
						user_id: locals.user.id,
						signatures_count: 0,
						signatures_status: 'init',
						transactions_status: 'init',
						parsing_status: 'init',
					})
					.returningAll()
					.executeTakeFirstOrThrow();
			} catch (error) {
				return fail(500);
			}

			return { success: true, inserted: insertedWallet };
		} else {
			console.log(parseResult.error);
			return fail(400);
		}
	},
};
