#!/usr/bin/env bash
# Astartes Primaris — Cloud Run Deploy Script
# Builds, pushes, and deploys services to Google Cloud Run.
#
# Usage:
#   ./infra/deploy.sh                    # Deploy all services
#   ./infra/deploy.sh primarch           # Deploy only Primarch
#   ./infra/deploy.sh aurum              # Deploy only Aurum
#   ./infra/deploy.sh forge              # Deploy only Forge (as Cloud Run Job)
#
# Environment variables:
#   GCP_PROJECT   — GCP project ID (required)
#   GCP_REGION    — GCP region (default: us-central1)
#   ENVIRONMENT   — dev/staging/prod (default: dev)

set -euo pipefail

PROJECT="${GCP_PROJECT:?Set GCP_PROJECT environment variable}"
REGION="${GCP_REGION:-us-central1}"
ENV="${ENVIRONMENT:-dev}"
REPO="${REGION}-docker.pkg.dev/${PROJECT}/astartes-primaris"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

TARGET="${1:-all}"

log() { echo "  → $*"; }

build_and_push() {
  local svc="$1"
  local ctx="$2"

  log "Building ${svc}..."
  docker build -t "${REPO}/${svc}:latest" -t "${REPO}/${svc}:${ENV}-$(git rev-parse --short HEAD)" "${ctx}"

  log "Pushing ${svc}..."
  docker push "${REPO}/${svc}:latest"
  docker push "${REPO}/${svc}:${ENV}-$(git rev-parse --short HEAD)"
}

deploy_service() {
  local svc="$1"
  local port="$2"

  log "Deploying ${svc} to Cloud Run..."
  gcloud run deploy "${svc}-${ENV}" \
    --image="${REPO}/${svc}:latest" \
    --region="${REGION}" \
    --platform=managed \
    --port="${port}" \
    --allow-unauthenticated \
    --set-env-vars="ENVIRONMENT=${ENV}" \
    --quiet
}

deploy_job() {
  local svc="$1"

  log "Deploying ${svc} as Cloud Run Job..."
  gcloud run jobs deploy "${svc}-${ENV}" \
    --image="${REPO}/${svc}:latest" \
    --region="${REGION}" \
    --set-env-vars="ENVIRONMENT=${ENV}" \
    --quiet
}

echo ""
echo "  DEPLOYING ASTARTES PRIMARIS"
echo "  Project:     ${PROJECT}"
echo "  Region:      ${REGION}"
echo "  Environment: ${ENV}"
echo "  Target:      ${TARGET}"
echo ""

# Ensure docker is authenticated with Artifact Registry
gcloud auth configure-docker "${REGION}-docker.pkg.dev" --quiet 2>/dev/null

if [[ "$TARGET" == "all" || "$TARGET" == "primarch" ]]; then
  build_and_push "primarch" "${ROOT}/services/primarch"
  deploy_service "primarch" 8401
fi

if [[ "$TARGET" == "all" || "$TARGET" == "aurum" ]]; then
  build_and_push "aurum" "${ROOT}/services/aurum"
  deploy_service "aurum" 8080
fi

if [[ "$TARGET" == "all" || "$TARGET" == "forge" ]]; then
  build_and_push "forge" "${ROOT}/services/forge"
  deploy_job "forge"
fi

echo ""
echo "  Deployment complete."

if [[ "$TARGET" == "all" || "$TARGET" == "aurum" ]]; then
  AURUM_URL=$(gcloud run services describe "aurum-${ENV}" --region="${REGION}" --format='value(status.url)' 2>/dev/null || echo "unknown")
  echo "  Aurum dashboard: ${AURUM_URL}"
fi

if [[ "$TARGET" == "all" || "$TARGET" == "primarch" ]]; then
  PRIMARCH_URL=$(gcloud run services describe "primarch-${ENV}" --region="${REGION}" --format='value(status.url)' 2>/dev/null || echo "unknown")
  echo "  Primarch API:    ${PRIMARCH_URL}"
fi

echo ""
