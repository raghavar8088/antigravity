# COMMAND CENTER HOME BLUEPRINT — ICCRP V3

**Route:** `/terminal`  
**Component:** `client/src/components/terminal/institutional/CommandCenterHome.tsx`  
**Page:** `client/src/app/terminal/page.tsx`

---

## Top Section (Implemented)

| Widget | Data Source | Proof |
|--------|-------------|-------|
| Risk Ribbon | `/api/risk-ribbon` | `CommandCenterHome.tsx` L24-26 |
| Portfolio Equity | `snapshot.analytics.equityCurve[-1].equity` | L13-16 |
| Gross Exposure | `snapshot.risk.grossExposureUsd` | L28 |
| Drawdown | `snapshot.risk.drawdownPct` | L29 |
| Portfolio PF | `snapshot.analytics.profitFactorTrend` | L30 |
| Sharpe | `snapshot.analytics.rollingSharpe30d` | L31 |
| Active Strategies | `strategies.filter(ACTIVE)` | L17, L32 |
| Open Positions | `snapshot.positions.length` | L18, L33 |
| Authority Status | `snapshot.hasAuthority` | L19, L34 |

Authority chain: Go Engine → MongoDB → `/api/paper-desk/snapshot` → `terminalStore` → `CommandCenterHome`

---

## Below Section (Implemented)

| Panel | Route Link | Component Lines |
|-------|------------|-----------------|
| Strategy Intelligence | `/terminal/strategies` | L38-58 |
| Portfolio Analytics | `/terminal/portfolio` | L60-76 |
| Risk Metrics | `/terminal/risk` | L78-94 |
| Recent Events | `/terminal/events` | L98-114 |
| Open Positions | `/terminal/execution` | L116-143 |
| Quick Links | execution, observability, health, diagnostics | L147-152 |

---

## Data Flow

```
terminalStore.tsx
  ├─ WS: NEXT_PUBLIC_TERMINAL_WS_URL (optional)
  └─ REST poll (5s):
       ├─ /api/paper-desk/snapshot
       ├─ /api/strategy-intelligence
       ├─ /api/paper-desk/equity
       └─ /api/btc/price
         ↓
mapSnapshotToTerminalDelta.ts
         ↓
CommandCenterHome
```

**Proof:** `terminalStore.tsx` L91-122

---

## Status: IMPLEMENTED
