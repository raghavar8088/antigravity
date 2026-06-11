# Stale Data Report — Phase 15
# Generated: 2026-06-11 | Auditor: Claude Code Forensic Audit

---

## 1. Caching Layers

### CACHE-1: AngelOne JWT Token (module-scope, in-memory, server-side)
- **File:** `client/src/lib/angelAuth.ts:81-83`
- **What:** AngelOne API JWT token for NSE/MCX data access
- **TTL:** 23 hours (`JWT_TTL_MS = 23 * 60 * 60 * 1000`)
- **Invalidation:** Time-based (refreshes after 23h) OR on HTTP 401 response (force-refresh path)
- **Stored in:** Module-scope variables `cachedJwt` and `cachedJwtAt`
- **Staleness risk:** MODERATE. Token cached for 23h. If AngelOne invalidates the token early (session limit, server restart, password change), all MCX/NIFTY API calls will fail with 401 until the cache expires or a 401 triggers refresh. Evidence shows 401 handling exists but depends on the API returning 401, not a silent token revocation.

### CACHE-2: MCX Token Cache (route-level, in-memory, server-side)
- **File:** `client/src/app/api/mcx/ltp/route.ts:19-23`
- **What:** AngelOne token specifically for MCX LTP lookups
- **TTL:** 4 hours (`TOKEN_CACHE_TTL = 4 * 60 * 60 * 1000`)
- **Invalidation:** Time-based only
- **Stored in:** Module-scope `tokenCache` object
- **Staleness risk:** MODERATE. Token valid for 4h but no error-based invalidation found in the route. MCX LTP data would silently fail/return stale if token is invalid within the 4h window.

### CACHE-3: NiftyBees LTP Token Cache
- **File:** `client/src/app/api/nifty-bees/ltp/route.ts:12,125`
- **What:** AngelOne token for NiftyBees LTP
- **TTL:** 4 hours
- **Invalidation:** Time-based only
- **Staleness risk:** Same as CACHE-2

### CACHE-4: BTC Options Snapshot (localStorage)
- **File:** `client/src/lib/optionsSnapshotCache.ts`
- **Keys:** `raig.options.buy.snapshot.v1`, `raig.options.sell.snapshot.v1`
- **What:** Positions, trades, strategies, stats from Go engine options endpoints
- **TTL:** None — no expiry. The cache key includes `savedAt` timestamp but there's no TTL check on read.
- **Invalidation:** Only on explicit `clearOptionsBuyCache()` / `clearOptionsSellCache()` calls (on reset)
- **Stored in:** Browser `localStorage`
- **Staleness risk:** HIGH. If the browser tab is reloaded, the options cache from the previous session is used immediately before the engine API is re-fetched. No staleness indicator or age check. A trade that closed in the previous session could appear as open briefly on reload.

### CACHE-5: BTC Spot Scalper State (localStorage)
- **File:** `client/src/hooks/useBTCSpotScalperEngine.ts:45, 1511-1529`
- **Key:** `btc_spot_scalper_paper_state`
- **What:** Full BTC spot scalper paper state (balance, positions, trades)
- **TTL:** None — loaded on mount, saved on every tick
- **Invalidation:** `localStorage.removeItem(LS_KEY)` on clear, or explicit reset
- **Stored in:** Browser `localStorage`
- **Staleness risk:** HIGH. State loaded synchronously from localStorage on component mount. If the Go engine and this hook both run simultaneously, localStorage state diverges from engine state. Trades completed in the engine are not reflected in localStorage until the hook polls.

### CACHE-6: Research AI Cache (in-memory, client-side)
- **File:** `client/src/internal/research_ai/index.ts:25,45`
- **What:** AI research responses
- **TTL:** 5 minutes (`DEFAULT_TTL_MS = 5 * 60 * 1000`)
- **Invalidation:** Time-based
- **Staleness risk:** LOW for trading decisions (research only)

