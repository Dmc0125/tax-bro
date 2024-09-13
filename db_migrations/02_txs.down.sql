DELETE FROM with_timestamps WHERE table_name = ANY(ARRAY[
    'transaction',
    'instruction',
    'inner_instruction',
    'transaction_logs',
    'instruction_event'
]);

DROP TABLE IF EXISTS instruction_event;

DROP TYPE IF EXISTS event_type;

DROP TABLE IF EXISTS transaction_logs;

DROP TABLE IF EXISTS inner_instruction;

DROP TABLE IF EXISTS instruction;

DROP TABLE IF EXISTS "transaction";

DROP TABLE IF EXISTS "signature";

DROP TABLE IF EXISTS "address";