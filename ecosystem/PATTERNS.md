# Astartes Ecosystem — Shared Patterns

Living document tracking shared patterns across the ecosystem.
Canonical implementations live in `ecosystem/shared/`.
Asset-specific repos may have adapted versions, but core logic stays in sync.

## Pattern Index

| Pattern | Canonical File | Used By | Description |
|---------|---------------|---------|-------------|
| Regime Detection | `shared/regime_detector.py` | Eversor, Fenris, Stormchaser | TREND/RANGE/CHAOS classification from price bars |
| Confidence Scoring | `shared/confidence_scorer.py` | All strategies | Base + modifier confidence framework |
| Staged Trailing Stops | `shared/staged_stops.py` | All strategies | 4-stage trailing stop (0.75R, 1.5R, 2.0R, 3.0R) |
| Risk Gates | `shared/risk_gates.py` | All engines | Universal pre-trade risk checks |
| Indicator Library | `shared/indicator_lib.py` | All strategies | ATR, RSI, VWAP, EMA, ADX — computed identically |
| Market Timing | `shared/timing.py` | All engines | Market hours, sessions, calendars |
| Strategy Template | `templates/strategy_template.py` | All strategies | Base template with regime + confidence + stops |
| Broker Adapter | `templates/broker_adapter.py` | All engines | Abstract broker interface |
| Sync Contract | `shared/sync_contract.py` | All engines | Typed dataclasses for engine → Primaris sync |

## Tagging Convention

When modifying code that implements a shared pattern, tag it:
```python
# ECOSYSTEM: regime-detection
# ECOSYSTEM: confidence-scoring
# ECOSYSTEM: staged-stops
# ECOSYSTEM: sync-contract
```

This enables automated detection and cross-pollination.
