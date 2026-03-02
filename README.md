# ASTARTES PRIMARIS

> Cloud-native algorithmic trading platform

A constellation of services that scale and instantiate on demand — strategies wake, execute,
and sleep as ephemeral containers while a persistent data layer and management plane
coordinate the operation.

## Quick Start

```bash
# Bring up the Imperium (local dev)
make up

# Check status
make status

# View logs
make logs

# Open monitoring
make grafana      # http://localhost:3001
make prometheus   # http://localhost:9090
```

## Architecture

See [ARCHITECTURE.md](./ARCHITECTURE.md) for the full system design.

### Services

| Service       | Role                          | Port(s)           |
|---------------|-------------------------------|--------------------|
| Primarch      | Control plane & orchestrator  | 8400-8402          |
| Librarium     | Persistent data layer         | 5432, 6379         |
| Auspex        | Market data collection        | 8300               |
| Tacticarium   | Strategy runtime engine       | 9000 (ephemeral)   |
| Forge         | Backtest & optimization       | 8500-8502          |
| Vox           | Event bus (NATS)              | 4222, 8222         |
| Logis         | Execution & positions         | 8600-8602          |
| Codex         | Configuration & rules         | 8700-8702          |
| Iron Halo     | Auth & security (Vault)       | 8200               |
| Apothecary    | Monitoring & observability    | 9090, 3001         |
| Aurum         | Web dashboard                 | 3000               |

## Project Structure

```
astartes-primaris/
├── services/           # Individual service codebases
│   ├── primarch/       # Control plane
│   ├── librarium/      # Database + migrations
│   ├── auspex/         # Data collectors
│   ├── tacticarium/    # Strategy runtime
│   ├── forge/          # Backtest engine
│   ├── vox/            # Event bus config
│   ├── logis/          # Order execution
│   ├── codex/          # Config management
│   ├── iron-halo/      # Security & secrets
│   ├── apothecary/     # Monitoring
│   └── aurum/          # Web UI
├── sdk/
│   └── marine-sdk/     # Python SDK for strategies
├── schemas/
│   ├── protobuf/       # Service contract definitions
│   ├── openapi/        # REST API specs
│   └── events/         # Vox event schemas
├── infra/
│   ├── terraform/      # Infrastructure as code
│   ├── helm/           # Kubernetes charts
│   └── docker/         # Shared Docker configs
├── docs/
│   ├── wireframes/     # UI designs
│   ├── runbooks/       # Operations guides
│   └── adr/            # Architecture decision records
├── docker-compose.yml  # Local dev stack
└── Makefile            # Build & operations commands
```
