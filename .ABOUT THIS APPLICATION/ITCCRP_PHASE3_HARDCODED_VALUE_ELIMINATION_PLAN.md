# PHASE 3 — HARDCODED VALUE ELIMINATION PLAN
## Institutional Trading Command Center Reconstruction Program (ITCCRP)
**Date:** 2026-06-11 | **Priority:** CRITICAL — Must execute before any UX work

---

## ELIMINATION REGISTER

Every hardcoded value catalogued with file, line, current value, and replacement authority.

---

### HV-001 — CRITICAL: FALLBACK_BALANCE in useEngineState

**File:** `client/src/hooks/useEngineState.ts`
**Line:** 5
**Current:** `const FALLBACK_BALANCE = 1000000.0;`
**Line:** 39 `return { engineOnline, balance: FALLBACK_BALANCE };`

**Problem:** Balance is NEVER fetched from any API. Returns `$1,000,000` unconditionally.

**Replacement Authority:** `GET /api/paper-desk/state` → `state.balance` or `state.equity`

**Implementation:**
```typescript
// useEngineState.ts — REPLACEMENT
export default function useEngineState() {
  const [engineOnline, setEngineOnline] = useState(false);
  const [balance, setBalance] = useState<number | null>(null);

  useEffect(() => {
    const apiUrl = resolveEngineApiUrl();
    let cancelled = false;

    const checkHealth = async () => {
      try {
        const res = await fetch(`${apiUrl}/health`);
        if (!cancelled) setEngineOnline(res.ok);
      } catch {
        if (!cancelled) setEngineOnline(false);
      }
    };

    const fetchBalance = async () => {
      try {
        const res = await fetch("/api/paper-desk/state", { credentials: "include", cache: "no-store" });
        if (!cancelled && res.ok) {
          const json = await res.json();
          if (json.ok && json.state?.equity != null) {
            setBalance(json.state.equity);
          }
        }
      } catch { /* ignore */ }
    };

    checkHealth();
    fetchBalance();
    const healthInterval = setInterval(checkHealth, 5000);
    const balanceInterval = setInterval(fetchBalance, 10000);

    return () => {
      cancelled = true;
      clearInterval(healthInterval);
      clearInterval(balanceInterval);
    };
  }, []);

  return { engineOnline, balance };
}
```

**Callers to update:** Grep for `useEngineState` — update callers to handle `balance: number | null` (show "—" when null, never show fake $1M).

---

### HV-002 — CRITICAL: initialTerminalSnapshot — Entire Object

**File:** `client/src/lib/terminal/terminalSnapshot.ts`
**Lines:** 17-121

**Current synthetic values:**
| Line | Field | Hardcoded Value |
|------|-------|----------------|
| 19 | `price` | `105_842.5` |
| 20 | `priceChange24hPct` | `1.42` |
| 21 | `spreadBps` | `1.8` |
| 22 | `fundingRate` | `0.00018` |
| 23 | `regime` | `"Trending Bull"` |
| 24-16 | `candles` | 90 synthetic OHLCV |
| 25-34 | `bids/asks` | 18+18 synthetic levels |
| 35-66 | `positions` | 2 fake positions |
| 67-87 | `risk` | All risk metrics fake |
| 81-87 | `strategies` | 5 fake strategy rows |
| 89-109 | `analytics` | 6-point equity curve, Sharpe, win rate |
| 110-114 | `journal` | 3 fake trades |
| 115-119 | `alerts` | 3 fake alerts |

**Problem:** `terminalStore.tsx:24` exits early if `NEXT_PUBLIC_TERMINAL_WS_URL` is unset. The initialSnapshot is shown forever. Since the WS endpoint doesn't exist in Go Engine, this is permanent.

**Two-track replacement strategy:**

