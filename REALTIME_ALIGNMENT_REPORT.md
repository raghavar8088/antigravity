# Realtime Alignment Report — Phase 12
# Generated: 2026-06-11 | Auditor: Claude Code Forensic Audit

---

## 1. WebSocket Connections

### WS-1: Coinbase WebSocket (Go Engine — PRIMARY BTC price feed)
- **File:** `engine/internal/marketdata/coinbase.go` (uses `github.com/gorilla/websocket`)
- **Endpoint:** Coinbase Advanced Trade WebSocket (BTC-USD)
- **Data:** Live BTC-USD tick prices
- **Connection management:** Started at engine boot (`main.go:419-429`). Uses `coinbaseClient.Connect(ctx, []string{"BTC-USD"})` in a goroutine. If `Connect` returns an error the goroutine calls `log.Fatalf` — this is a hard process kill on initial connect failure.
- **Reconnect:** Coinbase client implementation in `engine/internal/marketdata/coinbase.go` — not read in detail but gorilla/websocket is used. The goroutine runs until context cancellation.
- **Failure mode:** If Coinbase WS drops after initial connect, the engine has an Execution Watchdog (`execWatchdog`) that monitors for stale market data (`main.go:771-773`). If market data goes stale, it triggers the kill switch. Evidence: `execWatchdog.Health()` checks `StaleMarketData` boolean.
- **Fallback:** No automatic fallback to Binance for the primary BTC price feed. The execution watchdog activates kill switch on stale data but does NOT switch to Binance WS.
- **Silent failure risk:** YES — if gorilla/websocket reconnects silently but receives no data, the `StaleMarketData` flag may not trigger unless the watchdog timeout fires.

### WS-2: Binance WebSocket (Next.js client — `client/src/hooks/useLiveBTCMarket.ts`)
- **File:** `client/src/hooks/useLiveBTCMarket.ts:455-613`
- **Endpoint:** `wss://stream.binance.com:9443/stream?streams=btcusdt@ticker/btcusdt@kline_1m`
- **Data:** BTC live ticker price + 1m candles for dashboard display
- **Connection management:** Reconnect logic with exponential backoff (`attempt` counter). `reconnectTimer` fires on `socket.onclose`. `closedByHook` flag prevents reconnect after unmount.
- **Ping:** Bybit has 20s ping interval; Binance stream does not need pings (server-side keepalive).
- **Fallback:** Exchange selector supports `binance` or `bybit` (`wss://stream.bybit.com/v5/public/linear`). UI can switch exchange.
- **Silent failure risk:** MODERATE — `onclose` fires reconnect but if the WS receives data that fails JSON parse, the error is swallowed silently (catch block returns without state update).

### WS-3: Binance WebSocket (DeltaSpotBuy component — `client/src/components/DeltaSpotBuy.tsx:157`)
- **File:** `client/src/components/DeltaSpotBuy.tsx:152-173`
- **Endpoint:** `wss://stream.binance.com:9443/ws/btcusdt@trade`
- **Data:** BTC trade stream for real-time price in Delta Spot Buy UI
- **Connection management:** Simple `new WebSocket(url)` with `onmessage`, no reconnect logic visible at line 152-173.
- **Silent failure risk:** HIGH — no reconnect on close/error. Price display goes stale without warning.

### WS-4: Terminal WebSocket (client/src/lib/terminal/terminalStore.tsx:26-30)
- **File:** `client/src/lib/terminal/terminalStore.tsx:26-30`
- **Endpoint:** `wsUrl` from config — connects to terminal backend
- **Data:** Terminal command/output stream
- **Connection management:** Simple connect, no reconnect visible in the snippet.
- **Silent failure risk:** MODERATE

---

## 2. Server-Sent Events (SSE) Streams

