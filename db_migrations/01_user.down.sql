DELETE FROM with_timestamps WHERE table_name = ANY(ARRAY['account', 'github_auth_data']);

DROP TABLE IF EXISTS "session";

DROP TABLE IF EXISTS auth;

DROP TABLE IF EXISTS "account";

DROP TYPE IF EXISTS auth_provider_type;
