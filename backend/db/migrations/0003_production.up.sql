-- API keys for dashboard authentication
CREATE TABLE IF NOT EXISTS api_keys (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    key_hash    TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ
);

-- Enrollment tokens for agent registration
CREATE TABLE IF NOT EXISTS enrollment_tokens (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE,
    label       TEXT,
    used        BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Add output columns to commands table
ALTER TABLE commands
    ADD COLUMN IF NOT EXISTS exit_code     INTEGER,
    ADD COLUMN IF NOT EXISTS output        TEXT,
    ADD COLUMN IF NOT EXISTS error_message TEXT,
    ADD COLUMN IF NOT EXISTS completed_at  TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_api_keys_tenant_id ON api_keys (tenant_id);
CREATE INDEX IF NOT EXISTS idx_enrollment_tokens_tenant_id ON enrollment_tokens (tenant_id);
