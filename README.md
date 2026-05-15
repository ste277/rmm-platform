# RMM Platform

Cross-platform remote monitoring and management starter monorepo.

## Major Components

- Endpoint agent
- Backend control plane
- Admin dashboard

## Local Development

1. Start local infrastructure with `docker compose -f docker-compose.dev.yml up -d`
2. Run the agent in local mode:

   ```bash
   cd agents/core
   RMM_DEV_MODE=true \
   RMM_SERVER_URL=http://127.0.0.1:8080 \
   RMM_AGENT_ID=dev-agent-001 \
   RMM_TENANT_ID=dev-tenant-001 \
   go run ./cmd/agent
   ```

3. Run backend services, for example:

   ```bash
   cd backend/broker-service
   go run ./cmd/server
   ```

4. Run the dashboard in development mode

## Integration Verification

Run the broker-to-Postgres integration harness with:

```bash
make integration-broker-sql
```

The script will:

- start a local PostgreSQL instance on port `15432`
- create a fresh test database
- apply migrations
- start the broker and SQL-backed services on loopback ports
- run a dev agent against the broker over WebSocket
- assert database rows and API responses

## Notes

This repository is a scaffold intended to give you a clean starting structure for a production-grade RMM platform.
