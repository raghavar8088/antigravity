# FRONTEND → BACKEND MAPPING REPORT — Forensic Audit
# Date: 2026-06-11 | Auditor: Claude Code

Classifications:
  FULLY_WIRED    — frontend caller exists, backend handler exists, data flows correctly
  PARTIALLY_WIRED — one side exists but integration is incomplete
  STUB           — route file exists but returns hardcoded/empty data
  DEPRECATED     — route returns 410 Gone (intentionally retired)
  DEAD           — route exists with no known frontend caller
  PROXY          — Next.js route proxies to Go engine
  MOCK           — frontend-only simulation, no engine involvement

---

## 1. COMPLETE ROUTE CLASSIFICATION TABLE

### Authentication Routes

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET /api/auth/session` | FULLY_WIRED | `useOwnerAuth.ts:18` | `client/src/app/api/auth/session/route.ts` | JWT verify from raig_session cookie |
| `POST /api/auth/signin` | FULLY_WIRED | `useOwnerAuth.ts:46` | `client/src/app/api/auth/signin/route.ts` | Returns JWT |
| `POST /api/auth/signout` | FULLY_WIRED | `useOwnerAuth.ts:76` | `client/src/app/api/auth/signout/route.ts` | Clears cookie |
| `POST /api/auth/login` | FULLY_WIRED | `useOwnerAuth.ts` | `client/src/app/api/auth/login/route.ts` | Alias for signin |

### Paper Desk Routes (Go Engine data via MongoDB)

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET /api/paper-desk/snapshot` | FULLY_WIRED | `usePaperDesk.ts:pollSnapshot` (5s) | `client/src/app/api/paper-desk/snapshot/route.ts` | Aggregated snapshot → MongoDB |
| `GET /api/paper-desk/state` | FULLY_WIRED | `usePaperDesk.ts` | `client/src/app/api/paper-desk/state/route.ts` | Paper state from MongoDB |
| `GET /api/paper-desk/trades` | FULLY_WIRED | `usePaperDesk.ts` | `client/src/app/api/paper-desk/trades/route.ts` | Trade history from MongoDB |
| `GET /api/paper-desk/positions` | FULLY_WIRED | `usePaperDesk.ts` | `client/src/app/api/paper-desk/positions/route.ts` | Open positions from MongoDB |
| `GET /api/paper-desk/equity` | FULLY_WIRED | `usePaperDesk.ts` | `client/src/app/api/paper-desk/equity/route.ts` | Equity curve from MongoDB |
| `GET /api/paper-desk/strategy-health` | FULLY_WIRED | `usePaperDesk.ts` | `client/src/app/api/paper-desk/strategy-health/route.ts` | Strategy health from MongoDB |
| `GET /api/paper-desk/portfolio` | PARTIALLY_WIRED | Not confirmed in hooks | `client/src/app/api/paper-desk/portfolio/route.ts` | Portfolio from MongoDB |
| `GET /api/paper-desk/orders` | PARTIALLY_WIRED | `usePaperDesk.ts` (lazy) | `client/src/app/api/paper-desk/orders/route.ts` | OMS orders from MongoDB |
| `GET /api/paper-desk/validation` | DEAD | No confirmed caller | `client/src/app/api/paper-desk/validation/route.ts` | Validation checks |
| `GET /api/paper-desk/strategy-analytics` | DEAD | No confirmed caller | `client/src/app/api/paper-desk/strategy-analytics/route.ts` | Strategy analytics |
| `GET /api/paper-desk/diagnostics` | FULLY_WIRED | System health route + browser | `client/src/app/api/paper-desk/diagnostics/route.ts` → proxies to Go engine | Proxies to Go `/api/paper-desk/diagnostics` |