### SSE-1: NIFTY 50 Live Stream (`/api/nifty/stream`)
- **File:** `client/src/app/api/nifty/stream/route.ts`
- **Endpoint:** `GET /api/nifty/stream`
- **Data:** NIFTY price, open, high, low, close, change, percentChange from Yahoo Finance v8 chart API
- **Poll interval:** 3 seconds inside SSE while-loop
- **Connection management:**
  - Server-side: `ReadableStream` with `while(active)` loop, polls Yahoo Finance every 3s
  - `maxDuration = 60` (Vercel 60s max) — stream will close after 60s forcing client reconnect
  - When Yahoo is unavailable, sends `data: {"error":"Yahoo Finance unavailable"}` but does NOT close stream
  - Tries two Yahoo mirrors (`query1.finance.yahoo.com`, `query2.finance.yahoo.com`)
- **Client reconnect:** Not verified here — client must handle EventSource reconnection (browsers auto-reconnect EventSource by default)
- **Error handling:** Errors are sent as SSE data events with `"error"` field, not as HTTP errors. Client must inspect data to detect failure.
- **Silent failure risk:** MODERATE — Yahoo Finance rate limiting or blocking could cause all ticks to be error events. The stream stays open sending error JSON. Client may display stale price if it doesn't handle error ticks.
- **Fallback:** None from SSE itself. The Go engine independently polls NSE every 15s (session) or 60s (off-session) and uses AngelOne as fallback.

### SSE-2: Desk Worker Events (potential SSE via `/api/desk-worker-events`)
- **File:** `client/src/app/api/desk-worker-events/route.ts`
- **Data:** MongoDB desk_worker_events collection reads
- **Type:** REST polling (not SSE) — serves latest events, not a stream

---

## 3. Polling Mechanisms

| Hook / Component | Interval | Endpoint / Action | Stale Window | Notes |
|---|---|---|---|---|
| `useBTCFuturesScalperEngine` | 4,000ms | Worker state, strategy signals | 4s | Main paper desk tick |
| `useBTCFuturesScalperEngine` (save) | 30,000ms | MongoDB save (paperTradesSync) | 30s | Trade persistence |
| `useBTCSpotScalperEngine` | 2,000ms | BTC spot data, tick | 2s | BTC spot scalper |
| `useCryptoEquityEngine` | 3,000ms | Crypto equity data | 3s | |
| `useLiveBTCPrice` | 3,000ms | `/api/btc/price` (Binance/Coinbase REST) | 3s | |
| `useNiftyBeesEngine` | 4,000ms | Nifty Bees engine tick | 4s | |
| `useNiftyOptionsEngine` | 1,000ms | NIFTY options engine tick (client-side) | 1s | Client-side simulation |
| `useNiftyOptionsSellingEngine` | 1,000ms | NIFTY options selling tick | 1s | Client-side simulation |
| `useNiftyStocksEngine` | 3,000ms | NIFTY stocks engine tick | 3s | |
| `useMCXEngine` | 5,000ms | MCX engine tick + poll | 5s | |
| `usePaperDesk` | 5,000ms | Paper desk snapshot | 5s | |
| `useAIInsights` | 15,000ms | `/api/ai/insights` (via Go engine) | 15s | AI decisions |
| `useAngelOneOrders` | 15,000ms | AngelOne orders | 15s | |
| `useDeltaLive` | 10,000ms | `/api/delta-live/stats` | 10s | Delta live stats |
| `useEngineState` | 5,000ms | Engine health check | 5s | |
| `useMockTradingEngine` (signal trace) | 5,000ms | `/api/mock-trading/signals` | 5s | |
| `useFearGreed` | 600,000ms | Fear & greed index | 10 min | Not trading-critical |
| `useEngineLogs` | 4,000ms | `/api/logs` (Go engine) | 4s | |
| `useNiftyOptions` | 3,000ms | NIFTY options data | 3s | |
| `useNiftyOptionsSelling` | 3,000ms | NIFTY options selling data | 3s | |
| `useDeskPerformanceMonitor` | 60,000ms | Performance diagnostics | 60s | **STALE RISK** |
| `useNiftyOptionChain` | 30,000ms | NIFTY option chain | 30s | **STALE RISK** for options trading |
| `useNiftyCandles` | 60,000ms | NIFTY 1m candles | 60s | **STALE RISK** |
| `useLiveDataLab` | 10,000ms | Delta + Angel data | 10s | |
| `BTCLiveChart` | 2,000ms | BTC chart data | 2s | |
| `BTCFuturesScalper` component | 10,000ms | Strategy data load | 10s | |
| `CommandCenter` (pending) | 3,000ms | AI pending signals | 3s | |
| `CommandCenter` (status) | 5,000ms | Bridge status | 5s | |
| `MockResearchChartsPanel` | 30,000ms | Equity history | 30s | |
| `useNiftyBeesEngine` (persist) | 30,000ms | MongoDB persist | 30s | |

