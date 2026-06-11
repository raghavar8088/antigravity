# TERMINAL UX FORENSIC REPORT — ICCRP V3

**Date:** 2026-06-11  
**Auditor method:** Component + layout source review

---

## Scoring (1–10)

| Dimension | Before | After V3 | Evidence |
|-----------|--------|----------|----------|
| Information Density | 4 | 8 | Command Center 8-up KPI strip + 3-column intel grid |
| Operator Efficiency | 5 | 8 | Single nav bar, risk ribbon always visible |
| Decision Velocity | 4 | 7 | Risk ribbon + alerts on home; deep links to centers |
| Institutional Quality | 5 | 8 | Dark theme `#080b10`, compact `TerminalCard`, mono metrics |

---

## Violations Found (Before)

| Issue | File | Lines | Severity |
|-------|------|-------|----------|
| Paper Desk as primary product surface | `Sidebar.tsx` | L155-158 | CRITICAL |
| ExecutionCenter links to Paper Desk | `ExecutionCenter.tsx` | L107-109 | HIGH |
| Oversized legacy KPI cards | `TerminalDashboard.tsx` | L42-58 `KpiCard` | MEDIUM |
| Duplicate nav systems (AppShell + TerminalShell) | Both shells | — | MEDIUM |
| Dead space in mock-trading dashboard | `MockTradingDashboard.tsx` | ~1500 lines retail layout | MEDIUM |
| Low-density Paper Desk tabs | `PaperDeskDashboard.tsx` | L229+ | HIGH |

---

## Improvements Implemented

| Change | File | Impact |
|--------|------|--------|
| Risk ribbon embedded in shell header | `TerminalShell.tsx` L59-61 | Always-on system status |
| Command Center home with 8 KPIs + 4 quick links | `CommandCenterHome.tsx` | Bloomberg-style scan row |
| Compact `Metric` tiles (10px labels, mono values) | `TerminalCard.tsx` L30-52 | Higher density |
| 3-column execution layout (book \| chart \| alerts) | `ExecutionCenter.tsx` L20 | OMS-style |
| Institutional typography (`tracking-[0.16em]` headers) | `TerminalCard.tsx` L20 | Professional scan |

---

## Remaining UX Gaps

| Gap | Location | Priority |
|-----|----------|----------|
| `MockTradingDashboard` still retail-scale | `MockTradingDashboard.tsx` | P2 |
| `StrategyIntelligenceDashboard` inline styles vs TerminalCard | `StrategyIntelligenceDashboard.tsx` | P2 |
| `EventCenter` uses inline styles not design tokens | `EventCenter.tsx` | P3 |
| No multi-panel dock/resize (Bloomberg workspaces) | Terminal-wide | P3 |

---

## Operator Workflow (After)

```
Login → /terminal (Command Center Home)
  ├─ Risk Ribbon (engine, OMS, kill switch, PnL)
  ├─ KPI strip (equity, exposure, strategies, authority)
  └─ Deep links → Execution / Strategies / Portfolio / Risk / Events
```

**Reachability:** `app/page.tsx` → `/terminal` → `CommandCenterHome`
