# PHASES 4–9 — DASHBOARD INVENTORY, STRATEGY INTELLIGENCE, SEP INTEGRATION, PORTFOLIO ANALYTICS, CORRELATION, RISK COMMAND CENTER
## Institutional Trading Command Center Reconstruction Program (ITCCRP)
**Date:** 2026-06-11

---

# PHASE 4 — DASHBOARD INVENTORY REPORT

## Complete Component Inventory

### KEEP (Live, Authenticated, Real Data)

| Component | Route | Data Source | Refresh | Verdict |
|-----------|-------|-------------|---------|---------|
| `TerminalDashboard.tsx` | `/` | `usePaperDesk` + `useLiveBTCPrice` | 5s poll + WS | **KEEP — upgrade** |
| `PaperDeskDashboard.tsx` | `/paper-desk` | `usePaperDesk` + sub-routes | 5s + on-demand | **KEEP — primary** |
| KPI Grid (8 cards) | `/` | MongoDB `paper_state` | 5s | **KEEP** |
| Open Positions Table | `/` | `paper_positions` | 5s | **KEEP** |
| Strategy Leaderboard | `/` | `strategy_scores` | on connect | **KEEP — upgrade** |
| Signal Feed | `/` | `paper_orders` | on connect | **KEEP** |
| System Status panel | `/` | `usePaperDesk.connection` | 5s | **KEEP** |
| Market Stats panel | `/` | `useLiveBTCPrice` | real-time | **KEEP** |
| Trade Analytics panel | `/` | `paper_state` counters | 5s | **KEEP** |

### UPGRADE (Exists but Missing Key Data)

| Component | Route | Gap | Upgrade |
|-----------|-------|-----|---------|
| `ExecutionCenter.tsx` | `/terminal/execution` | Hardcoded snapshot | Wire to `usePaperDesk` + live price |
| `AnalyticsCenter.tsx` | `/terminal/analytics` | Hardcoded metrics | Wire to `/api/paper-desk/equity` |
| `ResearchCenter.tsx` | `/terminal/research` | Hardcoded 5 strategies | Wire to `strategy_scores` + `strategy_health` |
| `RiskModule.tsx` | `/terminal/risk` | Hardcoded VaR/CVaR | Wire to `paper_state` + portfolio accounting |
| `TradeJournalPro.tsx` | `/terminal/journal` | Hardcoded 3 trades | Wire to `/api/paper-desk/trades` |
| `StrategyLeaderboard` | `/` | Shows strategy_id not name | Map to strategy name registry |
| Profit Factor KPI | `/` | Avg of all scores (inaccurate) | Use portfolio-level PF calculation |

### MERGE (Duplicate Coverage)

| Components | Reason | Action |
|-----------|--------|--------|
| `/paper-desk` + `/` root dashboard | Both show equity/positions/trades | Merge: root = overview, paper-desk = detail |
| `usePaperDesk` + `useEngineState` | Both try to represent account state | Remove `useEngineState.balance`, use `usePaperDesk.state.equity` |
| `TradingDashboard.tsx` + `TerminalDashboard.tsx` | Both are "main dashboard" files | Remove `TradingDashboard.tsx` if unused |

### REMOVE (Dead, Disconnected, or Research-Only)

| Component | Reason | Action |
|-----------|--------|--------|
| `MockTradingDashboard.tsx` | Engine permanently disabled | Label as "Research Lab" — not trading |
| `InstitutionalBacktestDashboard.tsx` | Feeds from disabled mock engine | Move to dedicated `/research` route |
| `DeskHeroStrip.tsx` | Unknown consumption — needs audit | Audit callers; remove if dead |
| `LiveDeskStatusBar.tsx` | No verified backend connection | Upgrade to real status or remove |

---

# PHASE 5 — STRATEGY INTELLIGENCE COMMAND CENTER BLUEPRINT

## Design Specification

### Data Sources (Available Today)

