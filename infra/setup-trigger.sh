#!/usr/bin/env bash
# Astartes Primaris — Cloud Build Trigger Setup
# Creates a GitHub trigger that auto-deploys on push to main.
#
# Prerequisites:
#   1. GitHub repo must be connected to Cloud Build (one-time browser flow)
#   2. gcloud CLI authenticated with project owner permissions
#
# Usage:
#   ./infra/setup-trigger.sh

set -euo pipefail

PROJECT=$(gcloud config get-value project 2>/dev/null)
REGION="${GCP_REGION:-us-central1}"
REPO_OWNER="guaranaja"
REPO_NAME="astartes-primaris"
TRIGGER_NAME="astartes-primaris-main"

echo ""
echo "  ASTARTES PRIMARIS — Cloud Build Trigger Setup"
echo "  Project:  ${PROJECT}"
echo "  Region:   ${REGION}"
echo "  Repo:     ${REPO_OWNER}/${REPO_NAME}"
echo "  Trigger:  ${TRIGGER_NAME}"
echo ""

# ── Step 1: Check if GitHub connection exists ────────────────
echo "  Checking GitHub connection..."
CONNECTION_NAME="github-${REPO_OWNER}"
CONNECTION=$(gcloud builds connections list --region="${REGION}" --format="value(name)" --filter="name~${CONNECTION_NAME}" 2>/dev/null || true)

if [ -z "$CONNECTION" ]; then
  echo ""
  echo "  No GitHub connection found. Creating one..."
  echo "  This will open a browser window to authorize Cloud Build."
  echo ""
  gcloud builds connections create github "${CONNECTION_NAME}" \
    --region="${REGION}" \
    --authorizer-token-secret-version="projects/${PROJECT}/secrets/github-token/versions/latest" 2>/dev/null || {
    echo ""
    echo "  Automated connection failed. Please connect GitHub manually:"
    echo ""
    echo "    1. Go to: https://console.cloud.google.com/cloud-build/repositories/2nd-gen?project=${PROJECT}"
    echo "    2. Click 'Create Host Connection' → GitHub"
    echo "    3. Authorize and select the ${REPO_OWNER}/${REPO_NAME} repo"
    echo "    4. Re-run this script"
    echo ""
    exit 1
  }
  echo "  GitHub connection created."
fi

# ── Step 2: Link the repository ──────────────────────────────
echo "  Linking repository..."
REPO_LINK=$(gcloud builds repositories list --connection="${CONNECTION_NAME}" --region="${REGION}" --format="value(name)" --filter="remoteUri~${REPO_NAME}" 2>/dev/null || true)

if [ -z "$REPO_LINK" ]; then
  gcloud builds repositories create "${REPO_NAME}" \
    --connection="${CONNECTION_NAME}" \
    --region="${REGION}" \
    --remote-uri="https://github.com/${REPO_OWNER}/${REPO_NAME}.git" \
    --quiet 2>/dev/null || echo "  Repository may already be linked."
fi

# ── Step 3: Create the trigger ───────────────────────────────
echo "  Creating Cloud Build trigger..."

# Check if trigger already exists
EXISTING=$(gcloud builds triggers list --region="${REGION}" --format="value(name)" --filter="name=${TRIGGER_NAME}" 2>/dev/null || true)

if [ -n "$EXISTING" ]; then
  echo "  Trigger '${TRIGGER_NAME}' already exists. Deleting and recreating..."
  gcloud builds triggers delete "${TRIGGER_NAME}" --region="${REGION}" --quiet
fi

gcloud builds triggers create github \
  --name="${TRIGGER_NAME}" \
  --region="${REGION}" \
  --repository="projects/${PROJECT}/locations/${REGION}/connections/${CONNECTION_NAME}/repositories/${REPO_NAME}" \
  --branch-pattern="^main$" \
  --build-config="cloudbuild.yaml" \
  --substitutions="_REGION=${REGION},_ENVIRONMENT=dev" \
  --description="Auto-deploy Astartes Primaris on push to main" \
  --quiet

echo ""
echo "  Trigger created successfully!"
echo ""
echo "  Every push to main will now:"
echo "    1. Build all service images (primarch, aurum, auspex, forge, registry)"
echo "    2. Push to Artifact Registry"
echo "    3. Deploy to Cloud Run"
echo ""
echo "  View trigger: https://console.cloud.google.com/cloud-build/triggers?project=${PROJECT}"
echo "  View builds:  https://console.cloud.google.com/cloud-build/builds?project=${PROJECT}"
echo ""
