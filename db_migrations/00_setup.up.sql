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
