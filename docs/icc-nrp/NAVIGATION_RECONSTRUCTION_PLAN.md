# NAVIGATION RECONSTRUCTION PLAN — ICC-NRP

## Target Information Architecture

```
TRADING (sidebar section label: "Trading")
├── Mock Trading Engine      /terminal/mock-engine     count: MAIN_ENGINE
├── Mock Trading Grade 1     /terminal/grade-1         count: GRADE_1
├── Mock Trading Grade 2     /terminal/grade-2         count: GRADE_2
├── Mock Trading Grade 3     /terminal/grade-3         count: GRADE_3
├── Mock Trading Grade 4     /terminal/grade-4         count: GRADE_4
└── Mock Trading Grade 5     /terminal/grade-5         count: GRADE_5

MONITOR (sidebar section label: "Monitor")
├── Command Center           /terminal
├── Execution                /terminal/execution
├── Strategy Authority       /terminal/strategy-authority
├── Portfolio Intelligence   /terminal/portfolio-intelligence
├── Strategies               /terminal/strategies
├── Portfolio                /terminal/portfolio
├── Risk                     /terminal/risk
├── Analytics                /terminal/analytics
├── Research                 /terminal/research
├── Events                   /terminal/events
├── Health                   /terminal/health
├── Diagnostics              /terminal/diagnostics
└── Settings                 /terminal/settings
```

## Operator Mental Model

Visual pipeline (PromotionTower, top→bottom):

**Grade 5 → Grade 4 → Grade 3 → Grade 2 → Grade 1 → Main Engine**

Sidebar lists Engine first (destination), then grades 1–5 (progression stages). Pipeline tower on each grade/engine page reinforces downward flow.

## Implementation Phases

### Phase 1 — Route Registry
- Split `COMMAND_CENTER_NAV` into `TRADING_NAV` + `MONITOR_NAV`
- Add `TERMINAL_ROUTES` keys: `mock-engine`, `grade-1`…`grade-5`
- File: `client/src/lib/navRoutes.ts`

### Phase 2 — Pages
- Create 6 Next.js pages under `client/src/app/terminal/`
- Grade pages → `GradeStageCenter` component
- Engine page → `MockEngineCenter` component

### Phase 3 — Data Layer
- Add `getStrategiesByStatus()` to `strategyAuthorityMongo.ts`
- Add `GET /api/strategy-authority/stage?status=GRADE_N`

### Phase 4 — Navigation UI
- `M3AppShell`: render `TRADING_NAV` / `MONITOR_NAV`, fetch counts via `usePipelineCounts`
- `Sidebar.tsx`: mirror structure for legacy shell
- `NavIcons.tsx`: icons for 6 trading items

### Phase 5 — Command Palette
- Auto-derive from `COMMAND_CENTER_NAV`
- Trading items grouped as `"Trading Pipeline"`

### Phase 6 — Certification
- Unit tests: `navRoutes.test.ts`, `commandPaletteItems.test.ts`
- Deliverable docs (this folder)

## Non-Goals (per mission brief)
- No new dashboard shell
- No trading engine / OMS / risk changes
- `/mock-trading` execution desk remains accessible via command palette alias only
