# PHASES 10–16 — OBSERVABILITY, HEALTH CENTER, ALPHA MONITORING, EXECUTION QUALITY, EVENT CONSOLE, DATA CONSISTENCY, PERFORMANCE
## Institutional Trading Command Center Reconstruction Program (ITCCRP)
**Date:** 2026-06-11

---

# PHASE 10 — OBSERVABILITY RECONSTRUCTION PLAN

## Current State Assessment

**Existing Observability:**
- Go Engine exposes Prometheus `/metrics` on engine port
- `useEngineState.ts` polls `/health` every 5s → binary online/offline
- `usePaperDesk.ts` shows connection status: "connecting" | "live" | "stale" | "error" | "unauthorized"
- No structured logging UI
- No alert routing to frontend
- No metric time-series charts beyond equity curve

**Gaps:**
- Engine metrics not surfaced to UI
- No log stream in UI
- No structured alert system
- No latency visibility
- No fill quality display
- No strategy-level health stream

## Target Observability Model

### Layer 1 — Engine Health (Go Engine → API → UI)

```
Go Engine Prometheus /metrics
  → /api/engine/metrics (Next.js proxy)
  → useEngineMetrics hook (polls every 30s)
  → Health Center UI gauges

Key metrics to surface:
  - engine_uptime_seconds
  - strategy_signals_total (counter)
  - strategy_trades_opened_total (counter)
  - strategy_trades_closed_total (counter)
  - oms_orders_accepted_total
  - oms_orders_rejected_total
  - risk_gate_blocks_total
  - kill_switch_triggers_total
  - reconciliation_drift_usd
  - execution_latency_p50_ms
  - execution_latency_p99_ms
  - market_data_ticks_per_second
  - active_positions_count
  - total_open_exposure_usd
```

### Layer 2 — Log Stream (Go Engine → MongoDB `engine_logs` → UI)

Go Engine writes structured logs to MongoDB `engine_logs` collection:
```go
type EngineLog struct {
  Timestamp time.Time `bson:"ts"`
  Level     string    `bson:"level"`   // INFO | WARN | ERROR | CRITICAL
  Component string    `bson:"component"` // OMS | RISK | STRATEGY | MARKET_DATA
  Message   string    `bson:"msg"`
  Metadata  map[string]interface{} `bson:"meta"`
}
```

Next.js API:
```
GET /api/engine/logs?level=WARN&component=OMS&limit=100&since=ISO_DATE
```

UI: `useEngineLogs` hook polls every 5s for new log entries.

### Layer 3 — Alert Routing (Engine → `alerts` collection → UI)

```
Go Engine detects:
  - Drawdown > threshold → write CRITICAL alert to MongoDB `alerts`
  - Kill switch triggered → write CRITICAL alert
  - Reconciliation mismatch → write CRITICAL alert
  - Strategy P&L threshold → write WARNING alert
  - New trade opened → write INFO alert

/api/paper-desk/alerts (new route)
  → reads `alerts` collection, last 50
  → useAlerts hook polls every 10s
  → Risk Command Ribbon shows count badge
  → Alert panel shows detail
```

### Layer 4 — Latency Monitoring

```
Go Engine writes to `execution_latency` collection per fill:
  - signal_to_order_ms
  - order_to_fill_ms
  - fill_to_position_ms

/api/paper-desk/execution-quality (new route)
→ latency percentiles (p50, p95, p99) over last 100 fills
→ displayed in Execution Quality Center (Phase 13)
```

---

# PHASE 11 — HEALTH CENTER ARCHITECTURE

## Unified Health Center Design

### Health Dimensions (8 Systems)

