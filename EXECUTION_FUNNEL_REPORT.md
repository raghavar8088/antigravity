# Execution Funnel Report — Phase 22D

**Date:** 2026-06-04

---

## Complete Signal → Fill Funnel

```
TICK (every ~250 ms from Coinbase WS)
  │
  ▼
[Stage 1] processTickPipeline (loop.go:535)
  ├── Update price/volume window
  ├── Check open position SL/TP
  └── Feed tick to CandleAggregator
        │
        ├── (Candle close: 1m) → processStrategyGroup(M1, "1m")
        ├── (Candle close: 5m) → processStrategyGroup(M5, "5m")
        ├── (Candle close: 15m sim) → processStrategyGroup(M15, "15m")
        └── (Candle close: 1h sim) → processStrategyGroup(H1, "1h")
  │
  ▼
[Stage 2] Strategy Evaluation (parallel goroutines)
  ├── Each strategy: OnTick() / OnCandle() → []Signal
  ├── Stamp: sig.CreatedAt = now, sig.Timeframe = timeframe  ← Phase 22D
  └── Collect into rawSignals[]
        │
        Prometheus: StageTickToStrategy recorded ← Phase 22D
  │
  ▼
[Stage 3] FilterSignalsSelective (aggregator_selective.go)
  ├── Cooldown filter (30 s per strategy)
  ├── Dominance filter (ratio ≥ 1.10)
  ├── Score filter (≥ 1.10)
  ├── Per-category cap (≤ 2)
  └── Throughput cap (≤ 8 per batch)
  │
  ▼
[Stage 4] Per-Signal Pipeline Loop (loop.go:925+)
  │
  ├── Prometheus: StageStrategyToRisk recorded ← Phase 22D
  │
  ├── [NEW] STALE SIGNAL GUARD (Phase 22D)   ← Phase 22D
  │     └── Drop if age > signalMaxAge(timeframe)
  │
  ├── Position limit check (max 2/strategy)
  │
  ├── Regime filter (category × regime alignment)
  │
  ├── Execution weight filter (weight ≥ 0.50)
  │
  ├── Size normalization (1% capital = $10,000)
  │
  ├── [UPDATED] sanitizeSignalForProfit()    ← Phase 22D
  │     ├── Confidence gate (≥ 0.74)
  │     ├── SL: default 0.10%, cap 0.20%
  │     ├── If TP explicitly set: preserve it (only 0.10% floor applied)
  │     └── If TP = 0: floor 0.50%, then R:R floor 2.4×SL
  │
  ├── risk.Validate() (legacy risk engine)
  │
  ├── Prometheus: StageRiskToOMS recorded   ← Phase 22D
  │
  ├── Bridge parking check
  │     ├── Bridge online + not trusted → park in pendingSignals map
  │     └── Bridge offline OR trusted → continue to execution
  │
  ▼
[Stage 5] executeThroughInstitutionalPath (loop.go:211)
  ├── Generate ClientOrderID (AG-PAPER-{symbol}-{nano})
  ├── EventOrderCreated → ledger
  ├── OMS v3 replay validation
  ├── EventOrderValidated → ledger
  ├── PreTradeRiskPipeline.Check() (riskgate)
  ├── EventRiskApproved, EventOrderSubmitted → ledger
  ├── EventOrderAcked → ledger
  └── exec.ExecuteSignal() → FillResult
        │
        ├── Prometheus: StageOMSToExchange recorded    ← Phase 22D
        ├── Prometheus: StageExchangeToFill recorded   ← Phase 22D
        ├── Prometheus: StageFillToLedger recorded     ← Phase 22D
        └── pt.Finalise() → E2E histogram + SLO check ← Phase 22D
  │
  ▼
[Stage 6] Slippage + Position Open
  ├── [NEW] Log entry slippage in bps  ← Phase 22D
  ├── risk.NotifyFill(sig)
  ├── openAndTrackPosition() → positions.Manager
  └── emitPositionOpened() → EventPositionOpened (async)
  │
  ▼
[Stage 7] processCloseEvents (background goroutine)
  ├── SL/TP hit → CloseEvent
  ├── positions.Manager.Close()
  ├── TradeJournal.Record()
  └── EventPositionClosed → ledger
```

---

## Funnel Drop Rates (Estimated Steady State)

| Stage | Typical Drop Rate | Reason |
|-------|------------------|--------|
| Raw → Cooldown | 40–60% | Same strategy firing repeatedly |
| Cooldown → Score | 10–20% | Weak consensus / low score |
| Score → Regime | 5–15% | Category vs. regime mismatch |
| Regime → Weight | 5–10% | Underperforming strategies |
| Weight → Geometry | 1–3% | Confidence < 0.74 |
| Geometry → Risk | 2–5% | Heat/VaR/concentration limits |
| Risk → Execution | 20–40% | Bridge parking (when bridge online) |
| Execution → Fill | < 2% | Risk gate rejection, size limits |
| **Signal → Trade (net)** | **~5–15%** | Of raw signals reach a fill |

---

## Signal → Trade Conversion Rate

Tracked via `RecordSignalFlowStage(SignalStageExecution, 1, 1)`.

Prometheus query:
```promql
sum(rate(trading_aggregator_signal_flow_passed_total{stage="execution"}[5m])) /
sum(rate(trading_aggregator_signal_flow_total{stage="generated"}[5m]))
```

Target: > 8% conversion rate (signal → trade).
