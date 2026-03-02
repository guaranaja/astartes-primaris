# AUSPEX — Market Data Collection

> Redundant collectors that feed the Librarium. Always watching.

## Responsibilities

- Ingest real-time and historical market data from multiple sources
- Normalize data into standard OHLCV + tick formats
- Write to Librarium (TimescaleDB) and emit events on Vox
- Detect and handle data gaps, stale feeds, and source failovers

## Data Sources

| Source         | Type           | Assets                  |
|----------------|---------------|-------------------------|
| IBKR TWS API   | Real-time     | Futures, Options, Equities |
| TastyTrade API | Real-time     | Options, Equities       |
| Polygon.io     | Real-time     | Equities, Crypto        |
| Databento      | Historical    | Futures, Equities       |

## Redundancy Model

Each data source has **2+ collectors** running simultaneously.
Librarium deduplicates on `(symbol, timestamp, source)`.

```
  Source A ──▶ [Collector A1] ──▶ Librarium
           ──▶ [Collector A2] ──▶ Librarium (dedup)
```

## Ports

| Port  | Protocol | Purpose              |
|-------|----------|----------------------|
| 8300  | HTTP     | Health / Metrics     |
