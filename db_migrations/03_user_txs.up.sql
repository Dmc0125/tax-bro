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