### Engine Proxy Route

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `ANY /api/engine/[...path]` | PROXY | `useEngineState.ts`, various | `client/src/app/api/engine/[...path]/route.ts` | Security-gated proxy to Go engine. Allowlist: paper-desk/state, positions, trades, diagnostics, options, nifty, delta-live, security, health, metrics. Admin paths require ENGINE_ADMIN_SECRET. Blocked paths return 403. |

### Execution Routes

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `POST /api/execution/request` | FULLY_WIRED | Execution panels | `client/src/app/api/execution/request/route.ts` → Go `/api/execution/request` | Proxies to Go institutional gateway |
| `POST /api/angelone/order` | DEPRECATED | Was `AngelOneOrderPanel` | `client/src/app/api/angelone/order/route.ts` | Returns 410; use `/api/execution/request` |
| `GET/POST /api/delta/spot` | DEPRECATED | `DeltaSpotBuy.tsx:193` (GET blocked) | `client/src/app/api/delta/spot/route.ts` | Returns 410; use `/api/execution/request` |
| `GET /api/angelone/orders` | FULLY_WIRED | `useAngelOneOrders.ts:83` | `client/src/app/api/angelone/orders/route.ts` | Read-only order list |
| `GET /api/angelone/funds` | FULLY_WIRED | `useAngelOneOrders.ts:84` | `client/src/app/api/angelone/funds/route.ts` | Account funds |
| `POST /api/angelone/cancel-order` | PARTIALLY_WIRED | `AngelOneOrderPanel.tsx` | `client/src/app/api/angelone/cancel-order/route.ts` | Cancel order |

### BTC Market Data Routes

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET /api/btc/price` | FULLY_WIRED | `useLiveBTCPrice.ts:39` | `client/src/app/api/btc/price/route.ts` | BTC live price |
| `GET /api/btc/spot-klines` | FULLY_WIRED | `BTCLiveChart.tsx:83`, `useBTCSpotScalperEngine.ts:2138` | `client/src/app/api/btc/spot-klines/route.ts` | BTC klines |
| `GET /api/btc/spot-state` | FULLY_WIRED | `useBTCSpotScalperEngine.ts:1621,2115` | `client/src/app/api/btc/spot-state/route.ts` | BTC spot engine state (MongoDB) |
| `GET /api/btc/futures-klines` | PARTIALLY_WIRED | Not confirmed in hooks | `client/src/app/api/btc/futures-klines/route.ts` | BTC futures klines |
| `GET /api/btc/option-chain` | FULLY_WIRED | `useDeltaStrikes.ts:26` | `client/src/app/api/btc/option-chain/route.ts` | BTC option chain |

### NIFTY Routes

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET /api/nifty/candles` | FULLY_WIRED | `useNiftyCandles.ts:22`, `useNiftyOptionsEngine.ts:1593`, `useNiftyOptionsSellingEngine.ts:780` | `client/src/app/api/nifty/candles/route.ts` | NIFTY candles (Yahoo/NSE) |
| `GET /api/nifty/stream` | FULLY_WIRED | `useNiftyMarket.ts` (SSE) | `client/src/app/api/nifty/stream/route.ts` | SSE NIFTY price feed (Yahoo Finance) |
| `GET/POST /api/nifty/state` | FULLY_WIRED | `useNiftyOptionsEngine.ts:1267,1308` | `client/src/app/api/nifty/state/route.ts` | NIFTY engine state (MongoDB) |
| `GET/POST /api/nifty/selling-state` | FULLY_WIRED | `useNiftyOptionsSellingEngine.ts:358,370` | `client/src/app/api/nifty/selling-state/route.ts` | NIFTY selling state (MongoDB) |
| `GET/POST /api/nifty/stocks-state` | FULLY_WIRED | `useNiftyStocksEngine.ts:780,790` | `client/src/app/api/nifty/stocks-state/route.ts` | NIFTY stocks state (MongoDB) |
| `GET /api/nifty/vix` | FULLY_WIRED | `useNiftyVIX.ts:15` | `client/src/app/api/nifty/vix/route.ts` | NIFTY VIX |
| `GET /api/nifty/option-chain` | FULLY_WIRED | `useNiftyOptionChain.ts` | `client/src/app/api/nifty/option-chain/route.ts` | NIFTY option chain |
| `POST /api/nifty/seed-engine` | FULLY_WIRED | `useNiftyCandles.ts:34` | `client/src/app/api/nifty/seed-engine/route.ts` → Go engine | Blocked from Vercel proxy; called directly |

