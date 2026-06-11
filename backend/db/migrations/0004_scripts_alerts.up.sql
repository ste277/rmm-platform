-- Script library
CREATE TABLE IF NOT EXISTS scripts (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT,
    body        TEXT NOT NULL,
    created_by  TEXT NOT NULL DEFAULT 'dashboard',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Alerts
CREATE TABLE IF NOT EXISTS alert_rules (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    metric_name TEXT NOT NULL,
    operator    TEXT NOT NULL CHECK (operator IN ('>', '<', '>=', '<=')),
    threshold   DOUBLE PRECISION NOT NULL,
    duration_s  INTEGER NOT NULL DEFAULT 0,
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS alert_events (
    id          BIGSERIAL PRIMARY KEY,
    rule_id     TEXT NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
    agent_id    TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    metric_value DOUBLE PRECISION NOT NULL,
    fired_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_scripts_tenant ON scripts (tenant_id);
CREATE INDEX IF NOT EXISTS idx_alert_rules_tenant ON alert_rules (tenant_id);
CREATE INDEX IF NOT EXISTS idx_alert_events_rule ON alert_events (rule_id, fired_at DESC);
CREATE INDEX IF NOT EXISTS idx_alert_events_agent ON alert_events (agent_id, fired_at DESC);
