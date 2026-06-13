# Module Index

## `client/`

| Module | Responsibility | Notes |
|---|---|---|
| `src/app/` | Next.js pages and API routes | Route handlers proxy engine state, persist paper data, and expose dashboard data. |
| `src/components/` | React dashboards and UI primitives | Trading terminal, strategy dashboards, risk panels, BTC market widgets. |
| `src/hooks/` | Browser-side state and polling hooks | Includes BTC market/desk hooks; preserve behavior. |
| `src/lib/trading/` | BTC trading domain logic | Protected: strategies, signals, policy, PnL math, runtime, market data helpers. |
| `src/lib/analytics/` | Replay, reporting, scorecards | Protected where tied to BTC futures replay and validation. |
| `src/lib/portfolio/` | Paper trades and trade history | Mongo/Postgres persistence and analytics. |
| `src/lib/broker/` | Auth/session/broker clients | Server-side broker/session helpers. |
| `src/lib/strategyAuthority/` | Strategy lifecycle and ranking | Promotion, demotion, portfolio construction, catalogs. |
| `src/server/delta/` | Delta REST/testnet helpers | Protected BTC/Delta integration. |
| `scripts/` | Operational/replay/ranking scripts | Review before deletion; many scripts support BTC analysis. |

## Protected BTC Futures Files

Fast-load path for AI assistants:

- `client/src/app/mock-trading/page.tsx`
- `client/src/components/MockTradingDashboard.tsx`
- `client/src/hooks/useMockTradingEngine.ts`
- `client/src/hooks/useMockResearchRunner.ts`
- `client/src/hooks/useLiveBTCPrice.ts`
- `client/src/hooks/useLiveBTCMarket.ts`
- `client/src/hooks/useMarketRegime.ts`
- `client/src/hooks/useStrategyScoring.ts`
- `client/src/lib/trading/btcFuturesTrade.types.ts`
- `client/src/lib/trading/mockTradingEngine.ts`
- `client/src/lib/trading/mockTradingMongo.ts`
- `client/src/lib/trading/futuresDeskPolicy.ts`
- `client/src/lib/trading/futuresPaperMath.ts`
- `client/src/lib/trading/futuresSignals.ts`
- `client/src/lib/trading/futuresStrategies.ts`
- `client/src/lib/trading/btcFtResearch.ts`
- `client/src/lib/trading/btcFtRoster.ts`
- `client/src/lib/trading/futuresDeskRuntime.ts`
- `client/src/lib/analytics/futuresReplayEngine.ts`
- `client/src/lib/analytics/futuresReplayCompare.ts`
- `client/src/lib/analytics/futuresReplayUi.ts`
- `client/src/lib/ai/strategySignalTrace.ts`
- `client/src/lib/risk/noTradeRootCause.ts`
- `client/src/app/api/btc/*`
- `client/src/app/api/mock-trading/*`
- `client/src/app/api/cron/policy-snapshot/route.ts`
- related tests under `client/src/lib/**/*.test.ts`

Compatibility routes `/btc-future-trading`, `/paper-desk`, and `/paperdesk` redirect to `/mock-trading`; do not remove without checking external links and middleware behavior.

## `engine/`

| Module | Responsibility | Notes |
|---|---|---|
| `cmd/antigravity` | Production engine process | Main boot wiring, HTTP routes, feeds, risk/execution orchestration. |
| `internal/trading` | Orchestrator and trade loop | Protected execution path. |
| `internal/execution*` | Paper/live execution adapters and gateway | Protected; all orders flow through gates. |
| `internal/risk*` | Risk engines, gates, budgets | Protected business logic. |
| `internal/omsv3` | OMS event/aggregate model | Protected order authority. |
| `internal/ledger` | Event ledger and snapshots | Protected accounting/audit path. |
| `internal/reconciliation*` | Drift detection and kill-switch hooks | Protected safety path. |
| `internal/killswitch` | Kill switch service | Must remain wired. |
| `internal/marketdata` | BTC feeds and warmup bars | Protected market data path. |
| `internal/options*` | BTC options buy/sell paper engines | Protected BTC options logic. |
| `internal/persistence`, `paperpersist`, `mongopersist` | SQLite/file/Mongo persistence | Protected state durability. |
| `internal/strategy`, `alpha`, `backtest` | Strategy registry, alpha modules, backtesting | Protected unless proven unused. |
| `vendor/` | Vendored Go dependencies | Do not edit manually. |

## `graphify-out/`

Graphify directories are protected. They are used for repository graph context and must not be removed as “cache” unless the user explicitly asks.
