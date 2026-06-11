# COMPONENT RATIONALIZATION REPORT — ICCRP V3

---

## Legacy (Remove Phase 2)

| Component | Route Reachable | Lines | Recommendation |
|-----------|-----------------|-------|----------------|
| `PaperDeskDashboard.tsx` | **NO** — redirect | 667 | DELETE |
| `PaperDeskAuthBar.tsx` | NO | — | DELETE with dashboard |
| `TerminalDashboard.tsx` | **NO** — no app route | 744 | DELETE or archive |
| `desk/ui/*` | Paper desk only | — | DELETE after dashboard |

---

## Duplicate

| A | B | Resolution |
|---|---|------------|
| `AppShell` Sidebar nav | `TerminalShell` nav | Terminal is canonical; AppShell mirrors `COMMAND_CENTER_NAV` |
| `TerminalDashboard` | `CommandCenterHome` | CommandCenterHome replaces |
| Risk in `RiskModule` + `RiskRibbon` | Complementary | Keep both — ribbon=summary, page=detail |

---

## Active Institutional Components

| Component | Route |
|-----------|-------|
| `CommandCenterHome` | `/terminal` |
| `ExecutionCenter` | `/terminal/execution` |
| `StrategyIntelligenceDashboard` | `/terminal/strategies` |
| `PortfolioAnalyticsDashboard` | `/terminal/portfolio` |
| `RiskModule` | `/terminal/risk` |
| `AnalyticsCenter` | `/terminal/analytics` |
| `ResearchCenter` | `/terminal/research` |
| `EventCenter` | `/terminal/events` |
| `HealthCenter` | `/terminal/health` |
| `DiagnosticsCenter` | `/terminal/diagnostics` |
| `ObservabilityCenter` | `/terminal/observability` |

---

## Dead Hooks (Legacy UI only)

| Hook | Still Needed |
|------|--------------|
| `usePaperDesk.ts` | API layer — YES for mock-trading |
| `usePaperDeskAuth.ts` | Legacy — review |

---

## Dead Routes

| Route | Status |
|-------|--------|
| `/paper-desk` | Redirect only — not a product surface |
| `/paperdesk` | Redirect only |

---

## API Routes (NOT dead — backend authority)

All `/api/paper-desk/*` routes retained — feed Command Center via `terminalStore`.

---

## Status: UI rationalization 70% — delete unreachable legacy components in next sprint
