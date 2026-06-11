# PHASE 17 — INSTITUTIONAL DASHBOARD BLUEPRINT
## Forensic Audit | Trading Platform | 2026-06-11

---

## NOTE: This is a design specification only. No code is written or changed.

---

## ARCHITECTURE PRINCIPLES FOR REDESIGN

1. **Single source of truth**: One live data layer, not three independent polling systems
2. **Always-visible system status**: Persistent status bar on every page showing engine, broker, feed, kill switch state
3. **No demo data in production**: `initialTerminalSnapshot` must not contain plausible-looking data if the WS is unconnected
4. **Alert-first design**: Critical events surface immediately without requiring navigation
5. **Primary view = operational dashboard**: The home route must show actionable live data

---

## CENTER 1: SYSTEM COMMAND CENTER (new `/` home route)

**Purpose**: Single-glance operational awareness for the autonomous system.

**Panels**:

```
┌──────────────────────────────────────────────────────────┐
│ SYSTEM STATUS BAR (persistent, all pages)                │
│ Engine: ● LIVE  KS: ● INACTIVE  Recon: ● OK             │
│ Feeds: Coinbase ● / Binance ● / AngelOne ● / Delta ●    │
│ Last heartbeat: 00:00:18 ago   Uptime: 47h 23m           │
└──────────────────────────────────────────────────────────┘
┌─────────────────┬──────────────────┬─────────────────────┐
│ ACCOUNT SUMMARY │  TODAY'S P&L     │  RISK HEAT          │
│ Balance: $1.04M │  Realized: $840  │  Heat: 3.1% [●●○○]  │
│ Equity:  $1.04M │  Fees: -$42      │  Limit: 6.0%        │
│ DD:  -0.4%      │  Net: $798       │  VaR95: $1,820      │
└─────────────────┴──────────────────┴─────────────────────┘
┌──────────────────────┬───────────────────────────────────┐
│ STRATEGY HEALTH      │  ALERT FEED (live)                │
│ ● 58 Healthy         │  14:31 WARN Funding spike BTC-PERP│
│ △ 3 Warning          │  14:22 INFO Trade closed: +$64    │
│ ✗ 1 Critical: ID-204 │  13:55 CRIT KS check — heat OK   │
│ ? 4 Insufficient     │  [View all alerts]                │
└──────────────────────┴───────────────────────────────────┘
┌──────────────────────────────────────────────────────────┐
│ OPEN POSITIONS (live, all instruments)                   │
│ BTC-PERP LONG 0.18 @ 104,920 | Mark 105,842 | +$166    │
│ NIFTY-CE LONG 1 @ 198 | LTP 210 | +$12                  │
│ [No AngelOne equity positions]                           │
└──────────────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────────────┐
│ KILL SWITCH CONTROLS                                     │
│ Status: INACTIVE  [Block New Entries]  [HALT ALL]       │
│ Last triggered: Never                                    │
└──────────────────────────────────────────────────────────┘
```

**Data requirements**:
- System status: `GET /api/engine/api/admin/ks/status` + engine `/health` polling every 10s
- Broker feed: individual broker health checks every 30s
- Account summary: `/api/paper-desk/snapshot` every 5s
- Strategy health: `/api/paper-desk/strategy-health` every 30s
- Alert feed: new endpoint `GET /api/alerts/recent` — aggregates alerts from all sources
- Positions: current paper positions + broker position endpoints

---

## CENTER 2: EXECUTION CENTER (`/terminal/execution`)

**Changes from current**:
- Connect to real WebSocket OR replace with enhanced polling
- Add: Real order book (live broker depth, not BTC-PERP demo)
- Add: Order lifecycle panel (submitted → pending → filled/rejected, real-time)
- Add: Fill quality column (slippage vs expected)
- Add: Execution latency indicator
- Remove: `QuickTradePanel` stub if not wired (remove or wire — never show disconnected action buttons)

---

## CENTER 3: STRATEGY COMMAND CENTER (`/strategies`)

**New page required**

```
┌──────────────────────────────────────────────────────────┐
│ STRATEGY ADMINISTRATION                                  │
│ Active: 58 / 600+   Regime: TRENDING_BULL               │
├──────────────────────────────────────────────────────────┤
│ Filter: [Family ▼] [Health ▼] [Regime ▼] [Search...]    │
├───────┬────────────────────────┬───────┬──────┬─────────┤
│ ID    │ Name                   │ Health│ Win% │ Actions │
│ 91    │ ADX Long               │ ●     │ 64%  │ [Disable]│
│ 92    │ ADX Short              │ ●     │ 61%  │ [Disable]│
│ 204   │ Volume Profile LVN     │ ✗     │ 31%  │ [Review] │
└───────┴────────────────────────┴───────┴──────┴─────────┘
```

