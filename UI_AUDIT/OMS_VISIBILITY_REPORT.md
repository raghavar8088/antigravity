# PHASE 6 — OMS VISIBILITY REPORT
## Forensic Audit | Trading Platform | 2026-06-11

---

## AUDIT QUESTION: Can a trader understand the OMS order lifecycle?

---

## OMS ARCHITECTURE (Backend)

The Go engine contains:
- `engine/internal/omsv3/` — OMS v3 (Phase 13 feature, listed as untracked)
- Paper OMS client: `client/src/lib/paperOms.ts`
- OMS v2 reference: `client/src/lib/internal/oms/oms_v2.ts`
- Engine proxy admin path: `/api/paper-desk/oms` (admin tier — requires ENGINE_ADMIN_SECRET)

---

## ORDER LIFECYCLE VISIBILITY

### Order Creation
**Verdict: PARTIAL (paper only)**

Evidence:
- Paper orders created by the paper desk engine; visible in PaperOmsPanel after creation
- No UI to manually create a paper OMS order
- Live AngelOne orders: `AngelOneOrderPanel.tsx` (7KB) — form-based order entry for manual order placement
- **Gap**: Engine-initiated orders to AngelOne/Delta are not reflected in any OMS view

### Order State Transitions
**Verdict: PARTIAL (paper only)**

Evidence:
- `/api/paper-oms/orders` returns paper orders with state information
- `PaperOmsPanel.tsx` renders these orders
- Order states likely include: PENDING, FILLED, CANCELLED, REJECTED (inferred from `PaperOrderDoc` type)
- **Gap**: No real-time state transition animation or notification. No "order just changed state" event. A trader must scroll the OMS panel to notice state changes.

### Order Modification / Cancellation
**Verdict: ABSENT**

Evidence:
- `PaperOmsPanel.tsx` is view-only — no cancel button, no modify controls found in component
- No `/api/paper-oms/cancel` or `/api/paper-oms/modify` route in API inventory
- AngelOne: `/api/angelone/cancel-order` route EXISTS but requires manual use via `AngelOneOrderPanel`
- **Gap**: Trader cannot cancel an open paper order from the OMS panel. All OMS interaction is read-only.

### Position State
**Verdict: PARTIAL (paper only)**

Evidence:
- Paper Desk: open positions shown with entry price, current price, unrealized PnL
- `client/src/lib/paperDeskPositionMath.ts` — calculates position PnL, margin, liquidation price
- Liquidation price shown in terminal execution center (mock data)
- **Gap**: Live broker positions (AngelOne equity, Delta futures) have no unified position state view

### Portfolio State
**Verdict: PARTIAL**

Evidence:
- Paper Desk summary: balance, equity, unrealized PnL, realized PnL, peak equity, drawdown
- Multi-instrument portfolio (BTC + NIFTY + options) not aggregated anywhere

### Ledger State
**Verdict: ABSENT**

Evidence:
- `engine/internal/ledger/` exists in Go engine (untracked)
- No UI component or API route references the ledger
- Daily PnL ledger (`DailyPnlLedger.tsx`, 6KB) exists as a component
- `/api/mock-trading/daily-pnl` and paper desk daily PnL: provide daily PnL history
- **Gap**: The Go engine's internal ledger (position accounting, fee tracking, funding) has no direct UI exposure

---

## PAPER OMS PANEL — DETAILED ASSESSMENT

`PaperOmsPanel.tsx` (15KB):
- Fetches from `/api/paper-oms/orders` (via engine proxy or direct)
- Renders order table: likely columns include order ID, strategy, side, size, price, status, timestamp
- **No evidence of**:
  - Order filter controls
  - Order sort controls
  - Pagination (for large order volumes)
  - Order cancellation button
  - Order modification UI
  - Fill price vs limit price comparison
  - Slippage display
  - Time-in-force visibility

This is a monitoring panel, not a management panel.

---

## CRITICAL FINDING: ENGINE_ADMIN_SECRET NOT CONFIGURED

The engine proxy admin path `/api/paper-desk/oms` (for OMS writes) requires `ENGINE_ADMIN_SECRET` env var:

```typescript
// client/src/app/api/engine/[...path]/route.ts:197-199
if (!expected) {
  return deny(503, "UNCONFIGURED", "ENGINE_ADMIN_SECRET is not configured");
}
```

`ENGINE_ADMIN_SECRET` is NOT present in `client/.env.local`. This means any attempt to use admin OMS operations from Vercel returns HTTP 503. The kill switch block/release endpoints are similarly broken.

**Evidence**: `client/.env.local` — no `ENGINE_ADMIN_SECRET` entry
**Evidence**: `client/src/app/api/engine/[...path]/route.ts:197`

---

## OMS VISIBILITY SCORECARD

| OMS Capability | Status |
|---------------|--------|
| Order creation view | PARTIAL (paper) |
| Order state transitions | PARTIAL (paper) |
| Order cancellation | ABSENT |
| Order modification | ABSENT |
| Fill quality / slippage | ABSENT |
| Position state (paper) | PARTIAL |
| Position state (live broker) | ABSENT |
| Portfolio aggregate state | PARTIAL |
| Ledger state | ABSENT |
| OMS error / rejection | ABSENT |

**Score: 2/10 — OMS is view-only for paper trading, blind for live trading**
