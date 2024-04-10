import { FileMigrationProvider, Migrator } from 'kysely';
import fs from 'node:fs/promises';
import path from 'node:path';

import { createDb, requireEnvVar } from './utils';

const dbHost = requireEnvVar('DATABASE_HOST');
const dbUsername = requireEnvVar('DATABASE_USERNAME');
const dbPassword = requireEnvVar('DATABASE_PASSWORD');
const dbPort = requireEnvVar('DATABASE_PORT');
const dbDatabase = requireEnvVar('DATABASE');

const db = createDb(dbHost, dbUsername, dbPassword, dbPort, dbDatabase);
const migrator = new Migrator({
	db,
	provider: new FileMigrationProvider({
		fs,
		path,
		migrationFolder: path.join(__dirname, '/migrations'),
	}),
});

async function migrateUp() {
	const { error, results } = await migrator.migrateUp();

	results?.forEach((it) => {
		if (it.status === 'Success') {
			console.log(`migration "${it.migrationName}" was executed successfully`);
		} else if (it.status === 'Error') {
			console.error(`failed to execute migration "${it.migrationName}"`);
		}
	});

	if (error) {
		console.error('failed to migrate');
		console.error(error);
		process.exit(1);
	}

	await db.destroy();
}

async function migrateToLatest() {
	const { error, results } = await migrator.migrateToLatest();

	results?.forEach((it) => {
		if (it.status === 'Success') {
			console.log(`migration "${it.migrationName}" was executed successfully`);
		} else if (it.status === 'Error') {
			console.error(`failed to execute migration "${it.migrationName}"`);
		}
	});

	if (error) {
		console.error('failed to migrate');
		console.error(error);
		process.exit(1);
	}

	await db.destroy();
}

async function migrateDown() {
	const { error, results } = await migrator.migrateDown();

	results?.forEach((it) => {
		if (it.status === 'Success') {
			console.log(`migration down "${it.migrationName}" was executed successfully`);
		} else if (it.status === 'Error') {
			console.error(`failed to execute migration down "${it.migrationName}"`);
		}
	});

	if (error) {
		console.error('failed to migrate down');
		console.error(error);
		process.exit(1);
	}

	await db.destroy();
}

if (process.argv.includes('latest')) {
	console.log('Migrating to latest');
	await migrateToLatest();
} else if (process.argv.includes('down')) {
	console.log('Migrating down');
	await migrateDown();
} else {
	console.log('Migrating up');
	await migrateUp();
}
