# API Forensic Report — Phases 11
# Generated: 2026-06-11 | Auditor: Claude Code Forensic Audit

---

## 1. Engine HTTP Route Registry (Go — engine/cmd/antigravity/main.go)

All routes registered in main.go via `http.HandleFunc` or `http.Handle`:

| Engine Path | Method | Handler | Auth |
|---|---|---|---|
| `/health` | GET | inline — returns ok/strategies/uptime | None (CORS *) |
| `/api/health` | GET | inline — returns ok/timestamp/uptime | None (CORS *) |
| `/api/strategies` | GET | tracker.GetAllStats() | None (CORS *) |
| `/api/positions` | GET | posMgr.GetOpenPositions() | None (CORS *) |
| `/api/trades` | GET | dbStore.GetTrades(5000) fallback journal | None (CORS *) |
| `/api/stats` | GET | journal + paperExecute + riskEngine aggregate | None (CORS *) |
| `/api/logs` | GET | globalLogs ring buffer | None (CORS *) |
| `/api/ai/insights` | GET | aiOrchestrator.GetInsights() | None (CORS *) |
| `/api/ai/strategies` | GET | ai.GetAIStrategyLibrary() | None (CORS *) |
| `/api/ai/pending` | GET | orchestrator.GetPendingSignals() | None (CORS *) |
| `/api/ai/submit` | POST | orchestrator.ConfirmSignal() | None (CORS *) |
| `/api/ai/bridge-result` | POST | orchestrator.ConfirmSignalFromBridge() | None (CORS *) |
| `/api/ai/bridge-heartbeat` | GET | orchestrator.RecordBridgeHeartbeat() | None (CORS *) |
| `/api/ai/bridge-event` | POST | orchestrator.RecordBridgeEvent() | None (CORS *) |
| `/api/ai/test-signal` | POST | orchestrator.AddTestSignal() | None (CORS *) |
| `/api/ai/bridge-status` | GET | orchestrator.GetBridgeStatus() | None (CORS *) |
| `/api/execution/intelligence` | GET | orchestrator.ExecIntelSnapshot() | None (CORS *) |
| `/api/execution/request` | POST | executiongateway.NewHandler(orchestrator) | Via security gate |
| `/api/phase22e/certification` | GET | phase22e.NewValidator().Run() | None (CORS *) |
| `/api/options/positions` | GET | optionsEngine.HandlePositions | None |
| `/api/options/trades` | GET | optionsEngine.HandleTrades | None |
| `/api/options/strategies` | GET | optionsEngine.HandleStrategies | None |
| `/api/options/stats` | GET | optionsEngine.HandleStats | None |
| `/api/options/reset` | POST | optionsEngine.HandleReset | None |
| `/api/options/clear-history` | POST | optionsEngine.HandleClearHistory | None |
| `/api/options/btc-feed` | GET | inline — optionsEngineBTCSpot struct | None (CORS *) |
| `/api/options-selling/positions` | GET | optionsSellingEngine.HandlePositions | None |
| `/api/options-selling/trades` | GET | optionsSellingEngine.HandleTrades | None |
| `/api/options-selling/strategies` | GET | optionsSellingEngine.HandleStrategies | None |
| `/api/options-selling/stats` | GET | optionsSellingEngine.HandleStats | None |
| `/api/options-selling/reset` | POST | optionsSellingEngine.HandleReset | None |
| `/api/options-selling/clear-history` | POST | optionsSellingEngine.HandleClearHistory | None |
| `/paper/` | ANY | execution.PaperOMSHandler{OMS: paperOMS} | None |
| `/api/delta-live/stats` | GET | deltaBridge.Stats() | None (CORS *) |
| `/api/delta-live/trades` | GET | deltaBridge.Trades() | None (CORS *) |
| `/api/delta-live/open` | GET | deltaBridge.OpenTrades() | None (CORS *) |
| `/api/delta-live/enable` | POST | deltaBridge.SetEnabled() | None (CORS *) |
| `/api/delta-live/mode` | POST | deltaBridge.SetBuyingMode() | None (CORS *) |
| `/api/delta-live/order` | POST | orchestrator.ProcessExecutionRequest() | None |
| `/api/nifty-options/positions` | GET | niftyOptionsEngine.HandlePositions | None |
| `/api/nifty-options/trades` | GET | niftyOptionsEngine.HandleTrades | None |
| `/api/nifty-options/strategies` | GET | niftyOptionsEngine.HandleStrategies | None |
| `/api/nifty-options/stats` | GET | niftyOptionsEngine.HandleStats | None |
| `/api/nifty-options/reset` | POST | niftyOptionsEngine.HandleReset | None |
| `/api/nifty-options/clear-history` | POST | niftyOptionsEngine.HandleClearHistory | None |
| `/api/nifty-options-selling/positions` | GET | niftyOptionsSellingEngine.HandlePositions | None |
| `/api/nifty-options-selling/trades` | GET | niftyOptionsSellingEngine.HandleTrades | None |
| `/api/nifty-options-selling/strategies` | GET | niftyOptionsSellingEngine.HandleStrategies | None |
| `/api/nifty-options-selling/stats` | GET | niftyOptionsSellingEngine.HandleStats | None |
| `/api/nifty-options-selling/reset` | POST | niftyOptionsSellingEngine.HandleReset | None |
| `/api/nifty-options-selling/clear-history` | POST | niftyOptionsSellingEngine.HandleClearHistory | None |
| `/api/nifty-option-chain` | GET | niftyOptionsEngine.HandleOptionChain | None |
| `/api/nifty-options/inject-candles` | POST | handleNiftyInjectCandles | None |
| `/api/nifty-stocks/positions` | GET | niftyStocksEngine.HandlePositions | None |
| `/api/nifty-stocks/trades` | GET | niftyStocksEngine.HandleTrades | None |
| `/api/nifty-stocks/strategies` | GET | niftyStocksEngine.HandleStrategies | None |
| `/api/nifty-stocks/stats` | GET | niftyStocksEngine.HandleStats | None |
| `/api/nifty-stocks/reset` | POST | niftyStocksEngine.HandleReset | None |
| `/api/nifty-stocks/clear-history` | POST | niftyStocksEngine.HandleClearHistory | None |
| `/api/nifty-market` | GET | niftyMarketCache.HandleQuote | None |
| `/api/regime` | GET | inline optionsSellingEngine.RegimeInfo() | None (CORS *) |
| `/api/probe/delta-btc` | GET | handleDeltaBTCProbe | None (CORS *) |
| `/api/probe/angelone-nifty` | GET | handleAngelOneNiftyProbe | None (CORS *) |
| `/api/angel-proxy` | ANY | handleAngelOneProxy | None |
| `/api/option-chain` | GET | optionsEngine.HandleOptionChain | None |
| `/api/admin/kill` | POST | killswitch.HandleTrigger | None (security gate) |
| `/api/admin/close-all` | POST | killswitch.HandleCloseAll | None (security gate) |
| `/api/admin/reset` | POST | killswitch.HandleReset | None (security gate) |
| `/api/admin/clear-history` | POST | killswitch.HandleClearHistory | None (security gate) |
| `/api/admin/ks/block` | POST | ksSvc.Trigger() | Security gate (no CORS *) |
| `/api/admin/ks/release` | POST | ksSvc.Release() | Security gate (no CORS *) |
| `/api/admin/ks/status` | GET | ksSvc.IsActive() | None |
| `/api/health/mock-trading` | GET | execWatchdog.Health() | None (CORS *) |
| `/api/security/status` | GET | secGate.Projection().Snapshot() | None |
| `/api/security/audit` | GET | secGate.AuditLog(200) | None |
| `/api/security/incidents` | GET | secGate.Monitor().OpenIncidents() | None |
| `/api/paper-desk/diagnostics` | GET | mongoMgr.Diagnostics() | None (CORS *) |
| `/metrics` | GET | observability.MetricsHandler() | None |
| `/phase30/*` | ANY | mongopersist.NewHandler() | None |

