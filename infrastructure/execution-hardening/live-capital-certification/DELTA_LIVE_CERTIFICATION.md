# DELTA_LIVE_CERTIFICATION.md
## Phase 10 — Delta Live Execution Certification

**Audit Date:** 2026-06-09  
**Verdict: PARTIAL**

---

## Required Institutional Path

```
Institutional Execution Path
  → PMS
  → Kill Switch
  → RiskV2
  → OMS
  → Broker Fill
```

Any path that bypasses this chain = **FAIL**.

---

## Delta Execution Entry Points

| Entry | Route / Trigger | Institutional? | Evidence |
|-------|-----------------|----------------|----------|
| Execution gateway (delta venue) | `POST /api/execution/request` venue=delta | **YES** | `institutional_request.go:81–140` |
| Delta live order (engine) | `POST /api/delta-live/order` | **YES** | `main.go:1509–1511` → `ProcessExecutionRequest` |
| Paper options mirror open | `Bridge.OnOpen` | **YES** (if wired) | `institutional_request.go:212` |
| Paper options mirror close | `Bridge.OnClose` | **YES** (if wired) | `institutional_request.go:260` |
| Monitor auto-close | `live_bridge.go:monitorPositions` | **YES** (via OnClose) | `live_bridge.go:505, 567` |
| PlaceManualOrder | Direct bridge | **BLOCKED** | `live_bridge.go:576–581` returns error |
| Next.js delta/spot | Frontend | **BLOCKED** | 410 — `spot/route.ts:14–15` |
| Next.js delta/mirror | Frontend | **BLOCKED** | 410 — `mirror/route.ts:7–8` |
| Next.js delta/testnet/* | Frontend | **BLOCKED** | 410 — `testnet/place-order/route.ts:7` |
| Next.js delta/probe POST | Frontend | **BLOCKED** | 410 — `probe/route.ts:56` |

---

## Path Trace: Open Position

```
1. Signal/trigger (paper options open OR gateway request)
2. ProcessExecutionRequest kill check (institutional_request.go:16–20)     ✓
3. executeThroughInstitutionalPathWithFill (L133 or L212)
4. PMS CheckPortfolioRisk (loop.go:435–481)                               ✓
5. PreTradeRiskPipeline.Check + kill switch (pipeline.go:51–54)           ✓
6. RiskV2 sizing + EnforceExecutionFloor (loop.go:484–637)                ✓
7. submitInstitutionalOrder → OMS events (loop.go:640–726)                  ✓
8. fillFn → bridge.SubmitOrder (institutional_request.go:125 or L205)     ✓
9. delta.Client.PlaceOrder (client.go:182)                                ✓
10. UpdateTradeAfterFill → DeltaOrderID (live_bridge.go:177–187)          ✓ (bridge only)
```

---

## Path Trace: Close Position

```
1. OnClose / monitorPositions / gateway close
2. institutionalClose handler (live_bridge.go:345)
3. executeThroughInstitutionalPathWithFill (institutional_request.go:260)
4. [PMS → RiskV2 → OMS — same as open]
5. bridge.SubmitReduceOnlyOrder (institutional_request.go:253)
6. UpdateTradeAfterClose (live_bridge.go:161–173)
```

---

## Path Trace: Stop Loss / Take Profit

**BTC paper desk:** Software SL/TP via `CheckStopLossAndTakeProfit` — **no Delta broker order**.

**Delta options:** SL/TP from paper options engine triggers `OnClose` → institutional close path.

| Operation | Exchange Order? | Institutional? |
|-----------|-----------------|----------------|
| Stop loss (BTC) | **NO** — software price hit | N/A for Delta BTC |
| Take profit (BTC) | **NO** — software price hit | N/A |
| Delta option close | **YES** — reduce-only market | **YES** |
| Emergency flatten | **YES** — via institutional (skips PMS/RiskV2) | **PARTIAL** |

---

## Partial Close

- Engine: `emitPartialTakeProfit` defined (`manager.go:277`) — **no callers**
- Delta: Full contract close only via `SubmitReduceOnlyOrder` with `trade.Contracts`
- **PARTIAL close not implemented for Delta live**

---

## Kill Switch Coverage

| Check Point | Active? | Evidence |
|-------------|---------|----------|
| `ProcessExecutionRequest` | **YES** | `institutional_request.go:16–20` |
| `PreTradeRiskPipeline` | **YES** | `pipeline.go:51–54` |
| `Bridge.killCheck` on SubmitOrder | **NO** | `SetKillCheck` wired (L149–154) but `SubmitOrder` (live_bridge.go:131–141) never calls `killCheck` |
| `WireDeltaBridge` at boot | **YES** | `main.go:903` |

**Gap B2: killCheck is dead code at broker submit layer.**

---

## Exchange Order ID Gap

| Location | ID | Real? |
|----------|-----|-------|
| OMS ledger | `"paper-" + clientOrderID` | **NO** |
| `LiveTrade.DeltaOrderID` | Delta REST `result.id` | **YES** |
| Gateway response | `ClientOrderID` only | **NO** exchange ID |

OMS cannot correlate to real Delta order after restart.

---

## Delta Sizing Divergence

Risk V2 receives `TargetSize = PremiumUSD / 100000` (institutional_request.go:174).

Actual contracts: `int(PremiumUSD/100)` or hardcoded `1` for buying (L191–204).

**Risk V2 sizing semantics do not match actual contract exposure.**

---

## Delta Live Certification Matrix

| Operation | Institutional Path | Verdict |
|-----------|-------------------|---------|
| Open position | PMS → KS → RiskV2 → OMS → SubmitOrder | **PASS** |
| Close position | Same via SubmitReduceOnlyOrder | **PASS** |
| Partial close | Not implemented | **FAIL** |
| Stop loss | Software only (paper); Delta via OnClose | **PARTIAL** |
| Take profit | Software only (paper); Delta via OnClose | **PARTIAL** |
| Emergency flatten | Skips PMS/RiskV2 | **PARTIAL** |
| Frontend bypass | All blocked (410) | **PASS** |
| Unwired handler fallback | Returns FAILED, no direct broker | **PASS** |
| killCheck at submit | Not invoked | **FAIL** |
| Exchange ID in OMS | Not stored | **FAIL** |
| Partial fill handling | Not implemented | **FAIL** |
| PnL with fees | Not in bridge | **FAIL** |

---

## Delta Live Verdict: **PARTIAL**

Institutional routing is correctly wired for open/close. Critical gaps prevent live-capital certification: dead killCheck at submit, no exchange ID in OMS, no partial fills, sizing divergence, simplified PnL.