**Track A — Short term (2 weeks): Replace with loading/empty states + REST polling fallback**
```typescript
// terminalSnapshot.ts — REPLACEMENT
export const initialTerminalSnapshot: TerminalSnapshot = {
  connected: false,
  price: 0,
  priceChange24hPct: 0,
  spreadBps: 0,
  fundingRate: 0,
  regime: "Unknown",
  candles: [],
  bids: [],
  asks: [],
  positions: [],
  risk: {
    var95Usd: 0, var99Usd: 0, cvar95Usd: 0,
    heatPct: 0, drawdownPct: 0,
    netExposureUsd: 0, grossExposureUsd: 0,
    marginUsagePct: 0, longExposureUsd: 0, shortExposureUsd: 0,
    fundingPaidUsd: 0, fundingReceivedUsd: 0,
  },
  strategies: [],
  analytics: {
    equityCurve: [],
    rollingSharpe30d: 0, rollingSharpe90d: 0,
    profitFactorTrend: 0, winRatePct: 0, feeDragUsd: 0,
    rMultipleBuckets: [],
  },
  journal: [],
  alerts: [],
  updatedAt: "",
};
```

**Track B — Medium term (4 weeks): Wire terminal pages to paper desk REST APIs**

Replace `useTerminalSnapshot` calls in each terminal page with specific REST hooks:
- `ExecutionCenter` → `usePaperDesk` + `useLiveBTCPrice`
- `AnalyticsCenter` → `/api/paper-desk/equity` + `/api/paper-desk/strategy-analytics`
- `ResearchCenter` → `/api/paper-desk/strategy-health` + `/api/paper-desk/strategy-analytics`
- `RiskModule` → `/api/paper-desk/snapshot` portfolio section
- `TradeJournalPro` → `/api/paper-desk/trades`

**Track C — Long term (8 weeks): Implement WebSocket endpoint in Go Engine**

Go Engine emits `TerminalSnapshot` JSON deltas over WebSocket on position changes, price ticks, and risk recalculations. See Phase 11 for full spec.

---

### HV-003 — HIGH: STARTING_BALANCE in TerminalDashboard

**File:** `client/src/components/TerminalDashboard.tsx`
**Line:** 13 `const STARTING_BALANCE = 1_000_000;`

**Current usage:**
- Line 281: `const equity = state?.equity ?? stateBalance ?? STARTING_BALANCE;`
- Line 282: `realizedPnl = state?.realized_pnl ?? (stateBalance != null ? stateBalance - STARTING_BALANCE : 0);`
- Line 410: `change={fmtPct(totalPnl / STARTING_BALANCE)}` — PnL % always vs $1M denominator

**Problem:** When state is null, equity shows `$1.000M`. When state is loaded but `starting_balance` not in schema, PnL % calculation uses wrong denominator.

**Replacement Authority:** 
- `paper_state.balance` is the current cash balance (post-trade).
- Go Engine should provide `initial_balance` in `paper_state` schema.
- Short term: query `GET /api/paper-desk/state` and read `state.balance` for initial basis (it should equal $1M on day 0 and degrade/grow from there).

**Implementation:**
```typescript
// Replace STARTING_BALANCE usage:
const equity = state?.equity ?? null;
const initialBalance = state?.balance != null && state.equity != null
  ? state.balance + (state.equity - state.balance)  // equity IS the current value
  : null;
const pnlPct = equity != null && initialBalance != null && initialBalance > 0
  ? (equity - 1_000_000) / 1_000_000  // Keep $1M reference for paper account
  : null;

// UI: show "—" when equity is null instead of STARTING_BALANCE
```

---

### HV-004 — HIGH: Hardcoded Candles in terminalSnapshot.ts

**File:** `client/src/lib/terminal/terminalSnapshot.ts`
**Lines:** 3-14 (candle generator function) and line 24

**Current:** 90 synthetic candles generated from `Math.sin()` curves starting at `104_200`.

**Replacement Authority:** Binance REST API `/api/v3/klines?symbol=BTCUSDT&interval=1m&limit=90`

**Implementation:** Add `fetchInitialCandles()` call in `terminalStore.tsx` after component mount, before WebSocket connects:
```typescript
// In useTerminalSnapshot(), add:
useEffect(() => {
  const loadInitialCandles = async () => {
    const res = await fetch("/api/btc/spot-klines?interval=1m&limit=90");
    if (res.ok) {
      const { candles } = await res.json();
      setSnapshot(prev => ({ ...prev, candles }));
    }
  };
  void loadInitialCandles();
}, []);
```

---

### HV-005 — MEDIUM: Hardcoded Strategy Scores in terminalSnapshot.ts

**File:** `client/src/lib/terminal/terminalSnapshot.ts`
**Lines:** 81-87

