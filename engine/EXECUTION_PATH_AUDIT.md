# EXECUTION_PATH_AUDIT.md
## Phase 22D — Complete Execution Path Trace

**Date:** 2026-06-05  
**Scope:** Signal generation → approval → risk → sizing → OMS → fill → position → exit → PnL  
**Method:** Static trace with file:line evidence. No assumptions; every stage cites code.

---

## EXECUTION FLOW DIAGRAM

```
 marketdata tick / candle close
   │
   ▼
 Orchestrator.processStrategyGroup            loop.go:887  (parallel strategy eval)
   │   each entry.Strategy.OnTick(t)          loop.go:911
   │   → []AggregatedSignal (rawSignals)      loop.go:934-943
   ▼
 SignalAggregator.FilterSignalsSelective      aggregator_selective.go:38
   │   • hold filter                          aggregator_selective.go:53
   │   • cooldown filter                      aggregator_selective.go:58
   │   • dominance / weak-consensus           aggregator_selective.go:87
   │   • priority sort + score floor          aggregator_selective.go:98,111
   │   • category cap (5) / throughput (25)   aggregator_selective.go:33
   ▼  approved []AggregatedSignal
 for each approved signal                     loop.go:960
   │   ── execintel.Begin + SignalApproved    loop.go:967-986  (Phase 22D)
   │   • STALE/expiry guard      ── reject →   loop.go:986-1001  signalMaxAge:57 + execintel.IsExpired
   │   • position-limit          ── reject →   loop.go:1004-1010
   │   • regime filter           ── reject →   loop.go:1016-1023  isCategoryAlignedWithRegime:1742
   │   • capital sizing (1%)                   loop.go:1026-1031  targetSizeForCapital
   │   • execution-weight floor  ── reject →   loop.go:1032-1040
   │   • min-size                ── reject →   loop.go:1043-1049
   │   • sanitizeSignalForProfit ── reject →   loop.go:1056-1064  (TP/SL geometry override)
   │   ── execintel.RecordTPOverride          loop.go:1073-1080  (Phase 22D)
   │   • risk.Validate           ── reject →   loop.go:1082-1089
   │   ── execintel.RiskApproved               loop.go:1091      (Phase 22D)
   │   • bridge parking          ── reject →   loop.go:1112-1158
   │   ── execintel.OrderSubmitted             loop.go:1169      (Phase 22D)
   ▼
 executeThroughInstitutionalPath              loop.go:212
   │   • OMS v3 EventOrderCreated              loop.go:242
   │   • omsv3.Replay validation               loop.go:249
   │   • EventOrderValidated                   loop.go:252
   │   • PreTradeRiskPipeline.Check            loop.go:256  risk/gate
   │     └ blocked → EventRiskBlocked          loop.go:273
   │   • Kelly/dynamic size applied            loop.go:291
   │   • EventRiskApproved + EventOrderSubmitted loop.go:295-298
   │   • PaperClient.ExecuteSignal             loop.go:307  paper.go:127
   │     └ executionPrice (slippage model)     paper.go:54-80
   │     └ FillResult{ExecPrice,RequestedPrice,SlippageBps}  paper.go:135  (Phase 22D)
   │   • EventOrderFilled                      loop.go:326
   ▼  FillResult
   │   ── execintel OrderAcknowledged+Filled   loop.go:1182-1183  (Phase 22D)
   │   ── execintel.RecordSlippage             loop.go:1195-1207  (Phase 22D)
   │   • risk.NotifyFill                       loop.go:1213
   ▼
 openAndTrackPosition                          loop.go:366
   │   • posMgr.OpenPosition (SL/TP levels)    manager.go:111
   │     └ MinTakeProfitPct floor (TP override) manager.go:123  (Phase 22D audited)
   │   • emit EventPositionOpened (async)      loop.go:380
   │   ── execintel.PositionOpened             loop.go:1217  (Phase 22D)
   ▼
 (price ticks) CheckStopLossAndTakeProfit      manager.go:173
   │   • TP hit → emitClose(ReasonTakeProfit)  manager.go:201
   │   • SL hit → emitClose(ReasonStopLoss)    manager.go:211
   │   • max-age → emitClose(ReasonManual)     manager.go:293
   ▼  CloseEvent on posMgr.CloseEvents
 processCloseEvents                            loop.go:1175
   │   • SettlePosition (balance)              loop.go:1184
   │   • CalculateNetPnL                       loop.go:1185
   │   • journal.RecordTrade                   loop.go:1205
   │   • tracker.RecordTradeResult             loop.go:1208
   │   ── finalizeExecIntelClose               loop.go:1213  (Phase 22D)
   │       • TP/SLTriggered + PositionClosed + RecordTradeResult + RecordTPOutcome
   │   • risk.RecordPnL                        loop.go:1216
   │   • emit EventPositionClosed (async)      loop.go:1235
```

---

## STAGE-BY-STAGE DETAIL

