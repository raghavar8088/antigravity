# COMMAND PALETTE UPDATE PLAN — ICC-NRP

## Provider Location

`CommandPaletteProvider` wraps app shell in `M3AppShell.tsx:47`.

Trigger: `CommandPaletteTrigger` in top app bar (`M3AppShell.tsx:102`).

## Item Registry

**File:** `client/src/lib/commandPaletteItems.ts`

### Trading Pipeline Items (auto-derived)

```13:24:client/src/lib/commandPaletteItems.ts
...COMMAND_CENTER_NAV.map((item) => ({
  group: item.section === "trading" ? "Trading Pipeline" : "Navigate",
  ...
}))
```

### Required Search Targets — All Present

| Label | href | Group |
|-------|------|-------|
| Mock Trading Engine | `/terminal/mock-engine` | Trading Pipeline |
| Mock Trading Grade 1 | `/terminal/grade-1` | Trading Pipeline |
| Mock Trading Grade 2 | `/terminal/grade-2` | Trading Pipeline |
| Mock Trading Grade 3 | `/terminal/grade-3` | Trading Pipeline |
| Mock Trading Grade 4 | `/terminal/grade-4` | Trading Pipeline |
| Mock Trading Grade 5 | `/terminal/grade-5` | Trading Pipeline |

### Keywords

Each trading item includes: `grade`, `mock trading`, `pipeline` (grade items) or `engine`, `main engine` (engine item) — `commandPaletteItems.ts:18–23`.

### Execution Desk (non-nav, preserved)

`Open Mock Trading Desk (execution)` → `/mock-trading` — `commandPaletteItems.ts:31–36`

## Test Coverage

`commandPaletteItems.test.ts` — verifies all `TRADING_NAV` hrefs appear in palette with labels `Mock Trading Engine` and `Mock Trading Grade 5`.

## Navigation from Search

Each item has `href` — `CommandPalette` navigates via Next.js router (existing behavior, items now include all 6 pipeline routes).
