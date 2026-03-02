# TACTICARIUM — Strategy Runtime Engine

> Where Marines fight. Strategies are ephemeral containers that wake, execute, and sleep.

## Marine Lifecycle

```
WAKE → ORIENT → DECIDE → ACT → REPORT → SLEEP
```

1. **WAKE**: Container spins up on schedule trigger from Primarch
2. **ORIENT**: Pull latest market data from Librarium/Redis
3. **DECIDE**: Run strategy logic, generate trading signals
4. **ACT**: Submit orders via Logis execution service
5. **REPORT**: Log results to Librarium, emit events to Vox
6. **SLEEP**: Container shuts down, resources freed

## Marine Interface

Every strategy must implement the `Marine` interface from the SDK:

```python
from marine_sdk import Marine, Context, Signal

class MyStrategy(Marine):
    def orient(self, ctx: Context) -> None:
        """Gather data needed for decision."""
        ...

    def decide(self, ctx: Context) -> list[Signal]:
        """Generate trading signals."""
        ...

    def report(self, ctx: Context) -> None:
        """Log results and metrics."""
        ...
```

## Scheduling Modes

| Mode        | Description                              | Example          |
|-------------|------------------------------------------|------------------|
| `interval`  | Wake every N seconds/minutes             | Every 30s        |
| `cron`      | Standard cron expression                 | `*/5 * * * *`    |
| `event`     | Wake on Vox event (price alert, signal)  | On breakout      |
| `manual`    | Wake via Primarch API / dashboard        | On-demand        |

## Container Specs

- **Base Image**: `astartes/marine-base:latest` (Python 3.12 + SDK)
- **Memory**: 256MB default, configurable per Marine
- **CPU**: 0.25 vCPU default, configurable per Marine
- **Timeout**: 30s default execution timeout
- **Restart**: No auto-restart — next scheduled wake handles recovery

## Ports

| Port  | Protocol | Purpose              |
|-------|----------|----------------------|
| 9000  | HTTP     | Health check (while alive) |
