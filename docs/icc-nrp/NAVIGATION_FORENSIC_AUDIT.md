# NAVIGATION FORENSIC AUDIT — ICC-NRP

**Audit date:** 2026-06-12  
**Method:** Source-code only. No README/trust.

---

## Executive Finding (Pre-Reconstruction)

The Institutional Command Center navigation **did not** expose the institutional strategy lifecycle. A single `Mock Trading` link at `/mock-trading` sat in TRADING; all monitor surfaces were flat under one section. Grade pipeline, promotion flow, and main-engine population were buried inside Strategy Authority tabs — not first-class navigation.

---

## Navigation Control Files (Evidence)

| File | Function / Export | Lines | Role |
|------|-----------------|-------|------|
| `client/src/lib/navRoutes.ts` | `COMMAND_CENTER_NAV`, `TERMINAL_ROUTES` | 7–135 | **Canonical route registry** |
| `client/src/lib/commandPaletteItems.ts` | `COMMAND_PALETTE_ITEMS`, `PAGE_TITLES` | 12–86 | Ctrl+K search registry |
| `client/src/components/ui/M3AppShell.tsx` | `M3AppShell`, `ShellNavLink` | 27–165 | **Primary nav rail** (Terminal layout) |
| `client/src/components/terminal/Sidebar.tsx` | `Sidebar`, `NAV_SECTIONS` | 143–248 | Legacy/alternate sidebar |
| `client/src/components/terminal/institutional/TerminalShell.tsx` | `InstitutionalTerminalShell` | 9–30 | Wraps `M3AppShell` |
| `client/src/components/terminal/institutional/TerminalLayoutClient.tsx` | `TerminalLayoutClient` | 20–26 | Terminal layout provider |
| `client/src/app/terminal/layout.tsx` | `TerminalLayout` | 4–6 | Route layout binding |
| `client/src/components/terminal/institutional/NavIcons.tsx` | `NavIcon` | 21–175 | Rail icon mapping |

**Reachability proof:** All `/terminal/*` pages render inside `TerminalLayout` → `TerminalLayoutClient` → `InstitutionalTerminalShell` → `M3AppShell` (lines above). Navigation rail is rendered in `M3AppShell` lines 49–67.

---

## Pre-Reconstruction State

### `navRoutes.ts` (before ICC-NRP)

```90:105:client/src/lib/navRoutes.ts
// PRE-RECONSTRUCTION (replaced):
// TRADING: Mock Trading only (/mock-trading)
// MONITOR: 13 items — no grade pipeline
```

- **TRADING:** 1 item — `Mock Trading` → `/mock-trading` (execution desk, not pipeline stage view)
- **MONITOR:** 13 items — correct names but section label in `Sidebar.tsx` was `"Command Center"` not `"Monitor"` (line 154 pre-change)
- **Missing routes:** `/terminal/mock-engine`, `/terminal/grade-1` … `/terminal/grade-5` — **0 files** (glob search returned empty)
- **Counts:** Sidebar supported `badge` prop (Sidebar.tsx:210) but **never wired** to MongoDB for grade counts

### Command Palette (before)

- Derived from `COMMAND_CENTER_NAV` — included only `Mock Trading`, not grade pipeline items
- File: `commandPaletteItems.ts` lines 12–19

### Data Authority (existing, unused for nav)

| API | Mongo Function | File:Lines |
|-----|---------------|------------|
| `GET /api/strategy-authority/counts` | `getPipelineCounts`, `getTowerCounts` | `counts/route.ts:7–19`, `strategyAuthorityMongo.ts:691–757` |
| `GET /api/strategy-authority/stages` | `getPipelineState` | `stages/route.ts:7–19` |

Counts API existed but **was not consumed by navigation components** before ICC-NRP.

---

## Gap Summary

| Requirement | Pre-State | Verdict |
|-------------|-----------|---------|
| 6 TRADING nav items | 1 item | **FAIL** |
| Grade routes | None | **FAIL** |
| MongoDB sidebar counts | Not wired | **FAIL** |
| Pipeline visual on grade pages | Only in Strategy Authority tab | **FAIL** |
| Command palette grade search | Not present | **FAIL** |
| MONITOR mandatory order | Present (13 items) | **PASS** |

---

## Post-Reconstruction State (see IMPLEMENTATION_EXECUTION_REPORT.md)

All gaps addressed in this program execution.
