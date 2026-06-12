# ROUTE IMPLEMENTATION PLAN — ICC-NRP

## Routes Created

| Route | Page File | Component | ISPAP Status |
|-------|-----------|-----------|--------------|
| `/terminal/mock-engine` | `app/terminal/mock-engine/page.tsx:5–7` | `MockEngineCenter` | `MAIN_ENGINE` |
| `/terminal/grade-1` | `app/terminal/grade-1/page.tsx:5–7` | `GradeStageCenter status="GRADE_1"` | `GRADE_1` |
| `/terminal/grade-2` | `app/terminal/grade-2/page.tsx:5–7` | `GradeStageCenter status="GRADE_2"` | `GRADE_2` |
| `/terminal/grade-3` | `app/terminal/grade-3/page.tsx:5–7` | `GradeStageCenter status="GRADE_3"` | `GRADE_3` |
| `/terminal/grade-4` | `app/terminal/grade-4/page.tsx:5–7` | `GradeStageCenter status="GRADE_4"` | `GRADE_4` |
| `/terminal/grade-5` | `app/terminal/grade-5/page.tsx:5–7` | `GradeStageCenter status="GRADE_5"` | `GRADE_5` |

**Layout chain (reachability):**
`app/terminal/layout.tsx:4–6` → `TerminalLayoutClient` → `InstitutionalTerminalShell` → `M3AppShell`

## Routes Pre-Existing (unchanged)

| Route | Page | Registry |
|-------|------|----------|
| `/terminal` | `app/terminal/page.tsx` | `TERMINAL_ROUTES.home` |
| `/terminal/execution` | `app/terminal/execution/page.tsx` | line 17 |
| `/terminal/strategy-authority` | `app/terminal/strategy-authority/page.tsx` | line 18 |
| `/terminal/portfolio-intelligence` | `app/terminal/portfolio-intelligence/page.tsx` | line 19 |
| … (all MONITOR_NAV items) | respective `page.tsx` | `navRoutes.ts:118–131` |

## API Routes

| Endpoint | Handler | Data Source |
|----------|---------|-------------|
| `GET /api/strategy-authority/stage?status=` | `stage/route.ts:12–34` | `getStrategiesByStatus()` |
| `GET /api/strategy-authority/counts` | `counts/route.ts:7–19` | `getPipelineCounts()` |
| `GET /api/strategy-authority/allocation` | existing | allocation engine |
| `GET /api/strategy-authority/main-engine` | existing | `getMainEngineStrategies()` |

## Route Registry

```107:135:client/src/lib/navRoutes.ts
export const TRADING_NAV: CommandCenterNavItem[] = [ ... ];
export const MONITOR_NAV: CommandCenterNavItem[] = [ ... ];
export const COMMAND_CENTER_NAV = [...TRADING_NAV, ...MONITOR_NAV];
```

## Page Titles (top app bar)

```62:68:client/src/lib/commandPaletteItems.ts
[TERMINAL_ROUTES["mock-engine"]]: "Mock Trading Engine",
[TERMINAL_ROUTES["grade-1"]]: "Mock Trading Grade 1",
...
```

## Direct URL Access

All routes are standard Next.js App Router pages under `app/terminal/` — direct URL access verified by file existence and `isTerminalRoute("/terminal/grade-3")` test (`navRoutes.test.ts:95`).