**CRITICAL SECURITY FINDING:** The vast majority of engine endpoints (strategies, positions, trades, stats, AI, options, delta-live, nifty) have `Access-Control-Allow-Origin: *` and NO authentication. Anyone who discovers the engine IP can read all trading data, cancel/enable strategies, and trigger resets. The security gate (`secGate.Wrap`) is wired at the API Gateway layer but the policy only enforces auth on selected admin paths; read paths appear exempt. Evidence: `setCORS(w)` pattern used on all data endpoints sets `Access-Control-Allow-Origin: *`.

---

## 2. Next.js API Routes — Complete Table

| Path | Method | Backend | Status | Auth | Error Handling |
|---|---|---|---|---|---|
| `/api/auth/signin` | POST | MongoDB (JWT create) | USED | None required | Good (Zod validation) |
| `/api/auth/signout` | POST | Cookie clear | USED | None | Good |
| `/api/auth/session` | GET | JWT verify (cookie) | USED | raig_session cookie | Good |
| `/api/auth/login` | POST | MongoDB user lookup | USED | None | Good |
| `/api/engine/[...path]` | ANY | Go engine proxy | USED | JWT required | Good — allowlist, deny-by-default |
| `/api/paper-desk/state` | GET | MongoDB paper_state | USED | JWT required | Good |
| `/api/paper-desk/positions` | GET | MongoDB open_positions | USED | JWT required | Good |
| `/api/paper-desk/trades` | GET | MongoDB paper_trades | USED | JWT required | Good |
| `/api/paper-desk/equity` | GET | MongoDB equity_curve | USED | JWT required | Good |
| `/api/paper-desk/orders` | GET | MongoDB paper_oms_orders | USED | JWT required | Good |
| `/api/paper-desk/portfolio` | GET | MongoDB (multi-collection) | USED | JWT required | Good |
| `/api/paper-desk/snapshot` | GET | MongoDB paper_state | USED | JWT required | Good |
| `/api/paper-desk/strategy-analytics` | GET | MongoDB strategy_scores | USED | JWT required | Good |
| `/api/paper-desk/strategy-health` | GET | MongoDB strategy_health | USED | JWT required | Good |
| `/api/paper-desk/validation` | GET | Validation checks | USED | JWT required | Good |
| `/api/paper-desk/diagnostics` | GET | Go engine proxy | USED | JWT required | Good |
| `/api/paper-trades` | GET/POST | MongoDB paper_trades | USED | JWT or anon | Good |
| `/api/paper-trades/analytics` | GET | MongoDB analytics | USED | JWT required | Good |
| `/api/paper-trades/export` | GET | MongoDB paper_trades | USED | JWT required | Good |
| `/api/paper-trades/leaderboard` | GET | MongoDB paper_trades | USED | JWT required | Good |
| `/api/paper-trades/strategy-stats` | GET | MongoDB paper_trades | USED | JWT required | Good |
| `/api/paper-trades/strategy-research` | GET | MongoDB paper_trades | USED | JWT required | Good |
| `/api/paper-trades/clear` | POST | MongoDB (delete) | USED | JWT required | Good |
| `/api/paper-state` | GET/POST | MongoDB paper_state | USED | JWT/anon | Moderate |
| `/api/paper-state/repair` | POST | MongoDB paper_state | USED | JWT required | Good |
| `/api/paper-oms/orders` | GET | MongoDB paper_oms_orders | USED | JWT required | Good |
| `/api/paper-oms/summary` | GET | MongoDB paper_oms_orders | USED | JWT required | Good |
| `/api/paper-replay` | GET/POST | MongoDB paper_trades | USED | JWT required | Good |
| `/api/paper-replay-compare` | GET | MongoDB paper_trades | USED | JWT required | Good |
| `/api/paper-research` | GET | MongoDB paper_trades | USED | JWT required | Good |
| `/api/paper-diagnostics` | GET | MongoDB health | USED | JWT required | Good |
| `/api/paper-desk-smoke-test` | GET | Self-check | USED | None | Good |
| `/api/cron/paper-desk-tick` | GET | MongoDB + strategy tick | USED | Bearer CRON_SECRET | Good |
| `/api/cron/rank-strategies` | GET | MongoDB paper_trades → file write | USED | Bearer CRON_SECRET | Good |
| `/api/cron/policy-snapshot` | GET | MongoDB | USED | Bearer CRON_SECRET | Good |
| `/api/btc/price` | GET | Binance REST/Coinbase REST | USED | None | Good — fallback chain |
| `/api/btc/spot-state` | GET | Go engine proxy | USED | None | Moderate |
| `/api/btc/spot-klines` | GET | Binance REST | USED | None | Good |
| `/api/btc/futures-klines` | GET | Binance REST | USED | None | Good |
| `/api/btc/option-chain` | GET | Go engine proxy | USED | None | Moderate |
| `/api/nifty/candles` | GET | Yahoo Finance / AngelOne | USED | None | Good — fallback |
| `/api/nifty/stream` | GET (SSE) | Yahoo Finance v8 SSE poll | USED | None | Moderate — error as SSE event |
| `/api/nifty/state` | GET | MongoDB / Go engine | USED | JWT required | Good |
| `/api/nifty/selling-state` | GET | Go engine proxy | USED | None | Moderate |
| `/api/nifty/stocks-state` | GET | Go engine proxy | USED | None | Moderate |
| `/api/nifty/option-chain` | GET | AngelOne / NSE | USED | None | Good |
| `/api/nifty/vix` | GET | NSE/Yahoo API | USED | None | Good |
| `/api/nifty/seed-engine` | POST | Go engine (BLOCKED via proxy) | BLOCKED | — | Blocked at proxy |
| `/api/nifty-bees/candles` | GET | AngelOne / Yahoo | USED | None | Good |
| `/api/nifty-bees/ltp` | GET | AngelOne | USED | None | Good |
| `/api/options/paper-snapshot` | GET | Go engine proxy | USED | None | Moderate |
| `/api/mock-trading/trades` | GET/POST | MongoDB mock_trades | USED | OWNER_ACCOUNT_KEY | Good |
| `/api/mock-trading/trades/[id]` | GET | MongoDB mock_trades | USED | OWNER_ACCOUNT_KEY | Good |
| `/api/mock-trading/trades/[id]/close` | POST | MongoDB mock_trades | USED | OWNER_ACCOUNT_KEY | Good |
| `/api/mock-trading/account` | POST | DEPRECATED (410) | DEPRECATED | — | Returns 410 |
| `/api/mock-trading/account/latest` | GET | MongoDB mock_account | USED | OWNER_ACCOUNT_KEY | Good |
| `/api/mock-trading/analytics` | GET | MongoDB mock_trades | USED | OWNER_ACCOUNT_KEY | Good |
| `/api/mock-trading/logs` | GET | MongoDB mock_logs | USED | OWNER_ACCOUNT_KEY | Good |
| `/api/mock-trading/reset` | POST | MongoDB (delete collections) | USED | OWNER_ACCOUNT_KEY | Good |
| `/api/mock-trading/scores` | GET | MongoDB strategy_scores | USED | OWNER_ACCOUNT_KEY | Good |
| `/api/mock-trading/equity` | GET | MongoDB equity_curve | USED | OWNER_ACCOUNT_KEY | Good |
| `/api/mock-trading/daily-pnl` | GET | MongoDB daily_pnl_history | USED | OWNER_ACCOUNT_KEY | Good |
| `/api/mock-trading/signals` | GET/POST | MongoDB strategy_signals | USED | OWNER_ACCOUNT_KEY | Good |
| `/api/mock-trading/regime` | GET/POST | MongoDB regime_snapshots | USED | OWNER_ACCOUNT_KEY | Good |
| `/api/delta/account` | GET | Delta Exchange REST | USED | None | Good |
| `/api/delta/myip` | GET | Public IP lookup | USED | None | Good |
| `/api/delta/mirror` | POST | BLOCKED (blockedDirectExecutionRoute) | DEPRECATED | — | Returns 410 |
| `/api/delta/probe` | GET | Delta Exchange REST | USED | None | Good |
| `/api/delta/spot` | GET | Delta Exchange REST | USED | None | Good |
| `/api/delta/testnet/ping` | GET | Delta testnet | USED | None | Good |
| `/api/delta/testnet/positions` | GET | Delta testnet | USED | None | Good |
| `/api/delta/testnet/place-order` | POST | Delta testnet | USED | None | Moderate |
| `/api/delta/testnet/cancel-order` | POST | Delta testnet | USED | None | Moderate |
| `/api/angelone/order` | POST | BLOCKED (blockedDirectExecutionRoute) | DEPRECATED | — | Returns 410 |
| `/api/angelone/cancel-order` | POST | AngelOne | USED | None | Moderate |
| `/api/angelone/funds` | GET | AngelOne | USED | None | Good |
| `/api/angelone/orders` | GET | AngelOne | USED | None | Good |
| `/api/probe/delta-btc` | GET | Go engine proxy | USED | None | Good |
| `/api/probe/angelone-nifty` | GET | Go engine proxy | USED | None | Good |
| `/api/crypto/markets` | GET | Binance / CoinGecko | USED | None | Good |
| `/api/crypto/equity-state` | GET | Go engine proxy | USED | None | Moderate |
| `/api/stocks/ltp` | GET | AngelOne | USED | None | Good |
| `/api/mcx/state` | GET | AngelOne/MCX | USED | None | Moderate |
| `/api/mcx/ltp` | GET | AngelOne token cache (4h TTL) | USED | None | Good |
| `/api/mcx/candles` | GET | AngelOne | USED | None | Good |
| `/api/mcx/debug` | GET | AngelOne debug | USED | None | Good |
| `/api/health/paper-desk` | GET | MongoDB ping | USED | None | Good |
| `/api/health/storage` | GET | localStorage/MongoDB checks | USED | None | Good |
| `/api/health/desk-worker` | GET | MongoDB worker heartbeat | USED | None | Good |
| `/api/system/health` | GET | MongoDB + Go engine ping | USED | JWT required | Good |
| `/api/system/production-validation` | GET | Validation checks | USED | JWT required | Good |
| `/api/storage/health` | GET | localStorage check | STUB | None | Minimal |
| `/api/storage/backup` | GET | localStorage → MongoDB | USED | None | Good |
| `/api/storage/restore` | POST | MongoDB → localStorage | USED | None | Good |
| `/api/strategy-rankings` | GET | MongoDB rankings file | USED | None | Good |
| `/api/strategy-attribution/[id]` | GET | MongoDB paper_trades | USED | JWT required | Good |
| `/api/strategy-signal-trace` | GET | MongoDB signal_trace | USED | None | Good |
| `/api/trade-history/daily-run` | GET | MongoDB | USED | JWT required | Good |
| `/api/trade-history/dates` | GET | MongoDB | USED | JWT required | Good |
| `/api/trade-history/download` | GET | MongoDB (CSV export) | USED | JWT required | Good |
| `/api/shadow-trade-intents` | GET/POST | MongoDB shadow_intents | USED | None | Good |
| `/api/desk-worker-events` | GET | MongoDB desk_worker_events | USED | None | Good |
| `/api/desk-entry-funnel` | GET | MongoDB paper_state | USED | None | Moderate |
| `/api/execution/request` | POST | Go engine /api/execution/request | USED | JWT required | Good |
| `/api/ai-app-tracker/latest` | GET | MongoDB tracker | USED | None | Good |
| `/api/ai-app-tracker/reports` | GET | MongoDB tracker | USED | None | Good |
| `/api/ai-app-tracker/capture` | POST | MongoDB tracker | USED | None | Good |
| `/api/verification-track/events` | POST | MongoDB verification | USED | None | Good |
| `/api/verification-track/latest` | GET | MongoDB verification | USED | None | Good |
| `/api/verification-track/summary` | GET | MongoDB verification | USED | None | Good |
| `/api/verification-track/ai-context` | GET | MongoDB verification | USED | None | Good |
| `/api/research-edge-report` | GET | MongoDB paper_trades | USED | None | Good |
| `/api/replay-walkforward` | GET/POST | MongoDB paper_trades | USED | JWT required | Good |
| `/api/admin/migrate-owner` | POST | MongoDB (bulk update) | USED | JWT required | Good |