| System | Data Source | Health Endpoint | Polling |
|--------|-------------|-----------------|---------|
| Go Engine | `/api/engine/health` (proxy) | HTTP 200 = healthy | 5s |
| OMS | `/api/engine/oms/status` | `queue_depth`, `error_count` | 10s |
| Portfolio | `paper_state.current_drawdown` | drawdown < threshold | 5s |
| Market Data | `useLiveBTCPrice.connected` | WS connected + tick age | real-time |
| Reconciliation | `/api/engine/reconciliation/status` | drift < $100 | 10s |
| Kill Switch | `/api/engine/killswitch/status` | `armed` and not `triggered` | 10s |
| Strategies | `strategy_health.healthy / total` | > 80% healthy | 30s |
| Watchdog | `/api/engine/watchdog/status` | last ping < 5 min | 10s |
| Alpha Engines | computed from strategy signals | any signal in last hour | 30s |
| Funding Feed | last funding rate tick age | < 8h (funding period) | 30s |

### Aggregated Health Score

```typescript
// Health scoring:
// GREEN = all critical systems healthy
// AMBER = non-critical degradation (no fills in 5 min, etc)
// RED = critical system failure (engine offline, kill switch triggered, reconciliation mismatch)

function computeSystemHealth(systems: SystemHealthState[]): "healthy" | "degraded" | "critical" {
  if (systems.some(s => s.criticality === "critical" && s.status === "red")) return "critical";
  if (systems.some(s => s.status === "amber" || s.status === "red")) return "degraded";
  return "healthy";
}
```

### Component Spec

```
HealthCenter
├── HealthScorecard
│   ├── Overall score: 0-100
│   ├── Status badge: HEALTHY / DEGRADED / CRITICAL
│   └── Last checked timestamp
│
├── SystemHealthGrid (2×5 grid of system tiles)
│   Each tile shows:
│   ├── System name
│   ├── Status indicator (GREEN/AMBER/RED)
│   ├── Key metric (uptime, queue depth, drift USD, etc)
│   └── Last updated timestamp
│
├── HealthHistoryChart
│   └── 24h history of overall health score
│
└── IncidentLog
    └── Last 10 status changes with timestamp and reason
```

### WebSocket vs Polling Decision

Polling is correct for health checks given Vercel serverless constraints (documented in `usePaperDesk.ts` architecture decision). Use staggered intervals:
- Critical systems (engine, kill switch): 5s
- Important systems (OMS, reconciliation): 10s
- Informational (strategies, alpha): 30s

Aggregate with a single `/api/system-health` endpoint that fans out internally to avoid N parallel client polls.

---

# PHASE 12 — ALPHA MONITORING CENTER

## Alpha Engine Inventory

From `engine/internal/strategy/` and `FUTURES_STRAT_DEFS`:

| Alpha Engine | Family | Strategies | Signals |
|-------------|--------|-----------|---------|
| MSS Continuation | MSS | 15 variants | Market structure breaks |
| Funding Mean Reversion | Funding | 8 variants | Funding rate Z-score |
| Order Block | Smart Money | 12 variants | OB imbalances |
| FVG Retest | Smart Money | 10 variants | Fair value gap retest |
| Liquidity Sweep | Smart Money | 12 variants | Equal highs/lows sweeps |
| CVD Divergence | Order Flow | 8 variants | Cumulative volume delta |
| Delta Absorption | Order Flow | 6 variants | Large delta absorption |
| EMA Cross | Trend | 15 variants | Multi-timeframe EMA |
| RSI Threshold | Momentum | 8 variants | RSI overbought/oversold |
| Bollinger Band | Mean Reversion | 12 variants | BB squeeze/expansion |
| Volume Profile LVN | Volume Profile | 10 variants | LVN breakout |
| Breakout | Trend | 8 variants | Range breakout |

### Component Design

```
AlphaMonitoringCenter
├── AlphaFamilyCards (one per family)
│   Each card shows:
│   ├── Family name + active strategy count
│   ├── Last signal timestamp
│   ├── Signal count (today / 7d / 30d)
│   ├── Trade count + PnL
│   ├── Win rate
│   ├── Profit Factor
│   ├── Status: FIRING | QUIET | DISABLED
│   └── Health bar
│
├── AlphaSignalStream
│   └── Real-time feed of last 20 signals across all families
│
└── AlphaPerformanceMatrix
    ├── Family × Metric grid
    └── Sortable by any metric
```

