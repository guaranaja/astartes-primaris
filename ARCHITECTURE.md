# ASTARTES PRIMARIS — Cloud-Native Trading Platform Architecture

> *"They shall be my finest warriors... and they shall know no fear."*

## Vision

Astartes Primaris is a **cloud-first constellation** of services and containers designed to
trade across multiple asset classes, accounts, and strategies — all scaling and instantiating
on demand. Each component operates independently but gains superpowers when connected to the
ecosystem.

---

## Naming Convention & Hierarchy

The platform uses Warhammer 40K Adeptus Astartes organizational structure as its taxonomy.
This isn't just branding — the hierarchy maps directly to operational scaling tiers.

```
IMPERIUM (Platform)
├── FORTRESS MONASTERY (Environment: prod / staging / dev)
│   ├── FORTRESS (Datacenter = Asset Class)
│   │   ├── Fortress Primus    — Futures (ES, NQ, CL, etc.)
│   │   ├── Fortress Secundus  — Options
│   │   ├── Fortress Tertius   — Long-Term Equities
│   │   └── Fortress Quartus   — Crypto / Alt
│   │
│   ├── COMPANY (Cluster = Account Group / Sub-strategy Type)
│   │   ├── 1st Company (Veterans)  — Primary prop accounts
│   │   ├── 2nd Company (Battle)    — Secondary / scaling accounts
│   │   ├── 3rd Company (Reserve)   — Paper / staging accounts
│   │   └── Scout Company           — Experimental strategies
│   │
│   └── MARINE (VM = Individual Strategy Instance)
│       ├── Battle Brother Alpha-1  — ES Momentum @ Apex Account #1
│       ├── Battle Brother Alpha-2  — ES Momentum @ Apex Account #2
│       └── ...up to N instances
```

### Mapping Summary

| WH40K Term          | Cloud Equivalent | Trading Equivalent              |
|----------------------|------------------|---------------------------------|
| Imperium             | Platform         | Astartes Primaris (entire system) |
| Fortress Monastery   | Environment      | prod / staging / dev            |
| Fortress             | Datacenter       | Asset Class (Futures, Options, Equities) |
| Company              | Cluster          | Account Group / Strategy Family |
| Marine               | VM / Container   | Individual Strategy Instance    |
| Primarch             | Control Plane    | Platform Orchestrator           |
| Chapter Master       | Scheduler        | Strategy Lifecycle Manager      |
| Librarium            | Database Layer   | Persistent Market + Trade Data  |
| Auspex               | Data Ingestion   | Market Data Collectors          |
| Forge                | Compute Pool     | Backtest & Optimization Engine  |
| Codex                | Service Registry | Configuration & Rules Engine    |
| Vox                  | Message Bus      | Inter-service Event Stream      |
| Apothecary           | Health Checks    | Monitoring & Recovery           |
| Iron Halo            | Auth / Security  | Authentication & Secrets Vault  |

---

## Core Services

### 1. PRIMARCH — Control Plane & Orchestrator

The brain of the operation. Manages the lifecycle of all services, similar to vCenter.

- **Role**: Central management plane, service orchestration, scheduling
- **Tech**: Go or Rust service + gRPC API
- **Responsibilities**:
  - Register/deregister Fortresses, Companies, and Marines
  - Schedule strategy wake/sleep cycles (cron-like or event-driven)
  - Monitor resource allocation across the constellation
  - Provide the unified dashboard API
  - Manage deployments and rollbacks
- **Scaling**: Single leader with hot standby (Raft consensus)

```
┌─────────────────────────────────────────────┐
│              PRIMARCH (Control Plane)        │
│  ┌─────────┐  ┌──────────┐  ┌───────────┐  │
│  │Scheduler│  │ Registry │  │ Dashboard │  │
│  │ Engine  │  │ (Codex)  │  │    API    │  │
│  └────┬────┘  └────┬─────┘  └─────┬─────┘  │
│       └─────────┬──┘──────────────┘         │
│            ┌────┴─────┐                     │
│            │ gRPC Hub │                     │
│            └────┬─────┘                     │
└─────────────────┼───────────────────────────┘
                  │
        ┌─────────┼─────────┐
        ▼         ▼         ▼
   [Fortress] [Fortress] [Fortress]
```

### 2. LIBRARIUM — Persistent Data Layer

The memory of the Imperium. All market data, trade history, strategy state, and analytics
flow through here.

- **Role**: Persistent storage for time-series, relational, and document data
- **Tech**:
  - **TimescaleDB** (PostgreSQL extension) — OHLCV, tick data, time-series
  - **PostgreSQL** — Account state, positions, configuration, audit logs
  - **Redis** — Hot cache, real-time state, pub/sub for live data