### MCX Routes

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET/POST /api/mcx/state` | FULLY_WIRED | `useMCXEngine.ts:916,928` | `client/src/app/api/mcx/state/route.ts` | MCX state (MongoDB) |
| `GET /api/mcx/ltp` | FULLY_WIRED | `useMCXEngine.ts:1444` | `client/src/app/api/mcx/ltp/route.ts` | MCX LTP |
| `GET /api/mcx/candles` | PARTIALLY_WIRED | Not in confirmed hooks | `client/src/app/api/mcx/candles/route.ts` | MCX candles |
| `GET /api/mcx/debug` | DEAD | Not confirmed | `client/src/app/api/mcx/debug/route.ts` | Debug |

### NiftyBees Routes

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET /api/nifty-bees/candles` | FULLY_WIRED | `useNiftyBeesEngine.ts:916` | `client/src/app/api/nifty-bees/candles/route.ts` | NiftyBees candles |
| `GET /api/nifty-bees/ltp` | FULLY_WIRED | `useNiftyBeesEngine.ts:944` | `client/src/app/api/nifty-bees/ltp/route.ts` | NiftyBees LTP |

### Crypto Routes

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET /api/crypto/markets` | FULLY_WIRED | `useCryptoEquityEngine.ts:1117` | `client/src/app/api/crypto/markets/route.ts` | Crypto market data |
| `GET/POST /api/crypto/equity-state` | FULLY_WIRED | `useCryptoEquityEngine.ts:674,691` | `client/src/app/api/crypto/equity-state/route.ts` | Crypto equity engine state |

### Stocks Routes

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET /api/stocks/ltp` | FULLY_WIRED | `useNiftyStocksEngine.ts:1169` | `client/src/app/api/stocks/ltp/route.ts` | Stocks LTP |

