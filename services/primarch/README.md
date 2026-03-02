# PRIMARCH — Control Plane & Orchestrator

> The brain of the Imperium. Manages the lifecycle of all services.

## Responsibilities

- Service registry (Codex integration)
- Strategy scheduling (wake/sleep cycles for Marines)
- Resource allocation across Fortresses, Companies, and Marines
- Unified dashboard API for Aurum
- Deployment and rollback management

## Tech

- **Language**: Go
- **API**: gRPC + REST gateway
- **Scheduling**: Cron-like + event-driven triggers
- **HA**: Leader election via Raft consensus

## Ports

| Port  | Protocol | Purpose          |
|-------|----------|------------------|
| 8400  | gRPC     | Service-to-service |
| 8401  | HTTP     | REST API / Dashboard |
| 8402  | HTTP     | Health / Metrics |

## Environment Variables

```
PRIMARCH_DB_URL          — PostgreSQL connection string
PRIMARCH_NATS_URL        — Vox (NATS) connection
PRIMARCH_VAULT_ADDR      — Iron Halo (Vault) address
PRIMARCH_LOG_LEVEL       — debug | info | warn | error
```
