CREATE TYPE auth_provider_type AS ENUM ('github');

INSERT INTO with_timestamps (table_name) VALUES ('user'), ('github_auth_data');

CREATE TABLE "user" (
    id SERIAL PRIMARY KEY NOT NULL,
    auth_provider auth_provider_type NOT NULL
);

CREATE TABLE github_auth_data (
    user_id INTEGER NOT NULL,
    github_id INTEGER PRIMARY KEY NOT NULL,
    username VARCHAR(100) NOT NULL,
    email VARCHAR(255) NOT NULL,

    FOREIGN KEY (user_id) REFERENCES "user"(id)
);