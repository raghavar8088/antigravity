# ORDER_LIFECYCLE_CERTIFICATION.md
## Phase 1 — Complete Trade Lifecycle Audit

**Audit Date:** 2026-06-09

---

## Lifecycle Stage Certification

| # | Stage | Verdict | File | Function | Line |
|---|-------|---------|------|----------|------|
| 1 | Signal Generated | **PASS** | `engine/internal/trading/loop.go` | `e.Strategy.OnTick` | 1358 |
| 2 | Risk Approved | **PARTIAL** | `engine/internal/risk/gate/pipeline.go` | `PreTradeRiskPipeline.Check` | 46 |
| 3 | OMS Order Created | **PASS** | `engine/internal/trading/loop.go` | `appendOrderEvent(EventOrderCreated)` | 384 |
| 4 | Broker Order Submitted | **PARTIAL** | `engine/internal/trading/loop.go` | `fillFn` via `submitInstitutionalOrder` | 677 |
| 5 | Broker Acknowledged | **PARTIAL** | `engine/internal/trading/loop.go` | `appendOrderEvent(EventOrderAcked)` | 673 |
| 6 | Exchange Order ID Stored | **FAIL** | `engine/internal/trading/loop.go` | `ackPayload.ExchangeOrderID = "paper-" + clientOrderID` | 672 |
| 7 | Partial Fill Received | **FAIL** | — | Not implemented in live path | — |
| 8 | Final Fill Received | **PARTIAL** | `engine/internal/trading/loop.go` | `appendOrderEvent(EventOrderFilled)` | 710 |
| 9 | Position Updated | **PASS** | `engine/internal/positions/manager.go` | `OpenPosition` | 126 |
| 10 | Portfolio Updated | **PASS** | `engine/internal/trading/loop.go` | `portfolioLedger.RecordClose` / `syncPMSState` | 1729, 1678 |
| 11 | Risk Updated | **PASS** | `engine/internal/trading/loop.go` | `o.risk.NotifyFill` | 1674 |
| 12 | PnL Updated | **PARTIAL** | `engine/internal/trading/loop.go` | `CanonicalNetPnL` on close only | 1710 |
| 13 | Ledger Updated | **PARTIAL** | `engine/internal/trading/loop.go` | `eventLedger.Append` | 376 | MemoryStore — lost on restart |

---

## Stage Detail

### 1. Signal Generated — PASS

```
Input:  marketdata.Tick (price, time, symbol)
Output: strategy.Signal{Action, Symbol, TargetSize, StopLossPct, TakeProfitPct, Confidence}
Failure: strategy disabled (loop.go:1354), ActionHold skipped (L1374)
```

Evidence: `loop.go:1358` — `signals := e.Strategy.OnTick(t)`  
Aggregation: `loop.go:1403` — `FilterSignalsSelective`

### 2. Risk Approved — PARTIAL

Two risk layers run sequentially:
- **Legacy:** `risk/engine.go:Validate` at `loop.go:1546` (pre-institutional)
- **Institutional:** `risk/gate/pipeline.go:Check` at `loop.go:484–500`

**Gap:** Emergency flatten bypasses both (`loop.go:409–422`).

### 3. OMS Order Created — PASS

```
Event:     EventOrderCreated
Aggregate: clientOrderID (AG-PAPER-{symbol}-{nano})
Payload:   {ClientOrderID, Symbol, Side, Quantity}
OMS:       omsv3.Replay validates state machine (loop.go:402)
Mongo:     persistOMSTransition → OMSNew (loop.go:388–397)
```

### 4. Broker Order Submitted — PARTIAL

| Venue | Submit Function | File:Line |
|-------|----------------|-----------|
| Paper | `PaperClient.ExecuteSignal` | `execution/paper.go:137` |
| Delta gateway | `deltaBridge.SubmitOrder` | `institutional_request.go:125` |
| Delta options | `bridge.SubmitOrder` | `institutional_request.go:205` |

**Gap:** Paper path assumes instant fill. Delta path has no order status polling after PlaceOrder.

### 5. Broker Acknowledged — PARTIAL

OMS records `EventOrderAcked` **before** fill attempt (`loop.go:673`), not after broker confirmation.

```
ackPayload.ExchangeOrderID = "paper-" + clientOrderID  // NOT real exchange ID
```

For Delta: `PlaceOrderResult.OrderID` returned but **not written** to ack payload.

### 6. Exchange Order ID Stored — FAIL

| Location | ID Stored | Real? |
|----------|-----------|-------|
| OMS ledger ack | `"paper-" + clientOrderID` | **No** — synthetic |
| `LiveTrade.DeltaOrderID` | `result.OrderID` from Delta REST | **Yes** — bridge only |
| `paperpersist` Mongo | `OrderID = ClientOrderID` | **No** |
| Execution gateway response | `ClientOrderID` only | **No** exchange ID |

Evidence:
- `loop.go:672` — synthetic ExchangeOrderID
- `live_bridge.go:179` — real ID isolated in bridge struct
- `institutional_request.go:209` — `captured = result` but never patches ledger ack

### 7. Partial Fill — FAIL

OMS v3 supports `EventOrderPartial`:
- `omsv3/aggregate.go:132`
- `ledger/order_projection.go:74–76`

Live loop **never emits** `EventOrderPartial`. Only backtest: `backtest/v3/oms_bridge.go:83–101`.

Fill always full quantity: `loop.go:708` — `fillPayload.FillQuantity = sig.TargetSize`

### 8. Final Fill — PARTIAL

Assumes immediate complete fill. No fill confirmation wait. No fill poller.

### 9–13. Downstream Updates — PASS/PARTIAL

Position, portfolio, risk updates work for paper path. Ledger is in-memory only (`loop.go:232`, `SetEventLedger` never called from `main.go`).

---

## Order State Machine (OMS v3)

```
NEW → VALIDATED → RISK_CHECKED → ACCEPTED → SUBMITTED → ACKED → FILLED
                                              ↓
                                         REJECTED / CANCELLED / RISK_BLOCKED
```

Partial state exists in schema but unused in production:
`omsv3/aggregate_invariants.go:48` — `EventOrderPartial` mapped to `AggregateOrder`

---

## Exchange Order Tracking Answers

| Question | Answer | Evidence |
|----------|--------|----------|
| Can OMS know the real exchange order? | **NO** (live Delta) | Synthetic ID in ledger; real ID in bridge only |
| Can OMS recover after restart? | **NO** | `eventLedger` = MemoryStore; `SetEventLedger` unwired |
| Can OMS reconcile fills? | **NO** (prod) | `PaperSnapshotProvider` supplies no orders |
| Can OMS identify orphan orders? | **NO** (prod) | Order arrays empty in snapshot |

---

## Overall Order Lifecycle Verdict: **PARTIAL**

Institutional event ordering is architecturally sound. Exchange identity, partial fills, durable ledger, and broker-confirmed ack are not production-ready for live capital.