---

## 3. Authentication Coverage Summary

**Protected (JWT required):** paper-desk/*, paper-trades/*, paper-oms/*, trade-history/*, strategy-attribution/*, system/health, system/production-validation, execution/request, admin/migrate-owner, paper-state/repair, paper-replay, paper-replay-compare, paper-research, paper-diagnostics, replay-walkforward

**CRON_SECRET protected:** cron/paper-desk-tick, cron/rank-strategies, cron/policy-snapshot

**Completely unprotected (public):** btc/price, btc/spot-*, btc/futures-*, nifty/* (most), angelone/*, mcx/*, crypto/*, delta/* (most), stocks/ltp, health/*, storage/*, strategy-rankings, shadow-trade-intents, desk-worker-events, desk-entry-funnel, ai-app-tracker/*, verification-track/*, research-edge-report, paper-desk-smoke-test

**CRITICAL BUG — Engine direct exposure:** All engine routes (`/api/strategies`, `/api/positions`, `/api/trades`, etc.) have zero authentication. There is a security gate (`secGate.Wrap`) but it is configured with `EnforceAuth` that defaults to off for read paths. The engine `main.go:2123` wraps all traffic through `apiGateway := gateway.New(secGate.Wrap(http.DefaultServeMux))` but the security policy source is `secPolicy := security.LoadPolicy()` and enforcement depends on `EnforceAuth` flag.

---

## 4. Key Bugs and Issues Found

### BUG-1: ENGINE_EXECUTION_AUTHORITY defaults to TRUE (engine/cmd/main.go pattern)
`isEngineExecutionAuthority()` at `client/src/lib/engineAuthority.ts:6-10` returns `true` unless explicitly set to "0" or "false". If `ENGINE_EXECUTION_AUTHORITY` is unset, it returns `true`. This means the legacy paper-trades POST route returns 410 and the cron tick skips execution even in non-engine environments. This is intentional behavior but could break if the env var is accidentally cleared.

### BUG-2: OWNER_ACCOUNT_KEY hardcoded
`client/src/lib/ownerAuth.ts` uses `OWNER_ACCOUNT_KEY` which appears hardcoded to `"mock_trading_default"`. The `system/health` route at line 77 confirms: `const frontendAccountKey = OWNER_ACCOUNT_KEY; // "mock_trading_default" hardcoded`. This can cause account_key mismatches between frontend and engine.

### BUG-3: /api/angelone/cancel-order, /api/delta/testnet/* have no auth
These routes make real/testnet broker calls without any authentication. Anyone with the Vercel URL can call them.

### BUG-4: /api/shadow-trade-intents has no auth
Allows unauthenticated writes to MongoDB shadow_intents collection.

### BUG-5: Engine proxy allowlist is narrow
`engine/[...path]/route.ts` only proxies READ_PATH_PREFIXES (12 paths) and ADMIN_PATH_PREFIXES (6 paths). Many engine endpoints (AI, options, regime, delta-live) are NOT in the allowlist and return 403 from the Vercel proxy even with auth. Frontend must call the engine directly for these, bypassing auth entirely.

### BUG-6: /api/nifty/stream SSE has no reconnect on server side
The SSE stream at `client/src/app/api/nifty/stream/route.ts` runs a while loop inside `ReadableStream.start()`. When Yahoo Finance is unavailable it sends `{"error":"Yahoo Finance unavailable"}` but keeps the loop running. No server-side circuit breaker. The client must reconnect manually.

---

## 5. Routes in Engine Not Exposed via Next.js Proxy

The engine proxy allowlist is narrow. These engine routes are NOT proxied through Vercel and require direct engine access:
- `/api/ai/*` (insights, strategies, pending, submit, bridge-*)
- `/api/options/*` (all options engine routes)
- `/api/options-selling/*`
- `/api/nifty-options/*`
- `/api/nifty-options-selling/*`
- `/api/nifty-stocks/*`
- `/api/delta-live/*` (except stats which is in READ_PATHS)
- `/api/regime`
- `/api/admin/kill`, `/api/admin/reset`, `/api/admin/close-all`, `/api/admin/clear-history`
- `/api/admin/ks/*` (block/release not in allowlist)
- `/api/phase22e/certification`
- `/api/execution/intelligence`
- `/paper/*` (PaperOMS)
- `/phase30/*`

Evidence: `client/src/app/api/engine/[...path]/route.ts` lines 58-97, READ_PATH_PREFIXES and ADMIN_PATH_PREFIXES arrays.