**Data requirements**: Backend endpoint `GET /api/engine/api/strategies/status` (new engine route needed)

---

## CENTER 4: RISK CENTER (`/risk`)

**Changes from current**:
- Wire to Go engine risk gate (`/api/engine/api/security/status`)
- Show heat level with threshold bands visually:
  ```
  Heat: [████████░░░░░░░░] 4.2% / 6.0% REDUCE ZONE
  ```
- Show VaR vs realized daily range comparison
- Show live leverage
- Show position concentration by family as live bar chart
- Kill switch integration: one-click block from risk center

---

## CENTER 5: PORTFOLIO CENTER (`/portfolio`)

**New page required**

- Aggregate view: BTC paper + NIFTY equity + options + MCX
- Net exposure by instrument
- Gross exposure by instrument
- Daily PnL by instrument
- Drawdown by instrument
- Correlation matrix between instrument groups

---

## CENTER 6: OMS CENTER (`/oms`)

**Changes from current**:
- Upgrade PaperOmsPanel to show:
  - Order lifecycle timeline (submitted → pending → filled, with timestamps)
  - Cancel button per pending order
  - Slippage per filled order
  - Order latency per order
- Add live broker orders alongside paper orders
- Add "order stuck" warning (pending > N minutes)

---

## CENTER 7: RECONCILIATION CENTER (`/reconciliation`)

**New page required**

```
┌──────────────────────────────────────────────────────────┐
│ RECONCILIATION STATUS    Last run: 00:02:14 ago ● OK    │
├──────────────────────────────────────────────────────────┤
│ Broker Positions:  BTC-PERP LONG 0.18  [Match ●]       │
│ OMS Positions:     BTC-PERP LONG 0.18                   │
│ Engine Positions:  BTC-PERP LONG 0.18                   │
├──────────────────────────────────────────────────────────┤
│ Recent Events:                                           │
│ 14:28  Auto-heal: paper_positions corrected (1 record)  │
│ 12:15  Reconciliation pass — 0 drift                    │
└──────────────────────────────────────────────────────────┘
[Trigger Manual Reconciliation]
```

**Data requirements**: New engine endpoint `GET /api/engine/api/reconciliation/status`

---

## CENTER 8: SYSTEM HEALTH CENTER (`/health`)

**Consolidates existing but unconnected health components**

- Engine uptime and heartbeat
- Go engine `/health` response (live)
- Storage health (from `StorageHealthPanel`)
- Broker connectivity (new)
- Data feed status (new)
- Watchdog status (new)
- API route error rates (new)
- Recent error log from engine

---

## CENTER 9: ALPHA CENTER (`/analytics`) — enhance from current

**Changes**:
- Wire to real data (not mock snapshot)
- Add: Per-family PnL attribution (from `lib/futuresAttribution.ts` wired to live data)
- Add: OOS profit factor trend (strategy getting better or worse)
- Add: Alpha decay warning (per-strategy)
- Keep: R-multiple distribution, equity curve, rolling Sharpe

---

## PERSISTENT SYSTEM STATUS BAR

Must appear on every page. Contains:

| Element | Source | Update Rate |
|---------|--------|------------|
| Engine status dot | `/health` | 10s |
| Kill switch state | `/api/admin/ks/status` | 10s |
| Reconciliation state | `/api/reconciliation/status` | 30s |
| Feed status (4 brokers) | broker health checks | 30s |
| Critical alert count | alert aggregator | 5s |
| Last heartbeat age | computed | 1s |

---

## DESIGN PRINCIPLES FOR IMPLEMENTATION

1. **Status bar first** — implement this before any other redesign. It fixes the most dangerous operational gaps cheaply.
2. **Wire the WS or remove the demo** — if the WS server cannot be built now, replace `initialTerminalSnapshot` with explicit empty/disconnected state and show "Waiting for engine connection."
3. **Kill switch status before kill button** — showing state is more important than the button.
4. **Reconciliation read before reconciliation write** — a read-only reconciliation status page is achievable immediately.
5. **Dead code removal** — `InstitutionalRiskCenter.tsx` should be deleted or connected.
