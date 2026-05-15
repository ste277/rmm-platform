CREATE TABLE tenants (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE agents (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    hostname TEXT NOT NULL,
    os_family TEXT NOT NULL,
    os_version TEXT NOT NULL,
    architecture TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    enrolled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ
);

CREATE TABLE agent_inventory_snapshots (
    id BIGSERIAL PRIMARY KEY,
    agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    collected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cpu_cores INTEGER,
    memory_bytes BIGINT,
    payload JSONB NOT NULL DEFAULT '{}'::JSONB
);

CREATE TABLE installed_software (
    id BIGSERIAL PRIMARY KEY,
    agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    version TEXT,
    publisher TEXT,
    source TEXT,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE commands (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    command_type TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::JSONB,
    status TEXT NOT NULL DEFAULT 'queued',
    requested_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

CREATE TABLE command_results (
    id BIGSERIAL PRIMARY KEY,
    command_id TEXT NOT NULL REFERENCES commands(id) ON DELETE CASCADE,
    success BOOLEAN NOT NULL,
    exit_code INTEGER,
    output TEXT,
    error_message TEXT,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE compliance_reports (
    id BIGSERIAL PRIMARY KEY,
    agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    report_type TEXT NOT NULL,
    status TEXT NOT NULL,
    generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE compliance_findings (
    id BIGSERIAL PRIMARY KEY,
    report_id BIGINT NOT NULL REFERENCES compliance_reports(id) ON DELETE CASCADE,
    category TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    status TEXT NOT NULL,
    reason TEXT,
    action_hint TEXT
);

CREATE TABLE metric_samples (
    id BIGSERIAL PRIMARY KEY,
    agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    metric_name TEXT NOT NULL,
    metric_value DOUBLE PRECISION NOT NULL,
    collected_at TIMESTAMPTZ NOT NULL,
    tags JSONB NOT NULL DEFAULT '{}'::JSONB
);

CREATE INDEX idx_agents_tenant_id ON agents (tenant_id);
CREATE INDEX idx_inventory_agent_id ON agent_inventory_snapshots (agent_id, collected_at DESC);
CREATE INDEX idx_installed_software_agent_id ON installed_software (agent_id, observed_at DESC);
CREATE INDEX idx_commands_agent_id ON commands (agent_id, created_at DESC);
CREATE INDEX idx_compliance_reports_agent_id ON compliance_reports (agent_id, generated_at DESC);
CREATE INDEX idx_metric_samples_agent_id ON metric_samples (agent_id, collected_at DESC);
