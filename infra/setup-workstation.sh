#!/usr/bin/env bash
# ┌──────────────────────────────────────────────────────────────────┐
# │  Astartes Primaris — Cloud Workstation Setup                     │
# │                                                                  │
# │  This script sets up a Google Cloud Workstation from scratch.    │
# │  It handles everything: authentication, infrastructure, image    │
# │  building, and workstation creation.                             │
# │                                                                  │
# │  Run it from the repo root:                                      │
# │    bash infra/setup-workstation.sh                               │
# │                                                                  │
# │  What you need before running:                                   │
# │    1. A Google Cloud account (free trial works)                  │
# │    2. A GCP project with billing enabled                         │
# │    3. gcloud CLI installed (https://cloud.google.com/sdk/docs/install) │
# └──────────────────────────────────────────────────────────────────┘

set -euo pipefail

# ─── Colors ───────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m' # No color

step()  { echo -e "\n${BLUE}${BOLD}[$1/7]${NC} ${BOLD}$2${NC}"; }
ok()    { echo -e "  ${GREEN}OK${NC} $1"; }
warn()  { echo -e "  ${YELLOW}!!${NC} $1"; }
fail()  { echo -e "  ${RED}FAIL${NC} $1"; exit 1; }
info()  { echo -e "  $1"; }

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo ""
echo -e "${BOLD}  ASTARTES PRIMARIS — Cloud Workstation Setup${NC}"
echo ""
echo "  This will create a cloud-based dev environment on Google Cloud."
echo "  Your code runs on a VM in Google's network, with a VS Code"
echo "  editor in your browser. The database, cache, and other services"
echo "  run as managed GCP services — no Docker sidecar containers."
echo ""

# ──────────────────────────────────────────────────────────────
step 1 "Checking prerequisites"
# ──────────────────────────────────────────────────────────────

# gcloud CLI
if ! command -v gcloud &>/dev/null; then
  fail "gcloud CLI not found. Install it: https://cloud.google.com/sdk/docs/install"
fi
ok "gcloud CLI found ($(gcloud version 2>/dev/null | head -1 | awk '{print $NF}'))"

# docker
if ! command -v docker &>/dev/null; then
  fail "docker not found. Install it: https://docs.docker.com/get-docker/"
fi
ok "docker found"

# terraform
if ! command -v terraform &>/dev/null; then
  warn "terraform not found — will install it now"
  if command -v apt-get &>/dev/null; then
    curl -fsSL https://apt.releases.hashicorp.com/gpg | sudo gpg --dearmor -o /usr/share/keyrings/hashicorp-archive-keyring.gpg
    echo "deb [signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/hashicorp.list
    sudo apt-get update && sudo apt-get install -y terraform
  elif command -v brew &>/dev/null; then
    brew install terraform
  else
    fail "Can't auto-install terraform. Install it: https://developer.hashicorp.com/terraform/install"
  fi
fi
ok "terraform found ($(terraform version -json 2>/dev/null | jq -r '.terraform_version' 2>/dev/null || terraform version | head -1))"

# ──────────────────────────────────────────────────────────────
step 2 "Authenticating with Google Cloud"
# ──────────────────────────────────────────────────────────────

# Check if already logged in
ACCOUNT=$(gcloud config get-value account 2>/dev/null || true)
if [ -z "$ACCOUNT" ] || [ "$ACCOUNT" = "(unset)" ]; then
  info "Opening browser for Google Cloud login..."
  gcloud auth login
  ACCOUNT=$(gcloud config get-value account 2>/dev/null)
fi
ok "Logged in as ${ACCOUNT}"

# Also authenticate for Terraform and Docker
gcloud auth application-default login --quiet 2>/dev/null || true

# ──────────────────────────────────────────────────────────────
step 3 "Selecting GCP project"
# ──────────────────────────────────────────────────────────────

# Show available projects
echo ""
info "Your GCP projects:"
echo ""
gcloud projects list --format="table(projectId, name)" 2>/dev/null || true
echo ""

# Get current project or ask
CURRENT_PROJECT=$(gcloud config get-value project 2>/dev/null || true)
if [ -n "$CURRENT_PROJECT" ] && [ "$CURRENT_PROJECT" != "(unset)" ]; then
  info "Currently selected: ${BOLD}${CURRENT_PROJECT}${NC}"
  read -rp "  Use this project? [Y/n] " USE_CURRENT
  if [[ "${USE_CURRENT,,}" == "n" ]]; then
    CURRENT_PROJECT=""
  fi
fi

if [ -z "$CURRENT_PROJECT" ] || [ "$CURRENT_PROJECT" = "(unset)" ]; then
  read -rp "  Enter your GCP project ID: " CURRENT_PROJECT
  gcloud config set project "$CURRENT_PROJECT"
fi

PROJECT="$CURRENT_PROJECT"
ok "Using project: ${PROJECT}"

# Check billing is enabled
BILLING=$(gcloud billing projects describe "$PROJECT" --format="value(billingEnabled)" 2>/dev/null || echo "false")
if [ "$BILLING" != "True" ]; then
  fail "Billing is not enabled on project '${PROJECT}'. Enable it at: https://console.cloud.google.com/billing/linkedaccount?project=${PROJECT}"
fi
ok "Billing is enabled"

# Region
REGION="${GCP_REGION:-us-central1}"
info "Region: ${REGION} (set GCP_REGION env var to change)"

# ──────────────────────────────────────────────────────────────
step 4 "Enabling GCP APIs (this takes ~30 seconds)"
# ──────────────────────────────────────────────────────────────