### CACHE-7: Strategy Rankings File
- **File:** `fixtures/replay/btc_ft_strategy_rankings.json`
- **What:** Per-strategy performance rankings used by the futures hook's winners gate
- **Updated:** Nightly cron at midnight UTC (`/api/cron/rank-strategies`) — reads last 7 days of MongoDB trades
- **TTL:** 24 hours between refreshes
- **Staleness risk:** HIGH for strategy promotion/demotion. A strategy that loses heavily in the morning won't be demoted until midnight. The engine (when `ENGINE_EXECUTION_AUTHORITY=1`) ignores this file entirely and runs all curated strategies.

### CACHE-8: Go engine in-memory BTC spot (NiftyMarketCache)
- **File:** `engine/cmd/antigravity/main.go:421` — `niftyMarketCache := NewNiftyMarketCache(240)`
- **What:** NIFTY 50 index quote cache — max 240 entries (4 hours at 1-min intervals)
- **Updated:** Every 15s during session, every 60s outside session
- **TTL:** Implicit — new quotes overwrite oldest. No explicit expiry per entry.
- **Staleness risk:** MODERATE — during session, quotes are at most 15s old

---

## 2. Polling Intervals with Staleness Risk

| Poll | File:Line | Interval | What Data | Max Stale Window | Risk |
|---|---|---|---|---|---|
| `usePaperDesk` state snapshot | `usePaperDesk.ts:116` | 5,000ms | Paper desk balance/equity/positions | 5s + Snapshotter 10s = **15s** | MODERATE |
| `useAIInsights` | `useAIInsights.ts:99` | 15,000ms | AI trading decisions | **15s** | LOW |
| `useDeltaLive` | `useDeltaLive.ts:189` | 10,000ms | Delta bridge stats | **10s** | LOW |
| `useNiftyOptionChain` | `useNiftyOptionChain.ts:48` | 30,000ms | NIFTY option chain (strikes, IVs) | **30s** | HIGH for options entry |
| `useNiftyCandles` | `useNiftyCandles.ts:47` | 60,000ms | NIFTY 1m candles | **60s** | HIGH — stale candle data |
| `useDeskPerformanceMonitor` | `useDeskPerformanceMonitor.ts:389` | 60,000ms | Performance diagnostics | **60s** | MODERATE |
| `MockResearchChartsPanel` | `MockResearchChartsPanel.tsx:283` | 30,000ms | Equity history chart | **30s** | LOW |
| `useNiftyBeesEngine` persist | `useNiftyBeesEngine.ts:976` | 30,000ms | MongoDB persistence write | **30s** | LOW |
| `useBTCFuturesScalperEngine` save | `useBTCFuturesScalperEngine.ts:1713` | 30,000ms | MongoDB trade save | **30s** | MODERATE — trade records may be 30s delayed |
| NIFTY 50 price (engine, off-session) | `main.go:1323` | 60,000ms | NIFTY spot price | **60s** | LOW (market closed) |
| NIFTY 50 price (engine, session) | `main.go:1323` | 15,000ms | NIFTY spot price | **15s** | LOW |
| StrategyHealthMonitor | `main.go:859` | 900,000ms (15min) | Strategy health metrics | **15 min** | HIGH — stale strategy status |
| PortfolioMetricsWriter | `main.go:828` | 1,800,000ms (30min) | Portfolio-level metrics | **30 min** | MODERATE |
| Funding collector | `main.go:482` | 8 hours | BTC perpetual funding rate | **8 hours** | LOW (funding rates change slowly) |
| StateSnapshotter | `main.go:818` | 10,000ms | Full account state to MongoDB | **10s** | MODERATE |

---

## 3. Load-Once / Snapshot Patterns with Stale Risk

### SNAPSHOT-1: BTC futures scalper initial load
- **File:** `client/src/hooks/useBTCFuturesScalperEngine.ts:1386-1417`
- **Pattern:** On mount, reads legacy localStorage, migrates to MongoDB, then fetches from MongoDB. State loaded once at mount, then updated by 4s polling.
- **Risk:** First render uses localStorage (potentially hours old). MongoDB fetch starts async. User sees potentially stale data on initial page load for ~1-2 seconds.