| Source | API | Collection | Available Fields |
|--------|-----|-----------|-----------------|
| Go Engine strategy scores | `/api/paper-desk/strategy-analytics` | `strategy_scores` | strategy_id, total_pnl, win_rate, profit_factor, expectancy, sample_size, avg_win, avg_loss |
| Go Engine strategy health | `/api/paper-desk/strategy-health` | `strategy_health` | strategy_id, status, last_signal_at, signal_count, trade_count, health_score |
| Paper trades (closed) | `/api/paper-desk/trades` | `paper_trades` | per-trade strategy attribution |
| Open positions | `/api/paper-desk/positions` | `paper_positions` | per-position strategy_id |

### Missing Fields (Require Go Engine Work)

| Field | Source Needed | Priority |
|-------|-------------|----------|
| Strategy name (human-readable) | Map `strategy_id` → `FUTURES_STRAT_DEFS.name` in client | P1 |
| Sharpe Ratio | Compute from trade returns | P1 |
| Sortino Ratio | Compute from downside returns | P2 |
| Max Drawdown | Compute from trade sequence | P1 |
| Allocation Tier | `deskAllocationByEdgeEnabled` logic | P2 |
| SEP Evidence Score | SEP program output | P2 |
| Regime | Classify per strategy family | P3 |

### Component Hierarchy

```
StrategyIntelligenceCenter
├── StrategyFilterBar
│   ├── SearchInput
│   ├── FamilyFilter (Funding / Order Flow / Smart Money / MSS / Volume Profile)
│   ├── HealthFilter (ACTIVE / WATCHLIST / DISABLED)
│   └── RegimeFilter
│
├── StrategyKPIStrip
│   ├── TotalStrategies KPI
│   ├── ActiveStrategies KPI
│   ├── WatchlistStrategies KPI
│   ├── DisabledStrategies KPI
│   └── PortfolioPF KPI
│
├── StrategyMasterTable
│   ├── Columns: Rank | Name | Family | Status | PnL | Trades | WinRate | PF | Expectancy | Sharpe | MaxDD | Score
│   ├── Sort: by any column
│   ├── Pagination: 50 per page
│   └── RowActions: View Detail | Enable | Disable
│
└── StrategyDetailPanel (drawer/modal)
    ├── EquityCurveChart (per-strategy)
    ├── TradeHistoryTable
    ├── SignalQualityMetrics
    └── SEP Evidence Panel (Phase 6)
```

### API Contract

```typescript
// GET /api/paper-desk/strategy-intelligence
// Response:
{
  ok: true,
  strategies: {
    id: string;           // strategy_id from Go Engine
    name: string;         // mapped from FUTURES_STRAT_DEFS
    family: string;       // strategy family
    health: "ACTIVE" | "WATCHLIST" | "DISABLED";
    status: string;       // last known status
    enabled: boolean;
    last_signal_at: string;
    signal_count: number;
    trade_count: number;
    total_pnl: number;
    win_rate: number;     // [0,1]
    profit_factor: number;
    expectancy: number;   // USD per trade
    avg_win: number;
    avg_loss: number;
    sample_size: number;
    sharpe: number | null;      // computed client-side
    max_drawdown_pct: number | null;
    allocation_tier: "A" | "B" | "C" | "PROBATION" | null;
    sep_score: number | null;
  }[];
  total: number;
  active_count: number;
  watchlist_count: number;
  disabled_count: number;
}
```

### State Architecture