### Data Sources

- Signal counts: `strategy_scores.signal_count` per family aggregate
- Trade metrics: `paper_trades` grouped by strategy family
- Last signal: `strategy_health.last_signal_at` max per family
- Status: computed from signal age (< 1h = FIRING, 1-4h = QUIET, > 4h or 0 trades = CHECK)

---

# PHASE 13 — EXECUTION QUALITY CENTER

## Design Specification

### Metrics to Display

| Metric | Source | Computation |
|--------|--------|------------|
| Signals generated | `strategy_scores.signal_count` sum | Direct |
| Orders placed | `paper_orders` count | Direct |
| Orders accepted | `paper_orders` where `transition_to = ACCEPTED` | Filter |
| Orders rejected | `paper_orders` where `transition_to = REJECTED` | Filter |
| Risk blocks | `paper_orders` where `transition_to = RISK_BLOCKED` | Filter |
| Kill switch blocks | `paper_orders` where `transition_to = KILL_SWITCH_BLOCKED` | Filter |
| OMS errors | `paper_orders` where `transition_to = ERROR` | Filter |
| Fills | `paper_trades` count | Direct |
| Fill quality | `paper_trades.fill_quality` avg | Aggregate |
| Signal-to-fill latency | `paper_trades.latency_ms` percentiles | Compute |
| Slippage bps | `paper_trades.slippage_bps` avg | Aggregate |

### Component Design

```
ExecutionQualityCenter
├── ExecutionFunnelChart
│   └── Sankey: Signals → Scored → Risk Checked → OMS Accepted → Filled
│
├── ExecutionKPIs
│   ├── Accept Rate (accepted / total orders)
│   ├── Fill Rate (fills / accepted orders)
│   ├── Risk Block Rate
│   ├── Avg Fill Quality (0-100)
│   └── Avg Slippage bps
│
├── LatencyChart
│   └── p50 / p95 / p99 latency over last 100 fills
│
└── OrderLogTable
    ├── Columns: Time | Strategy | Side | Status | Reason | Latency
    └── Filters: by status, strategy, time range
```

---

# PHASE 14 — REAL-TIME EVENT CONSOLE

## Bloomberg-Style Event Stream Design

### Event Types and Sources

| Event Type | Source | Priority |
|-----------|--------|----------|
| Signal Raised | `strategy_health.last_signal_at` | Medium |
| Order Placed | `paper_orders.transition_to = PENDING` | High |
| Order Accepted | `paper_orders.transition_to = ACCEPTED` | High |
| Order Filled | `paper_orders.transition_to = FILLED` | Critical |
| Position Opened | `paper_positions` new entry | Critical |
| Position Closed (Win) | `paper_trades.net_pnl > 0` | Critical |
| Position Closed (Loss) | `paper_trades.net_pnl < 0` | High |
| Risk Block | `paper_orders.transition_to = RISK_BLOCKED` | Critical |
| Kill Switch Event | `alerts` collection | Critical |
| Reconciliation Event | `alerts` collection | Critical |
| Drawdown Alert | `paper_state.current_drawdown` change | Critical |

### Component Design

```
EventConsole
├── EventFilterBar
│   ├── Type filter (Signals / Orders / Fills / Risk / System)
│   ├── Severity filter (ALL / CRITICAL / WARNING / INFO)
│   └── Time range (Last 1h / 4h / 24h)
│
├── EventStream (virtualized list, newest at top)
│   Each row:
│   ├── Timestamp (ISO with ms)
│   ├── Type badge (color-coded)
│   ├── Source (strategy name / system)
│   ├── Message
│   └── Metadata expandable row
│
└── EventStats (top of stream)
    ├── Events/min rate
    ├── Error rate
    └── Last critical event timestamp
```

### Polling Strategy

