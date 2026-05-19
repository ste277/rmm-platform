#!/bin/bash
# start-backend.sh — starts all RMM backend services with Postgres wired up
# Run from the repo root: ./start-backend.sh

set -e

REPO_ROOT="$(cd "$(dirname "$0")" && pwd)"
DATABASE_URL="postgres://postgres:postgres@127.0.0.1:5432/postgres?sslmode=disable"

export DATABASE_URL

echo "==> Running DB migrations..."
cd "$REPO_ROOT/backend/db"
go run ./cmd/migrate
echo "    Migrations done."

echo ""
echo "==> Starting backend services..."
echo "    (each service runs in the background — use 'kill $(cat /tmp/rmm-pids)' to stop all)"
echo ""

pids=()

start_service() {
  local name=$1
  local dir=$2
  local addr=$3

  cd "$REPO_ROOT/$dir"
  DATABASE_URL="$DATABASE_URL" go run ./cmd/server &
  local pid=$!
  pids+=($pid)
  echo "    $name started (pid=$pid) on $addr"
  sleep 0.5
}

start_service "broker-service     " "backend/broker-service"      ":8081"
start_service "registration-service" "backend/registration-service" ":8082"
start_service "inventory-service  " "backend/inventory-service"    ":8083"
start_service "command-service    " "backend/command-service"      ":8084"
start_service "compliance-service " "backend/compliance-service"   ":8085"
start_service "gateway-service    " "backend/gateway-service"      ":8080"

# Save PIDs for easy cleanup
printf "%s " "${pids[@]}" > /tmp/rmm-pids
echo "${pids[@]}" > /tmp/rmm-pids

echo ""
echo "==> All services running. PIDs saved to /tmp/rmm-pids"
echo ""
echo "    Health checks:"
sleep 2
for port in 8080 8081 8082 8083 8084 8085; do
  status=$(curl -s http://127.0.0.1:$port/healthz 2>/dev/null || echo '{"status":"unreachable"}')
  echo "    :$port -> $status"
done

echo ""
echo "==> Agent connect command:"
echo "    cd $REPO_ROOT/agents/core"
echo "    RMM_DEV_MODE=true \\"
echo "    RMM_SERVER_URL=ws://127.0.0.1:8081/ws \\"
echo "    RMM_AGENT_ID=dev-agent-001 \\"
echo "    RMM_TENANT_ID=dev-tenant-001 \\"
echo "    go run ./cmd/agent"
echo ""
echo "Press Ctrl+C to stop all services."

# Wait for Ctrl+C then kill all
trap 'echo ""; echo "Stopping all services..."; kill "${pids[@]}" 2>/dev/null; exit 0' INT
wait
