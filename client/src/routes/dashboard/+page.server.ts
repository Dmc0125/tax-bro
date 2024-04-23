import { z } from 'zod';
import { PublicKey } from '@solana/web3.js';
import { fail } from '@sveltejs/kit';

import type { Actions } from './$types';
import { API_URL } from '$env/static/private';
import { db } from '../auth';

const addressesSchema = z.array(
	z
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
		),
);

export const actions: Actions = {
	add: async ({ request, cookies, locals }) => {
		if (!locals.user || !locals.session) {
			return fail(401);
		}

		const data = await request.formData();
		const addresses = data.getAll('address');
		const parseResult = addressesSchema.safeParse(addresses);

		if (parseResult.success) {
			await db
				.insertInto('wallet')
				.values(
					parseResult.data.map((a) => ({
						address: a,
						// @ts-expect-error
						user_id: locals.user.id,
						status: 'in_queue',
					})),
				)
				.execute();
		} else {
			console.log(parseResult.error);
			return fail(400);
		}
	},
};
