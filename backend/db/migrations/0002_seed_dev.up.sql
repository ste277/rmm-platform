INSERT INTO tenants (id, name)
VALUES ('dev-tenant-001', 'Development Tenant')
ON CONFLICT (id) DO NOTHING;

INSERT INTO agents (id, tenant_id, hostname, os_family, os_version, architecture, status, last_seen_at)
VALUES ('dev-agent-001', 'dev-tenant-001', 'local-agent', 'darwin', 'dev-local', 'arm64', 'online', NOW())
ON CONFLICT (id) DO NOTHING;
