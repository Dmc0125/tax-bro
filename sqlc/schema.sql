CREATE TABLE with_timestamps (
    table_name VARCHAR(50) PRIMARY KEY NOT NULL
);

CREATE OR REPLACE FUNCTION update_ts()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION add_update_ts_trigger()
RETURNS event_trigger
LANGUAGE plpgsql
AS $$
DECLARE
    obj record;
    should_add boolean;
BEGIN
    FOR obj IN SELECT * FROM pg_event_trigger_ddl_commands() WHERE command_tag = 'CREATE TABLE'
    LOOP
        -- Check if the new table should have timestamp columns
        SELECT EXISTS (
            SELECT 1 FROM with_timestamps
            WHERE table_name = obj.object_identity::regclass::text
        ) INTO should_add;

        IF should_add THEN
            EXECUTE format('
                CREATE TRIGGER update_timestamp
                BEFORE UPDATE ON %s
                FOR EACH ROW
                EXECUTE FUNCTION update_ts()
            ', obj.object_identity);
        END IF;
    END LOOP;
END;
$$;

CREATE EVENT TRIGGER add_timestamps_trigger
ON ddl_command_end
WHEN TAG IN ('CREATE TABLE')
EXECUTE FUNCTION add_update_ts_trigger();

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE TYPE auth_provider_type AS ENUM ('github', 'discord', 'google');

INSERT INTO with_timestamps (table_name) VALUES ('account'), ('auth');

CREATE TABLE account (
    id SERIAL PRIMARY KEY NOT NULL,
    selected_auth_provider auth_provider_type NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX account_email ON account(email);

CREATE TABLE auth (
    account_id INTEGER NOT NULL,
    provider_id VARCHAR PRIMARY KEY NOT NULL,

    p_type auth_provider_type NOT NULL,
    username VARCHAR(100) NOT NULL,
    avatar_url VARCHAR(1000) NOT NULL,

    UNIQUE (account_id, p_type),
    UNIQUE (provider_id, p_type),
    FOREIGN KEY (account_id) REFERENCES account(id),

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE session (
    id uuid PRIMARY KEY NOT NULL DEFAULT uuid_generate_v4(),
    account_id INTEGER NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,

    FOREIGN KEY (account_id) REFERENCES account(id)
);
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
INSERT INTO with_timestamps (table_name) VALUES
    ('wallet'),
    ('wallet_to_signature'),
    ('associated_account'),
    ('sync_wallet_request');

CREATE TABLE wallet (
    id SERIAL PRIMARY KEY NOT NULL,
    account_id INTEGER NOT NULL,
    last_signature_id INTEGER,
    address_id INTEGER NOT NULL,

    FOREIGN KEY (account_id) REFERENCES "account"(id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (last_signature_id) REFERENCES "signature"(id),
    FOREIGN KEY (address_id) REFERENCES "address"(id),
    UNIQUE (account_id, address_id),

    label VARCHAR(50),
    signatures INTEGER NOT NULL DEFAULT 0,
    associated_accounts INTEGER NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE wallet_to_signature (
    id uuid PRIMARY KEY NOT NULL DEFAULT uuid_generate_v4(),
    wallet_id INTEGER NOT NULL,
    signature_id INTEGER NOT NULL,

    FOREIGN KEY (wallet_id) REFERENCES wallet(id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (signature_id) REFERENCES "signature"(id),
    UNIQUE (wallet_id, signature_id),

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TYPE associated_account_type AS ENUM ('token');

CREATE TABLE associated_account (
    id SERIAL PRIMARY KEY NOT NULL,
    wallet_id INTEGER NOT NULL,
    address_id INTEGER NOT NULL,
    last_signature_id INTEGER,
    "type" associated_account_type NOT NULL,

    FOREIGN KEY (wallet_id) REFERENCES wallet(id) ON DELETE CASCADE ON UPDATE CASCADE,
    FOREIGN KEY (last_signature_id) REFERENCES "signature"(id),
    FOREIGN KEY (address_id) REFERENCES "address"(id),
    UNIQUE (wallet_id, address_id),

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TYPE sync_wallet_request_status AS ENUM ('queued', 'fetching_transactions', 'parsing_events');

CREATE TABLE sync_wallet_request (
    id SERIAL PRIMARY KEY NOT NULL,
    wallet_id INTEGER UNIQUE NOT NULL,
    "status" sync_wallet_request_status NOT NULL DEFAULT 'queued',

    FOREIGN KEY (wallet_id) REFERENCES wallet (id) ON DELETE CASCADE ON UPDATE CASCADE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
