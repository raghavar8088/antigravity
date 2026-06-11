# PHASE 3 — OPERATIONAL VISIBILITY REPORT
## Forensic Audit | Trading Platform | 2026-06-11

---

## SCORING KEY
- VISIBLE: Confirmed in source code, live data, user-facing
- PARTIAL: Exists but limited (mock data / view-only / missing context)
- ABSENT: No UI representation found anywhere in codebase

---

## VISIBILITY AUDIT BY SYSTEM

### 1. ENGINE STATUS
**Verdict: PARTIAL**

Evidence:
- `usePaperDesk()` exposes `connection: PaperDeskConnection` with states: `"connecting" | "live" | "stale" | "error" | "unauthorized"` — `client/src/hooks/usePaperDesk.ts:54`
- Paper Desk Dashboard renders connection status indicator based on `connection` state
- `/api/system/health` route exists but no component polls or renders it
- Go Engine `/health` endpoint is in the engine proxy READ_PATHS allowlist but no UI auto-polls it
- Terminal module: no engine status indicator (mock data appears always "connected")
- **Gap**: No persistent engine health badge on any page. No uptime display. No last-seen-alive timestamp for Go engine.

### 2. EXECUTION STATUS
**Verdict: PARTIAL**

Evidence:
- Paper Desk shows recent trades with entry/exit, strategy, PnL — `client/src/components/PaperDeskDashboard.tsx`
- BTC Futures dashboard shows active trades with hold times, exit reasons, PnL
- Terminal execution center shows positions (mock data only)
- **Gap**: No real-time order status feed from Go OMS (submitted → pending → filled). Orders are visible after the fact. No rejected-order alert.

### 3. STRATEGY STATUS
**Verdict: PARTIAL**

Evidence:
- Paper Desk: `HealthSummary` shows counts: `{healthy: N, warning: N, critical: N, insufficient_data: N}` — `client/src/hooks/usePaperDesk.ts:36`
- Strategy health fetched via `/api/paper-desk/strategy-health` (lazy tab)
- Terminal Research Center: 5 hardcoded strategies with health labels (ACTIVE, WATCHLIST, DISABLED)
- **Gap**: No list of all 600+ strategies with per-strategy status. Cannot see which strategies are currently active vs disabled vs suspended. Cannot see which specific strategies are in "critical" health.

### 4. OMS STATUS
**Verdict: PARTIAL**

Evidence:
- `PaperOmsPanel.tsx` (15KB) renders paper OMS orders — accessible as a tab in Paper Desk
- `/api/paper-oms/orders` and `/api/paper-oms/summary` exist
- Engine proxy allowlists `/api/paper-desk/oms` under ADMIN_PATHS
- **Gap**: No real-time OMS order status stream. No order rejection reasons. No partial fill indicators. OMS panel is view-only — no order management actions.

### 5. RISK STATUS
**Verdict: PARTIAL (mock only for primary terminal)**

Evidence:
- Terminal Risk Module (`/terminal/risk`): VaR 95/99, CVaR, heat, drawdown — ALL MOCK DATA
- Paper Desk: drawdown metric shown in summary KPIs (live)
- `MockRiskAnalyticsPanel`: kill-switch conditions evaluated locally against mock account
- `InstitutionalRiskCenter`: dead code, never rendered
- **Gap**: No live risk dashboard backed by Go engine risk gate. The only live risk data is paper desk drawdown.

### 6. PORTFOLIO STATUS
**Verdict: PARTIAL**

Evidence:
- Paper Desk: balance, equity, unrealized PnL, realized PnL, peak equity, drawdown — live from MongoDB
- Terminal Execution Center: Net/Gross exposure — MOCK DATA
- **Gap**: No aggregate portfolio view across all instruments (BTC + NIFTY + options simultaneously). No multi-account portfolio.

### 7. BROKER STATUS
**Verdict: ABSENT**