Since Vercel serverless prevents SSE, use MongoDB change-detection polling:
```typescript
// useEventConsole.ts
const EVENT_POLL_MS = 2000; // 2s for near-real-time event feel

const useEventConsole = () => {
  const [events, setEvents] = useState<TradeEvent[]>([]);
  const lastFetchRef = useRef<string>(new Date(0).toISOString());

  useEffect(() => {
    const poll = async () => {
      const res = await fetch(`/api/paper-desk/events?since=${lastFetchRef.current}&limit=50`);
      const data = await res.json();
      if (data.ok && data.events.length > 0) {
        lastFetchRef.current = data.events[0].ts; // newest first
        setEvents(prev => [...data.events, ...prev].slice(0, 500));
      }
    };
    const id = setInterval(poll, EVENT_POLL_MS);
    return () => clearInterval(id);
  }, []);

  return { events };
};
```

---

# PHASE 15 — DATA CONSISTENCY CERTIFICATION

## Verification Protocol

For each metric, a live verification query is specified.

### Balance Consistency

**Claim:** UI balance = Go Engine balance

**Verification query:**
```typescript
// Compare paper_state.equity (what UI shows) with
// sum of: initial_balance + realized_pnl + unrealized_pnl

const expectedEquity = INITIAL_BALANCE + state.realized_pnl + state.unrealized_pnl;
const actualEquity = state.equity;
const drift = Math.abs(expectedEquity - actualEquity);
// PASS if drift < $0.01
// FAIL if drift > $1.00 (accounting error)
```

**Current status:** ⚠️ UNVERIFIED — No cross-check endpoint exists. `portfolioAccountingService.ts` recomputes equity client-side but Go Engine's `paper_state.equity` may drift from this if Go Engine uses different fee calculations.

**Fix:** Add `/api/paper-desk/validate` endpoint that runs Go Engine consistency check and returns any drift detected.

### Positions Consistency

**Claim:** `paper_positions` count = `paper_state.open_position_count`

**Verification query:**
```typescript
const positionCount = openPositions.length;
const stateCount = state.open_position_count;
const mismatch = positionCount !== stateCount;
// FAIL if mismatch > 0
```

**Current status:** ✅ LIKELY CONSISTENT — Both come from same Go Engine write path.

### Trades Consistency

**Claim:** `paper_trades` total = `paper_state.total_trades`

**Verification:**
```typescript
const tradeTotalFromState = state.total_trades;
const tradeCountFromCollection = await db.collection("paper_trades").countDocuments({ account_key });
// PASS if equal
```

**Current status:** ⚠️ UNVERIFIED — Potential for desync if Go Engine fails to increment `paper_state.total_trades` during crash recovery.

### Implementation: Consistency Check Route

```typescript
// GET /api/paper-desk/consistency-check
// Runs all consistency checks and returns pass/fail
{
  ok: true,
  checks: [
    { name: "equity_accounting", status: "PASS" | "FAIL", drift_usd: number | null },
    { name: "position_count", status: "PASS" | "FAIL", state_count: number, collection_count: number },
    { name: "trade_count", status: "PASS" | "FAIL", state_count: number, collection_count: number },
    { name: "pnl_sum", status: "PASS" | "FAIL", state_pnl: number, computed_pnl: number, drift: number },
  ],
  all_pass: boolean,
  checked_at: string,
}
```

---

# PHASE 16 — PERFORMANCE OPTIMIZATION PLAN

## Audit Findings

### P1 — HIGH: Paper Desk Polling All Pages Simultaneously

**Current:** Every mounted component using `usePaperDesk` creates its own 5s interval.

**Problem:** If `/paper-desk/positions` tab, `/paper-desk/trades` tab, and root dashboard are all open, 3× 5s polls run in parallel to the SAME endpoint.

**Fix:** Promote `usePaperDesk` to a React Context or Zustand store. Single poll per app session.

```typescript
// PaperDeskProvider.tsx
const PaperDeskContext = createContext<PaperDeskState>(null!);

export function PaperDeskProvider({ children }: { children: React.ReactNode }) {
  const desk = usePaperDeskPollLoop(); // single poll instance
  return <PaperDeskContext.Provider value={desk}>{children}</PaperDeskContext.Provider>;
}

export const usePaperDesk = () => useContext(PaperDeskContext);
```