### Delta Routes

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET /api/delta/account` | FULLY_WIRED | `useDeltaLive.ts:136` | `client/src/app/api/delta/account/route.ts` | Delta account balance |
| `GET /api/delta/myip` | DEAD | Not confirmed | `client/src/app/api/delta/myip/route.ts` | IP address check |
| `POST /api/delta/testnet/ping` | FULLY_WIRED | `TestnetOpsPanel.tsx:56` | `client/src/app/api/delta/testnet/ping/route.ts` | Delta testnet ping |
| `GET /api/delta/testnet/positions` | FULLY_WIRED | `TestnetOpsPanel.tsx:57` | `client/src/app/api/delta/testnet/positions/route.ts` | Delta testnet positions |

### Mock Trading Routes (Browser-side engine + MongoDB)

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET /api/mock-trading/account` | FULLY_WIRED | `useMockTradingEngine.ts:327` | `client/src/app/api/mock-trading/account/route.ts` | DEPRECATED POST, GET reads from MongoDB |
| `GET /api/mock-trading/account/latest` | PARTIALLY_WIRED | Not confirmed | `client/src/app/api/mock-trading/account/latest/route.ts` | Latest account state |
| `GET/POST /api/mock-trading/trades` | FULLY_WIRED | `useBTCFuturesScalperEngine.ts` | `client/src/app/api/mock-trading/trades/route.ts` | POST DEPRECATED (410); GET from MongoDB |
| `GET /api/mock-trading/trades/[id]` | PARTIALLY_WIRED | Not confirmed | route exists | Single trade lookup |
| `POST /api/mock-trading/trades/[id]/close` | PARTIALLY_WIRED | Not confirmed | route exists | Close trade |
| `GET/POST /api/mock-trading/signals` | FULLY_WIRED | `useMockResearchRunner.ts:519` | `client/src/app/api/mock-trading/signals/route.ts` | Strategy signals (MongoDB) |
| `GET/POST /api/mock-trading/regime` | FULLY_WIRED | `MockTradingDashboard.tsx:1278`, `useMarketRegime.ts` | `client/src/app/api/mock-trading/regime/route.ts` | Regime snapshot (MongoDB) |
| `GET/POST /api/mock-trading/equity` | FULLY_WIRED | `MockTradingDashboard.tsx:1290`, `MockResearchChartsPanel.tsx:252` | `client/src/app/api/mock-trading/equity/route.ts` | Equity curve (MongoDB) |
| `GET/POST /api/mock-trading/daily-pnl` | FULLY_WIRED | `MockTradingDashboard.tsx:1314` | `client/src/app/api/mock-trading/daily-pnl/route.ts` | Daily P&L (MongoDB) |
| `GET/POST /api/mock-trading/scores` | FULLY_WIRED | `MockTradingDashboard.tsx:1333` | `client/src/app/api/mock-trading/scores/route.ts` | Strategy scores (MongoDB) |
| `GET /api/mock-trading/analytics` | FULLY_WIRED | `MockRiskAnalyticsPanel.tsx` | `client/src/app/api/mock-trading/analytics/route.ts` | Analytics (MongoDB) |
| `GET /api/mock-trading/logs` | DEAD | Not confirmed | `client/src/app/api/mock-trading/logs/route.ts` | Worker logs |
| `POST /api/mock-trading/reset` | FULLY_WIRED | `useMockTradingEngine.ts:1004` | `client/src/app/api/mock-trading/reset/route.ts` | Reset account |

### Paper State / Legacy Routes

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET/POST /api/paper-state` | PARTIALLY_WIRED | `useBTCFuturesScalperEngine.ts:1378,1398` | `client/src/app/api/paper-state/route.ts` | Legacy paper state; POST deprecated when engine authority active |
| `POST /api/paper-state/repair` | FULLY_WIRED | `BTCFuturesScalper.tsx:276` | `client/src/app/api/paper-state/repair/route.ts` | State repair |

### Paper Trades / Analytics Routes

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET/POST /api/paper-trades` | FULLY_WIRED | `useBTCFuturesScalperEngine.ts` | `client/src/app/api/paper-trades/route.ts` | Multi-account trade store (MongoDB) |
| `GET /api/paper-trades/analytics` | DEAD | Not confirmed | route | Analytics |
| `GET /api/paper-trades/export` | DEAD | Not confirmed | route | CSV export |
| `GET /api/paper-trades/leaderboard` | FULLY_WIRED | `MockStrategyLeaderboardPanel.tsx` | `client/src/app/api/paper-trades/leaderboard/route.ts` | Strategy leaderboard |
| `GET /api/paper-trades/strategy-stats` | FULLY_WIRED | Strategy panels | route | Strategy stats |
| `POST /api/paper-trades/strategy-research` | FULLY_WIRED | `StrategyResearchPanel.tsx:148` | route | Strategy research |

### Trade History Routes

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET /api/trade-history/dates` | DEAD | Not confirmed | route | Trade history dates |
| `GET /api/trade-history/daily-run` | DEAD | Not confirmed | route | Daily run data |
| `GET /api/trade-history/download` | DEAD | Not confirmed | route | Download trade history |

### Strategy Routes

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET /api/strategy-rankings` | FULLY_WIRED | `useBTCFuturesScalperEngine.ts:1629` | `client/src/app/api/strategy-rankings/route.ts` | Strategy rankings from MongoDB/file |
| `GET /api/strategy-attribution/[id]` | FULLY_WIRED | `AttributionPanel.tsx` | route | Per-strategy attribution |
| `GET /api/strategy-signal-trace` | FULLY_WIRED | `useBTCFuturesScalperEngine.ts:3771` | route | Signal trace (MongoDB) |