| # | Stage | Function | File:Line | Entry condition | Reject condition |
|---|-------|----------|-----------|-----------------|------------------|
| 1 | Strategy eval | `processStrategyGroup` | loop.go:887 | tick/candle arrives; strategy enabled (`tracker.IsEnabled`) | strategy disabled → skipped silently |
| 2 | Aggregation | `FilterSignalsSelective` | aggregator_selective.go:38 | ≥1 raw signal | hold / cooldown / weak-consensus / below score floor / category cap / throughput cap |
| 3 | Stale + hard expiry | inline guard | loop.go:986 | signal has CreatedAt | `age > signalMaxAge(tf)` OR `execintel.IsExpired(tf,age)` |
| 4 | Position limit | `posMgr.CanOpenPosition` | loop.go:1004 | — | strategy at `MaxPerStrategy` (2) |
| 5 | Regime filter | `isCategoryAlignedWithRegime` | loop.go:1016 / 1742 | — | category not aligned with current regime |
| 6 | Capital sizing | `targetSizeForCapital` | loop.go:1026 | currentPrice > 0 | — (normalizes to 1% capital) |
| 7 | Execution-weight floor | `tracker.GetExecutionWeight` | loop.go:1032 | — | weight `< minExecutionWeightToTrade (0.50)` |
| 8 | Min size | inline | loop.go:1043 | — | size `< minExecutionSizeBTC (0.01)` |
| 9 | Profit sanitize (TP/SL) | `sanitizeSignalForProfit` | loop.go:1056 | — | confidence `< 0.68` / R:R `< 2.40` |
| 10 | Pre-trade risk | `risk.Validate` | loop.go:1082 | — | drawdown / exposure / kill switch |
| 11 | Bridge parking | `IsBridgeOnline` | loop.go:1112 | bridge heartbeat < 15s AND not trusted | parked (not a true reject; queued) |
| 12 | OMS + risk gate + fill | `executeThroughInstitutionalPath` | loop.go:212 | — | OMS replay error / `PreTradeRiskPipeline` blocked / no market price |
| 13 | Position open | `openAndTrackPosition` | loop.go:366 | fill returned | — |
| 14 | Exit | `CheckStopLossAndTakeProfit` | manager.go:173 | price crosses SL/TP or max-age | — |
| 15 | Close + PnL | `processCloseEvents` | loop.go:1175 | CloseEvent received | — |

---

## KEY FINDINGS (pre-22D state, with evidence)

### F1 — Slippage was computed then discarded
`loop.go` (pre-22D) computed `slippageBps := math.Abs(execPrice-currentPrice)/currentPrice*10000` and only `log.Printf`-ed it. `FillResult` (routing.go:15) had **no slippage field**. There was **no aggregation** by strategy/session/regime/direction. → **Fixed:** `FillResult.RequestedPrice` + `FillResult.SlippageBps` populated in `ExecuteSignal` (paper.go:135), recorded via `execIntel.RecordSlippage` (loop.go:1195).

### F2 — Latency stages 4–6 were zero-width
`loop.go:1127-1129` (pre-22D) called `RecordPipelineStage(StageOMSToExchange)`, `…ExchangeToFill`, `…FillToLedger` back-to-back with the same `stageStart`, so each measured ≈0ms — they did not bracket real operations. Only `tick_to_strategy` and `strategy_to_risk` were genuine. → **Addressed:** execintel derives real spans from lifecycle transition timestamps (`signal_to_submit`, `submit_to_ack`, `ack_to_fill`, `fill_to_position`, `position_to_close`, `signal_to_fill_e2e`), tracker.go:`recordLatencyLocked`.

### F3 — No per-signal lifecycle
`SignalFlowMetrics` (signal_flow_metrics.go) tracked aggregate stage counts only — no SignalID, no per-signal timestamps, no state machine. → **Fixed:** `execintel.Tracker` records a 14-state lifecycle per signal with timestamps.

### F4 — TP overrides un-audited
Two TP overrides exist — `sanitizeSignalForProfit` geometry (loop.go:1056) and `Manager.OpenPosition` `MinTakeProfitPct` floor (manager.go:123) — but their realized impact was never measured. → **Fixed:** `execIntel.RecordTPOverride` + `RecordTPOutcome` (tpaudit.go).

### F5 — No trade-conversion / quality / bottleneck outputs
No cohesive funnel, execution-quality score, or ranked bottleneck list existed. → **Fixed:** `execintel.Snapshot` produces all three.

---

## TELEMETRY INVENTORY (pre-existing, retained)

| Component | File | Role |
|---|---|---|
| `SignalFlowMetrics` | trading/signal_flow_metrics.go | Aggregate stage funnel + per-strategy approval/exec counts |
| `PipelineTimer` / Prometheus histograms | observability/latency.go | tick→fill stage histograms (bucketed) |
| `TradeJournal` | execution/trade_journal.go | Per-trade entry/exit/PnL records |
| `StrategyTracker` | risk/strategy_tracker.go | Per-strategy win-rate, PnL, execution weight |

Phase 22D `execintel` complements (does not replace) these with per-signal lifecycle, in-process percentiles, slippage attribution, TP-override audit, conversion, and a composite quality score.
