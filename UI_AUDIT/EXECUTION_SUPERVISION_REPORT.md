# PHASE 5 — EXECUTION SUPERVISION REPORT
## Forensic Audit | Trading Platform | 2026-06-11

---

## AUDIT QUESTION: Can a trader see what the execution engine is doing?

---

## ORDERS SUBMITTED
**Verdict: ABSENT (live) / PARTIAL (paper)**

Evidence:
- Paper OMS Panel (`PaperOmsPanel.tsx`, 15KB) shows paper orders from `/api/paper-oms/orders`
- No UI shows live engine orders being submitted to real brokers in real-time
- AngelOne order panel (`AngelOneOrderPanel.tsx`, 7KB): form to PLACE orders, not to VIEW submitted orders from the engine
- Delta Live: `/api/delta/testnet/*` routes exist; `TestnetOpsPanel.tsx` shows testnet operations
- **Gap**: Engine-initiated order submissions to AngelOne / Delta Exchange have no real-time visibility

## ORDERS PENDING
**Verdict: ABSENT**

Evidence:
- Paper OMS shows paper orders; no equivalent for live broker orders
- No component polls a "pending orders" endpoint from a live broker
- `useAngelOneOrders()` hook exists in `client/src/hooks/` — fetches orders from AngelOne
- **Gap**: `useAngelOneOrders()` existence needs verification that it renders somewhere visible; component location not confirmed in primary dashboard routing

## ORDERS REJECTED
**Verdict: ABSENT**

Evidence:
- No component shows order rejection reasons from live brokers
- Signal trace panel shows WHY signals were blocked before submission (funnel diagnostics), but this is for the in-browser engine — not for Go engine → broker rejections
- **Gap**: If the Go engine submits an order to AngelOne or Delta and it's rejected (invalid price, insufficient margin, market closed), the trader has no UI notification

## ORDERS FILLED
**Verdict: PARTIAL (paper) / ABSENT (live broker)**

Evidence:
- Paper Desk: recent trades shown with fill price, PnL — live from MongoDB
- Go engine fills persisted to SQLite/MongoDB and surfaced via paper desk
- **Gap**: For live broker fills (AngelOne equity trades, Delta futures), no real-time fill notification UI

## PARTIAL FILLS
**Verdict: ABSENT**

Evidence:
- `PaperTradeDoc` type exists with `fill_price`, `quantity`, etc.
- No UI component checks or displays partial fill status
- No "partial fill" indicator anywhere in the codebase

## EXECUTION LATENCY
**Verdict: ABSENT**

Evidence:
- `lib/internal/execution/engine_v2.ts` exists (execution engine v2)
- No UI component displays:
  - Order submission latency
  - Fill confirmation latency
  - Signal-to-order latency
  - Broker round-trip time
- **Gap**: A trader has no visibility into whether execution is slow or degraded

## BROKER RESPONSES
**Verdict: ABSENT**

Evidence:
- AngelOne order panel shows form inputs; no response/confirmation display after submission
- No component shows raw broker API response codes, error messages, or ACK timestamps
- `DeltaLiveScalper.tsx` (56KB) — large component but routing unclear
- **Gap**: When a broker rejects, throttles, or errors, the trader sees nothing

---

## QUICKTRADEPANEL — CRITICAL FINDING

The `ExecutionCenter.tsx` at line ~80+ includes a `QuickTradePanel` component. This is rendered in the primary terminal execution page. Reading the component:

**Evidence from ExecutionCenter.tsx:**
- `QuickTradePanel` is defined inline in ExecutionCenter.tsx
- Its implementation needs verification — if it's only a UI stub with no real order submission wiring, it creates a dangerous UI affordance

**Risk**: A trader might press "buy/sell" in `QuickTradePanel` believing it places a real order, when it may be a non-functional stub.

---

## SIGNAL TRACE PANEL — WHAT IT DOES WELL

`SignalTracePanel.tsx` (22KB) is the strongest execution-related component:
- Shows signal funnel: generated → confidence pass → scoring gate → risk gate → family conflict → executed
- Shows dominant blocker with % of evaluations
- Shows why entries were skipped
- Configured via `NEXT_PUBLIC_DESK_ENTRY_DEBUG=1` env var

**However**: This traces the in-browser JavaScript engine, not the Go engine's execution decisions. The two are not the same system.

---

## EXECUTION SUPERVISION SCORECARD

| Capability | Status |
|-----------|--------|
| Orders submitted (live broker) | ABSENT |
| Orders submitted (paper) | PARTIAL |
| Orders pending | ABSENT |
| Orders rejected | ABSENT |
| Orders filled (live) | ABSENT |
| Orders filled (paper) | PARTIAL |
| Partial fills | ABSENT |
| Execution latency | ABSENT |
| Broker responses | ABSENT |
| Signal-to-fill trace | PARTIAL (in-browser only) |

**Score: 2/10 — Critical deficiency for live trading supervision**

The paper desk provides adequate post-hoc execution visibility. Live broker execution is a complete blind spot. A trader running AngelOne equity strategies or Delta futures cannot see any execution detail in real time.
