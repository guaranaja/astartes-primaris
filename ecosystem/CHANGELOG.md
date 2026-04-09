# Astartes Ecosystem — Cross-Pollination Changelog

Track improvements that should propagate across repos.
The ecosystem sync agent reads this to identify unpropagated changes.

## Format

```
## [date] [origin-repo] — [pattern-name]
**What changed:** Description
**Why:** Motivation
**Propagate to:** list of repos
**Status:** pending | propagated
```

---

## 2026-04-08 astartes-primaris — sync-contract
**What changed:** Created typed sync contract dataclasses (sync_contract.py)
**Why:** Slippage bug in astartes-futures caused by raw dict returns. All engine→Primaris
communication must use typed dataclasses to prevent silent field drops.
**Propagate to:** astartes-futures, astartes-equities, astartes-futures-client, astartes-equities-client
**Status:** pending

## 2026-04-08 astartes-primaris — multi-asset-schema
**What changed:** Added 007_multi_asset.sql — instrument_type, options fields, Greeks on positions,
wheel_cycles/wheel_legs tables, payout_allocations, portfolio views.
**Why:** Enable equities + options trading alongside futures in the Imperium.
**Propagate to:** astartes-equities (will use from day one)
**Status:** pending

## 2026-04-08 astartes-primaris — model-zoo
**What changed:** Initialized Model Zoo with INDEX.md and directory structure.
**Why:** Shared ONNX model library across asset classes.
**Propagate to:** astartes-futures (copy existing models here), astartes-equities (consume models)
**Status:** pending
