CREATE TABLE "address" (
    id SERIAL PRIMARY KEY NOT NULL,
    value VARCHAR(100) UNIQUE NOT NULL
);

CREATE INDEX address_val ON "address"(value);

CREATE TABLE "signature" (
    id SERIAL PRIMARY KEY NOT NULL,
    value VARCHAR(150) UNIQUE NOT NULL
);

CREATE INDEX signature_val ON "signature"(value);

CREATE TABLE "transaction" (
    id SERIAL PRIMARY KEY NOT NULL,
    signature_id INTEGER UNIQUE NOT NULL,
    accounts_ids INTEGER[] NOT NULL,

    "timestamp" TIMESTAMPTZ NOT NULL,
    timestamp_granularized TIMESTAMPTZ NOT NULL,
    slot BIGINT NOT NULL,
    block_index INTEGER,

    err BOOLEAN NOT NULL,
    fee BIGINT NOT NULL,

    FOREIGN KEY (signature_id) REFERENCES "signature"(id)
);

CREATE TABLE instruction (
    signature_id INTEGER NOT NULL,
    "index" SMALLINT NOT NULL,

    FOREIGN KEY (signature_id) REFERENCES "signature"(id),
    PRIMARY KEY (signature_id, "index"),

    program_account_id INTEGER NOT NULL,
    accounts_ids INTEGER[] NOT NULL,
    "data" BYTEA NOT NULL,

    FOREIGN KEY (program_account_id) REFERENCES "address"(id)
);

CREATE INDEX instruction_program_account_id ON instruction(program_account_id);

CREATE TABLE inner_instruction (
    signature_id INTEGER NOT NULL,
    -- index of instruction it belongs to
    ix_index INTEGER NOT NULL,
    -- position of inner ix
    "index" SMALLINT NOT NULL,

    FOREIGN KEY (signature_id, ix_index) REFERENCES instruction(signature_id, "index"),
    PRIMARY KEY (signature_id, ix_index, "index"),

    program_account_id INTEGER NOT NULL,
    accounts_ids INTEGER[] NOT NULL,
    "data" BYTEA NOT NULL,

    FOREIGN KEY (program_account_id) REFERENCES "address"(id)
);


CREATE TABLE transaction_logs (
    signature_id INTEGER NOT NULL,
    logs VARCHAR[] NOT NULL,

    FOREIGN KEY (signature_id) REFERENCES "signature"(id)
);

CREATE TYPE event_type AS ENUM ('transfer');

CREATE TABLE instruction_event (
    signature_id INTEGER NOT NULL,
    -- index of instruction it belongs to
    ix_index INTEGER NOT NULL,
    -- position of event
    "index" SMALLINT NOT NULL,

    FOREIGN KEY (signature_id, ix_index) REFERENCES instruction(signature_id, "index"),
    PRIMARY KEY (signature_id, ix_index, "index"),

    "type" event_type NOT NULL,
    "data" BYTEA NOT NULL
);