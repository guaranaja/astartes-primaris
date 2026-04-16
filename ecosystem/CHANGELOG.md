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

## 2026-04-16 astartes-primaris — prop-firm-identity
**What changed:** Added `prop_firm` field to `TradingAccount` and `AccountSnapshot` identifying the funded-account vendor (topstep/apex/tpt/tradeday/mff). Added `PropFirmRegistry` with per-firm rules (profit split, consistency %, payout cycle days, default eval/activation/reset fees). New endpoint `GET /api/v1/council/prop-firms`. New `prop_fees` table + `POST /api/v1/council/prop-fees` that dual-writes a Firefly III withdrawal tagged `prop-fee`. New `POST /api/v1/council/accounts/{id}/mark-passed` for manual Rubicon (combine→fxt) transition, plus auto-detection in engine snapshot ingest that broadcasts `combine.passed` over SSE. Prop-account payouts now auto-record a `taxes` PayoutAllocation (default 30%, via `TRADING_TAX_WITHHOLD_PCT`).
**Why:** Combine 2/3 complete + second in flight — prop earnings need to hit the budget correctly. Different firms have different split/consistency rules; fees (eval/reset) were invisible to the budget; 1099 income needs tax reserve; Rubicon transition was manual-only.
**Propagate to:** astartes-futures (set `prop_firm` in `AccountSnapshot` when reporting to Primarch; today engines only pass `Broker`), astartes-futures-client (same), astartes-equities (n/a — no prop accounts).
**Status:** pending

## 2026-04-09 astartes-primaris — cash-value-demarcation
**What changed:** Added `AccountPhase` enum and `account_phase` field to `AccountSnapshot` in sync_contract.py. Engines must report whether accounts are combine/fxt/live/paper/blown so Primaris can distinguish real cash from sim capital. Default is "combine" (backward compatible). Primaris now excludes sim accounts from TradingValue, net worth, withdrawal advice, and payout eligibility.
**Why:** Prop desk aggregates were mixing combine capital (no cash value) with funded capital (real money), making financial metrics misleading. "Crossing the Rubicon" (combine→FXT) is now a first-class concept.
**Propagate to:** astartes-futures (set account_phase when account crosses to FXT), astartes-equities (set account_phase for paper vs live)
**Status:** pending

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