```typescript
// useStrategyIntelligence.ts — new hook
const useStrategyIntelligence = () => {
  const [strategies, setStrategies] = useState<StrategyRow[]>([]);
  const [filter, setFilter] = useState<StrategyFilter>({ family: "all", health: "all", search: "" });
  const [sort, setSort] = useState<{ field: keyof StrategyRow; dir: "asc" | "desc" }>({ field: "total_pnl", dir: "desc" });
  const [page, setPage] = useState(1);

  // Poll /api/paper-desk/strategy-intelligence every 30s
  // (strategy scores updated by Go Engine on each settlement)
  useEffect(() => {
    const load = async () => {
      const res = await fetch("/api/paper-desk/strategy-intelligence", { credentials: "include" });
      const data = await res.json();
      if (data.ok) setStrategies(mergeWithStrategyDefs(data.strategies));
    };
    void load();
    const interval = setInterval(() => void load(), 30_000);
    return () => clearInterval(interval);
  }, []);

  const filtered = useMemo(() => applyFilter(strategies, filter), [strategies, filter]);
  const sorted = useMemo(() => applySort(filtered, sort), [filtered, sort]);
  const paged = useMemo(() => sorted.slice((page - 1) * 50, page * 50), [sorted, page]);

  return { strategies: paged, total: filtered.length, filter, setFilter, sort, setSort, page, setPage };
};
```

---

# PHASE 6 — SEP INTEGRATION ARCHITECTURE

## SEP Current State Assessment

**File:** `engine/cmd/sep_evidence` (referenced in CLAUDE.md, exists as Go cmd)

**What the SEP program generates:**
- Walk-forward validation results
- Out-of-sample profit factor scores
- Monte Carlo simulation results
- Expectancy rankings
- Strategy retirement candidates

**Available Exports (from Go Engine):**
- SEP writes to MongoDB collections: `strategy_scores.oos_profit_factor`, `strategy_scores.expectancy`
- Walk-forward results: unclear if persisted to MongoDB currently

### SEP API Design

```
GET /api/sep/rankings
  Returns: strategies ranked by combined SEP score (OOS PF × expectancy × walk-forward score)
  
GET /api/sep/top?limit=20
  Returns: top N strategies by SEP composite score

GET /api/sep/bottom?limit=20
  Returns: bottom N (retirement candidates)

GET /api/sep/walk-forward?strategyId=:id
  Returns: walk-forward validation results for specific strategy

GET /api/sep/monte-carlo?strategyId=:id
  Returns: Monte Carlo simulation distribution

GET /api/sep/retirement-candidates
  Returns: strategies with expectancy < 0 for last N trades, OOS PF < 1.0
```

### SEP Dashboard Design

```
SEPIntelligenceCenter
├── SEPScorecard (composite ranking)
│   ├── Top 20 by SEP Score
│   ├── Top 50 by OOS PF
│   └── Bottom 20 (retirement watch)
│
├── WalkForwardResultsTable
│   ├── Strategy | IS PF | OOS PF | WF Efficiency | Verdict
│   └── Color: GREEN (OOS>1.2) | AMBER (1.0-1.2) | RED (<1.0)
│
├── MonteCarloChart (per strategy)
│   └── 95th/50th/5th percentile equity paths
│
└── RetirementCandidatesPanel
    ├── Triggered by: expectancy < 0 for 20+ trades
    └── Action: Operator can mark RETIRE (writes to Go Engine kill-switch)
```

---

# PHASE 7 — PORTFOLIO ANALYTICS COMMAND CENTER

## Design Specification

### Data Sources

| Metric | Source | API | Notes |
|--------|--------|-----|-------|
| Portfolio Equity | `paper_state.equity` | `/api/paper-desk/snapshot` | ✅ Available |
| Portfolio PnL (realized) | `paper_state.realized_pnl` | `/api/paper-desk/snapshot` | ✅ Available |
| Portfolio PnL (unrealized) | `paper_state.unrealized_pnl` | `/api/paper-desk/snapshot` | ✅ Available |
| Equity Curve | `equity_curve` collection | `/api/paper-desk/equity` | ✅ Available |
| Daily PnL | `daily_pnl_history` | `/api/paper-desk/equity` | ✅ Available |
| Portfolio Drawdown | `paper_state.max_drawdown` | `/api/paper-desk/snapshot` | ✅ Available |
| Portfolio Win Rate | `paper_state.win_rate` | `/api/paper-desk/snapshot` | ✅ Available |
| Portfolio Profit Factor | avg of `strategy_scores` | `/api/paper-desk/strategy-analytics` | ⚠️ Approx |
| Sharpe Ratio | compute from equity curve | client-side | ⚠️ Needs equity curve |
| Sortino | compute from equity curve | client-side | ⚠️ Needs equity curve |
| Volatility | compute from equity returns | client-side | ⚠️ Needs equity curve |
| Capital Exposure | `portfolioAccountingService` | `/api/paper-desk/snapshot.portfolio` | ✅ Available |
| Strategy Distribution | aggregate `paper_trades.strategy_id` | `/api/paper-desk/trades` | ⚠️ Compute client-side |
| Regime Distribution | aggregate `paper_trades` + regime tag | `/api/paper-desk/trades` | ⚠️ Needs regime in trade record |

