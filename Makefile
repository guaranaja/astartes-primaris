# Astartes Primaris — Build & Operations
# "The Codex Astartes does support this action."

.PHONY: help up down logs proto build test

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ─── Local Development ──────────────────────────────────────

up: ## Start the full Imperium (local dev stack)
	docker compose up -d

down: ## Shut down the Imperium
	docker compose down

logs: ## Tail logs from all services
	docker compose logs -f

status: ## Show status of all services
	docker compose ps

# ─── Protobuf ───────────────────────────────────────────────

proto: ## Generate protobuf code (Go + Python)
	@echo "Generating protobuf..."
	@mkdir -p gen/go gen/python
	protoc --proto_path=schemas/protobuf \
		--go_out=gen/go --go_opt=paths=source_relative \
		--go-grpc_out=gen/go --go-grpc_opt=paths=source_relative \
		--python_out=gen/python \
		--grpc_python_out=gen/python \
		schemas/protobuf/*.proto
	@echo "Done."

# ─── Build ──────────────────────────────────────────────────

build-primarch: ## Build Primarch container
	docker build -t astartes/primarch:latest services/primarch/

build-forge: ## Build Forge worker container
	docker build -t astartes/forge-worker:latest services/forge/

build-marine-base: ## Build Marine base image
	docker build -t astartes/marine-base:latest services/tacticarium/

build-all: build-primarch build-forge build-marine-base ## Build all containers

# ─── Database ───────────────────────────────────────────────

db-migrate: ## Run Librarium migrations
	@echo "Running migrations..."
	docker compose exec librarium-timescale psql -U librarium -d librarium -f /docker-entrypoint-initdb.d/001_initial_schema.sql

db-shell: ## Open psql shell to Librarium
	docker compose exec librarium-timescale psql -U librarium -d librarium

redis-shell: ## Open Redis CLI
	docker compose exec librarium-redis redis-cli

# ─── Forge ──────────────────────────────────────────────────

forge-submit: ## Submit a backtest job (usage: make forge-submit STRATEGY=es-momentum)
	@echo "Submitting forge job for $(STRATEGY)..."

# ─── Monitoring ─────────────────────────────────────────────

grafana: ## Open Grafana in browser
	@echo "Grafana: http://localhost:3001 (admin / admin)"

prometheus: ## Open Prometheus in browser
	@echo "Prometheus: http://localhost:9090"

vault: ## Open Vault UI in browser
	@echo "Vault: http://localhost:8200 (token: dev-root-token)"
