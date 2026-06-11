# GOOGLE UI FORENSIC AUDIT — GM3-ICCTP Phase 1

**Date:** 2026-06-11  
**Scope:** `/terminal/*` routes, shell components, institutional pages  
**Method:** Source-code only (no doc assumptions)

---

## Executive Verdict

| Dimension | Score | Status |
|-----------|-------|--------|
| UI Quality | 42/100 | Material UX debt remains |
| Design Consistency | 35/100 | Three parallel styling systems |
| Navigation Unity | 38/100 | Dual shell architectures |
| Information Hierarchy | 55/100 | Modern pages OK; legacy pages poor |
| First Impression | 40/100 | Mixed Bloomberg + inline slate dashboards |

**VERDICT:** VERDICT 4 — UI REQUIRES MAJOR RECONSTRUCTION (in progress via GM3-ICCTP)

---

## Critical Finding: Dual Shell Architecture

| Shell | File | Used By | CSS System |
|-------|------|---------|------------|
| `InstitutionalTerminalShell` | `TerminalShell.tsx` L12–86 | All `/terminal/*` | Was Tailwind hex; **now M3 nav rail** |
| `AppShell` + `Sidebar` + `TopBar` | `AppShell.tsx` L73–118 | `MockTradingDashboard.tsx` L1529 only | `globals.css` classes |

**Impact:** Operators see completely different navigation on Mock Trading vs Command Center.  
**Fix applied:** Phase 4 reconstructed `TerminalShell.tsx` to Google Cloud Console nav rail + top app bar.

---

## Per-Page Audit Summary

### Command Center (`/terminal`)
- **File:** `CommandCenterHome.tsx` L20–157
- **Issue:** Duplicate `RiskRibbon` (shell + page) — **FIXED** (removed page copy L22–24)
- **Issue:** KPI grid cramped at `xl:grid-cols-8` — **FIXED** (`m3-kpi-strip` auto-fill)
- **Components:** Modern `TerminalCard`/`Metric` — migrated to M3 tokens

### Strategies (`/terminal/strategies`)
- **File:** `StrategyIntelligenceDashboard.tsx` L143–341
- **Issue:** 100% inline styles, slate palette `#0f172a` — **PENDING** Phase 8 migration
- **Issue:** Duplicates `/terminal/research` functionally
- **Fix:** Rebuild with `TerminalCard` + terminal snapshot (see STRATEGY_CENTER_V4.md)

### Portfolio (`/terminal/portfolio`)
- **File:** `PortfolioAnalyticsDashboard.tsx` L234–240
- **Issue:** Inline `outerStyle`, deprecated `/api/paper-desk/*` — **PENDING**
- **Fix:** Rebuild with M3 Card + mock-trading authority APIs

### Analytics (`/terminal/analytics`)
- **File:** `AnalyticsCenter.tsx` L14–98
- **Status:** Modern — minor chart color hardcoding L25
- **Score:** 72/100

### Risk (`/terminal/risk`)
- **File:** `RiskModule.tsx` L33–147
- **Issue:** Drawdown uses `tone="positive"` L40 — semantically wrong
- **Fix:** Change to `"warning"` or `"negative"`

### Research (`/terminal/research`)
- **File:** `ResearchCenter.tsx` L32–94
- **Status:** Modern institutional
- **Issue:** Stale column header `"Sharpe (N/A)"` L46

### Events (`/terminal/events`)
- **File:** `EventCenter.tsx` L107–232
- **Issue:** Legacy inline dashboard — **PENDING** Phase 9
- **Fix:** Rebuild as Google Cloud Logs Explorer pattern

### Health / Diagnostics / Settings
- **Files:** `HealthCenter.tsx`, `DiagnosticsCenter.tsx`, `settings/page.tsx`
- **Status:** Modern shell; Settings is stub content
- **Issue:** Health loading uses text "LOADING..." not skeleton

### Execution (`/terminal/execution`)
- **File:** `ExecutionCenter.tsx` L19–129
- **Status:** Richest modern page — 78/100
- **Issue:** Chart height hardcoded 520px L59

---

## Component Audit

| Component | Path | Issue | Priority |
|-----------|------|-------|----------|
| `RiskRibbon` | `RiskRibbon.tsx` L89–140 | 100% inline styles, not M3 | P1 |
| `TerminalCard` | `TerminalCard.tsx` | Was zinc Tailwind — **FIXED** to M3 | Done |
| `Sidebar` | `Sidebar.tsx` | Unused by /terminal; inline styles L224–238 | P2 |
| `TopBar` | `TopBar.tsx` | Unused by /terminal; incomplete PAGE_TITLES | P2 |
| `TerminalAuthorityGuard` | L23–30 | Over-blocks self-fetching pages | P1 |

---

## Navigation Issues

1. Horizontal tab bar duplicated sidebar concept — **FIXED** with M3 nav rail
2. Mock Trading separated from monitor nav — preserved as "Trading" section in rail
3. No global search — **FIXED** with Cmd+K command palette
4. No page titles in header — **FIXED** via `resolvePageTitle()` in `commandPaletteItems.ts`

---

## Evidence Index

All findings trace to:
- `client/src/components/terminal/institutional/TerminalShell.tsx`
- `client/src/components/terminal/institutional/CommandCenterHome.tsx`
- `client/src/components/terminal/AppShell.tsx`
- `client/src/app/terminal/*/page.tsx`
- `client/src/app/globals.css` L11–83
