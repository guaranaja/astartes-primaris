#!/usr/bin/env bash
# Astartes Primaris — Cloud Workstation startup script.
# Runs on every workstation boot to connect to GCP-managed services.
# Placed in /etc/workstation-startup.d/ and executed by the workstation agent.

set -euo pipefail

echo "[astartes] Initializing Fortress Monastery workstation..."

# ─── Cloud SQL Proxy ──────────────────────────────────────────
# Starts the Cloud SQL Auth Proxy in the background so that
# psql / application code can connect via localhost:5432.
# The workstation's attached service account provides IAM auth.
if [ -n "${CLOUD_SQL_CONNECTION:-}" ]; then
  echo "[astartes] Starting Cloud SQL Proxy → ${CLOUD_SQL_CONNECTION}"
  cloud-sql-proxy "${CLOUD_SQL_CONNECTION}" \
    --port=5432 \
    --quiet &
  disown
fi

# ─── Build Primarch (if source present) ──────────────────────
if [ -d "/home/user/astartes-primaris/services/primarch" ]; then
  echo "[astartes] Building Primarch..."
  cd /home/user/astartes-primaris/services/primarch
  go build ./... 2>/dev/null || true
  cd /home/user/astartes-primaris
fi

echo "[astartes] Workstation ready."
