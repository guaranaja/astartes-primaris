# AUSPEX — Market Data Collection

> Redundant collectors that feed the Librarium. Always watching.

## Architecture

```
IB Gateway / TWS
       │
       ▼
   ┌─────────┐      ┌─────────────────┐      ┌──────────┐
   │  IBKR   │─────▶│  Librarium      │      │   Vox    │
   │ ib_insync│      │  (TimescaleDB)  │      │  (NATS)  │
   └─────────┘      │  market_bars    │      │          │
       │            │  market_ticks   │      │ market.> │
       └───────────▶│                 │──────▶│          │
                    └─────────────────┘      └──────────┘
```

## Data Flow

1. **Connect** to IBKR via `ib_insync` (TWS API)
2. **Resolve** contracts for configured symbols (ES, NQ, MES, MNQ, etc.)
3. **Backfill** historical OHLCV bars (configurable days)
4. **Stream** real-time 5-second bars and bid/ask/trade ticks
5. **Write** to TimescaleDB `market_bars` + `market_ticks` hypertables (batched)
6. **Publish** to NATS JetStream on `market.{symbol}.bar.{tf}` and `market.{symbol}.tick`

## Prerequisites

- **IB Gateway** or **TWS** running and accepting API connections
- API settings: enable Socket Clients, set trusted IP, note the port
  - Live: port 4001 (TWS) or 4002 (Gateway)
  - Paper: port 7497 (TWS) or 7496 (Gateway)

## Configuration (Environment Variables)

### IBKR Connection
| Variable | Default | Description |
|----------|---------|-------------|
| `IBKR_HOST` | `127.0.0.1` | TWS/Gateway host |
| `IBKR_PORT` | `4002` | TWS/Gateway port |
| `IBKR_CLIENT_ID` | `10` | API client ID |
| `IBKR_READONLY` | `true` | Read-only mode (no order submission) |
| `IBKR_ACCOUNT` | | Account ID (optional, for multi-account) |

### Data Collection
| Variable | Default | Description |
|----------|---------|-------------|
| `AUSPEX_SYMBOLS` | `ES:CME:FUT:USD,...` | Comma-separated symbol specs |
| `AUSPEX_BAR_TIMEFRAMES` | `1 min,5 mins,...` | Bar sizes to backfill |
| `AUSPEX_BACKFILL_DAYS` | `5` | Days of history to fetch on startup |
| `AUSPEX_STREAM_TICKS` | `true` | Enable real-time tick streaming |
| `AUSPEX_STREAM_BARS` | `true` | Enable real-time 5s bar streaming |
| `AUSPEX_SOURCE` | `ibkr` | Source identifier for dedup |

### Librarium (TimescaleDB)
| Variable | Default | Description |
|----------|---------|-------------|
| `LIBRARIUM_HOST` | `127.0.0.1` | TimescaleDB host |
| `LIBRARIUM_PORT` | `5432` | TimescaleDB port |
| `LIBRARIUM_DB` | `librarium` | Database name |
| `LIBRARIUM_USER` | `librarium` | Database user |
| `LIBRARIUM_PASSWORD` | `dev_password` | Database password |
| `LIBRARIUM_BATCH_SIZE` | `100` | Rows buffered before flush |
| `LIBRARIUM_FLUSH_INTERVAL` | `2.0` | Seconds between flush cycles |

### Vox (NATS)
| Variable | Default | Description |
|----------|---------|-------------|
| `VOX_URL` | `nats://127.0.0.1:4222` | NATS server URL |
| `VOX_ENABLED` | `true` | Enable/disable Vox publishing |

## Symbol Format

Symbols use `SYMBOL:EXCHANGE:SECTYPE:CURRENCY` format:

```
ES:CME:FUT:USD       # E-mini S&P 500 futures
NQ:CME:FUT:USD       # E-mini Nasdaq futures
MES:CME:FUT:USD      # Micro E-mini S&P futures
MNQ:CME:FUT:USD      # Micro E-mini Nasdaq futures
AAPL:SMART:STK:USD   # Apple stock
SPY:SMART:STK:USD    # SPY ETF
```

## Running

```bash
# Via docker-compose (recommended)
make up                  # Starts full stack including Auspex
make auspex-logs         # Tail Auspex logs
make auspex-health       # Check health endpoint
make auspex-metrics      # View collection stats

# Standalone (requires Librarium + Vox running)
cd services/auspex
pip install -r requirements.txt
python -m auspex
```

## Health & Metrics

| Endpoint | Description |
|----------|-------------|
| `GET /health` | Readiness check (IBKR + Librarium + Vox status) |
| `GET /metrics` | Collection stats (bars/ticks written, buffer size) |

## Ports

| Port  | Protocol | Purpose              |
|-------|----------|----------------------|
| 8300  | HTTP     | Health / Metrics     |
