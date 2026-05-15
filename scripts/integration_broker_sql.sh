#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GOCACHE_DIR="${ROOT_DIR}/.gocache"
POSTGRES_PREFIX="${POSTGRES_PREFIX:-/opt/homebrew/opt/postgresql@16}"
PG_CTL="${POSTGRES_PREFIX}/bin/pg_ctl"
PSQL="${POSTGRES_PREFIX}/bin/psql"
PG_DATA="${PG_DATA:-/opt/homebrew/var/postgresql@16}"
PG_LOG="${ROOT_DIR}/.postgres.integration.log"
PG_PORT="${PG_PORT:-15432}"
DB_NAME="${DB_NAME:-rmm_integration}"
DB_USER="${DB_USER:-stephen}"
DATABASE_URL="postgres://${DB_USER}@127.0.0.1:${PG_PORT}/${DB_NAME}?sslmode=disable"

BROKER_ADDR="127.0.0.1:28081"
GATEWAY_ADDR="127.0.0.1:28080"
REGISTRATION_ADDR="127.0.0.1:28082"
INVENTORY_ADDR="127.0.0.1:28083"
COMMAND_ADDR="127.0.0.1:28084"
COMPLIANCE_ADDR="127.0.0.1:28085"

AGENT_ID="integration-agent"
TENANT_ID="integration-tenant"

mkdir -p "${GOCACHE_DIR}"

PIDS=()
POSTGRES_STARTED_BY_SCRIPT=0

cleanup() {
  local code=$?
  for pid in "${PIDS[@]:-}"; do
    if kill -0 "${pid}" >/dev/null 2>&1; then
      kill "${pid}" >/dev/null 2>&1 || true
      wait "${pid}" 2>/dev/null || true
    fi
  done

  if [[ "${POSTGRES_STARTED_BY_SCRIPT}" == "1" ]]; then
    "${PG_CTL}" -D "${PG_DATA}" stop >/dev/null 2>&1 || true
  fi

  exit "${code}"
}
trap cleanup EXIT

log() {
  printf '\n[%s] %s\n' "$(date +%H:%M:%S)" "$*"
}

wait_for_http() {
  local url="$1"
  for _ in {1..30}; do
    if curl -fsS "${url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "Timed out waiting for ${url}" >&2
  return 1
}

wait_for_sql() {
  local sql="$1"
  for _ in {1..30}; do
    local result
    result="$("${PSQL}" -h 127.0.0.1 -p "${PG_PORT}" -d "${DB_NAME}" -Atqc "${sql}" 2>/dev/null || true)"
    if [[ -n "${result}" && "${result}" != "0" ]]; then
      return 0
    fi
    sleep 1
  done
  echo "Timed out waiting for SQL condition: ${sql}" >&2
  return 1
}

start_postgres() {
  if "${PSQL}" -h 127.0.0.1 -p "${PG_PORT}" -d postgres -Atqc "SELECT 1" >/dev/null 2>&1; then
    log "PostgreSQL already running on ${PG_PORT}"
    return 0
  fi

  log "Starting PostgreSQL on port ${PG_PORT}"
  "${PG_CTL}" -D "${PG_DATA}" -l "${PG_LOG}" -o "-p ${PG_PORT}" start
  POSTGRES_STARTED_BY_SCRIPT=1
  "${PSQL}" -h 127.0.0.1 -p "${PG_PORT}" -d postgres -Atqc "SELECT 1" >/dev/null
}

reset_database() {
  log "Resetting database ${DB_NAME}"
  "${PSQL}" -h 127.0.0.1 -p "${PG_PORT}" -d postgres \
    -c "DROP DATABASE IF EXISTS ${DB_NAME};" \
    -c "CREATE DATABASE ${DB_NAME};" >/dev/null
}

run_migrations() {
  log "Running migrations"
  (
    cd "${ROOT_DIR}/backend/db"
    env "DATABASE_URL=${DATABASE_URL}" "GOCACHE=${GOCACHE_DIR}" go run ./cmd/migrate --direction up >/dev/null
  )
}

start_service() {
  local name="$1"
  local workdir="$2"
  shift 2

  log "Starting ${name}"
  (
    cd "${workdir}"
    env "DATABASE_URL=${DATABASE_URL}" "GOCACHE=${GOCACHE_DIR}" "$@" >/tmp/"${name}".log 2>&1
  ) &
  PIDS+=("$!")
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  if [[ "${haystack}" != *"${needle}"* ]]; then
    echo "Expected output to contain: ${needle}" >&2
    echo "Actual output: ${haystack}" >&2
    return 1
  fi
}

start_postgres
reset_database
run_migrations

start_service broker "${ROOT_DIR}/backend/broker-service" \
  BROKER_ADDR="${BROKER_ADDR}" go run ./cmd/server
