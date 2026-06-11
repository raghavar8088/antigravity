# PAPER DESK DEPENDENCY MAP — ICCRP V3

**Audit date:** 2026-06-11  
**Method:** Source-only forensic grep + route reachability tracing  
**Scope:** `client/src/`, `client/src/app/`

---

## Executive Summary

| Layer | Paper Desk UI | Paper Desk API | Removal Risk |
|-------|---------------|----------------|--------------|
| Routes (pages) | **REDIRECTED** → `/terminal/*` | N/A | LOW — redirects in place |
| Navigation | **REMOVED** from Sidebar + TerminalShell | N/A | LOW |
| Components | Legacy files remain, **unreachable** | N/A | MEDIUM — dead code |
| API routes | N/A | **RETAINED** (Mongo authority layer) | HIGH if renamed prematurely |
| State/store | Uses `/api/paper-desk/snapshot` as data pipe | Backend naming | LOW — rename is cosmetic |

---

## UI Routes (Reachability)

| File | Function | Route | Status | Removal Risk |
|------|----------|-------|--------|--------------|
| `client/src/app/paper-desk/page.tsx` | `PaperDeskLegacyPage` | `/paper-desk` | **REDIRECT** → `legacyPaperDeskRedirect()` L8-11 | LOW |
| `client/src/app/paperdesk/page.tsx` | `PaperdeskAliasPage` | `/paperdesk` | **REDIRECT** L8-11 | LOW |
| `client/src/app/page.tsx` | `Home` | `/` | **REDIRECT** → `/terminal` L3-5 | LOW |
| `client/src/app/terminal/page.tsx` | `CommandCenterPage` | `/terminal` | **ACTIVE** Command Center home | N/A |

**Proof:** Build output shows `/paper-desk` and `/paperdesk` as `ƒ` (dynamic redirect), `/terminal` as `○` (static shell).

---

## Navigation References

| File | Lines | Usage | Action Taken | Risk |
|------|-------|-------|--------------|------|
| `client/src/components/terminal/Sidebar.tsx` | L140-157 | Was "Paper Desk" nav item | **REPLACED** with `COMMAND_CENTER_NAV` | LOW |
| `client/src/components/terminal/institutional/TerminalShell.tsx` | L75-90 | Terminal sub-nav | **UPDATED** — no Paper Desk label | LOW |
| `client/src/lib/navRoutes.ts` | L1-80 | Route constants | **REFACTORED** — `COMMAND_CENTER_PATH`, `legacyPaperDeskRedirect()` | LOW |
| `client/src/components/terminal/TopBar.tsx` | L30-35 | Page titles | **UPDATED** — "Command Center" | LOW |

---

## Legacy UI Components (Dead / Unreachable)

| File | Function | Reachability | Removal Risk |
|------|----------|--------------|--------------|
| `client/src/components/PaperDeskDashboard.tsx` | `PaperDeskDashboard` L144 | **UNREACHABLE** — `/paper-desk` redirects | MEDIUM — safe to delete in Phase 2 |
| `client/src/components/PaperDeskAuthBar.tsx` | Auth bar | Only imported by PaperDeskDashboard | MEDIUM |
| `client/src/components/TerminalDashboard.tsx` | `TerminalDashboard` L254 | **UNREACHABLE** — no app route imports | MEDIUM |
| `client/src/hooks/usePaperDesk.ts` | Data hook | Used by legacy dashboards only | LOW — API layer still valid |

---

## API Layer (Retained — Backend Authority)

| Route | File | Data Source | UI Consumer |
|-------|------|-------------|-------------|
| `/api/paper-desk/snapshot` | `client/src/app/api/paper-desk/snapshot/route.ts` | MongoDB via `paperDeskClient.ts` | `terminalStore.tsx` L93 REST poll |
| `/api/paper-desk/equity` | `equity/route.ts` | MongoDB | `terminalStore.tsx` L95 |
| `/api/paper-desk/trades` | `trades/route.ts` | MongoDB | Journal, Event Center |
| `/api/paper-desk/positions` | `positions/route.ts` | MongoDB | ExecutionCenter via snapshot |
| `/api/paper-desk/diagnostics` | `diagnostics/route.ts` | Go engine proxy | `DiagnosticsCenter.tsx` |
| `/api/cron/paper-desk-tick` | `cron/paper-desk-tick/route.ts` | Worker tick | Cron only |
| `/api/risk-ribbon` | `risk-ribbon/route.ts` | Mongo + engine | `RiskRibbon.tsx`, Command Center home |

**Data source proof:** `client/src/lib/paperDeskClient.ts` L1-2 — MongoDB query layer for Go Engine collections.

---

## Engine / Worker Dependencies

| File | Reference | Purpose | Removal Risk |
|------|-----------|---------|--------------|
| `client/src/lib/paperDeskWorker/runPaperDeskPollTick.ts` | Worker tick | Headless execution path | **CRITICAL** — do not remove |
| `client/scripts/btc-ft-paper-worker.ts` | Process entry | Long-running worker | **CRITICAL** |
| `engine/internal/paperpersist/` | Go persistence | Engine-side Mongo writes | **CRITICAL** |

---

## Middleware

| File | Line | Reference | Status |
|------|------|-----------|--------|
| `client/src/middleware.ts` | L99 | `/paper-desk`, `/paperdesk` in `PUBLIC_PATHS` | OK — allows redirect without auth block |

---

## Legacy Tab → Command Center Mapping

| Legacy Tab | Redirect Target | Proof |
|------------|-----------------|-------|
| `positions` | `/terminal/execution` | `navRoutes.ts` LEGACY_TAB_REDIRECTS |
| `trades` | `/terminal/journal` | same |
| `orders` | `/terminal/events` | same |
| `equity` | `/terminal/analytics` | same |
| `strategies` | `/terminal/strategies` | same |

---

## Verdict

**Paper Desk UI eradication:** COMPLETE for operator-visible surfaces.  
**Paper Desk API/worker layer:** RETAINED by design — feeds Command Center authority chain.
