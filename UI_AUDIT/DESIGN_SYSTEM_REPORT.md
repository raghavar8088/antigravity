# PHASE 15 — DESIGN SYSTEM REPORT
## Forensic Audit | Trading Platform | 2026-06-11

---

## DESIGN SYSTEM INVENTORY

### Component Library: `desk/ui/`
Located in `client/src/components/desk/ui/`

| Component | Purpose |
|-----------|---------|
| `DeskButton.tsx` | Primary action button |
| `DeskCard.tsx` | Content card container |
| `DeskChip.tsx` | Status/label chip with tone variants |
| `DeskDataTable.tsx` | Generic data table |
| `DeskEmptyState.tsx` | Empty state display |
| `DeskLinearProgress.tsx` | Progress bar |
| `DeskMetricTile.tsx` | KPI metric display tile |
| `DeskSearchField.tsx` | Search input |
| `DeskSectionHeader.tsx` | Section title with subtitle + actions |
| `DeskShell.tsx` | Page shell wrapper |
| `DeskSwitch.tsx` | Toggle switch |
| `DeskTabs.tsx` | Tab navigation |
| `DeskAppBar.tsx` | Top app bar |
| `DeskBanner.tsx` | System banner |
| `StatusBadge.tsx` | Status indicator badge |
| `cn.ts` | Classname utility |

### Terminal Component Library
Located in `client/src/components/terminal/institutional/`

| Component | Purpose |
|-----------|---------|
| `TerminalCard.tsx` | Dark-mode card container |
| `TerminalShell.tsx` | Terminal page shell |
| `AppShell.tsx` | Full application shell |
| `TopBar.tsx` | Navigation bar |
| `Sidebar.tsx` | Side navigation |

---

## CONSISTENCY AUDIT

### Issue 1: TWO VISUAL SYSTEMS — Light vs Dark
**Severity: HIGH**

Evidence:
- Terminal (`/`, `/terminal/*`): dark zinc palette (`bg-zinc-900`, `text-zinc-100`, `text-emerald-300`, `text-rose-300`)
- Paper Desk (`/paper-desk`): light palette (`border-zinc-200`, `bg-zinc-50/80`, `text-zinc-900`)
- These are not two themes of one system — they are two separate visual languages

A trader moving from Paper Desk to the Terminal is hit by a full contrast reversal with no transition. This creates cognitive load at the moment of context switching, which is exactly when traders are most stressed.

### Issue 2: NO SHARED TOKEN SYSTEM
**Severity: MEDIUM**

Evidence:
- Terminal uses hardcoded Tailwind classes directly in component JSX
- Paper Desk uses CSS variables (`var(--green)`, `var(--red)`, `var(--accent)`, `var(--text-secondary)`) in `DashboardHeader.tsx`
- `InstitutionalRiskCenter.tsx` mixes Tailwind utilities directly
- No shared design token file found (no `tokens.css` or `variables.css` in codebase map)

### Issue 3: DUPLICATE CARD PATTERNS
**Severity: LOW**

Evidence:
- `DeskCard.tsx` used in paper desk / research components
- `TerminalCard.tsx` used in terminal module
- Both solve the same problem (content card) with different implementations

### Issue 4: DUPLICATE STATUS BADGE PATTERNS
**Severity: MEDIUM**

Evidence:
- `StatusBadge.tsx` in desk/ui
- `DeskChip.tsx` also renders badge-like chips with tone variants
- `RegimeBadge` in `DashboardHeader.tsx` is an inline function, not using either badge component
- Health status display in Paper Desk is ad-hoc, not using StatusBadge

### Issue 5: INCONSISTENT TYPOGRAPHY
**Severity: LOW**

Evidence:
- Terminal: `font-mono` for prices, `text-[10px]` for table headers with `tracking-[0.12em]`
- Paper desk: standard Tailwind text sizing
- `DashboardHeader.tsx`: `fontFamily: "var(--font-display)"` CSS variable
- No evidence of a typography scale document

---

## COMPONENT REUSE

| Usage Pattern | Evidence |
|--------------|---------|
| DeskMetricTile used across paper desk panels | YES — consistent use in `InstitutionalRiskCenter`, dashboard summaries |
| DeskSectionHeader used consistently | YES — appears in multiple panels |
| TerminalCard used in all terminal sub-pages | YES — consistent use |
| Status indicators | NO — 3 different implementations |
| Alert banners | NO — each component implements its own |
| Loading states | INCONSISTENT — some show spinners, others show nothing |

---

## ACCESSIBILITY

Evidence gathered:
- No `aria-label` found on icon-only buttons in search
- Keyboard navigation: no evidence of keyboard shortcut implementation
- Color contrast: terminal dark mode uses `text-zinc-400` on `bg-zinc-900` — borderline WCAG AA
- `text-[10px]` table headers are below minimum readable font size (11px minimum for display)
- No skip navigation link
- No focus management in modal/tab flows confirmed

---

## RESPONSIVENESS

Evidence:
- Terminal: `xl:grid-cols-[280px_minmax(0,1fr)_320px]` — 3-column layout assumes wide screen
- Paper desk: `md:grid-cols-4` — adapts at medium breakpoint
- No `sm:` breakpoints on primary trading panels
- No mobile-first responsive design strategy found
- Trading terminals are typically desktop-only — this is acceptable but means no mobile monitoring

---

## DESIGN SYSTEM SCORECARD

| Dimension | Score |
|-----------|-------|
| Component consistency | 5/10 — desk/ui library exists but not universal |
| Visual theme unity | 3/10 — light/dark split with no bridge |
| Token usage | 3/10 — partial CSS variable usage, mostly Tailwind direct |
| Accessibility | 3/10 — no confirmed a11y implementation |
| Responsiveness | 4/10 — medium breakpoints only |
| Component reuse | 6/10 — desk/ui library is well-used within its scope |
| Documentation | 0/10 — no Storybook or component docs found |

**Overall Score: 3.4/10**

The `desk/ui/` component library is a solid foundation. The primary problem is the visual schism between the terminal module (dark, Tailwind-direct) and the paper desk module (light, CSS variables + library), which creates a fragmented user experience.