- **Redundancy**: Primary + 2 replicas (synchronous replication)
- **Scaling**: Read replicas for analytics, write-ahead log shipping

```
┌────────────────────────────────────────┐
│            LIBRARIUM                   │
│  ┌──────────────┐  ┌───────────────┐  │
│  │ TimescaleDB  │  │  PostgreSQL   │  │
│  │ (Time-Series)│  │  (Relational) │  │
│  └──────┬───────┘  └──────┬────────┘  │
│         └──────┬───────────┘           │
│         ┌──────┴───────┐               │
│         │    Redis     │               │
│         │  (Hot Cache) │               │
│         └──────────────┘               │
│  Replicas: Primary + 2 Standby        │
└────────────────────────────────────────┘
```

### 3. AUSPEX — Market Data Collection

Redundant data collectors that feed the Librarium. Always on, always watching.

- **Role**: Ingest market data from multiple sources
- **Tech**: Python or Rust microservices
- **Sources**: Broker APIs (IBKR, TastyTrade, etc.), exchange feeds, third-party data
- **Pattern**: Multiple redundant collectors per data source
- **Output**: Normalized data → Librarium (TimescaleDB) + Vox (event stream)
- **Scaling**: Horizontal — spin up additional collectors per source

### 4. TACTICARIUM — Strategy Runtime Engine

This is where Marines live. Strategies are **ephemeral microservices** that wake on schedule,
execute their logic, and go dormant.

- **Role**: Execute trading strategies on demand
- **Tech**: Python containers (or any language) with standardized interface
- **Lifecycle**:
  1. **WAKE** — Container spins up on schedule (30s, 1min, 5min, etc.)
  2. **ORIENT** — Pull latest market data from Librarium/Redis
  3. **DECIDE** — Run strategy logic, generate signals
  4. **ACT** — Submit orders via Logis (execution service)
  5. **REPORT** — Log results to Librarium, emit events to Vox
  6. **SLEEP** — Container shuts down, resources freed

```
    Schedule Trigger (Primarch)
            │
            ▼
    ┌───────────────┐
    │  TACTICARIUM   │
    │  ┌───────────┐ │      ┌──────────┐
    │  │  Marine    │─┼─────▶│ Librarium│  (read data)
    │  │ Container  │ │      └──────────┘
    │  │           │─┼─────▶┌──────────┐
    │  │ wake →    │ │      │  Logis   │  (submit orders)
    │  │ orient →  │ │      └──────────┘
    │  │ decide →  │─┼─────▶┌──────────┐
    │  │ act →     │ │      │   Vox    │  (emit events)
    │  │ report →  │ │      └──────────┘
    │  │ sleep     │ │
    │  └───────────┘ │
    └───────────────┘
         │
         ▼
    Container stops (resources freed)
```

- **Scaling**: Each Marine is an independent container instance
- **Isolation**: Strategies share nothing — crash isolation guaranteed
- **Config**: Each Marine reads its config from Codex at wake time

### 5. FORGE — Backtest & Optimization Engine

Ephemeral compute that fires up, crunches numbers, and shuts down.

- **Role**: Run backtests, parameter optimization, walk-forward analysis
- **Tech**: Python + NumPy/Pandas compute containers
- **Pattern**: Job-based — submit a job, Forge spins up workers, results land in Librarium
- **Scaling**: Massively parallel — spin up N workers for parameter sweeps
- **Lifecycle**: Fire up → compute → report → shut down
- **Output**: Performance reports, optimized parameters → Librarium + Primarch dashboard

### 6. VOX — Event Bus & Messaging

The nervous system connecting all services.

- **Role**: Async inter-service communication, event streaming
- **Tech**: **NATS** (lightweight, cloud-native) or **Redis Streams**
- **Channels**:
  - `market.{asset}.{timeframe}` — Real-time market data events
  - `signal.{fortress}.{company}.{marine}` — Strategy signals
  - `order.{status}` — Order lifecycle events
  - `system.{service}.{event}` — Health, alerts, lifecycle events
- **Scaling**: NATS cluster with JetStream for persistence

### 7. LOGIS — Execution & Position Management

The hands that pull the trigger.

- **Role**: Order routing, position tracking, account management
- **Tech**: Go or Python service
- **Responsibilities**:
  - Route orders to correct broker/account
  - Track positions per Marine, Company, and Fortress
  - Enforce risk limits (max position, daily loss, etc.)
  - Reconcile positions with broker state
- **Integrations**: IBKR API, TastyTrade API, broker FIX connections

### 8. CODEX — Configuration & Rules Engine

The sacred text that governs all behavior.

