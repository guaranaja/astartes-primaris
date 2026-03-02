# LOGIS — Execution & Position Management

> The hands that pull the trigger.

## Responsibilities

- Route orders to correct broker and account
- Track positions per Marine, Company, and Fortress
- Enforce risk limits before order submission
- Reconcile positions with broker state
- Emit order lifecycle events to Vox

## Risk Limits (enforced pre-order)

| Level     | Limit                  | Example                    |
|-----------|------------------------|----------------------------|
| Marine    | Max position size      | 2 ES contracts             |
| Marine    | Max daily loss         | -$500                      |
| Company   | Aggregate exposure     | 10 ES contracts total      |
| Fortress  | Total capital at risk  | $50,000                    |
| Imperium  | Kill switch            | Halt all trading           |

## Order Flow

```
Marine → Logis → Risk Check → Broker API → Confirm → Vox Event
                    │
                    ▼ (rejected)
              Vox Event + Marine notified
```

## Broker Integrations

| Broker       | Protocol    | Account Types           |
|-------------|-------------|-------------------------|
| IBKR        | TWS API     | Futures, Options, Equity|
| TastyTrade  | REST API    | Options, Equity         |
| Apex         | FIX         | Futures (prop accounts) |

## Ports

| Port  | Protocol | Purpose              |
|-------|----------|----------------------|
| 8600  | gRPC     | Order submission     |
| 8601  | HTTP     | REST API             |
| 8602  | HTTP     | Health / Metrics     |