# These are the Google Cloud services we need turned on.
# Think of them like feature flags for your GCP project.
APIS=(
  "workstations.googleapis.com"         # Cloud Workstations (the dev environment)
  "artifactregistry.googleapis.com"     # Docker image storage (like Docker Hub but private)
  "run.googleapis.com"                  # Cloud Run (where your services deploy)
  "cloudbuild.googleapis.com"           # Builds Docker images in the cloud
  "sqladmin.googleapis.com"             # Cloud SQL (managed PostgreSQL database)
  "secretmanager.googleapis.com"        # Stores passwords and API keys securely
  "vpcaccess.googleapis.com"            # Lets services talk to each other privately
)

for api in "${APIS[@]}"; do
  SHORT_NAME="${api%%.*}"
  gcloud services enable "$api" --quiet 2>/dev/null && ok "$SHORT_NAME" || warn "could not enable $SHORT_NAME (may need permissions)"
done

# ──────────────────────────────────────────────────────────────
step 5 "Provisioning infrastructure with Terraform"
# ──────────────────────────────────────────────────────────────

info "Terraform is an infrastructure-as-code tool. It reads the .tf"
info "files in infra/terraform/ and creates the actual GCP resources"
info "(database, image registry, workstation cluster, etc)."
echo ""

cd "${ROOT}/infra/terraform"

# Initialize Terraform (downloads the Google Cloud plugin)
info "Initializing Terraform..."
terraform init -input=false -no-color 2>&1 | tail -1
ok "Terraform initialized"

# Show what will be created
info "Planning what to create..."
terraform plan \
  -var="project_id=${PROJECT}" \
  -var="region=${REGION}" \
  -input=false \
  -no-color 2>&1 | grep -E "^(Plan:|  #|No changes)" || true
echo ""

read -rp "  Apply these changes? [Y/n] " APPLY
if [[ "${APPLY,,}" == "n" ]]; then
  fail "Aborted. No changes made."
fi

info "Creating infrastructure (this takes 5-10 minutes)..."
terraform apply \
  -var="project_id=${PROJECT}" \
  -var="region=${REGION}" \
  -input=false \
  -auto-approve \
  -no-color 2>&1 | tail -5

ok "Infrastructure provisioned"

cd "$ROOT"

# ──────────────────────────────────────────────────────────────
step 6 "Building and pushing workstation image"
# ──────────────────────────────────────────────────────────────

info "Building a custom Docker image with all your dev tools"
info "(Go, Python, Node, protoc, database clients, etc)."
info "This is a one-time build — takes ~5 minutes."
echo ""

REPO="${REGION}-docker.pkg.dev/${PROJECT}/astartes-primaris"

# Authenticate Docker with Google's image registry
gcloud auth configure-docker "${REGION}-docker.pkg.dev" --quiet 2>/dev/null
ok "Docker authenticated with Artifact Registry"

# Build the image
info "Building workstation image..."
docker build \
  -t "${REPO}/workstation:latest" \
  "${ROOT}/infra/docker/workstation/" 2>&1 | tail -3
ok "Image built"

# Push it to the registry
info "Pushing to Artifact Registry..."
docker push "${REPO}/workstation:latest" 2>&1 | tail -3
ok "Image pushed"

# ──────────────────────────────────────────────────────────────
step 7 "Creating your workstation"
# ──────────────────────────────────────────────────────────────

CLUSTER="astartes-dev-dev"
CONFIG="astartes-config-dev"
WS_NAME="astartes-dev"

info "Creating workstation '${WS_NAME}'..."
gcloud workstations create "$WS_NAME" \
  --cluster="$CLUSTER" \
  --config="$CONFIG" \
  --region="$REGION" \
  --project="$PROJECT" \
  --quiet 2>&1 | tail -3 || warn "Workstation may already exist"

info "Starting workstation..."
gcloud workstations start "$WS_NAME" \
  --cluster="$CLUSTER" \
  --config="$CONFIG" \
  --region="$REGION" \
  --project="$PROJECT" \
  --quiet 2>&1 | tail -3

ok "Workstation is running"

# Get the URL
WS_URL=$(gcloud workstations describe "$WS_NAME" \
  --cluster="$CLUSTER" \
  --config="$CONFIG" \
  --region="$REGION" \
  --project="$PROJECT" \
  --format="value(host)" 2>/dev/null || echo "")

echo ""
echo -e "${GREEN}${BOLD}  Setup complete!${NC}"
echo ""
if [ -n "$WS_URL" ]; then
  echo -e "  Open your workstation: ${BOLD}${WS_URL}${NC}"
else
  echo "  Open your workstation:"
  echo "    https://console.cloud.google.com/workstations/list?project=${PROJECT}"
fi
echo ""
echo "  Or connect via SSH:"
echo "    gcloud workstations ssh ${WS_NAME} --cluster=${CLUSTER} --config=${CONFIG} --region=${REGION}"
echo ""
echo "  Quick reference:"
echo "    Stop:    make workstation-stop GCP_PROJECT=${PROJECT}"
echo "    Start:   make workstation-start GCP_PROJECT=${PROJECT}"
echo "    SSH:     make workstation-ssh GCP_PROJECT=${PROJECT}"
echo "    Rebuild: make workstation-push GCP_PROJECT=${PROJECT}"
echo ""
echo "  Estimated cost: ~\$0.50/hr when running, \$0 when stopped."
echo "  Auto-stops after 2 hours of inactivity."
echo ""
