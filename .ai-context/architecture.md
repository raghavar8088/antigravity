# Architecture

## Runtime Layers
- UI and API: `client/` runs on Vercel as a Next.js app.
- Engine: `engine/` runs as a Go service on AWS Lightsail.
- AI/research: `brain/` and strategy intelligence modules support ranking, scoring, and analysis.
- Broker bridge: `bridge/` and broker-specific client libraries connect external execution and market data sources.
- Persistence: MongoDB Atlas, PostgreSQL Neon, Redis, and SQLite fallback.

## Data Flow
Market data is collected from Coinbase, Binance, Delta Exchange, AngelOne, Yahoo/NSE fallbacks, and internal state sources. Strategies consume normalized data, pass signals through policy/risk gates, submit orders through OMS v3, record fills in ledger state, reconcile broker/internal state, enforce kill switch safety, persist state, and expose results through API routes and dashboards.

```text
Market Data
-> Strategy Registry / Signal Generation
-> Policy Gates / Risk Gate
-> OMS v3
-> Execution Adapter
-> Fill / Position Update
-> Ledger
-> Reconciliation
-> Kill Switch
-> Persistence
-> Next.js API
-> Dashboard UI
```

## Safety Invariants
- Do not bypass kill switch checks in production execution paths.
- Preserve fee, funding, liquidation, PnL, and position sizing math.
- Gate NSE/BSE logic by market hours; crypto paths can run 24/7.
- Keep WINNERS_ONLY filtering active for strategy selection.
- Prefer real integration test databases for engine DB tests.

## Query Hints
- For UI/API behavior, query the `client` scoped graph.
- For Go trading execution behavior, query the `engine-internal` scoped graph.
- For process startup or command wiring, query the `engine-cmd` scoped graph.