Evidence:
- Searched entire `client/src/` for: `data.feed`, `feed.fail`, `broker.down`, `broker.*status`, `BrokerStatus`, `feedStatus` — **zero results in UI components**
- `client/src/lib/btcResearchStrategyRegistry.ts` mentions broker fallbacks but in research context only
- AngelOne, Binance, Delta Exchange, Coinbase — none have a live connectivity indicator
- No badge, indicator, or alert that shows whether any broker is connected, degraded, or down
- **This is a critical operational gap for autonomous 72-hour operation**

### 8. KILL SWITCH STATUS
**Verdict: ABSENT (action button exists, status display does not)**

Evidence:
- `DashboardHeader.tsx:264` — `"/api/admin/kill"` button exists with confirm dialog
- `DashboardHeader.tsx:273` — button label changes to "Stopping…" while in-flight
- **NO component polls `/api/admin/ks/status` to show current kill switch state**
- Engine proxy READ_PATHS includes `/api/admin/ks/status` but no UI hook calls it
- `MockRiskAnalyticsPanel` has local kill-switch simulation but this is mock/research only
- A trader cannot tell from any dashboard whether the kill switch is currently ACTIVE or INACTIVE
- **This is the single most dangerous operational gap**

### 9. RECONCILIATION STATUS
**Verdict: ABSENT**

Evidence:
- Grep for "reconcili" across all `client/src/**/*.{tsx,ts}` — **zero results**
- `engine/internal/reconciliation/` exists in Go engine
- No route, component, hook, or page exposes reconciliation state, drift, mismatch alerts, or auto-healing events
- **Complete blind spot**

### 10. WATCHDOG STATUS
**Verdict: ABSENT**

Evidence:
- `engine/cmd/antigravity/main.go` has execution watchdog (per CLAUDE.md)
- No UI component shows watchdog alive/dead status
- No "last watchdog ping" timestamp visible anywhere

### 11. DATA FEED STATUS
**Verdict: ABSENT**

Evidence:
- Coinbase WS, Binance REST, AngelOne, Delta Exchange — none have a UI connectivity indicator
- No `useFeedHealth()` hook
- No feed latency display
- No stale-data warning (except implicit: paper desk connection badge showing "stale" if API fails)

### 12. HEALTH STATUS (System-Level)
**Verdict: PARTIAL**

Evidence:
- `/api/system/health` route exists (`client/src/app/api/system/health/`)
- `/api/storage/health` route exists, consumed by `StorageHealthPanel.tsx`
- `StorageHealthPanel.tsx` (12KB) — shows storage backend health
- **Gap**: No navigation to `StorageHealthPanel` confirmed in main dashboards. System health not surfaced in primary operator view.

---

## OPERATIONAL VISIBILITY SCORECARD

| System | Visible | Partial | Absent |
|--------|---------|---------|--------|
| Engine Status | | ✓ | |
| Execution Status | | ✓ | |
| Strategy Status | | ✓ | |
| OMS Status | | ✓ | |
| Risk Status | | ✓ (mock) | |
| Portfolio Status | | ✓ | |
| Broker Status | | | **✗** |
| Kill Switch Status | | | **✗** |
| Reconciliation Status | | | **✗** |
| Watchdog Status | | | **✗** |
| Data Feed Status | | | **✗** |
| Health Status | | ✓ | |

**5 of 12 critical operational systems have ZERO UI visibility.**

---

## CRITICAL FINDING: KILL SWITCH BLIND SPOT

A trader can press the kill button but cannot see whether the kill switch was successfully activated or is currently active. If the kill switch fires automatically (circuit breaker) the UI has no mechanism to alert the trader. This means:

1. Autonomous trading could halt silently with no UI notification
2. A trader resuming a session has no way to know if the kill switch was triggered while they were away
3. There is no "reset kill switch" confirmation flow — only a raw `/api/admin/kill` POST

**Severity: CRITICAL**
**Files**: `client/src/components/DashboardHeader.tsx:264`, `client/src/app/api/engine/[...path]/route.ts:75`
