CREATE TYPE auth_provider_type AS ENUM ('github', 'discord', 'google');

INSERT INTO with_timestamps (table_name) VALUES ('account'), ('auth');

CREATE TABLE account (
    id SERIAL PRIMARY KEY NOT NULL,
    selected_auth_provider auth_provider_type NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL
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
    FOREIGN KEY (account_id) REFERENCES account(id)
);

CREATE TABLE session (
    id uuid PRIMARY KEY NOT NULL DEFAULT uuid_generate_v4(),
    account_id INTEGER NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,

    FOREIGN KEY (account_id) REFERENCES account(id)
);
