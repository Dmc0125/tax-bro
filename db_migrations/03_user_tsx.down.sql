DELETE FROM with_timestamps WHERE table_name = ANY(ARRAY[
    'wallet',
    'wallet_to_signature',
    'associated_account',
    'sync_wallet_request'
]);

DROP TABLE IF EXISTS sync_wallet_request;
DROP TYPE IF EXISTS sync_wallet_request_status;
DROP TABLE IF EXISTS associated_account;
DROP TYPE IF EXISTS associated_account_type;
DROP TABLE IF EXISTS wallet_to_signature;
DROP TABLE IF EXISTS wallet;
