# Astartes Primaris — Build & Operations
# "The Codex Astartes does support this action."

.PHONY: help up down logs proto build test deploy

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

build-aurum: ## Build Aurum dashboard container
	docker build -t astartes/aurum:latest services/aurum/

build-forge: ## Build Forge worker container
	docker build -t astartes/forge-worker:latest services/forge/

build-marine-base: ## Build Marine base image
	docker build -t astartes/marine-base:latest services/tacticarium/

build-auspex: ## Build Auspex market data collector
	docker build -t astartes/auspex:latest services/auspex/

build-registry: ## Build Registry marketplace container
	docker build -t astartes/registry:latest services/registry/

build-all: build-primarch build-aurum build-auspex build-forge build-marine-base build-registry ## Build all containers

# ─── Database ───────────────────────────────────────────────

auspex-logs: ## Tail Auspex logs
	docker compose logs -f auspex

auspex-health: ## Check Auspex health
	@curl -s http://localhost:8300/health | python3 -m json.tool

auspex-metrics: ## Show Auspex metrics
	@curl -s http://localhost:8300/metrics | python3 -m json.tool

db-migrate: ## Run Librarium migrations
	@echo "Running migrations..."
	docker compose exec librarium-timescale psql -U librarium -d librarium -f /docker-entrypoint-initdb.d/001_initial_schema.sql
	docker compose exec librarium-timescale psql -U librarium -d librarium -f /docker-entrypoint-initdb.d/002_council_schema.sql
	docker compose exec librarium-timescale psql -U librarium -d librarium -f /docker-entrypoint-initdb.d/005_registry_marketplace.sql

db-shell: ## Open psql shell to Librarium
	docker compose exec librarium-timescale psql -U librarium -d librarium

redis-shell: ## Open Redis CLI
	docker compose exec librarium-redis redis-cli

# ─── Forge ──────────────────────────────────────────────────

forge-submit: ## Submit a backtest job (usage: make forge-submit STRATEGY=es-momentum)
	@echo "Submitting forge job for $(STRATEGY)..."

# ─── Registry (Marketplace) ────────────────────────────────

registry-logs: ## Tail Registry logs
	docker compose logs -f registry

registry-health: ## Check Registry health
	@curl -s http://localhost:8701/healthz | python3 -m json.tool

registry-clients: ## List registered clients (requires REGISTRY_MASTER_TOKEN)
	@curl -s -H "Authorization: Bearer $${REGISTRY_MASTER_TOKEN}" http://localhost:8701/api/v1/admin/clients | python3 -m json.tool

registry-billing: ## View billing overview (requires REGISTRY_MASTER_TOKEN)
	@curl -s -H "Authorization: Bearer $${REGISTRY_MASTER_TOKEN}" http://localhost:8701/api/v1/admin/billing/overview | python3 -m json.tool

registry-integrity: ## Check client integrity status (requires REGISTRY_MASTER_TOKEN)
	@curl -s -H "Authorization: Bearer $${REGISTRY_MASTER_TOKEN}" http://localhost:8701/api/v1/admin/integrity | python3 -m json.tool

# ─── Monitoring ─────────────────────────────────────────────

grafana: ## Open Grafana in browser
	@echo "Grafana: http://localhost:3001 (admin / admin)"

prometheus: ## Open Prometheus in browser
	@echo "Prometheus: http://localhost:9090"

vault: ## Open Vault UI in browser
	@echo "Vault: http://localhost:8200 (token: dev-root-token)"

# ─── Cloud Run Deployment ─────────────────────────────────

deploy: ## Deploy all services to Cloud Run (requires GCP_PROJECT)
	./infra/deploy.sh

deploy-primarch: ## Deploy Primarch to Cloud Run
	./infra/deploy.sh primarch

deploy-aurum: ## Deploy Aurum to Cloud Run
	./infra/deploy.sh aurum

deploy-forge: ## Deploy Forge to Cloud Run
	./infra/deploy.sh forge

deploy-registry: ## Deploy Registry marketplace to Cloud Run
	./infra/deploy.sh registry

infra-init: ## Initialize Terraform for GCP
	cd infra/terraform && terraform init

infra-plan: ## Plan infrastructure changes (requires GCP_PROJECT)
	cd infra/terraform && terraform plan -var="project_id=$(GCP_PROJECT)"

infra-apply: ## Apply infrastructure changes (requires GCP_PROJECT)
	cd infra/terraform && terraform apply -var="project_id=$(GCP_PROJECT)"
