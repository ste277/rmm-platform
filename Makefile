PROTO_DIR=proto

.PHONY: help agent broker gateway registration inventory command compliance integration-broker-sql
help:
	@echo "Available targets: help agent broker gateway registration inventory command compliance integration-broker-sql"

agent:
	cd agents/core && go run ./cmd/agent

broker:
	cd backend/broker-service && go run ./cmd/server

gateway:
	cd backend/gateway-service && go run ./cmd/server

registration:
	cd backend/registration-service && go run ./cmd/server

inventory:
	cd backend/inventory-service && go run ./cmd/server

command:
	cd backend/command-service && go run ./cmd/server

compliance:
	cd backend/compliance-service && go run ./cmd/server

integration-broker-sql:
	./scripts/integration_broker_sql.sh
