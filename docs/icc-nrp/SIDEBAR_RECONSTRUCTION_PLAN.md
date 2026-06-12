# SIDEBAR RECONSTRUCTION PLAN — ICC-NRP

## Primary Navigation Surface

**Component:** `M3AppShell` (`client/src/components/ui/M3AppShell.tsx`)

### Section Labels
- Line 60: `Trading` — renders `TRADING_NAV`
- Line 64: `Monitor` — renders `MONITOR_NAV`

### Count Badge Wiring

```
MongoDB strategy_authority_profiles
  → getPipelineCounts() [strategyAuthorityMongo.ts:691]
  → GET /api/strategy-authority/counts [counts/route.ts:12]
  → usePipelineCounts() [hooks/usePipelineCounts.ts:18]
  → formatNavCount() [hooks/usePipelineCounts.ts:44]
  → ShellNavLink count prop [M3AppShell.tsx:61–66, 137–165]
```

### Display Format

```
Mock Trading Engine (42)
Mock Trading Grade 1 (8)
...
```

Implementation: `M3AppShell.tsx:157–159`
```tsx
{item.label}
{count != null ? <span className="m3-nav-item__count"> ({count})</span> : null}
```

### Unavailable Authority

When MongoDB not configured or fetch fails:
- `usePipelineCounts` sets `hasAuthority: false` (`usePipelineCounts.ts:34–38`)
- `formatNavCount` returns `"—"` (`usePipelineCounts.ts:48`)
- No hardcoded fallback counts

### countStatus Mapping

| Nav Label | countStatus | navRoutes.ts |
|-----------|-------------|--------------|
| Mock Trading Engine | `MAIN_ENGINE` | line 109 |
| Mock Trading Grade 1 | `GRADE_1` | line 110 |
| Mock Trading Grade 2 | `GRADE_2` | line 111 |
| Mock Trading Grade 3 | `GRADE_3` | line 112 |
| Mock Trading Grade 4 | `GRADE_4` | line 113 |
| Mock Trading Grade 5 | `GRADE_5` | line 114 |

## Secondary Navigation Surface

**Component:** `Sidebar.tsx` — updated to mirror `TRADING_NAV` / `MONITOR_NAV` with same count hook (lines 143–170, 209–212).

## Visual Tokens

- Count styling: `m3-tokens.css` — `.m3-nav-item__count`
- Icons: `NavIcons.tsx` — cases `Mock Trading Engine`, `Mock Trading Grade 1`…`5`

## Collapsed Rail

Collapsed state hides labels (existing `m3-nav-rail--collapsed` rule, `m3-tokens.css:325`). Counts visible in `title` attribute on link (`M3AppShell.tsx:154`).
