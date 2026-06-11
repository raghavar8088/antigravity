# PHASE 16 — MODERNIZATION OPPORTUNITIES
## Forensic Audit | Trading Platform | 2026-06-11

---

## QUICK WINS (1–3 days each)

### QW-1: Fix Terminal Demo Data — CRITICAL SAFETY FIX
**Effort**: 2 hours
**Impact**: Eliminates the most dangerous false-confidence scenario in the system

Change `initialTerminalSnapshot` to empty/disconnected state:
```typescript
// current (dangerous):
connected: true, price: 105_842.5, positions: [fake...], ...

// correct:
connected: false, price: null, positions: [], risk: null, alerts: [
  { severity: "WARNING", title: "Not Connected", 
    message: "Waiting for engine WebSocket connection." }
], ...
```

A trader loading the terminal with no WS would immediately see a "Not Connected" state instead of plausible-looking fake data.

**Files**: `client/src/lib/terminal/terminalSnapshot.ts`

---

### QW-2: Add Kill Switch Status Polling
**Effort**: 4 hours
**Impact**: Eliminates the #1 operational visibility gap

Add a hook `useKillSwitchStatus()` that polls `/api/engine/api/admin/ks/status` every 10s. Render a persistent status chip in DashboardHeader and TopBar showing:
- 🟢 KS: INACTIVE
- 🔴 KS: ACTIVE
- ⚪ KS: UNKNOWN (engine unreachable)

**Files**: New hook + `DashboardHeader.tsx` + `AppShell.tsx` TopBar

---

### QW-3: Set ENGINE_ADMIN_SECRET in .env.local
**Effort**: 5 minutes
**Impact**: Unblocks all admin proxy operations (kill/block/release/OMS writes)

```bash
ENGINE_ADMIN_SECRET=<generate 32-char hex>
```

Without this, all admin engine operations silently return 503.

**Files**: `client/.env.local`, and add to Vercel environment variables

---

### QW-4: Add "DEMO MODE" Banner When WS Disconnected
**Effort**: 2 hours
**Impact**: Prevents traders from trusting mock data

In `AppShell.tsx` or `TopBar.tsx`, detect `snapshot.connected === false` and show a persistent amber banner:

```
⚠ ENGINE NOT CONNECTED — Data shown is demo/stale. Connect engine WebSocket to resume live monitoring.
```

**Files**: `client/src/components/terminal/AppShell.tsx` or `TopBar.tsx`

---

### QW-5: Add Persistent Last-Updated Timestamp to Paper Desk
**Effort**: 1 hour
**Impact**: Trader always knows data freshness

Show "Updated 2s ago" / "Updated 8s ago (stale)" next to Paper Desk summary header. Color-code: green < 6s, amber 6–15s, red > 15s.

**Files**: `client/src/components/PaperDeskDashboard.tsx`, data available from `usePaperDesk()`

---

### QW-6: Wire Strategy Health Detail to Show Names
**Effort**: 3 hours
**Impact**: "2 critical" becomes "ID-204 Volume Profile LVN: CRITICAL — Win rate dropped below threshold"

The health endpoint returns strategy IDs and health status. Map IDs to strategy names using `FUTURES_STRAT_DEFS` catalog. Show the critical strategies by name in the health tab.

**Files**: `client/src/components/PaperDeskDashboard.tsx`, health tab component

---

## MEDIUM IMPROVEMENTS (1–2 weeks each)

### MI-1: Reconciliation Status Page
**Effort**: 1 week
**Impact**: Closes the single largest operational blind spot

Requires new Go engine endpoint + new Next.js page. The UI layer alone can be built in 2 days — the backend endpoint is the dependency.

### MI-2: Persistent System Status Bar
**Effort**: 1 week
**Impact**: Every page becomes operationally aware

Polls: engine health, KS status, broker connectivity, alert count. Fixed height bar at top of every layout. Replaces the need to navigate to check individual system states.

### MI-3: Real-time Alert Feed
**Effort**: 1 week
**Impact**: Traders get notified of events without navigating

Replace terminal mock alert tape with a live alert component that:
- Polls `/api/system/health` + strategy health + KS status
- Surfaces new events as toasts and in a persistent feed
- Allows acknowledge / dismiss

### MI-4: OMS Order Management (Cancel)
**Effort**: 1 week
**Impact**: OMS becomes actionable, not just viewable

Add cancel button to PaperOmsPanel. Route to `/api/angelone/cancel-order` for live orders, and a new paper OMS cancel endpoint for paper orders.

### MI-5: Unite Terminal Modules Under Single Live Data Source
**Effort**: 2 weeks
**Impact**: All terminal pages show live data

Options (in order of effort):
1. Build a Go engine WebSocket endpoint that streams `TerminalSnapshot` deltas
2. Replace terminal's WebSocket with enhanced polling from real endpoints (paper desk data + engine metrics)

Option 2 is achievable without a new Go WebSocket endpoint: replace `useTerminalSnapshot()` with a composite hook that aggregates `/api/paper-desk/snapshot` + `/api/engine/api/admin/ks/status` + `/api/system/health` + Go engine `/metrics`.

---

## MAJOR REDESIGN OPPORTUNITIES (1–3 months)

### MR-1: System Command Center (New Home Route)
Full implementation of the blueprint in Phase 17. Requires new aggregation endpoints and redesigned home page. Highest operational impact redesign.

### MR-2: Strategy Administration Panel
A full 600-strategy management interface with health, PnL, enable/disable. Requires new Go engine endpoints for per-strategy state management.

### MR-3: Multi-Instrument Portfolio Dashboard
Aggregate BTC + NIFTY + MCX into one portfolio view. Requires a new portfolio aggregation service or endpoint.

### MR-4: Move Signal Engine to Go
The in-browser 171KB signal engine should eventually run server-side in Go (or already does in the Go engine). Removing it from the browser eliminates the CPU and memory concerns identified in Phase 14 and resolves the "two engines" confusion.

### MR-5: Unified Design System
Merge the two visual systems (terminal dark mode + paper desk light mode) into a single design system with proper dark/light theme switching. Adopt a shared token file.

---

## PRIORITY MATRIX

| Item | Impact | Effort | Priority |
|------|--------|--------|----------|
| QW-1: Fix demo data | CRITICAL safety | 2h | DO IMMEDIATELY |
| QW-3: ENGINE_ADMIN_SECRET | Unblocks admin ops | 5min | DO IMMEDIATELY |
| QW-2: KS status polling | High safety | 4h | THIS WEEK |
| QW-4: Demo mode banner | High safety | 2h | THIS WEEK |
| QW-5: Paper desk timestamp | Medium | 1h | THIS WEEK |
| QW-6: Strategy health names | Medium | 3h | THIS WEEK |
| MI-1: Reconciliation page | Critical | 1 week | NEXT SPRINT |
| MI-2: System status bar | High | 1 week | NEXT SPRINT |
| MI-3: Alert feed | High | 1 week | NEXT SPRINT |
| MI-5: Wire terminal to live data | Critical | 2 weeks | NEXT SPRINT |
| MR-1: Command center | Very high | 1 month | NEXT QUARTER |
