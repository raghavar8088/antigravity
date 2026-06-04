# Fill Quality Report — Phase 22D

**Date:** 2026-06-04

---

## Fill Quality Dimensions

| Dimension | Measurement Location | Status |
|-----------|---------------------|--------|
| Fill price vs. expected | `[SLIPPAGE]` log, `loop.go` post-fill | ✅ Logged |
| Fill delay (signal → fill) | Signal age in `[✅ TRADE EXECUTED]` line | ✅ Logged |
| Fill rate | Aggregator `RecordSignalFlowStage` Prometheus | ✅ Recorded |
| Partial fills | `FillResult.ExecQty` vs `sig.TargetSize` | ⚠️ Not yet checked |
| Rejected fills | `[EXECUTION FAILED]` log line | ✅ Logged |
| Latency per stage | `trading_latency_pipeline_stage_ms` histogram | ✅ Recorded |

---

## Fill Rate

Fill rate = signals that reach `executeThroughInstitutionalPath` and succeed / total approved signals.

Current funnel losses (per `RecordSignalFlowStage`):
1. `SignalStageGenerated` → all raw signals
2. `SignalStageCooldownFilter` → 30 s cooldown removes duplicates
3. `SignalStageAggregator` → dominance + score filter
4. `SignalStageRegimeFilter` → category ≠ regime
5. `SignalStageExecutionWeightFilter` → weight < 0.50
6. `SignalStageConfidenceFilter` → confidence/SL/TP validation
7. `SignalStageRiskFilter` → risk engine validation
8. `SignalStageExecution` → bridge parking + actual execution

Each stage's ratio (passed / total) is the fill rate for that gate. The Prometheus
metric `trading_aggregator_signal_flow_*` tracks this per stage.

---

## Fill Delay Measurement (Phase 22D)

Every executed trade now logs signal age at execution:
```
[✅ TRADE EXECUTED] BUY | BTC-USD 0.0100 BTC @ $68451.47 | Strategy: TripleFilter_Alpha_Scalp | Age: 23ms
```

**Target age bands:**

| Timeframe | Good | Acceptable | Stale (dropped) |
|-----------|------|------------|-----------------|
| tick | < 50 ms | 50–500 ms | > 500 ms |
| 1m | < 15 s | 15–90 s | > 90 s |
| 5m | < 60 s | 60–420 s | > 420 s |
| 15m | < 5 min | 5–20 min | > 20 min |

---

## Partial Fill Detection

`execution.FillResult` currently returns:
```go
type FillResult struct {
    ExecPrice     float64
    OrderMode     execution.OrderMode
    ClientOrderID string
}
```

`ExecQty` is not present. For paper trading this is irrelevant (always full fill).
For live trading, partial fill detection requires adding `ExecQty` to `FillResult`
and comparing against `sig.TargetSize` post-fill.

**Recommended next step:** Add `ExecQty float64` to `FillResult` and log:
```
[PARTIAL FILL] strategy filled 0.0072 / 0.0100 BTC (72%)
```

---

## Rejected Fill Rate

The `[EXECUTION FAILED]` log plus `RecordSignalFlowRejection(SignalStageExecution, err.Error())`
captures all execution failures. Typical rejection reasons:
- `"risk blocked"` — pre-trade risk gate hard limit hit
- `"size below minimum"` — order size < 0.01 BTC
- `"insufficient equity"` — paper account equity depleted

Target: rejected fills < 2% of approved signals.

---

## Paper vs Live Fill Quality

| Metric | Paper | Live (Expected) |
|--------|-------|-----------------|
| Slippage | 0 bps | 1–20 bps |
| Partial fills | Never | 5–15% of large orders |
| Latency (Stage 4) | < 1 ms | 5–50 ms (network) |
| Fill rate | ~100% | ~92–98% |
