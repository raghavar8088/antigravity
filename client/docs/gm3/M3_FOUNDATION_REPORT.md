# M3 FOUNDATION REPORT — GM3-ICCTP Phase 3

**Deliverable:** `client/src/styles/m3-tokens.css`  
**Wired:** `globals.css` L2 `@import "../styles/m3-tokens.css"`

---

## Color System (M3 Semantic Roles)

| Role | Dark Token | Light Token |
|------|------------|-------------|
| Primary | `--m3-primary` #8ab4f8 | #1a73e8 |
| On Primary | `--m3-on-primary` | #ffffff |
| Primary Container | `--m3-primary-container` | #e8f0fe |
| Secondary | `--m3-secondary` | #5f6368 |
| Tertiary | `--m3-tertiary` | #188038 |
| Error | `--m3-error` | #d93025 |
| Surface | `--m3-surface` | #ffffff |
| Surface Container | `--m3-surface-container` | #f8f9fa |
| Outline | `--m3-outline-variant` | #c4c7c5 |
| Profit / Loss | `--m3-profit` / `--m3-loss` | Trading semantics |

---

## Typography Scale

| Token | Size/Weight |
|-------|-------------|
| Display lg/md/sm | 2.8125rem → 2rem, weight 700 |
| Headline lg/md/sm | 1.75rem → 1.25rem, weight 600 |
| Title lg/md/sm | 1.125rem → 0.875rem, weight 600 |
| Body lg/md/sm | 1rem → 0.75rem, weight 400 |
| Label lg/md/sm | 0.875rem → 0.6875rem, weight 500 |

**Base body:** 14px (overrides globals.css 13px)

---

## Spacing (8dp Grid)

`4 · 8 · 12 · 16 · 24 · 32 · 48 · 64` via `--m3-space-1` through `--m3-space-8`

---

## Motion

| Token | Value |
|-------|-------|
| `--m3-duration-short` | 150ms |
| `--m3-duration-medium` | 300ms |
| `--m3-easing-standard` | cubic-bezier(0.2, 0, 0, 1) |

`prefers-reduced-motion: reduce` disables transitions on nav rail.

---

## Elevation

`--m3-elevation-0` through `--m3-elevation-4` — dark-optimized shadows; light theme overrides included.

---

## Bridge Aliases

All `--desk-*` and legacy `:root` tokens (`--bg`, `--accent`, `--green`, etc.) alias to M3 values in `m3-tokens.css` L118–175.

---

## Layout Tokens

| Token | Value |
|-------|-------|
| `--m3-nav-rail-width` | 72px (collapsed) |
| `--m3-nav-rail-expanded` | 256px |
| `--m3-top-app-bar-height` | 64px |
| `--m3-content-max-width` | 1600px |

---

## Component Classes Included

- App shell: `.m3-app-shell`, `.m3-nav-rail`, `.m3-top-app-bar`, `.m3-content-area`
- Surfaces: `.m3-surface-card`, `.m3-metric-tile`
- Interactive: `.m3-btn`, `.m3-icon-btn`, `.m3-status-chip`, `.m3-chip`
- Feedback: `.m3-empty-state`, `.m3-skeleton`
- Command palette: `.m3-cmdk-*`, `.m3-search-trigger`

---

## Migration Map

| Old | New |
|-----|-----|
| `bg-[#080b10]` | `var(--m3-surface-dim)` |
| `border-zinc-800` | `var(--m3-outline-variant)` |
| `text-sky-400` | `var(--m3-primary)` / `.m3-link-action` |
| `text-emerald-400` | `var(--m3-profit)` |
| `--accent` #2962ff | `--m3-primary` |
