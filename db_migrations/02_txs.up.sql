INSERT INTO with_timestamps (table_name) VALUES
    ('transaction'),
    ('instruction'),
    ('inner_instruction'),
    ('instruction_event');

CREATE TABLE "address" (
    id SERIAL PRIMARY KEY NOT NULL,
    value VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX address_val ON "address"(value);

CREATE TABLE "signature" (
    id SERIAL PRIMARY KEY NOT NULL,
    value VARCHAR(150) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
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

    logs VARCHAR[] NOT NULL DEFAULT ARRAY[]::VARCHAR[],
    err BOOLEAN NOT NULL,
    fee BIGINT NOT NULL,

    FOREIGN KEY (signature_id) REFERENCES "signature"(id),

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE instruction (
    signature_id INTEGER NOT NULL,
    "index" SMALLINT NOT NULL,
    -- TODO:
    -- only insert events to isntructions with isknown = false
    is_known BOOLEAN NOT NULL DEFAULT false,

    FOREIGN KEY (signature_id) REFERENCES "signature"(id),
    PRIMARY KEY (signature_id, "index"),

    program_account_id INTEGER NOT NULL,
    accounts_ids INTEGER[] NOT NULL,
    -- base64 encoded bytes because bytea is stupid
    "data" VARCHAR NOT NULL,

    FOREIGN KEY (program_account_id) REFERENCES "address"(id),

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX instruction_program_account_id ON instruction(program_account_id);

CREATE TABLE inner_instruction (
    signature_id INTEGER NOT NULL,
    -- index of instruction it belongs to
    ix_index SMALLINT NOT NULL,
    -- position of inner ix
    "index" SMALLINT NOT NULL,

    FOREIGN KEY (signature_id, ix_index) REFERENCES instruction(signature_id, "index"),
    PRIMARY KEY (signature_id, ix_index, "index"),

    program_account_id INTEGER NOT NULL,
    accounts_ids INTEGER[] NOT NULL,
    -- base64 encoded bytes because bytea is stupid
    "data" VARCHAR NOT NULL,

    FOREIGN KEY (program_account_id) REFERENCES "address"(id),

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TYPE event_type AS ENUM ('transfer', 'mint', 'burn', 'close_account');

CREATE TABLE instruction_event (
    signature_id INTEGER NOT NULL,
    -- index of instruction it belongs to
    ix_index SMALLINT NOT NULL,
    -- position of event
    "index" SMALLINT NOT NULL,

    FOREIGN KEY (signature_id, ix_index) REFERENCES instruction(signature_id, "index"),
    PRIMARY KEY (signature_id, ix_index, "index"),

    "type" event_type NOT NULL,
    "data" JSONB NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE OR REPLACE FUNCTION get_ordered_accounts(accounts_ids INTEGER[])
RETURNS jsonb AS $$
    SELECT COALESCE(
        jsonb_agg(
            jsonb_build_object(
                'id', address.id,
                'value', address.value
            )
            ORDER BY array_position(accounts_ids, address.id)
        ),
        '[]'::jsonb
    )
    FROM
        address
    WHERE
        address.id = ANY(accounts_ids);
$$ LANGUAGE SQL STABLE;

CREATE OR REPLACE VIEW view_inner_instructions AS
SELECT
    signature_id,
    ix_index,
    jsonb_agg(
        jsonb_build_object(
            'program_address', address.value,
            'data', inner_instruction.data,
            'accounts', get_ordered_accounts(inner_instruction.accounts_ids)
        )
        ORDER BY inner_instruction.index
    ) as inner_instructions
FROM
    inner_instruction
JOIN
    address ON address.id = inner_instruction.program_account_id
GROUP BY
    signature_id, ix_index;

CREATE OR REPLACE VIEW view_instructions AS
SELECT
    instruction.signature_id,
    jsonb_agg(
        jsonb_build_object(
            'program_address', address.value,
            'data', instruction.data,
            'accounts', get_ordered_accounts(instruction.accounts_ids),
            'inner_ixs', COALESCE(view_inner_instructions.inner_instructions, '[]'::jsonb)
        )
        ORDER BY instruction.index
    ) as instructions
FROM
    instruction
JOIN
    address ON address.id = instruction.program_account_id
LEFT JOIN
    view_inner_instructions ON
        view_inner_instructions.signature_id = instruction.signature_id
        AND view_inner_instructions.ix_index = instruction.index
GROUP BY
    instruction.signature_id;