**Current:**
```typescript
strategies: [
  { id: 201, name: "Funding Mean Reversion Alpha", ..., sharpe: 2.18, expectancy: 32.4, ... },
  { id: 202, name: "CVD Divergence Alpha", ..., sharpe: 1.92, expectancy: 24.7, ... },
  // 3 more fake rows
],
```

**Replacement Authority:** `GET /api/paper-desk/strategy-analytics` → `strategy_scores` collection

**Note:** These 5 hardcoded strategies are named plausibly but have no correspondence to the 600+ real strategies in Go Engine. Any operator viewing the Research tab sees these 5 fake rows.

---

### HV-006 — MEDIUM: Hardcoded Risk Metrics in terminalSnapshot.ts

**File:** `client/src/lib/terminal/terminalSnapshot.ts`
**Lines:** 67-87

**Current hardcoded values:**
```typescript
risk: {
  var95Usd: 1_840,      // fake
  var99Usd: 2_760,      // fake
  cvar95Usd: 2_380,     // fake
  heatPct: 3.7,         // fake
  drawdownPct: 1.4,     // fake
  netExposureUsd: 7_210, // fake
  grossExposureUsd: 30_720, // fake
  marginUsagePct: 18.6, // fake
  ...
}
```

**Replacement Authority:**
- `drawdownPct` → `paper_state.current_drawdown`
- `netExposureUsd` → `portfolioAccountingService.exposure`
- `marginUsagePct` → computed from open positions / equity
- `var95Usd`, `cvar95Usd` → Go Engine risk module (add to paper_state snapshot or new `risk_snapshot` collection)
- `heatPct` → computed: `netExposureUsd / equity * 100`

---

### HV-007 — MEDIUM: Hardcoded Analytics in terminalSnapshot.ts

**File:** `client/src/lib/terminal/terminalSnapshot.ts`
**Lines:** 89-109

**Current hardcoded:**
- Equity curve: 6 hardcoded points (09:15→14:15 today)
- `rollingSharpe30d: 1.84`
- `rollingSharpe90d: 1.42`
- `profitFactorTrend: 1.58`
- `winRatePct: 54.2`
- `feeDragUsd: 214.8`
- R-multiple buckets: 5 fake distribution entries

**Replacement Authority:**
- Equity curve → `GET /api/paper-desk/equity` → `equity_curve` collection
- Sharpe → computed from equity curve returns
- Win rate → `paper_state.win_rate`
- Profit factor → avg from `strategy_scores.profit_factor`
- Fee drag → `paper_state.total_fees`
- R-multiple buckets → compute from `paper_trades.net_pnl` distribution

---

### HV-008 — MEDIUM: Hardcoded Journal Trades

**File:** `client/src/lib/terminal/terminalSnapshot.ts`
**Lines:** 110-114

**Replacement Authority:** `GET /api/paper-desk/trades?limit=20` → `paper_trades` collection

**Transform needed:** `PaperTradeDoc` → `JournalTrade` type (entry/exit prices, R-multiple calculation: `net_pnl / stop_loss_usd`)

---

### HV-009 — LOW: Hardcoded Alerts

**File:** `client/src/lib/terminal/terminalSnapshot.ts`
**Lines:** 115-119

**Replacement Authority:** Go Engine should write to an `alerts` collection. Short term: derive alerts from state:
- If `paper_state.current_drawdown > 0.05` → CRITICAL drawdown alert
- If `paper_state.open_position_count == 0` for > 30 min → WARNING no activity
- Healthy state → INFO with last trade summary

---

## ELIMINATION PRIORITY ORDER

| Priority | ID | File | Effort | ROI |
|----------|-----|------|--------|-----|
| P0 CRITICAL | HV-002 | terminalSnapshot.ts | 2h | Eliminates 5 broken pages |
| P0 CRITICAL | HV-001 | useEngineState.ts | 1h | Fixes fake balance globally |
| P1 HIGH | HV-004 | terminalSnapshot.ts | 2h | Real BTC chart on load |
| P1 HIGH | HV-005 | terminalSnapshot.ts | 1h | Real strategy data |
| P1 HIGH | HV-006 | terminalSnapshot.ts | 3h | Real risk metrics |
| P2 MEDIUM | HV-007 | terminalSnapshot.ts | 4h | Real analytics |
| P2 MEDIUM | HV-008 | terminalSnapshot.ts | 2h | Real trade journal |
| P2 MEDIUM | HV-003 | TerminalDashboard.tsx | 1h | PnL % fix |
| P3 LOW | HV-009 | terminalSnapshot.ts | 3h | Real alerts |

