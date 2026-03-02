# Vox Event Schemas

Standard event definitions for the Astartes Primaris event bus.

## Channel Hierarchy

| Channel Pattern                           | Publisher | Subscribers              |
|-------------------------------------------|-----------|--------------------------|
| `market.{symbol}.{timeframe}`             | Auspex    | Tacticarium, Forge, Aurum |
| `signal.{fortress}.{company}.{marine}`    | Marine    | Logis, Aurum, Apothecary |
| `order.{status}`                          | Logis     | Marine, Aurum, Apothecary |
| `position.{fortress}.{company}`           | Logis     | Aurum, Apothecary        |
| `system.{service}.{event}`               | Any       | Apothecary, Aurum        |
| `forge.{job_id}.{status}`                | Forge     | Aurum, Primarch          |
| `config.{scope}.{key}`                   | Codex     | All services             |
| `lifecycle.{marine_id}.{phase}`          | Marine    | Primarch, Apothecary     |

## Event Guarantees

- **At-least-once delivery** via NATS JetStream
- **Ordering** guaranteed per subject
- **Replay** available for last 7 days (90 days for orders/positions)
- **Schema versioning** via protobuf (backward compatible changes only)