### Widget Hierarchy

```
PortfolioAnalyticsCenter
├── PortfolioEquityPanel
│   ├── Equity Curve Chart (100 candles of equity vs BTC benchmark)
│   ├── Daily PnL Bars chart
│   └── Drawdown chart (underwater equity)
│
├── PortfolioKPIStrip
│   ├── Sharpe (30d / 90d / all-time)
│   ├── Sortino
│   ├── Profit Factor
│   ├── Max Drawdown %
│   ├── Calmar Ratio (return / max DD)
│   └── Volatility (annualized)
│
├── CapitalAllocationPanel
│   ├── Exposure pie: Long vs Short vs Cash
│   ├── Exposure by Strategy Family
│   └── Margin Utilization gauge
│
└── ReturnDistributionPanel
    ├── R-Multiple Histogram
    ├── Daily Return Distribution
    └── Win/Loss ratio
```

### API Contract

```typescript
// GET /api/paper-desk/portfolio-analytics
{
  ok: true,
  equity: number;
  realized_pnl: number;
  unrealized_pnl: number;
  total_pnl: number;
  peak_equity: number;
  current_drawdown_pct: number;
  max_drawdown_pct: number;
  win_rate: number;
  total_trades: number;
  total_fees: number;
  equity_curve: { time: string; equity: number; btc_benchmark: number }[];
  daily_pnl: { date: string; pnl: number; pnl_pct: number }[];
  computed: {
    sharpe_30d: number | null;
    sharpe_90d: number | null;
    sharpe_all: number | null;
    sortino: number | null;
    profit_factor: number | null;
    calmar: number | null;
    volatility_ann: number | null;
  };
  exposure: {
    net_usd: number;
    gross_usd: number;
    long_usd: number;
    short_usd: number;
    margin_usage_pct: number;
    heat_pct: number;
  };
}
```

---

# PHASE 8 — CORRELATION ANALYTICS CENTER

## Design Specification

### Data Requirements

| Analysis | Data Source | Computation |
|----------|-------------|------------|
| Correlation Matrix | `paper_trades` by strategy | Client-side: compute daily return series per strategy, Pearson correlation |
| Cluster Analysis | Correlation matrix | K-means or hierarchical clustering (client-side) |
| Duplicate Detection | Strategy signal overlap | Compare entry signals within 5-min window |
| Concentration | Strategy group exposure | Sum positions by family |
| Diversification Score | Eigenvalue analysis of correlation matrix | `1 - λ₁/ΣΛ` |

### Implementation Notes

Since all correlation is computed from `paper_trades`, no new backend endpoints are needed. The client-side computation should:
1. Fetch full trade history from `/api/paper-desk/trades` (paginated, all pages)
2. Bin trades into daily buckets per strategy
3. Compute correlation matrix using Pearson correlation coefficient
4. Display as heatmap

### Component Design

```
CorrelationAnalyticsCenter
├── CorrelationHeatmap
│   ├── Strategy × Strategy matrix
│   ├── Color: RED (high +corr) | WHITE (no corr) | BLUE (neg corr)
│   └── Click cell → show trade overlap detail
│
├── ClusterDendrogram
│   └── Shows which strategy families cluster together
│
├── ConcentrationRiskPanel
│   ├── "Top 3 strategies = X% of PnL" warning
│   └── Herfindahl-Hirschman Index of strategy PnL concentration
│
└── DiversificationScore
    └── Score: 0 (fully correlated) → 100 (fully diversified)
```