### Replay / Research Routes

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET/POST /api/paper-replay` | FULLY_WIRED | `ReplayBacktestPanel.tsx` | route | Replay backtest (MongoDB) |
| `GET /api/paper-replay-compare` | FULLY_WIRED | `ReplayBacktestPanel.tsx` | route | Replay comparison |
| `POST /api/replay-walkforward` | FULLY_WIRED | `ReplayWalkForwardLab.tsx` | route | Walk-forward lab |
| `GET /api/paper-research` | DEAD | Not confirmed | route | Paper research |
| `GET /api/research-edge-report` | DEAD | Not confirmed | route | Edge report |

### Shadow Trade Routes

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET/POST /api/shadow-trade-intents` | FULLY_WIRED | `ShadowIntentLogPanel.tsx` | route | Shadow trade intents (MongoDB) |

### AI App Tracker Routes

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET /api/ai-app-tracker/latest` | FULLY_WIRED | `AiAppTrackerPanel.tsx:30` | route | Latest tracker entry |
| `GET /api/ai-app-tracker/reports` | DEAD | Panel only shows latest | route | Reports list |
| `POST /api/ai-app-tracker/capture` | FULLY_WIRED | `AiAppTrackerPanel.tsx:53` | route | Capture AI event |

### Verification Track Routes

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET /api/verification-track/latest` | FULLY_WIRED | `VerificationTrackPanel.tsx` | route | Latest verification event |
| `GET /api/verification-track/summary` | FULLY_WIRED | `VerificationTrackPanel.tsx` | route | Summary |
| `POST /api/verification-track/events` | DEAD | Not confirmed in hooks | route | Write verification event |
| `GET /api/verification-track/ai-context` | DEAD | Not confirmed | route | AI context |

### Paper OMS Routes (Go Engine Paper OMS via MongoDB)

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET /api/paper-oms/orders` | FULLY_WIRED | `PaperOmsPanel.tsx` | route → MongoDB | OMS orders from MongoDB |
| `GET /api/paper-oms/summary` | FULLY_WIRED | `PaperOmsPanel.tsx` | route → MongoDB | OMS summary |

### System Health Routes

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET /api/system/health` | FULLY_WIRED | System health panels | route | Full system health check |
| `GET /api/system/production-validation` | DEAD | Not confirmed | route | Production validation |
| `GET /api/health/paper-desk` | FULLY_WIRED | `BTCFuturesScalper.tsx:263,327` | route | Desk worker health |
| `GET /api/health/desk-worker` | FULLY_WIRED | `BTCFuturesScalper.tsx:263,327` | route | Desk worker status |
| `GET /api/health/storage` | DEAD | Not confirmed | route | Storage health |

### Desk Worker Events

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET /api/desk-worker-events` | FULLY_WIRED | `DeskCommandCenter.tsx:171` | route → MongoDB | Worker event log |
| `GET /api/desk-entry-funnel` | FULLY_WIRED | `useBTCFuturesScalperEngine.ts:3812` | route | Entry funnel snapshot |

### Storage Routes

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET /api/storage/health` | FULLY_WIRED | `StorageHealthPanel.tsx:87` | route | Storage health (MongoDB + local) |
| `POST /api/storage/backup` | FULLY_WIRED | `StorageHealthPanel.tsx:100` | route | Trigger backup |
| `GET /api/storage/restore` | DEAD | Not confirmed | route | Restore from backup |

