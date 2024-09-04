CREATE TABLE wallet (
    id SERIAL PRIMARY KEY NOT NULL,
    user_id INTEGER NOT NULL,
    last_signature_id INTEGER,
    address_id INTEGER NOT NULL,

    FOREIGN KEY (user_id) REFERENCES "user"(id),
    FOREIGN KEY (last_signature_id) REFERENCES "signature"(id),
    FOREIGN KEY (address_id) REFERENCES "address"(id),
    UNIQUE (user_id, address_id),

    label VARCHAR(50) NOT NULL,
    signatures_count INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE wallet_to_signature (
    id uuid PRIMARY KEY NOT NULL,
    wallet_id INTEGER NOT NULL,
    signature_id INTEGER NOT NULL,

    FOREIGN KEY (wallet_id) REFERENCES wallet(id),
    FOREIGN KEY (signature_id) REFERENCES "signature"(id),
    UNIQUE (wallet_id, signature_id)
);

CREATE TABLE associated_account (
    id SERIAL PRIMARY KEY NOT NULL,
    wallet_id INTEGER NOT NULL,
    address_id INTEGER NOT NULL,
    last_signature_id INTEGER,

    FOREIGN KEY (wallet_id) REFERENCES wallet(id),
    FOREIGN KEY (last_signature_id) REFERENCES "signature"(id),
    FOREIGN KEY (address_id) REFERENCES "address"(id),
    UNIQUE (wallet_id, address_id)
);