DELETE FROM with_timestamps WHERE table_name = ANY(ARRAY['user', 'github_auth_data']);

DROP TABLE IF EXISTS github_auth_data;

DROP TABLE IF EXISTS "user";

DROP TYPE IF EXISTS auth_provider_type;