---

# PHASE 9 — RISK COMMAND CENTER SPECIFICATION

## Permanent Global Risk Ribbon

### Design: Always-Visible Top Bar (not collapsible)

Position: Fixed top bar, above navigation, 48px height, full width.

### Fields and Sources

| Field | Source | Update Frequency | Color Logic |
|-------|--------|-----------------|-------------|
| Kill Switch Status | `/api/engine/killswitch` → Go Engine | 10s poll | GREEN=armed/OK, RED=triggered |
| Reconciliation | `/api/engine/reconciliation/status` | 10s poll | GREEN=ok, AMBER=drift, RED=mismatch |
| OMS Status | `/api/engine/oms/health` | 10s poll | GREEN=ok, RED=error |
| Portfolio Risk | `paper_state.current_drawdown` | 5s (via snapshot) | GREEN<2%, AMBER<5%, RED>5% |
| Market Data | `useLiveBTCPrice.connected` | Real-time | GREEN=live, RED=offline |
| Watchdog | `/api/engine/watchdog/status` | 10s poll | GREEN=ok, RED=triggered |
| Execution Health | `/api/engine/execution/health` | 10s poll | GREEN=ok, RED=error |
| Daily DD | `paper_state.current_drawdown` | 5s | Value + % |
| Max DD | `paper_state.max_drawdown` | 5s | Value + % |
| Exposure | `portfolio.net_usd` | 5s | Value in USD |
| Margin Util | `portfolio.margin_usage_pct` | 5s | GREEN<50%, AMBER<80%, RED>80% |

### Color Model

```
GREEN  → status === "ok" || value within safe thresholds
AMBER  → warning threshold exceeded, monitoring required
RED    → critical threshold exceeded or system offline, action required
PULSE  → any RED field → entire ribbon border pulses red
```

### Component Spec

```typescript
// RiskCommandRibbon.tsx
interface RiskRibbonState {
  killSwitch: "armed" | "triggered" | "unknown";
  reconciliation: "ok" | "drift" | "mismatch" | "unknown";
  oms: "ok" | "error" | "unknown";
  marketData: "live" | "offline";
  watchdog: "ok" | "triggered" | "unknown";
  execution: "ok" | "degraded" | "error";
  dailyDrawdownPct: number | null;
  maxDrawdownPct: number | null;
  exposureUsd: number | null;
  marginUsagePct: number | null;
}

// Polling strategy:
// - Use a single /api/risk-ribbon endpoint that aggregates all engine status
// - Engine endpoints are proxied via /api/engine/[...path]
// - Poll every 10s from a dedicated hook: useRiskRibbon()
// - On RED status: increase poll to 2s for that specific field
```

### API Contract

```typescript
// GET /api/risk-ribbon — new aggregated endpoint
// Proxies to Go Engine /risk/ribbon or aggregates from multiple engine calls
{
  ok: true;
  kill_switch: { status: "armed" | "triggered"; triggered_at: string | null; reason: string | null };
  reconciliation: { status: "ok" | "drift" | "mismatch"; drift_usd: number | null; last_checked_at: string };
  oms: { status: "ok" | "error"; queue_depth: number; last_fill_at: string | null };
  market_data: { btc_connected: boolean; last_tick_at: string; ticks_per_second: number };
  watchdog: { status: "ok" | "triggered"; last_ping_at: string };
  portfolio: {
    daily_drawdown_pct: number;
    max_drawdown_pct: number;
    exposure_net_usd: number;
    margin_usage_pct: number;
    open_positions: number;
  };
  server_time: string;
}
```

### Placement in Layout

```typescript
// client/src/components/terminal/AppShell.tsx — add before children:
<RiskCommandRibbon />
<NavigationBar />
{children}
```

The ribbon must be rendered ABOVE the navigation so it is always visible regardless of page scroll position.
