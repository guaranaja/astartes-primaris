# CODEX — Configuration & Rules Engine

> The sacred text that governs all behavior.

## Responsibilities

- Centralized configuration for all services and strategies
- Strategy parameters (per Marine)
- Risk limits (per Company / Fortress)
- Scheduling rules and feature flags
- Kill switches for emergency shutdown

## Configuration Hierarchy

```
imperium/
├── defaults/                    — Global defaults
│   ├── risk.yaml
│   └── scheduling.yaml
├── fortress/
│   ├── primus/                  — Futures config
│   │   ├── risk.yaml
│   │   └── companies/
│   │       ├── first/
│   │       │   └── marines/
│   │       │       ├── alpha-1.yaml
│   │       │       └── alpha-2.yaml
│   │       └── scout/
│   └── secundus/                — Options config
└── system/
    ├── services.yaml
    └── feature-flags.yaml
```

## Config Inheritance

Marine config inherits: `defaults → fortress → company → marine`

Each level can override the parent. Most specific wins.

## Change Propagation

Config changes emit events on Vox channel `config.{scope}.{key}`.
Marines pick up changes at their next wake cycle.
Critical changes (kill switch) are pushed immediately via Vox.

## Tech

- **Backend**: etcd (distributed, consistent)
- **API**: REST + gRPC
- **Format**: YAML (human-editable) stored as structured data

## Ports

| Port  | Protocol | Purpose              |
|-------|----------|----------------------|
| 8700  | gRPC     | Service config API   |
| 8701  | HTTP     | REST API / Aurum     |
| 8702  | HTTP     | Health / Metrics     |
