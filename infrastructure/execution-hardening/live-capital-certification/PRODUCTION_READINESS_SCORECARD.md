# PRODUCTION_READINESS_SCORECARD.md
## Phase 11 — Live Capital Readiness Scoring

**Audit Date:** 2026-06-09  
**Scale:** 1–10 (10 = production-ready for live capital)

---

## Scorecard

| Dimension | Score | Rationale (Source Evidence) |
|-----------|-------|----------------------------|
| **Strategy Correctness** | 7/10 | `OnTick` signal generation wired (`loop.go:1358`); aggregation/filtering active (`L1403`); WINNERS_ONLY gate active per repo rules. Browser desk is parallel untested path. |
| **Execution Correctness** | 4/10 | Institutional path exists and is used (`executeThroughInstitutionalPathWithFill` L346). Assumes instant full fills (`L708`). Synthetic ExchangeOrderID (`L672`). SL/TP are software-only, no exchange orders. |
| **OMS Reliability** | 5/10 | OMS v3 state machine + event ordering correct (`omsv3.Replay` L402). Ledger is in-memory (`loop.go:232`); `SetEventLedger` unwired. Partial fills unsupported live. |
| **Broker Reliability** | 3/10 | Delta REST PlaceOrder with no status polling (`client.go:182`). `killCheck` never invoked at submit (`live_bridge.go:131–141`). No fill listener. |
| **Risk Controls** | 6/10 | PMS + RiskV2 + Kelly + heat/exposure caps enforced (`loop.go:435–637`). Emergency flatten bypasses gates (`L409–422`). Kill-switch state not restored on boot (`killswitch/service.go:49–56`). |
| **Sizing Controls** | 6/10 | Risk V2 formulas correct (`risk/v2/kelly.go:39–69`). Delta contract sizing diverges from Risk V2 (`institutional_request.go:174, 191–204`). Client desk separate. |
| **PnL Accuracy** | 6/10 | Engine paper: `CanonicalNetPnL` + fees (`fees.go:30`, `loop.go:1710`). Tested (`futuresPaperMath.test.ts`). Delta live: no fees (`live_bridge.go:169–171`). |
| **Recovery** | 4/10 | SQLite+Mongo snapshots work (`recovery.go:90–155`, `store.go:224–251`). 10–15s RPO. Ledger/OMS/kill-switch state lost. `paper_orders` not recovered. |
| **Reconciliation** | 2/10 | Production mirror provider (`paper_provider.go:44–48`). v2 with real broker APIs exists but unwired (`main.go` — zero `reconciliationv2` refs). False kill-switch comment (`main.go:883`). |
| **Observability** | 6/10 | Pipeline timers (`observability.NewPipelineTimerAt` L1340). OMS Mongo transitions (`persistOMSTransition`). Kill-switch logs only (`killswitch_executor.go:86–89`). No PagerDuty. |
| **Scalability** | 5/10 | Per-strategy goroutines (`loop.go:1350`). 600+ strategies per registry. Single-process engine. No horizontal execution partitioning evidenced. |
| **Production Readiness** | 3/10 | Frontend broker routes blocked (410). Institutional gateway wired. Critical operational gaps: reconciliation, order ID, fill management, recovery completeness. |

---

## Weighted Assessment

| Tier | Dimensions (Score ≤4) |
|------|----------------------|
| **Critical** | Broker Reliability (3), Reconciliation (2), Production Readiness (3), Execution Correctness (4), Recovery (4) |
| **Adequate** | Strategy (7), Risk (6), Sizing (6), PnL (6), Observability (6) |
| **Middle** | OMS (5), Scalability (5) |

---

## Composite Score

**Arithmetic mean: 4.8 / 10**

**Live capital threshold (institutional standard): ≥8.0 required across Execution, OMS, Broker, Reconciliation, Recovery.**

**Result: BELOW THRESHOLD — NOT READY**

---

## Minimum Viable Fixes for Live Capital (Evidence-Based Priority)

1. Wire `reconciliationv2` with Delta/Binance adapters in `main.go`
2. Bridge reconciliation CRITICAL drift → `ksSvc.Trigger(TriggerOMSDesync)`
3. Store real `ExchangeOrderID` from broker response in OMS ledger ack
4. Wire `SetEventLedger` to Postgres durable store
5. Invoke `killCheck` in `Bridge.SubmitOrder` / `SubmitReduceOnlyOrder`
6. Implement `EventOrderPartial` + fill polling for Delta
7. Replay kill-switch active state from Postgres on boot
8. Recover `paper_orders` in `Recover()` per documented intent
