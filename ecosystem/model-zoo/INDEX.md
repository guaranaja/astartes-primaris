# Astartes Model Zoo

All ONNX models shared across the Astartes ecosystem.
Models are organized by asset class. Cross-asset models are trained on data from multiple fortresses.

## Registry

| Model | Asset Class | Version | Input | Output | Accuracy | Trained On | Path |
|-------|------------|---------|-------|--------|----------|------------|------|
| es_regime_v3 | futures/ES | 3.0 | 30s bars (20 features) | TREND/RANGE/CHAOS | 72% | 2024-01–2025-12 | futures/es_regime_v3.onnx |
| es_exit_timing_v2 | futures/ES | 2.0 | position features | hold/exit probability | 68% | 2024-06–2025-12 | futures/es_exit_timing_v2.onnx |
| es_slippage_v1 | futures/ES | 1.0 | order context (8 features) | predicted ticks | MAE 0.8 | 2024-01–2025-12 | futures/es_slippage_v1.onnx |
| es_confidence_cal_v2 | futures/ES | 2.0 | signal features | calibrated confidence | 0.71 Brier | 2024-06–2025-12 | futures/es_confidence_cal_v2.onnx |

## Adding a Model

1. Train and validate the model
2. Export to ONNX format
3. Place in the appropriate asset class directory
4. Add an entry to this INDEX with all fields
5. Tag the commit with `# ECOSYSTEM: model-zoo` for cross-pollination

## Cross-Asset Models

When a model concept proves valuable in one asset class (e.g., regime detection in futures),
train a universal version on multi-asset data and place it in `cross-asset/`.
