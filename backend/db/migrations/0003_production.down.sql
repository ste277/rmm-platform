ALTER TABLE commands
    DROP COLUMN IF EXISTS exit_code,
    DROP COLUMN IF EXISTS output,
    DROP COLUMN IF EXISTS error_message,
    DROP COLUMN IF EXISTS completed_at;

DROP TABLE IF EXISTS enrollment_tokens;
DROP TABLE IF EXISTS api_keys;