### Probe Routes

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET /api/probe/delta-btc` | DEAD | Not confirmed in frontend | `client/src/app/api/probe/delta-btc/route.ts` → proxies to Go engine | Connectivity probe |
| `GET /api/probe/angelone-nifty` | DEAD | Not confirmed in frontend | `client/src/app/api/probe/angelone-nifty/route.ts` → proxies to Go engine | Connectivity probe |

### Cron Routes

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET /api/cron/rank-strategies` | FULLY_WIRED | Vercel cron (midnight UTC) | route → MongoDB | Nightly strategy ranking |
| `GET /api/cron/policy-snapshot` | FULLY_WIRED | Vercel cron (12:05 UTC) | route | Policy snapshot |
| `GET /api/cron/paper-desk-tick` | PARTIALLY_WIRED | NOT in vercel.json crons (only in code comments) | route | Backup tick — NOT scheduled |

### Options Snapshot

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET /api/options/paper-snapshot` | PARTIALLY_WIRED | Not confirmed | route | Options paper snapshot |

### MCX State

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `GET /api/mcx/state` | FULLY_WIRED | `useMCXEngine.ts:916,928` | route → MongoDB | MCX engine state |

### Admin Routes

| Route | Type | Frontend Caller | Backend Handler | Notes |
|-------|------|----------------|----------------|-------|
| `POST /api/admin/migrate-owner` | DEAD | Not confirmed | route | Migrate owner account key |

---

## 2. ORPHANED ROUTES (Exist in api/ but no confirmed frontend caller)

| Route | File | Probable Status |
|-------|------|----------------|
| `/api/mcx/debug` | `client/src/app/api/mcx/debug/route.ts` | DEAD — debug endpoint |
| `/api/delta/myip` | `client/src/app/api/delta/myip/route.ts` | DEAD — IP check utility |
| `/api/trade-history/dates` | route | DEAD |
| `/api/trade-history/daily-run` | route | DEAD |
| `/api/trade-history/download` | route | DEAD |
| `/api/paper-trades/analytics` | route | DEAD |
| `/api/paper-trades/export` | route | DEAD (or accessed via direct link) |
| `/api/paper-research` | route | DEAD |
| `/api/research-edge-report` | route | DEAD |
| `/api/verification-track/events` (POST) | route | DEAD |
| `/api/verification-track/ai-context` | route | DEAD |
| `/api/mock-trading/logs` | route | DEAD |
| `/api/system/production-validation` | route | DEAD |
| `/api/health/storage` | route | DEAD |
| `/api/storage/restore` | route | DEAD |
| `/api/probe/delta-btc` | route | DEAD (admin use only?) |
| `/api/probe/angelone-nifty` | route | DEAD (admin use only?) |
| `/api/admin/migrate-owner` | route | DEAD |
| `/api/btc/futures-klines` | route | PARTIALLY — not found in hooks |
| `/api/ai-app-tracker/reports` | route | PARTIALLY — panel may use it |
| `/api/paper-desk/validation` | route | DEAD |
| `/api/paper-desk/strategy-analytics` | route | DEAD |
| `/api/paper-desk/portfolio` | route | Called from snapshot route but not directly from components |

---

## 3. DEPRECATED ROUTES (Return 410 Gone)

| Route | Returns | Old Purpose | Replacement |
|-------|---------|------------|------------|
| `POST /api/mock-trading/trades` | 410 | Browser trade creation | Go engine writes to paper_trades |
| `POST /api/mock-trading/account` | 410 | Browser account snapshots | Go engine writes to paper_state |
| `POST /api/angelone/order` | 410 | Direct AngelOne execution | `/api/execution/request` |
| `POST /api/delta/spot` | 410 | Direct Delta spot execution | `/api/execution/request` |
| `POST /api/paper-state` | 410 (when `ENGINE_EXECUTION_AUTHORITY=1`) | Browser paper state writes | Go engine writes to MongoDB |

---

## 4. CRITICAL ARCHITECTURAL FINDING: DUAL ENGINE PROBLEM

**Two parallel trading engines exist and can conflict:**

### Engine A: Go Engine (engine/cmd/antigravity/)
- Account key: `btc-paper-1` (set by paperpersist.AccountKey() from env)
- Writes to MongoDB collections: `paper_trades`, `paper_positions`, `paper_state`
- Displayed on: `/paper-desk` page via `usePaperDesk` hook
- Authority: PRIMARY (all write routes defer to it)

### Engine B: Browser-side Engine (client/src/hooks/useBTCFuturesScalperEngine.ts)
- Account key: `OWNER_ACCOUNT_KEY = "mock_trading_default"` (hardcoded in `ownerAuth.ts`)
- Writes to MongoDB collections: `mock_trades`, `mock_account_state`
- Displayed on: `/btc-future-trading`, `/mock-trading` pages
- Authority: Secondary; routes return 410 when `ENGINE_EXECUTION_AUTHORITY=1`

**When `ENGINE_EXECUTION_AUTHORITY=1` env var is set**, the browser engine POST routes return 410. When it is NOT set, both engines can write concurrently under different account keys.

---

## 5. FRONTEND → GO ENGINE CALL CHAIN (Full Trace)

### Example: Paper Desk page loads positions

```
Browser /paper-desk
  → PaperDeskDashboard component
  → usePaperDesk hook (every 5s)
  → GET /api/paper-desk/snapshot (Next.js API route)
  → client/src/app/api/paper-desk/snapshot/route.ts
  → paperDeskClient.listOpenPositions(accountKey)
  → MongoDB Atlas paper_positions collection
  (written by Go engine paperpersist every 10s)
