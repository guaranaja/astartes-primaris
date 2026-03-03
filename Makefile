# Astartes Primaris — Build & Operations
# "The Codex Astartes does support this action."

.PHONY: help up down logs proto build test deploy build-python-base

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

PYTHON_VERSION ?= 3.12.8

build-python-base: ## Build shared Python base image with stdlib extensions
	docker build \
		--build-arg PYTHON_VERSION=$(PYTHON_VERSION) \
		-t astartes/python-base:3.12 \
		infra/docker/python-base/

build-primarch: ## Build Primarch container
	docker build -t astartes/primarch:latest services/primarch/

build-aurum: ## Build Aurum dashboard container
	docker build -t astartes/aurum:latest services/aurum/

build-forge: build-python-base ## Build Forge worker container
	docker build -t astartes/forge-worker:latest services/forge/

build-auspex: build-python-base ## Build Auspex data collector container
	docker build -t astartes/auspex:latest services/auspex/

build-marine-base: build-python-base ## Build Marine base image
	docker build -t astartes/marine-base:latest services/tacticarium/

build-workstation: ## Build Cloud Workstation dev image
	docker build -t astartes/workstation:latest infra/docker/workstation/

build-all: build-python-base build-primarch build-aurum build-forge build-auspex build-marine-base ## Build all containers

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

# ─── Cloud Run Deployment ─────────────────────────────────

deploy: ## Deploy all services to Cloud Run (requires GCP_PROJECT)
	./infra/deploy.sh

deploy-primarch: ## Deploy Primarch to Cloud Run
	./infra/deploy.sh primarch

deploy-aurum: ## Deploy Aurum to Cloud Run
	./infra/deploy.sh aurum

deploy-forge: ## Deploy Forge to Cloud Run
	./infra/deploy.sh forge

infra-init: ## Initialize Terraform for GCP
	cd infra/terraform && terraform init

infra-plan: ## Plan infrastructure changes (requires GCP_PROJECT)
	cd infra/terraform && terraform plan -var="project_id=$(GCP_PROJECT)"

infra-apply: ## Apply infrastructure changes (requires GCP_PROJECT)
	cd infra/terraform && terraform apply -var="project_id=$(GCP_PROJECT)"

# ─── Cloud Workstations ──────────────────────────────────────
# Run 'make workstation-setup' for guided first-time setup.
# After that, use start/stop/ssh to manage your workstation.

GCP_REGION ?= us-central1

workstation-setup: ## First-time setup — creates everything from scratch (interactive)
	bash infra/setup-workstation.sh

workstation-push: build-workstation ## Rebuild and push workstation image (requires GCP_PROJECT)
	@gcloud auth configure-docker $(GCP_REGION)-docker.pkg.dev --quiet 2>/dev/null
	@docker tag astartes/workstation:latest \
		$(GCP_REGION)-docker.pkg.dev/$(GCP_PROJECT)/astartes-primaris/workstation:latest
	docker push $(GCP_REGION)-docker.pkg.dev/$(GCP_PROJECT)/astartes-primaris/workstation:latest

workstation-start: ## Start your Cloud Workstation (requires GCP_PROJECT)
	gcloud workstations start astartes-dev \
		--cluster=astartes-dev-dev \
		--config=astartes-config-dev \
		--region=$(GCP_REGION) \
		--project=$(GCP_PROJECT)

workstation-stop: ## Stop your Cloud Workstation (requires GCP_PROJECT)
	gcloud workstations stop astartes-dev \
		--cluster=astartes-dev-dev \
		--config=astartes-config-dev \
		--region=$(GCP_REGION) \
		--project=$(GCP_PROJECT)

workstation-ssh: ## SSH into your Cloud Workstation (requires GCP_PROJECT)
	gcloud workstations ssh astartes-dev \
		--cluster=astartes-dev-dev \
		--config=astartes-config-dev \
		--region=$(GCP_REGION) \
		--project=$(GCP_PROJECT)