start_service gateway "${ROOT_DIR}/backend/gateway-service" \
  GATEWAY_ADDR="${GATEWAY_ADDR}" go run ./cmd/server
start_service registration "${ROOT_DIR}/backend/registration-service" \
  REGISTRATION_ADDR="${REGISTRATION_ADDR}" go run ./cmd/server
start_service inventory "${ROOT_DIR}/backend/inventory-service" \
  INVENTORY_ADDR="${INVENTORY_ADDR}" go run ./cmd/server
start_service command "${ROOT_DIR}/backend/command-service" \
  COMMAND_ADDR="${COMMAND_ADDR}" go run ./cmd/server
start_service compliance "${ROOT_DIR}/backend/compliance-service" \
  COMPLIANCE_ADDR="${COMPLIANCE_ADDR}" go run ./cmd/server

wait_for_http "http://${BROKER_ADDR}/healthz"
wait_for_http "http://${GATEWAY_ADDR}/healthz"
wait_for_http "http://${REGISTRATION_ADDR}/healthz"
wait_for_http "http://${INVENTORY_ADDR}/healthz"
wait_for_http "http://${COMMAND_ADDR}/healthz"
wait_for_http "http://${COMPLIANCE_ADDR}/healthz"

log "Starting integration agent"
(
  cd "${ROOT_DIR}/agents/core"
  env \
    "GOCACHE=${GOCACHE_DIR}" \
    RMM_DEV_MODE=true \
    "RMM_SERVER_URL=ws://${BROKER_ADDR}/ws?agent_id=${AGENT_ID}" \
    "RMM_AGENT_ID=${AGENT_ID}" \
    "RMM_TENANT_ID=${TENANT_ID}" \
    go run ./cmd/agent >/tmp/integration-agent.log 2>&1
) &
PIDS+=("$!")

wait_for_sql "SELECT COUNT(*) FROM agents WHERE id = '${AGENT_ID}';"
wait_for_sql "SELECT COUNT(*) FROM installed_software WHERE agent_id = '${AGENT_ID}';"
wait_for_sql "SELECT COUNT(*) FROM metric_samples WHERE agent_id = '${AGENT_ID}';"

log "Posting command and compliance data"
command_response="$(curl -fsS -X POST "http://${COMMAND_ADDR}/api/v1/commands" \
  -H 'Content-Type: application/json' \
  -d "{\"agent_id\":\"${AGENT_ID}\",\"command_type\":\"script\",\"script_body\":\"echo integration\",\"timeout_seconds\":15}")"
assert_contains "${command_response}" "\"status\":\"queued\""

compliance_response="$(curl -fsS -X POST "http://${COMPLIANCE_ADDR}/api/v1/compliance/reports" \
  -H 'Content-Type: application/json' \
  -d "{\"agent_id\":\"${AGENT_ID}\",\"status\":\"non_compliant\",\"findings\":[{\"category\":\"software\",\"resource_id\":\"winget:git\",\"status\":\"failed\",\"reason\":\"source unavailable\",\"action_hint\":\"retry later\"}]}")"
assert_contains "${compliance_response}" "\"status\":\"stored\""

wait_for_sql "SELECT COUNT(*) FROM commands WHERE agent_id = '${AGENT_ID}';"
wait_for_sql "SELECT COUNT(*) FROM compliance_reports WHERE agent_id = '${AGENT_ID}';"

log "Verifying API responses"
agents_response="$(curl -fsS "http://${GATEWAY_ADDR}/api/v1/agents")"
commands_response="$(curl -fsS "http://${COMMAND_ADDR}/api/v1/commands")"
compliance_response="$(curl -fsS "http://${COMPLIANCE_ADDR}/api/v1/compliance/reports")"

assert_contains "${agents_response}" "\"id\":\"${AGENT_ID}\""
assert_contains "${commands_response}" "\"agent_id\":\"${AGENT_ID}\""
assert_contains "${compliance_response}" "\"agent_id\":\"${AGENT_ID}\""

log "Verifying SQL state"
"${PSQL}" -h 127.0.0.1 -p "${PG_PORT}" -d "${DB_NAME}" \
  -c "SELECT id, tenant_id, hostname, status FROM agents WHERE id = '${AGENT_ID}';" \
  -c "SELECT agent_id, name, version FROM installed_software WHERE agent_id = '${AGENT_ID}';" \
  -c "SELECT agent_id, metric_name, metric_value FROM metric_samples WHERE agent_id = '${AGENT_ID}' ORDER BY id DESC LIMIT 2;" \
  -c "SELECT id, agent_id, command_type, status FROM commands WHERE agent_id = '${AGENT_ID}';" \
  -c "SELECT cr.agent_id, cr.status, cf.category, cf.resource_id, cf.status FROM compliance_reports cr JOIN compliance_findings cf ON cf.report_id = cr.id WHERE cr.agent_id = '${AGENT_ID}';"

log "Integration verification passed"
