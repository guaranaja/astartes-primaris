#!/usr/bin/env bash
# Astartes Primaris — Post-create setup for Codespaces
set -euo pipefail

echo "⚔ Initializing Fortress Monastery..."

# Install Go tools
echo "  → Installing Go tools..."
go install golang.org/x/tools/gopls@latest 2>/dev/null || true
go install github.com/go-delve/delve/cmd/dlv@latest 2>/dev/null || true
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest 2>/dev/null || true
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest 2>/dev/null || true

# Install Python deps for Forge/Marines
echo "  → Installing Python dependencies..."
pip install --quiet numpy pandas scipy numba 2>/dev/null || true

# Install protoc
if ! command -v protoc &>/dev/null; then
  echo "  → Installing protoc..."
  PROTOC_VERSION="25.1"
  curl -sLO "https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip"
  sudo unzip -qo "protoc-${PROTOC_VERSION}-linux-x86_64.zip" -d /usr/local
  rm "protoc-${PROTOC_VERSION}-linux-x86_64.zip"
fi

# Install gcloud CLI
if ! command -v gcloud &>/dev/null; then
  echo "  → Installing Google Cloud CLI..."
  curl -sL https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-linux-x86_64.tar.gz | tar -xz -C /tmp
  /tmp/google-cloud-sdk/install.sh --quiet --path-update true
  echo 'source /tmp/google-cloud-sdk/path.bash.inc' >> ~/.bashrc
fi

# Build Primarch
echo "  → Building Primarch..."
cd /workspace/services/primarch && go build ./... 2>/dev/null || true

echo ""
echo "  Fortress Monastery initialized."
echo "  Run 'make up' to start the full Imperium."
echo ""
