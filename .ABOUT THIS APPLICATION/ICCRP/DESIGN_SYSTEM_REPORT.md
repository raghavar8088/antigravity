# DESIGN SYSTEM REPORT — ICCRP V3

**Date:** 2026-06-11

---

## Institutional Design Library

### Core Components

| Component | File | Purpose |
|-----------|------|---------|
| `TerminalCard` | `institutional/TerminalCard.tsx` | Panel container — 11px uppercase headers |
| `Metric` | same L30-52 | KPI tile — mono value + tone colors |
| `InstitutionalTerminalShell` | `TerminalShell.tsx` | App chrome + nav + ribbon |
| `InstitutionalChart` | `InstitutionalChart.tsx` | TradingView-style candle chart |
| `TerminalNoData` | `TerminalAuthorityGuard.tsx` | Authority-missing empty state |
| `RiskRibbon` | `RiskRibbon.tsx` | Status strip GREEN/AMBER/RED |
| `format.ts` | `institutional/format.ts` | `usd()`, `px()`, `pct()`, `pnlClass()` |

### Legacy Components (Retail — Not Design System)

| Component | Issue |
|-----------|-------|
| `KpiCard` in `TerminalDashboard.tsx` | Oversized, CSS vars retail theme |
| `DeskMetricTile` | Paper desk era |
| `EventCenter` inline styles | Not using TerminalCard |
| `StrategyIntelligenceDashboard` inline styles | Partial institutional |

---

## Tokens

| Token | Value | Usage |
|-------|-------|-------|
| `--green` / emerald-300 | PnL positive | Metric tone, depth rows |
| `--red` / rose-300 | PnL negative, critical | Alerts, ribbon |
| `--amber` / amber-300 | Warning | Drawdown, margin |
| `border-zinc-800` | Panel borders | All TerminalCard |
| `bg-[#080b10]` | Shell background | TerminalShell |

---

## Consistency Audit

| Element | Institutional | Legacy | Action |
|---------|---------------|--------|--------|
| Cards | TerminalCard | desk cards | Migrate EventCenter P2 |
| Tables | mono 10-12px in ExecutionCenter | mixed | OK in terminal |
| Badges | rounded-full uppercase 10px | varied | Standardize P3 |
| Buttons/Links | sky-400 text links | accent CSS vars | OK |

---

## Status: PARTIAL — Terminal path unified; legacy mock-trading path retains desk tokens