```

### Example: Execution request

```
Browser clicks "Execute"
  → POST /api/execution/request (Next.js API route)
  → client/src/app/api/execution/request/route.ts
  → getAuthenticatedApiSession() — verifies raig_session JWT
  → fetch(INTERNAL_API_URL + "/api/execution/request")
  → Go engine executiongateway.Handler
  → orchestrator.ProcessExecutionRequest()
  → Delta Bridge (if venue="delta")
  → deltaBridge.OnOpen()
  → Delta Exchange REST API
```

### Example: Kill switch activation

```
Admin POST /api/admin/ks/block (via engine proxy or direct)
  → Go engine /api/admin/ks/block handler
  → ksSvc.Trigger(TriggerManualOperator, ActionBlockNewOrders)
  → ledger.Store.Append(kill_switch event)
  → orchestrator.PreTradeRiskPipeline blocks all new orders
```

---

## 6. SUMMARY COUNTS

| Classification | Count |
|---------------|-------|
| FULLY_WIRED | ~58 routes |
| PARTIALLY_WIRED | ~12 routes |
| DEPRECATED (410) | 5 routes |
| DEAD (no caller found) | ~23 routes |
| PROXY (Go engine) | 1 catch-all + 5 specific |
| MOCK (browser-only) | ~15 mock-trading routes |

Total Next.js API routes: ~113

---

## 7. ROUTES MISSING FROM Go ENGINE (Frontend calls but engine may not handle)

| Frontend Call | Go Engine Handler |
|--------------|-----------------|
| `GET /api/paper-desk/positions` (via engine proxy) | `GET /api/paper-desk/positions` registered? — NOT found in main.go HTTP handlers. The MongoDB path serves this from Next.js directly, NOT from Go engine. |
| `GET /api/paper-desk/trades` (via engine proxy) | Same — served from Next.js MongoDB, not Go engine. |

FINDING: The `/api/engine/[...path]` proxy includes `/api/paper-desk/positions` and `/api/paper-desk/trades` in its READ_PATH allowlist, but the Go engine does NOT register these endpoints. The Go engine only registers `/api/paper-desk/diagnostics`. The paper-desk data comes directly from MongoDB via Next.js routes, NOT via the engine proxy.

The proxy allowlist for `paper-desk/*` paths is MISLEADING — those paths proxy to the Go engine, but the Go engine only handles `/api/paper-desk/diagnostics`. All other `/api/paper-desk/*` routes are served directly from Next.js → MongoDB without going through the engine.
