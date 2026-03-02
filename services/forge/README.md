# FORGE — Backtest & Optimization Engine

> Ephemeral compute that fires up, crunches numbers, and shuts down.

## Capabilities

- **Backtest**: Run a strategy against historical data with full simulation
- **Optimization**: Parameter sweep across N dimensions
- **Walk-Forward**: Rolling window optimization + out-of-sample testing
- **Monte Carlo**: Stress testing with randomized execution

## Job Types

| Job Type          | Workers | Duration     | Output                    |
|-------------------|---------|-------------|---------------------------|
| Single Backtest   | 1       | Seconds-min | Performance report         |
| Parameter Sweep   | N       | Minutes-hrs | Heatmap + optimal params   |
| Walk-Forward      | N       | Minutes-hrs | Robustness analysis        |
| Monte Carlo       | N       | Minutes     | Risk distribution          |

## Architecture

```
  Primarch / Aurum
       │
       ▼ (submit job)
  ┌─────────────┐
  │ Forge Master │ ─── Tracks jobs, allocates workers
  └──────┬──────┘
         │ spawn
    ┌────┼────┐
    ▼    ▼    ▼
  [W1] [W2] [W3]  ─── Ephemeral worker containers
    │    │    │
    └────┼────┘
         ▼
    Librarium (results)
```

## Worker Container Specs

- **Base Image**: `astartes/forge-worker:latest`
- **Memory**: 1GB default (configurable)
- **CPU**: 1 vCPU default (configurable)
- **Max Lifetime**: 2 hours (kill switch)
- **Auto-shutdown**: On job completion

## Ports

| Port  | Protocol | Purpose              |
|-------|----------|----------------------|
| 8500  | gRPC     | Job submission       |
| 8501  | HTTP     | Job status / results |
| 8502  | HTTP     | Health / Metrics     |
