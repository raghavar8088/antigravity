# BLOOMBERG-STYLE BLUEPRINT — ICCRP V3

**Design target:** Institutional dark terminal — not retail fintech

---

## Color System

| Token | Value | Usage |
|-------|-------|-------|
| Background | `#080b10` | Shell base — `TerminalShell.tsx` L33 |
| Panel | `#0d1118` | Cards — `TerminalCard.tsx` L17 |
| Border | `border-zinc-800` | Panel separation |
| Positive PnL | `text-emerald-300/400` | `format.ts` `pnlClass()` |
| Negative PnL | `text-rose-300/400` | same |
| Authority live | `bg-emerald-500/10 text-emerald-300` | `TerminalShell.tsx` L59-65 |
| Accent nav | `bg-sky-500/15 text-sky-200` | Active tab |

---

## Typography

| Element | Spec | File |
|---------|------|------|
| Panel title | 11px uppercase, tracking 0.16em | `TerminalCard.tsx` L20 |
| Metric label | 10px uppercase, tracking 0.12em | `TerminalCard.tsx` L49 |
| Metric value | `font-mono text-sm font-semibold` | L50 |
| Price tape | `font-mono text-lg` | `TerminalShell.tsx` L45 |

---

## Layout Patterns

### Risk Ribbon (Permanent)
- Component: `RiskRibbon.tsx`
- Embedded: `TerminalShell.tsx` L59-61
- Data: `/api/risk-ribbon` — GREEN/AMBER/RED status chips

### 3-Column Execution Grid
```
[280px book] | [flex chart] | [320px alerts]
```
`ExecutionCenter.tsx` L20 `xl:grid-cols-[280px_minmax(0,1fr)_320px]`

### Command Center Home
```
[Risk Ribbon full width]
[8-up KPI strip]
[3-col: Strategy Intel | Portfolio | Risk]
[2-col: Events | Positions]
[4-up quick links]
```
`CommandCenterHome.tsx`

---

## Real-Time Indicators

| Indicator | Source | Refresh |
|-----------|--------|---------|
| BTC mark | WS or `/api/btc/price` | `terminalStore.tsx` 3-5s REST |
| Authority badge | `terminalAuthorityLabel()` | On delta |
| Risk ribbon | `/api/risk-ribbon` | 5s (`RiskRibbon.tsx` L66) |
| Critical alert count | `snapshot.alerts` | WS/REST delta |

---

## Multi-Monitor Guidance

- Nav bar horizontal scroll on laptop (`overflow-x-auto`)
- Execution grid collapses to single column below `xl`
- Risk ribbon wraps items — suitable for ultrawide strip

**Future:** CSS grid template areas for 4K 3-panel dock (P3 roadmap).
