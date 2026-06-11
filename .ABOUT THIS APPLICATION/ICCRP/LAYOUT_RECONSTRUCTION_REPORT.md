# LAYOUT RECONSTRUCTION REPORT — ICCRP V3

---

## Breakpoints

| Breakpoint | Behavior | File Evidence |
|------------|----------|---------------|
| Mobile | Sidebar drawer via `AppShell` | `AppShell.tsx` L70-92 |
| `sm` (640px) | 2-col KPI grids | `CommandCenterHome.tsx` L27 |
| `lg` (1024px) | 4-col KPI, shell padding `lg:p-4` | `TerminalShell.tsx` L92 |
| `xl` (1280px) | 3-col execution grid, 3-col home panels | `ExecutionCenter.tsx` L20 |

---

## Multi-Monitor

| Setup | Support | Notes |
|-------|---------|-------|
| Laptop 13-15" | ✅ Nav scroll, stacked panels | Risk ribbon wraps |
| Ultrawide | ✅ 3-col execution fills width | Chart gets flex space |
| 4K | ⚠️ Panels don't scale beyond max-width | Add `2xl:grid-cols-4` P3 |
| Dual monitor | ✅ Open `/terminal/execution` + `/terminal/events` | Standard browser |

---

## Density Improvements

- Command Center: 8 KPIs in one row on xl (`xl:grid-cols-8`)
- Execution: fixed side columns 280px/320px — book + tape always visible
- Risk ribbon: horizontal chip strip — no vertical waste

---

## Status: LAPTOP + ULTRAWIDE READY — 4K max-width tuning deferred P3
