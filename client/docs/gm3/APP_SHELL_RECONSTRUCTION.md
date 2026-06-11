# APP SHELL RECONSTRUCTION — GM3-ICCTP Phase 4

**Target:** Google Cloud Console layout  
**File:** `client/src/components/terminal/institutional/TerminalShell.tsx`

---

## Before (TerminalShell.tsx L19–84)

- Full-width sticky header with horizontal scroll tabs
- Hardcoded Tailwind: `bg-[#080b10]`, `border-zinc-800`, `sky-*`
- RiskRibbon embedded in header border-t
- No global search, no theme toggle, no collapsible nav

## After (Implemented)

```
┌─────────────────────────────────────────────────────────────┐
│ Nav Rail (256px)  │  Top App Bar (64px)                     │
│ ├ Trading         │  [≡] Page Title + BTC price             │
│ │ Mock Trading    │  [Search ⌘K]  [Status] [Theme]        │
│ ├ Monitor         ├─────────────────────────────────────────┤
│ │ Command Center  │  Risk Ribbon                            │
│ │ Execution       ├─────────────────────────────────────────┤
│ │ ...             │  Content Area (max 1600px, padded 24px) │
│ └ Collapse        │                                         │
└─────────────────────────────────────────────────────────────┘
```

---

## Components Delivered

| Piece | File | Description |
|-------|------|-------------|
| Nav Rail | `TerminalShell.tsx` L36–68 | Sectioned: Trading / Monitor |
| Top App Bar | `TerminalShell.tsx` L84–118 | Title, price, search, status chips |
| Command Palette | `ui/CommandPalette.tsx` | Cmd/Ctrl+K via cmdk + Radix Dialog |
| Theme Toggle | `ui/ThemeProvider.tsx` | `data-theme` light/dark + localStorage |
| Status Chips | `ui/StatusChip.tsx` | Authority + critical alert count |
| Nav Icons | `NavIcons.tsx` | SVG icons per route |
| Global Risk Ribbon | `GlobalRiskRibbon.tsx` | Hides on /terminal to prevent duplicate |

---

## Responsive Behavior

| Breakpoint | Behavior |
|------------|----------|
| >1024px | Expanded nav rail, search visible |
| ≤1024px | Nav rail off-canvas; hamburger opens drawer |
| Collapsed rail | 72px icon-only mode |

---

## Page Title Resolution

`resolvePageTitle()` in `commandPaletteItems.ts` maps all `TERMINAL_ROUTES` to human titles.

---

## Remaining Work

1. Migrate `AppShell` (mock-trading) to same M3 shell — Phase 17
2. Style `RiskRibbon` with M3 tokens (still inline styles)
3. Add account menu dropdown (Radix DropdownMenu)