### SNAPSHOT-2: Options snapshot localStorage load
- **File:** `client/src/lib/optionsSnapshotCache.ts:16-32`
- **Pattern:** Synchronous read from localStorage on component mount. No TTL check. `savedAt` field stored but never checked.
- **Risk:** If browser tab is left open overnight, options snapshot could be hours old when component re-renders. No warning shown.
- **Evidence:** `readKey()` does not check `savedAt` against any max-age threshold.

### SNAPSHOT-3: Engine strategy warmup (one-time on boot)
- **File:** `engine/cmd/antigravity/main.go:900-924`
- **Pattern:** 3-attempt fetch of historical candles on engine boot. If all attempts fail, engine warms up from live data (cold start with no historical context).
- **Risk:** Cold-started strategies have no historical indicator buffers — they may generate signals based on incomplete data until live data fills the buffer (55+ minutes for some indicators).

### SNAPSHOT-4: Strategy rankings file (load-once per process)
- **Pattern:** The rankings file at `fixtures/replay/btc_ft_strategy_rankings.json` is read by the frontend hook at startup. It's not re-read during runtime.
- **Risk:** Rankings can be up to 24 hours old. The hook's winners gate makes entry decisions based on stale rankings.

---

## 4. Scenarios Where Stale Data Could Cause Incorrect Trading Decisions

### SCENARIO-1: NIFTY Option Chain 30s Stale During Entry
`useNiftyOptionChain` polls every 30s. If a high-volatility event moves option prices significantly within those 30 seconds, the displayed strike prices and IVs are stale. A trader using the UI to place an options order would see incorrect prices.

### SCENARIO-2: BTC Position After Close — 15s UI Lag
A BTC futures position closes at the Go engine. Engine updates in-memory state immediately. MongoDB update occurs within ~10s (StateSnapshotter). Frontend poll occurs within ~4s after MongoDB update. User sees the position as open with unrealized PnL displayed for up to **14 seconds** after actual close.

### SCENARIO-3: AngelOne Token Invalid — Silent MCX Failures
If AngelOne invalidates a token mid-session (e.g., concurrent login from another device), the module-scope cache holds the invalid token for up to 4 hours for MCX routes. MCX LTP calls return errors silently (route returns 503 or error JSON). MCX engine continues with stale price or last known price.

### SCENARIO-4: Strategy Health — Losing Strategy Not Demoted for 15 Minutes
A strategy enters a drawdown. The `StrategyHealthMonitor` updates MongoDB every 15 minutes. The UI shows the strategy as healthy for up to 15 minutes. If the strategy's `disabled_strategies` status is manually set in the UI (only works when `ENGINE_EXECUTION_AUTHORITY=0`), the engine running in production ignores it.

### SCENARIO-5: Options Snapshot Cache After Browser Sleep
Browser tab sleeps (modern browsers throttle background tabs). On re-activation, localStorage options snapshot is read synchronously. Snapshot could be hours old. Options positions displayed are stale until the next engine API poll completes (~3s for options hook).

---

## 5. Verdict: Maximum Age of Displayed Trading Data

Under normal operation:

| Data Type | Maximum Age | Acceptable? |
|---|---|---|
| BTC live price (WebSocket) | ~1s (last tick) | YES |
| BTC paper balance/equity | ~14s | ACCEPTABLE |
| Open positions | ~14s | ACCEPTABLE |
| Options positions | ~3s (direct engine poll) | YES |
| NIFTY price (UI, SSE) | ~3s | YES |
| NIFTY option chain | **30s** | MARGINAL |
| NIFTY candles | **60s** | RISKY |
| Strategy health status | **15 min** | RISKY |
| Portfolio metrics (drawdown) | **30 min** | RISKY |
| Strategy rankings (winners gate) | **24h** | RISKY in production |
| BTC options snapshot (after tab sleep) | **Unbounded** (localStorage no TTL) | CRITICAL |

**Overall verdict: MIXED.** Live price feeds are fresh (sub-5s). Account state has acceptable 14s lag. However, three data types have unacceptable staleness for active trading: NIFTY candles (60s), strategy health (15 min), and the localStorage options cache (no TTL). Strategy rankings used for the winners gate are refreshed only daily, meaning any intraday strategy performance changes are invisible to the entry gate for up to 24 hours.
