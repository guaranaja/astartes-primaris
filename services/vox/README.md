# VOX — Event Bus & Messaging

> The nervous system connecting all services.

## Tech

NATS with JetStream for persistent event streaming.

## Channel Taxonomy

```
market.{asset}.{timeframe}              — Real-time market data events
signal.{fortress}.{company}.{marine}    — Strategy signals
order.{status}                          — Order lifecycle (submitted, filled, rejected)
position.{fortress}.{company}           — Position updates
system.{service}.{event}               — Health, lifecycle, alerts
forge.{job_id}.{status}                — Backtest job progress
config.{scope}.{key}                   — Configuration change notifications
```

## Message Format

All messages are Protocol Buffers with a standard envelope:

```protobuf
message VoxEnvelope {
  string id = 1;
  string subject = 2;
  google.protobuf.Timestamp timestamp = 3;
  string source_service = 4;
  bytes payload = 5;
}
```

## Persistence

- JetStream stores last 7 days of all events
- Critical channels (orders, positions) retained for 90 days
- Replay capability for debugging and audit

## Ports

| Port  | Protocol | Purpose              |
|-------|----------|----------------------|
| 4222  | NATS     | Client connections   |
| 6222  | NATS     | Cluster routing      |
| 8222  | HTTP     | Monitoring           |
