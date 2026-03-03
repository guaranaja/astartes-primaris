# Astartes Primaris — Cloud Workstations Dev Image
# Custom container for Google Cloud Workstations with all platform
# tooling pre-installed. Connects to GCP-managed services (Cloud SQL,
# Memorystore, etc.) instead of running them as sidecar containers.
#
# Build:
#   docker build -t ${REGION}-docker.pkg.dev/${PROJECT}/astartes-primaris/workstation:latest \
#     infra/docker/workstation/
#
# Base: Google's Code OSS (VS Code) workstation image with full IDE.

FROM us-central1-docker.pkg.dev/cloud-workstations-images/predefined/code-oss:latest

USER root

# ─── System packages ─────────────────────────────────────────
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    curl \
    git \
    jq \
    unzip \
    wget \
    postgresql-client \
    redis-tools \
    && rm -rf /var/lib/apt/lists/*

# ─── Go 1.24 ─────────────────────────────────────────────────
ARG GO_VERSION=1.24.0
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" \
    | tar -xz -C /usr/local
ENV PATH="/usr/local/go/bin:/root/go/bin:${PATH}"
ENV GOPATH="/root/go"

# Go tools for development
RUN go install golang.org/x/tools/gopls@latest && \
    go install github.com/go-delve/delve/cmd/dlv@latest && \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# ─── Python 3.12 ─────────────────────────────────────────────
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3.12 \
    python3.12-venv \
    python3-pip \
    && rm -rf /var/lib/apt/lists/* \
    && ln -sf /usr/bin/python3.12 /usr/local/bin/python3 \
    && ln -sf /usr/bin/python3.12 /usr/local/bin/python

RUN pip install --no-cache-dir --break-system-packages \
    numpy pandas scipy numba \
    nats-py psycopg[binary] redis structlog httpx

# ─── Node.js 20 ──────────────────────────────────────────────
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && \
    apt-get install -y --no-install-recommends nodejs && \
    rm -rf /var/lib/apt/lists/*

# ─── Protobuf compiler ───────────────────────────────────────
ARG PROTOC_VERSION=25.1
RUN curl -fsSL -o /tmp/protoc.zip \
    "https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip" && \
    unzip -qo /tmp/protoc.zip -d /usr/local && \
    rm /tmp/protoc.zip

# ─── Cloud SQL Auth Proxy ────────────────────────────────────
# Allows workstation to connect to Cloud SQL via IAM auth over
# the VPC without needing a public IP on the database.
RUN curl -fsSL -o /usr/local/bin/cloud-sql-proxy \
    "https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v2.14.1/cloud-sql-proxy.linux.amd64" && \
    chmod +x /usr/local/bin/cloud-sql-proxy

# ─── VS Code extensions (pre-installed in image) ─────────────
# The code-oss base handles extension installs via a startup mechanism.
# We provide a settings file that the workstation picks up on boot.
COPY settings.json /etc/workstation-startup.d/settings.json

# ─── Startup script ──────────────────────────────────────────
COPY startup.sh /etc/workstation-startup.d/050-astartes.sh
RUN chmod +x /etc/workstation-startup.d/050-astartes.sh

USER user
