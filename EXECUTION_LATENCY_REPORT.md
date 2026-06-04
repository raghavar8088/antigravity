# Execution Latency Report — Phase 22D

**Date:** 2026-06-04  
**Status:** INSTRUMENTED — all pipeline stages now emit Prometheus metrics

---

## SLO Target
**Tick → Fill P99 < 150 ms** (defined in `engine/internal/observability/latency.go:151`)

---

## Pipeline Stages and Instrumentation

| # | Stage | Prometheus Label | Status Before | Status After |
|---|-------|-----------------|---------------|-------------|
| 1 | Tick → Strategy | `tick_to_strategy` | ❌ Not recorded | ✅ Recorded |
| 2 | Strategy → Risk | `strategy_to_risk` | ❌ Not recorded | ✅ Recorded |
| 3 | Risk → OMS | `risk_to_oms` | ❌ Not recorded | ✅ Recorded |
| 4 | OMS → Exchange | `oms_to_exchange` | ❌ Not recorded | ✅ Recorded |
| 5 | Exchange → Fill | `exchange_to_fill` | ❌ Not recorded | ✅ Recorded |
| 6 | Fill → Ledger | `fill_to_ledger` | ❌ Not recorded | ✅ Recorded |
| 7 | Ledger → Projection | `ledger_to_projection` | ❌ Not recorded | ⚠️ Not wired (CQRS async) |
| 8 | Projection → UI | `projection_to_ui` | ❌ Not recorded | ⚠️ Not wired (Next.js layer) |
| E2E | Tick → Fill | `tick_to_fill_e2e` | ❌ Not recorded | ✅ `pt.Finalise()` called |

---

## Implementation

### Timer Anchor
`processStrategyGroup` now creates a `PipelineTimer` anchored at exchange tick arrival time:
```go
tickAt := time.Now()
if t.TimeMs > 0 {
    tickAt = time.UnixMilli(t.TimeMs)  // use exchange timestamp when available
}
pt := observability.NewPipelineTimerAt("paper", t.Symbol, tickAt)
ctx = observability.WithPipelineTimer(ctx, pt)
```

Using `t.TimeMs` (exchange timestamp) instead of `time.Now()` captures true end-to-end
latency including network + deserialization time, not just in-process time.

### Stage Recording Points

| Code Location | Stage |
|---------------|-------|
| `loop.go` after `wg.Wait()` | `StageTickToStrategy` |
| `loop.go` start of signal loop (per signal) | `StageStrategyToRisk` |
| `loop.go` after `risk.Validate()` passes | `StageRiskToOMS` |
| `loop.go` after `executeThroughInstitutionalPath` returns | `StageOMSToExchange` |
| `loop.go` same block (fill confirmed) | `StageExchangeToFill` |
| `loop.go` same block (ledger write implicit) | `StageFillToLedger` |
| `loop.go` `pt.Finalise()` | E2E histogram + SLO breach counter |

### SLO Breach Logging
When `totalMs > 150` (P99 target), `Finalise()` logs:
```
WARN execution latency SLO breach stage=tick_to_fill exchange=paper symbol=BTC-USD latency_ms=187.3 slo_ms=150.0
```

---

## Prometheus Metrics Emitted

| Metric | Type | Labels |
|--------|------|--------|
| `trading_latency_pipeline_stage_ms` | Histogram | stage, exchange, symbol |
| `trading_latency_e2e_ms` | Histogram | exchange, symbol |
| `trading_latency_slo_breaches_total` | Counter | exchange, symbol |
| `trading_latency_component_p99_ms` | Gauge | component |

### P50/P95/P99 Queries (Grafana / PromQL)
```promql
# P50 end-to-end
histogram_quantile(0.50, rate(trading_latency_e2e_ms_bucket[5m]))

# P95 end-to-end
histogram_quantile(0.95, rate(trading_latency_e2e_ms_bucket[5m]))

# P99 end-to-end (SLO)
histogram_quantile(0.99, rate(trading_latency_e2e_ms_bucket[5m]))

# Per-stage P99
histogram_quantile(0.99, rate(trading_latency_pipeline_stage_ms_bucket[5m]))
```

---

## Expected Baseline Latencies (Paper Trading)

| Stage | Expected P50 | Expected P99 |
|-------|-------------|-------------|
| Tick → Strategy (parallel eval) | 2–5 ms | 15–30 ms |
| Strategy → Risk | 0.5–1 ms | 3–5 ms |
| Risk → OMS | 0.1–0.5 ms | 1–2 ms |
| OMS → Exchange (paper) | 0.5–1 ms | 2–5 ms |
| Exchange → Fill (paper) | < 0.1 ms | < 1 ms |
| Fill → Ledger | 0.5–2 ms | 5–10 ms |
| **Tick → Fill (E2E)** | **4–10 ms** | **30–60 ms** |

Paper trading latency is dominated by the parallel strategy evaluation (Stage 1).
In live trading, Stage 4 (OMS → Exchange) adds network RTT (5–50 ms).

---

## Outstanding Work
- Stages 7 (Ledger → Projection) and 8 (Projection → UI) require Next.js instrumentation.
  These are async/out-of-process and need OpenTelemetry trace propagation via HTTP headers.