- **Role**: Centralized configuration, feature flags, strategy parameters
- **Tech**: etcd or Consul-backed config service
- **Stores**:
  - Strategy parameters (per Marine)
  - Risk limits (per Company / Fortress)
  - Scheduling rules
  - Feature flags and kill switches
- **Pattern**: Config changes emit events on Vox — Marines pick up changes at next wake

### 9. IRON HALO — Authentication & Security

- **Role**: Service-to-service auth, API key management, secrets vault
- **Tech**: HashiCorp Vault + mutual TLS
- **Responsibilities**:
  - Broker API key storage and rotation
  - Service identity and mTLS certificates
  - User authentication for dashboard
  - Audit logging of all access

### 10. APOTHECARY — Monitoring & Observability

- **Role**: Health monitoring, alerting, recovery
- **Tech**: Prometheus + Grafana + custom health checks
- **Monitors**:
  - Service health and uptime
  - Strategy P&L in real-time
  - Data freshness (stale data detection)
  - Resource utilization
  - Anomaly detection on strategy behavior
- **Alerts**: PagerDuty / Slack / SMS integration

### 11. AURUM — Interface Layer

AAA quality frontends for commanding the Imperium.

- **Role**: Web dashboard, CLI tools, mobile alerts
- **Tech**: Next.js + TypeScript (web), CLI in Go
- **Views**:
  - **Command Throne** — Overview of entire Imperium (all Fortresses, Companies, Marines)
  - **Tactical Display** — Real-time strategy execution view
  - **Forge Console** — Backtest submission and results
  - **Librarium Browser** — Data exploration and charting
  - **Codex Editor** — Configuration management UI
- **Quality**: Dark theme, real-time WebSocket updates, sub-100ms interactions

---

## System Architecture Diagram

```
                        ┌─────────────────────────────────┐
                        │          AURUM (UI)              │
                        │  Command Throne │ Forge Console  │
                        └────────┬────────────────────────┘
                                 │ HTTPS / WebSocket
                        ┌────────┴────────────────────────┐
                        │      PRIMARCH (Control Plane)    │
                        │  Scheduler │ Registry │ API      │
                        └──┬─────┬──────┬──────┬──────┬───┘
                           │     │      │      │      │
              ┌────────────┘     │      │      │      └──────────────┐
              │                  │      │      │                     │
              ▼                  ▼      │      ▼                     ▼
   ┌──────────────────┐  ┌──────────┐  │  ┌────────┐    ┌───────────────┐
   │    TACTICARIUM    │  │   FORGE  │  │  │  LOGIS │    │  APOTHECARY   │
   │ ┌──────┐┌──────┐ │  │ Workers  │  │  │ Orders │    │  Monitoring   │
   │ │Marine││Marine│ │  │ Backtest │  │  │ Positions│   │  Alerting     │
   │ │  #1  ││  #2  │ │  │ Optimize │  │  │ Risk   │    └───────────────┘
   │ └──┬───┘└──┬───┘ │  └────┬─────┘  │  └───┬────┘
   └────┼───────┼─────┘       │        │      │
        │       │              │        │      │
        └───────┴──────────────┴────┬───┘──────┘
                                    │
                        ┌───────────┴──────────────┐
                        │        VOX (Event Bus)    │
                        │   NATS / Redis Streams    │
                        └───────────┬──────────────┘
                                    │
                   ┌────────────────┼────────────────┐
                   │                │                 │
                   ▼                ▼                 ▼
          ┌──────────────┐  ┌─────────────┐  ┌──────────────┐
          │   AUSPEX     │  │  LIBRARIUM  │  │  IRON HALO   │
          │ Data Collect │  │ TimescaleDB │  │   Security   │
          │  Redundant   │  │ PostgreSQL  │  │    Vault     │
          │  Collectors  │  │   Redis     │  │    mTLS      │
          └──────────────┘  └─────────────┘  └──────────────┘
                               ▲
                        ┌──────┴───────┐
                        │    CODEX     │
                        │   Config     │
                        │   Registry   │
                        └──────────────┘
```

---

## Marine Lifecycle (Strategy Container)

The core innovation: strategies as **ephemeral, scheduled microservices**.