### P2 — HIGH: TerminalDashboard Re-renders on Every BTC Tick

**File:** `TerminalDashboard.tsx:254-258`
```typescript
const live = useLiveBTCPrice();  // ticks every ~500ms
const desk = usePaperDesk();     // updates every 5s
```

**Problem:** `live.price` changes every 500ms, causing full re-render of the dashboard including strategy leaderboard, position table, and all KPI cards.

**Fix:** Memoize stable components. Separate price-sensitive from price-insensitive sections.

```typescript
// Wrap non-price components in React.memo:
const StrategyLeaderboard = React.memo(({ scores }) => ...);
const SignalFeed = React.memo(({ signals }) => ...);
const SystemStatus = React.memo(({ connection, regime }) => ...);
const TradeAnalytics = React.memo(({ state }) => ...);

// Only open positions table and BTC price card need live.price
```

### P3 — MEDIUM: No Virtualization in Trade History Tables

**Files:** `PositionsTable.tsx`, `TradeHistory.tsx`, `RunningTrades.tsx`

With 100+ trades or positions, DOM node count causes scroll jank.

**Fix:** Use `react-virtual` (already likely in Next.js project) for tables exceeding 50 rows.

### P4 — MEDIUM: Strategy Analytics Fetched on Every Connection Change

**File:** `TerminalDashboard.tsx:263-277`
```typescript
useEffect(() => {
  if (desk.connection !== "live") return;
  void Promise.all([fetchStrategyHealth(), fetchOrders(40)])
    .then(([health, orders]) => {
      setStrategyScores(health.scores ?? []);
      setRecentOrders(orders ?? []);
    });
}, [desk.connection, desk.lastUpdated]);  // fires every 5s when lastUpdated changes
```

**Problem:** `desk.lastUpdated` changes every 5s, causing `fetchStrategyHealth()` and `fetchOrders(40)` to fire every 5s. Strategy health doesn't change that frequently.

**Fix:** Add separate slower refresh for strategy data:
```typescript
// Strategy scores: refresh every 30s (Go Engine updates on settlement, not every poll)
useEffect(() => {
  if (desk.connection !== "live") return;
  const load = async () => {
    const [health, orders] = await Promise.all([fetchStrategyHealth(), fetchOrders(40)]);
    setStrategyScores(health.scores ?? []);
    setRecentOrders(orders ?? []);
  };
  void load();
  const interval = setInterval(() => void load(), 30_000);
  return () => clearInterval(interval);
}, [desk.connection]); // Only dep: connection status
```

### P5 — LOW: useMockCandleBuilder Runs Regardless of Usage

**File:** `TerminalDashboard.tsx:257`
```typescript
const candles = useMockCandleBuilder(live.price); // builds candles from price ticks
```

**Problem:** The candle builder runs on every price tick even if the chart isn't visible. For a $1M paper account, this is low-value computation.

**Fix:** Only run if market regime feature is needed. Gate with feature flag.

### P6 — LOW: React DevTools Render Count

**Observation:** With 43 hooks each managing state independently, React reconciliation overhead is high. The hook-based micro-store pattern with no shared context causes cascading re-renders.

**Fix (long-term):** Migrate to a single Zustand store for all dashboard state. Each "domain" (paper desk, BTC price, engine health, strategies) becomes a store slice. This eliminates 80% of re-renders.

```typescript
// Zustand store design:
const useTradingStore = create<TradingStore>((set) => ({
  paperDesk: initialPaperDeskState,
  btcPrice: initialBTCPriceState,
  engineHealth: initialEngineHealthState,
  strategies: initialStrategiesState,
  setPaperDesk: (state) => set({ paperDesk: state }),
  setBTCPrice: (price) => set(s => ({ btcPrice: { ...s.btcPrice, price } })),
  // ... etc
}));
```
