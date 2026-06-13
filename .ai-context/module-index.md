# Module Index

## Client
- `client/src/app/api/`: API routes for auth, paper trading, BTC options, NIFTY, NIFTY options, NIFTY stocks, Delta live, AngelOne, engine proxy, cron/admin, diagnostics, and AI tracker.
- `client/src/components/`: dashboard panels, trading views, order/position controls, UI shell, and reusable UI primitives.
- `client/src/hooks/`: polling and state hooks for market data, options, orders, engines, and dashboard state.
- `client/src/lib/`: domain logic for BTC futures, paper trades, broker persistence, analytics, strategy authority, trading calculations, engine API access, and shared types.
- `client/src/internal/`: internal execution/OMS/event abstractions used by the client-side terminal stack.

## Engine
- `engine/cmd/antigravity/`: main service boot path.
- `engine/cmd/backtest/`: offline backtesting.
- `engine/cmd/perfbench/`: performance benchmark entry point.
- `engine/internal/marketdata/`: exchange and market data providers.
- `engine/internal/strategy/`: curated strategy registry and strategy implementations.
- `engine/internal/risk/` and `engine/internal/risk/gate/`: risk policy, gates, loss limits, and position controls.
- `engine/internal/omsv3/`: event-driven OMS and aggregate invariants.
- `engine/internal/execution/`: execution flow and adapter-facing logic.
- `engine/internal/ledger/`: financial event and balance tracking.
- `engine/internal/reconciliation/`: broker/internal state reconciliation.
- `engine/internal/killswitch/`: production safety stop controls.
- `engine/internal/persistence/`: SQLite/filesystem persistence fallback.
- `engine/internal/observability/`: metrics, logging, tracing, and health.

## Support
- `bridge/`: broker bridge and integration support.
- `brain/`: Python AI/research support.
- `infrastructure/`: database and deployment infrastructure.
- `.ai-context/`: compressed AI context, generated maps, and documentation.
- `graphify-out/`: generated Graphify graph; query it instead of reading it directly.