```
                    ┌─────────────────────────┐
                    │     Primarch Scheduler   │
                    │  "Wake Marine Alpha-1"   │
                    │  Schedule: every 30s     │
                    └───────────┬─────────────┘
                                │
                    ┌───────────▼─────────────┐
                    │  1. CONTAINER SPINS UP   │
                    │     Pull image if needed │
                    │     Load config (Codex)  │
                    └───────────┬─────────────┘
                                │
                    ┌───────────▼─────────────┐
                    │  2. ORIENT               │
                    │     Query Librarium      │
                    │     Get latest bars      │
                    │     Check positions      │
                    └───────────┬─────────────┘
                                │
                    ┌───────────▼─────────────┐
                    │  3. DECIDE               │
                    │     Run strategy logic   │
                    │     Generate signals     │
                    └───────────┬─────────────┘
                                │
                    ┌───────────▼─────────────┐
                    │  4. ACT                  │
                    │     Submit via Logis     │
                    │     Wait for confirm     │
                    └───────────┬─────────────┘
                                │
                    ┌───────────▼─────────────┐
                    │  5. REPORT               │
                    │     Log to Librarium     │
                    │     Emit Vox events      │
                    │     Update P&L           │
                    └───────────┬─────────────┘
                                │
                    ┌───────────▼─────────────┐
                    │  6. SLEEP                │
                    │     Container stops      │
                    │     Resources freed      │
                    │     State persisted      │
                    └─────────────────────────┘
```

### Why Ephemeral?

- **Cost**: Only pay for compute when strategies are actually running
- **Isolation**: One strategy crash doesn't affect others
- **Scaling**: Run 10 accounts × 5 strategies = 50 Marines, each independent
- **Updates**: Deploy new strategy version without touching others
- **Testing**: Same container runs in paper (Scout Company) and live (1st Company)

---

## Account Topology (Initial Plan)

```
FORTRESS PRIMUS (Futures)
├── 1st Company — Prop Accounts (Live)
│   ├── Marine Alpha-1   (ES Momentum   @ Apex Acct #1)
│   ├── Marine Alpha-2   (ES Momentum   @ Apex Acct #2)
│   ├── Marine Beta-1    (ES Mean Rev   @ Apex Acct #3)
│   ├── Marine Gamma-1   (NQ Breakout   @ Apex Acct #4)
│   └── ... (10 prop futures accounts)
├── Scout Company — Paper Trading
│   ├── Scout Alpha-1    (ES Momentum   @ Paper)
│   └── Scout Beta-1     (ES Mean Rev   @ Paper)

FORTRESS SECUNDUS (Options)
├── 1st Company — Options Accounts
│   ├── Marine Delta-1   (SPX Spreads   @ TastyTrade)
│   ├── Marine Delta-2   (Wheel Strat   @ TastyTrade)
│   └── ...

FORTRESS TERTIUS (Long-Term Equities)
├── 1st Company — Investment Accounts
│   ├── Marine Epsilon-1 (Value Picks   @ IBKR)
│   ├── Marine Epsilon-2 (Growth Port   @ IBKR)
│   └── ...
```

---

## Tech Stack Summary

| Layer              | Technology                         | Why                                    |
|--------------------|------------------------------------|----------------------------------------|
| Orchestration      | Docker + Kubernetes (or Nomad)     | Container scheduling, auto-scaling     |
| Control Plane      | Go / Rust + gRPC                   | Performance, type safety               |
| Strategy Runtime   | Python (containerized)             | Ecosystem, pandas/numpy, fast iteration|
| Database           | TimescaleDB + PostgreSQL + Redis   | Time-series + relational + cache       |
| Event Bus          | NATS JetStream                     | Lightweight, cloud-native, persistent  |
| Config Store       | etcd                               | Distributed, consistent, Kubernetes-native |
| Security           | HashiCorp Vault + mTLS             | Industry standard secrets management   |
| Monitoring         | Prometheus + Grafana               | Battle-tested observability stack      |
| Frontend           | Next.js + TypeScript               | SSR, real-time, AAA quality            |
| CLI                | Go                                 | Fast, single binary, cross-platform    |
| IaC                | Terraform + Helm                   | Reproducible infrastructure            |
| CI/CD              | GitHub Actions                     | Native to repo                         |

---

## Design Principles

1. **Independence First** — Every service runs standalone. The ecosystem is an accelerator, not a dependency.
2. **Ephemeral by Default** — If it can be a short-lived container, it should be.
3. **Redundancy Always** — No single point of failure for data or execution.
4. **Config Over Code** — Strategy behavior changes via Codex, not redeployment.
5. **Observable Everything** — If it's not monitored, it doesn't exist.
6. **Scale to Zero** — Services that aren't needed right now shouldn't cost anything.
7. **AAA Interfaces** — The dashboard should feel like a Bloomberg terminal meets a modern SaaS app.

---

## Next Steps

- [ ] Define service contracts (protobuf / OpenAPI schemas)
- [ ] Scaffold service directories with Dockerfiles
- [ ] Set up Librarium schema (TimescaleDB + PostgreSQL migrations)
- [ ] Build Marine SDK (Python package strategies import to get superpowers)
- [ ] Implement Primarch scheduler (MVP: cron-based container orchestration)
- [ ] Create Aurum dashboard wireframes
- [ ] Set up CI/CD pipeline
- [ ] Define Vox event schemas
