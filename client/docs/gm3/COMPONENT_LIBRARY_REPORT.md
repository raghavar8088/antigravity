# COMPONENT LIBRARY REPORT — GM3-ICCTP Phase 11

**Location:** `client/src/components/ui/`

---

## Primitives Delivered

| Component | File | M3 Pattern | a11y |
|-----------|------|------------|------|
| Button | `Button.tsx` | filled/tonal/outlined/text/danger | focus-visible via CSS |
| Card + Metric | `Card.tsx` | Surface container + metric tile | semantic `<section>` |
| StatusChip | `StatusChip.tsx` | Google Cloud status pills | color + dot indicator |
| Chip / Badge | `StatusChip.tsx` | Filter chips | button role when clickable |
| Skeleton | `Skeleton.tsx` | Shimmer loading | `aria-hidden`, `aria-busy` on card |
| EmptyState | `EmptyState.tsx` | Google Cloud empty panels | `role="status"` |
| CommandPalette | `CommandPalette.tsx` | Workspace Cmd+K | Radix Dialog + cmdk |
| ThemeProvider | `ThemeProvider.tsx` | Light/dark toggle | `data-theme` on `<html>` |

---

## Radix UI Usage

| Package | Used For |
|---------|----------|
| `@radix-ui/react-dialog` | Command palette overlay |
| `@radix-ui/react-slot` | Button `asChild` polymorphism |
| `cmdk` | Keyboard-navigable command list |

**Pending Radix adoption:** DropdownMenu (account), Tooltip (metric hints), Tabs (panel switching)

---

## Relationship to desk/ui

| desk/ui | ui/ Strategy |
|---------|--------------|
| `DeskCard` | Keep for mock-trading; terminal uses `Card` / `TerminalCard` |
| `DeskButton` | Converge styles to `.m3-btn` in Phase 17 |
| `DeskEmptyState` | Terminal uses `ui/EmptyState` |
| `DeskDataTable` | Extend in Phase 12 |

---

## Barrel Export

`client/src/components/ui/index.ts` — single import path for shell and pages.

---

## Tests

- `commandPaletteItems.test.ts` — 3 tests, nav coverage ✅