---

## 4. Silent Failure Analysis

### Can Coinbase WS fail silently?
**YES.** The Execution Watchdog checks for stale market data, but only triggers the kill switch after a timeout. During that window (not specified in code) the engine continues to use the last known price. Evidence: `execWatchdog.Health()` field `StaleMarketData` and `NoTradeAlertLevel`.

### Can the NIFTY SSE fail silently?
**YES.** If Yahoo Finance returns HTTP errors, the SSE sends error JSON events. If the frontend `EventSource` handler doesn't inspect the `error` field in data events (only checks HTTP connection errors), it will display the last valid price indefinitely.

### Can Binance WS fail silently?
**YES** for `DeltaSpotBuy.tsx` (WS-3) — no reconnect. For `useLiveBTCMarket.ts` (WS-2) — reconnect logic present but JSON parse errors are silently discarded.

### Can polling fail silently?
**YES** for any hook that doesn't update UI state on error. Most hooks catch errors and either set error state or log. However `useNiftyOptionChain` polling 30s and `useNiftyCandles` polling 60s provide stale data for up to 60s between refreshes with no visual staleness indicator found.

---

## 5. Market Data Stream Fallbacks

| Feed | Primary | Fallback | Fallback Active? |
|---|---|---|---|
| BTC price (engine) | Coinbase WS | None (kill switch fires) | PARTIAL — kill switch only |
| BTC price (options engine) | Delta REST (10s poll) | Binance REST | YES — code verified |
| BTC price (frontend UI) | Binance WS | Bybit WS (user-selectable) | YES but manual |
| BTC price (REST, frontend) | Binance REST | Coinbase REST | YES |
| NIFTY 50 (engine) | NSE REST (15s) | AngelOne | YES |
| NIFTY 50 (SSE stream) | Yahoo Finance v8 | query2 mirror | YES (same provider) |
| NIFTY candles (Next.js) | Yahoo Finance | AngelOne | YES |
| MCX (Next.js) | AngelOne token | None | NO |
| AngelOne JWT (MCX/NiftyBees) | AngelOne login | None | NO |

---

## 6. Verdict: Realtime Reliability

**MIXED.** 

Critical paths have reasonable fallbacks: BTC options feed uses Delta→Binance→synthetic fallback; NIFTY feed uses NSE→AngelOne→synthetic. The execution watchdog triggers a kill switch on BTC stale data, which is conservative and correct.

**Weaknesses:**
1. Coinbase WS has no automatic fallback to Binance — kill switch fires instead of seamlessly switching feeds.
2. DeltaSpotBuy WebSocket has no reconnect logic (`client/src/components/DeltaSpotBuy.tsx:157`).
3. NIFTY SSE has no staleness indicator — errors are sent as data but clients may not detect them.
4. `useNiftyOptionChain` polls every 30s — options chain data can be 30s stale during entry decisions.
5. `useDeskPerformanceMonitor` has 60s polling — performance diagnostics are 60s stale.
6. No single authoritative "market data age" indicator in the UI.
