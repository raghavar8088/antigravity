# DESIGN SYSTEM FORENSIC REPORT — GM3-ICCTP Phase 2

**Date:** 2026-06-11

---

## Active Import Chain (Proven)

```
layout.tsx:3 → globals.css:1-4
  ├── tailwindcss
  ├── m3-tokens.css          ← NEW canonical (Phase 3)
  ├── desk-tokens.css
  └── desk-trading.css
```

**Terminal layout:** `terminal/layout.tsx` — no CSS import; inherits root chain.

---

## System Inventory

| System | File | Lines | Status |
|--------|------|-------|--------|
| Legacy terminal | `globals.css` :root | L11–83 | Aliased to M3 — deprecate gradually |
| M3 canonical | `m3-tokens.css` | 700+ | **ACTIVE — source of truth** |
| Desk M3 partial | `desk-tokens.css` | 219 | Aliased from m3-tokens |
| Desk trading domain | `desk-trading.css` | 373 | Active (AiAppTracker, desk chrome) |
| Gmail orphan | `globals.gmail.css` | 707 | **DEAD — never imported** |

---

## Token Conflicts (Proven)

| Legacy | Desk/M3 | Values Match? |
|--------|---------|---------------|
| `--accent` #2962ff | `--desk-primary` / `--m3-primary` | **NO** — unified to M3 |
| `--bg` #0d1117 | `--m3-surface-dim` | Yes |
| `--green` #26a69a | `--m3-profit` | Yes |

### Ghost Tokens (Undefined — Broke Mock Panels)

| Token | Used In | Fix |
|-------|---------|-----|
| `--desk-profit` | MockStrategyLeaderboardPanel | Aliased in m3-tokens.css |
| `--desk-loss` | MockRiskAnalyticsPanel | Aliased |
| `--desk-muted` | Mock panels | Aliased |
| `--desk-border` | MockStrategyLeaderboardPanel | Aliased |
| `--radius-sm` | DeltaSpotBuy.tsx | **Still undefined — add alias** |

---

## Three UI Architectures Coexisting

```
System A: globals.css classes → AppShell (mock-trading)
System B: --desk-* tokens → DeskCard, DeskButton (mock-trading panels)
System C: Tailwind hex literals → was InstitutionalTerminalShell (now M3)
```

---

## Recommendations (Implemented / Pending)

| # | Action | Status |
|---|--------|--------|
| 1 | Make `m3-tokens.css` canonical | ✅ Done |
| 2 | Alias legacy `:root` + `--desk-*` to M3 | ✅ Done |
| 3 | Fix ghost tokens | ✅ Done |
| 4 | Delete `globals.gmail.css` | Pending (safe to archive) |
| 5 | Wire theme toggle (`data-theme`) | ✅ ThemeProvider |
| 6 | Map Tailwind `@theme` to M3 | Pending Phase 17 |
| 7 | Remove unused workspace CSS (globals.css L474–576) | Pending |

---

## Package Audit

**Radix UI:** Installed Phase 11 — `@radix-ui/react-dialog`, `dropdown-menu`, `tooltip`, `tabs`, `slot`  
**cmdk:** Installed for command palette  
**No MUI/shadcn** — intentional; custom M3 + Radix a11y primitives
