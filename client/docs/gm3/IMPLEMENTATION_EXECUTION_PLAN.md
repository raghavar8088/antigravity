# IMPLEMENTATION EXECUTION PLAN — GM3-ICCTP Phase 17

---

## Phase Completion Status

| Phase | Deliverable | Status |
|-------|-------------|--------|
| 1 | GOOGLE_UI_FORENSIC_AUDIT.md | ✅ |
| 2 | DESIGN_SYSTEM_FORENSIC_REPORT.md | ✅ |
| 3 | m3-tokens.css + M3_FOUNDATION_REPORT.md | ✅ |
| 4 | App shell reconstruction | ✅ Code + doc |
| 5 | Command Center V4 | ✅ Partial (KPI strip, M3 cards) |
| 6 | Strategy Center V4 | 📋 Blueprint only |
| 7 | Portfolio Center V4 | 📋 Blueprint only |
| 8 | Risk Center V4 | 📋 Minor fix pending |
| 9 | Event Center V4 | 📋 Blueprint only |
| 10 | Observability V4 | 📋 Blueprint only |
| 11 | ui/ component library | ✅ Core primitives |
| 12 | Table system | 📋 Pending |
| 13 | Command palette | ✅ Cmd+K live |
| 14 | Loading states | ✅ Skeleton + EmptyState |
| 15 | Accessibility | 📋 Pending |
| 16 | Performance | 📋 Pending |
| 17 | This document | ✅ |
| 18 | SCREEN_REBUILD_ROADMAP.md | ✅ |
| 19 | UI_ARCHITECTURE_TRANSFORMATION.md | ✅ |
| 20 | GM3_FINAL_CERTIFICATION.md | ✅ |

---

## Exact File Migration Order

```
1. src/styles/m3-tokens.css          [DONE]
2. src/app/globals.css               [DONE — import]
3. src/components/ui/*               [DONE]
4. TerminalShell.tsx                 [DONE]
5. TerminalCard.tsx                  [DONE]
6. CommandCenterHome.tsx             [DONE]
7. GlobalRiskRibbon.tsx              [DONE]
8. layout.tsx + ThemeProvider        [DONE]
9. EventCenter.tsx                   [NEXT]
10. PortfolioAnalyticsDashboard.tsx  [NEXT]
11. StrategyIntelligenceDashboard.tsx [NEXT]
12. RiskRibbon.tsx                   [NEXT — M3 inline → tokens]
13. AppShell.tsx + Sidebar.tsx       [LATER — mock-trading unification]
14. globals.gmail.css                [DELETE when safe]
```

---

## Token Replacement Guide

When editing any component, replace:

| Pattern | Replacement |
|---------|-------------|
| `#080b10`, `#0f172a`, `#0d1118` | `var(--m3-surface-dim)` or container tokens |
| `#30363d`, `#334155`, zinc borders | `var(--m3-outline-variant)` |
| `#2962ff`, `#3b82f6`, sky-* | `var(--m3-primary)` |
| `#22c55e`, emerald-* | `var(--m3-profit)` |
| `#ef4444`, rose-* | `var(--m3-loss)` |
| `font-size: 13px` body | `var(--m3-body-md)` / 14px |
| Inline `style={{}}` blocks | M3 CSS classes or ui/ components |

---

## Validation Criteria (Per PR)

1. `npm run test` — no new failures in touched modules
2. Visual check: /terminal, /terminal/execution, /terminal/risk
3. Cmd+K opens palette, navigates correctly
4. Theme toggle persists across reload
5. No PnL/funding/liquidation logic changes
6. Mobile nav drawer works at ≤1024px

---

## Feature Flag (Optional Rollout)

```env
NEXT_PUBLIC_UI_M3=1
```

Currently M3 is always on for `/terminal` via InstitutionalTerminalShell. Flag can gate mock-trading migration later.
