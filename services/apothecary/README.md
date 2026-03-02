# APOTHECARY — Monitoring & Observability

> Keeps the Imperium healthy. Detects and recovers from failure.

## Stack

- **Metrics**: Prometheus (scrapes all services)
- **Dashboards**: Grafana (AAA quality dashboards)
- **Logs**: Loki (centralized log aggregation)
- **Traces**: Jaeger (distributed tracing)
- **Alerts**: Alertmanager → Slack / PagerDuty / SMS

## Dashboard Panels

| Dashboard           | Metrics                                     |
|---------------------|---------------------------------------------|
| Imperium Overview   | All services health, total P&L, active Marines |
| Fortress Detail     | Per-asset-class P&L, positions, risk usage  |
| Marine Performance  | Individual strategy P&L, signals, execution |
| Data Health         | Feed latency, gaps, staleness               |
| Infrastructure      | CPU, memory, container lifecycle            |

## Alert Rules

| Alert                    | Severity | Condition                        |
|--------------------------|----------|----------------------------------|
| Marine Execution Failure | Critical | Strategy container crash         |
| Data Feed Stale          | High     | No new data for >60s             |
| Daily Loss Limit         | Critical | Company loss exceeds threshold   |
| Service Down             | Critical | Health check fails 3x            |
| High Latency             | Warning  | Order execution >5s              |
| Disk Usage               | Warning  | >80% on any Librarium node       |

## Ports

| Port  | Protocol | Purpose              |
|-------|----------|----------------------|
| 9090  | HTTP     | Prometheus           |
| 3000  | HTTP     | Grafana              |
| 3100  | HTTP     | Loki                 |
| 16686 | HTTP     | Jaeger UI            |
