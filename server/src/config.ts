import { Connection } from '@solana/web3.js';
import { createDb } from 'db';
import { z } from 'zod';

const envSchema = z.object({
	RPC_URL: z.string().min(1),
	DATABASE_HOST: z.string().min(1),
	DATABASE_USERNAME: z.string().min(1),
	DATABASE_PASSWORD: z.string().min(1),
	DATABASE_PORT: z.string().min(1),
	DATABASE: z.string().min(1),
	API_SECRET: z.string().min(1),
	PORT: z.string().min(1),
});

const envParseResult = envSchema.safeParse(process.env);

if (!envParseResult.success) {
	console.log(envParseResult.error);
	throw Error('ENV VARIABLES MISSING');
}

export const env = envParseResult.data;

export const { db } = createDb(
	env.DATABASE_HOST,
	env.DATABASE_USERNAME,
	env.DATABASE_PASSWORD,
	env.DATABASE_PORT,
	env.DATABASE,
);
export const connection = new Connection(env.RPC_URL);
