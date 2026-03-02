# Marine SDK

Python SDK that every strategy imports to become a Marine in the Imperium.

## Install

```bash
pip install marine-sdk
```

## Quick Start

```python
from marine_sdk import Marine, Context, Signal, SignalType

class ESMomentum(Marine):
    """ES Futures momentum strategy."""

    name = "es-momentum"
    version = "1.0.0"

    def orient(self, ctx: Context) -> None:
        """Gather market data."""
        self.bars = ctx.librarium.get_bars("ES", "1min", limit=100)
        self.position = ctx.logis.get_position("ES")

    def decide(self, ctx: Context) -> list[Signal]:
        """Generate trading signals."""
        momentum = self.bars.close.pct_change(20).iloc[-1]

        if momentum > 0.01 and self.position.quantity == 0:
            return [Signal(SignalType.BUY, "ES", quantity=1)]
        elif momentum < -0.01 and self.position.quantity > 0:
            return [Signal(SignalType.SELL, "ES", quantity=1)]

        return []

    def report(self, ctx: Context) -> None:
        """Log metrics."""
        ctx.metrics.gauge("momentum", momentum)
```

## What the SDK Provides

| Component          | Description                                  |
|--------------------|----------------------------------------------|
| `Marine` base      | Abstract class with lifecycle hooks          |
| `Context`          | Injected connections to Librarium, Logis, Vox |
| `Signal`           | Standardized trading signal format           |
| `Runner`           | Entrypoint that manages wake/sleep lifecycle |
| `Metrics`          | Prometheus metrics helpers                   |
| `Config`           | Auto-loads from Codex at wake time           |

## Lifecycle Hooks

```python
class Marine:
    def on_wake(self, ctx): ...     # Called when container starts
    def orient(self, ctx): ...      # Gather data
    def decide(self, ctx): ...      # Generate signals
    def on_act(self, ctx, signals): ...  # Called after order submission
    def report(self, ctx): ...      # Log results
    def on_sleep(self, ctx): ...    # Called before shutdown
    def on_error(self, ctx, error): ... # Called on any exception
```
