# LIBRARIUM — Persistent Data Layer

> The memory of the Imperium. All market data, trade history, and strategy state lives here.

## Components

### TimescaleDB (Time-Series)
- OHLCV bars (1s through monthly)
- Tick data
- Strategy signals and indicators

### PostgreSQL (Relational)
- Account state and positions
- Strategy configuration history
- Audit logs
- User management

### Redis (Hot Cache)
- Real-time price snapshots
- Current positions per Marine
- Session state
- Pub/sub for live data distribution

## Redundancy

- Primary + 2 synchronous replicas
- WAL shipping for disaster recovery
- Automated failover via Patroni
- Read replicas for analytics workloads

## Ports

| Port  | Service      | Purpose              |
|-------|-------------|----------------------|
| 5432  | TimescaleDB | Time-series data     |
| 5433  | PostgreSQL  | Relational data      |
| 6379  | Redis       | Cache + pub/sub      |

## Backup Strategy

- Continuous WAL archiving to object storage
- Daily base backups with 30-day retention
- Point-in-time recovery capability