**Total elimination effort: ~19 hours of focused engineering**

---

## STEP 1 IMPLEMENTATION (HV-001 + HV-002): Execute Immediately

### Step 1a: Zero out initialTerminalSnapshot

```typescript
// client/src/lib/terminal/terminalSnapshot.ts — FULL REPLACEMENT
import type { TerminalSnapshot } from "./terminalTypes";

export const initialTerminalSnapshot: TerminalSnapshot = {
  connected: false,
  price: 0,
  priceChange24hPct: 0,
  spreadBps: 0,
  fundingRate: 0,
  regime: "",
  candles: [],
  bids: [],
  asks: [],
  positions: [],
  risk: {
    var95Usd: 0, var99Usd: 0, cvar95Usd: 0,
    heatPct: 0, drawdownPct: 0,
    netExposureUsd: 0, grossExposureUsd: 0,
    marginUsagePct: 0, longExposureUsd: 0, shortExposureUsd: 0,
    fundingPaidUsd: 0, fundingReceivedUsd: 0,
  },
  strategies: [],
  analytics: {
    equityCurve: [],
    rollingSharpe30d: 0, rollingSharpe90d: 0,
    profitFactorTrend: 0, winRatePct: 0, feeDragUsd: 0,
    rMultipleBuckets: [],
  },
  journal: [],
  alerts: [],
  updatedAt: "",
};
```

### Step 1b: Add REST polling fallback to terminalStore.tsx

```typescript
// Add inside useTerminalSnapshot() useEffect, after wsUrl check:
useEffect(() => {
  // REST fallback: populate snapshot from authenticated paper desk APIs
  const populate = async () => {
    const [snapshotRes, equityRes] = await Promise.all([
      fetch("/api/paper-desk/snapshot", { credentials: "include", cache: "no-store" }),
      fetch("/api/paper-desk/equity", { credentials: "include", cache: "no-store" }),
    ]);
    if (snapshotRes.ok) {
      const data = await snapshotRes.json();
      if (data.ok && data.state) {
        setSnapshot(prev => ({
          ...prev,
          connected: true,
          // Map paper_state → TerminalSnapshot fields
          risk: {
            ...prev.risk,
            drawdownPct: data.state.current_drawdown ?? 0,
            heatPct: data.portfolio?.heat_pct ?? 0,
            netExposureUsd: data.portfolio?.exposure?.net_usd ?? 0,
            grossExposureUsd: data.portfolio?.exposure?.gross_usd ?? 0,
            marginUsagePct: data.portfolio?.margin_usage_pct ?? 0,
          },
          positions: (data.open_positions ?? []).map(mapPaperPositionToTerminal),
          updatedAt: data.server_time ?? new Date().toISOString(),
        }));
      }
    }
    if (equityRes.ok) {
      const data = await equityRes.json();
      if (data.ok && data.equity_curve?.length > 0) {
        setSnapshot(prev => ({
          ...prev,
          analytics: {
            ...prev.analytics,
            equityCurve: data.equity_curve.map((pt: EquityCurveDoc) => ({
              time: pt.snapped_at,
              equity: pt.equity,
              btcBenchmark: pt.btc_benchmark ?? pt.equity,
            })),
          },
        }));
      }
    }
  };
  void populate();
  const interval = setInterval(() => void populate(), 10_000);
  return () => clearInterval(interval);
}, []);
```

### Step 1c: Add null-state guards to terminal components

Each terminal component must handle empty `snapshot`:
```typescript
// ExecutionCenter.tsx — add at top:
if (!snapshot.connected && snapshot.positions.length === 0) {
  return <TerminalLoadingState message="Connecting to Paper Desk..." />;
}
```

---

## OUTCOME

After executing these 3 steps:
- ❌ No component will display `$105,842.50` as current BTC price when engine is offline
- ❌ No component will display fake "Funding Mean Reversion Alpha" positions
- ❌ No component will display `Sharpe: 1.84` when it has not been computed
- ✅ Terminal pages will show loading states until real data arrives
- ✅ Balance will reflect actual MongoDB `paper_state.equity`
- ✅ Risk metrics will reflect actual drawdown and exposure
