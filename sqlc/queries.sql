-- name: InsertAccount :one
INSERT INTO account (selected_auth_provider, email) VALUES (sqlc.arg(auth_provider), sqlc.arg(email)) RETURNING id;

-- name: InsertAddress :one
INSERT INTO address (value) VALUES (sqlc.arg(address)) RETURNING id;

-- name: InsertWallet :one
INSERT INTO wallet (account_id, address_id) VALUES (sqlc.arg(account_id), sqlc.arg(address_id)) RETURNING id;

-- name: InsertSyncWalletRequest :exec
INSERT INTO sync_wallet_request (wallet_id) VALUES (sqlc.arg(wallet_id));

-- name: UpdateSyncWalletRequestStatus :exec
UPDATE sync_wallet_request SET status = sqlc.arg(status) WHERE wallet_id = sqlc.arg(wallet_id);

-- name: GetAssociatedAccountsForWallet :many
SELECT
    address.value AS address, signature.value as last_signature
FROM
    associated_account
INNER JOIN
    address ON address.id = associated_account.address_id
LEFT JOIN
    signature ON signature.id = associated_account.last_signature_id
WHERE
    associated_account.wallet_id = sqlc.arg(wallet_id);

-- name: GetLatestSyncWalletRequest :one
SELECT
	sync_wallet_request.wallet_id,
	address.value AS address,
	signature.value AS last_signature
FROM
	sync_wallet_request
INNER JOIN
	wallet ON wallet.id = sync_wallet_request.wallet_id
INNER JOIN
	address ON address.id = wallet.address_id
LEFT JOIN
	signature ON signature.id = wallet.last_signature_id
WHERE
	sync_wallet_request.status = 'queued'
ORDER BY
	sync_wallet_request.created_at ASC
LIMIT
	1;

-- name: GetTransactionsFromSignatures :many
SELECT
    COALESCE(view_instructions.instructions, '[]'::jsonb) as ixs,
    transaction.logs,
    transaction.err,
    signature.value as signature,
    signature.id as signature_id
FROM
    signature
JOIN
    transaction ON transaction.signature_id = signature.id
LEFT JOIN
    view_instructions ON view_instructions.signature_id = signature.id
WHERE
    signature.value = ANY($1::VARCHAR[]);

-- -- name: GetUnknownProgramAddresses :many
SELECT DISTINCT address.value as program_address
FROM instruction
JOIN address ON address.id = instruction.program_account_id
WHERE instruction.is_known = false;


-- name: GetAddressesFromAddresses :many
SELECT value, id FROM address WHERE value = ANY($1::VARCHAR[]);

-- name: InsertAddresses :many
INSERT INTO address (value) (SELECT * FROM unnest($1::VARCHAR[])) ON CONFLICT (value) DO NOTHING RETURNING value, id;

-- name: InsertSignatures :many
INSERT INTO signature (value) (
    SELECT * FROM unnest($1::VARCHAR[])
) ON CONFLICT (
    value
) DO NOTHING RETURNING id, value;

-- name: InsertTransactions :copyfrom
INSERT INTO "transaction" (
    signature_id, accounts_ids, "timestamp", timestamp_granularized, slot, logs, err, fee
) VALUES (
    sqlc.arg(signature_id),
    sqlc.arg(accounts_ids),
    sqlc.arg(timestamp),
    sqlc.arg(timestamp_granularized),
    sqlc.arg(slot),
    sqlc.arg(logs),
    sqlc.arg(err),
    sqlc.arg(fee)
);

-- name: InsertInstructions :copyfrom
INSERT INTO instruction (
    signature_id, index, program_account_id, accounts_ids, data
) VALUES (
    sqlc.arg(signature_id),
    sqlc.arg(index),
    sqlc.arg(program_account_id),
    sqlc.arg(accounts_ids),
    sqlc.arg(data)
);

-- name: InsertInnerInstructions :copyfrom
INSERT INTO inner_instruction (
    signature_id, ix_index, index, program_account_id, accounts_ids, data
) VALUES (
    sqlc.arg(signature_id),
    sqlc.arg(ix_index),
    sqlc.arg(index),
    sqlc.arg(program_account_id),
    sqlc.arg(accounts_ids),
    sqlc.arg(data)
);

-- name: InsertInstructionEvents :copyfrom
INSERT INTO instruction_event (
    signature_id, ix_index, index, type, data
) VALUES (
    sqlc.arg(signature_id),
    sqlc.arg(ix_index),
    sqlc.arg(index),
    sqlc.arg(type),
    sqlc.arg(data)
);

-- name: AssignInstructionsToWallet :execrows
INSERT INTO wallet_to_signature (wallet_id, signature_id) VALUES (
    unnest(sqlc.arg(wallet_id)::INTEGER[]),
    unnest(sqlc.arg(signature_id)::INTEGER[])
) ON CONFLICT (wallet_id, signature_id) DO NOTHING;

-- name: InsertAssociatedAccounts :copyfrom
INSERT INTO associated_account (
    wallet_id, address_id, type
) VALUES (
    sqlc.arg(wallet_id),
    sqlc.arg(address_id),
    sqlc.arg(type)
);

-- name: UpdateWalletAggregateCounts :exec
UPDATE wallet SET
    signatures = sqlc.arg(signatures_count),
    associated_accounts = sqlc.arg(associated_accounts_count)
WHERE id = sqlc.arg(wallet_id);

-- name: UpdateWalletAggregateCountsAndLastSignature :exec
UPDATE wallet SET
    signatures = signatures + sqlc.arg(signatures_count),
    associated_accounts = associated_accounts + sqlc.arg(associated_accounts_count),
    last_signature_id = (SELECT signature.id FROM signature WHERE signature.value = sqlc.arg(last_signature))
WHERE wallet.id = sqlc.arg(wallet_id);

-- name: UpdateAssociatedAccountLastSignature :exec
UPDATE associated_account SET
    last_signature_id = (SELECT signature.id FROM signature WHERE signature.value = sqlc.arg(last_signature))
WHERE
    address_id = (SELECT address.id FROM address WHERE address.value = sqlc.arg(associated_account_address))
    AND wallet_id = sqlc.arg(wallet_id);

-- name: GetTransactionsWithDuplicateTimestamps :many
SELECT
    t1.id,
    t1.slot,
    signature.value as signature
FROM
    transaction t1
INNER JOIN
    signature ON signature.id = t1.signature_id
WHERE
    EXISTS(
    	SELECT
    		1
    	FROM
    		transaction t2
    	WHERE
    		t1.id != t2.id AND t1.timestamp = t2.timestamp AND t1.slot = t2.slot AND t1.block_index IS NULL AND t2.block_index IS NULL
    )
    AND t1.id > sqlc.arg(start_id)
ORDER BY
    t1.id ASC
LIMIT
    500;

-- name: UpdateTransactionsBlockIndexes :exec
UPDATE transaction SET block_index = v.bi FROM (
    SELECT
        unnest(sqlc.arg(transactions_ids)::INTEGER[]) as txid,
        unnest(sqlc.arg(block_indexes)::INTEGER[]) as bi
) AS v WHERE transaction.id = v.txid;
