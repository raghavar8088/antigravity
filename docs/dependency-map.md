# Dependency Map

Graphify CLI was not available in this shell (`graphify` command not found). Existing Graphify folders are protected and preserved.

## Client Dependency Shape

```text
Next pages/components
  -> hooks
  -> client/src/lib/trading
  -> client/src/lib/analytics
  -> client/src/lib/portfolio
  -> API routes
  -> MongoDB / PostgreSQL / Go engine / Delta APIs
```

Key packages:

- `next`, `react`, `react-dom`: application runtime.
- `mongodb`: Mongo-backed paper/mock state, strategy authority, logs.
- `pg`: Postgres-backed snapshots and trade history.
- `zod`: request/schema validation.
- `lightweight-charts`: chart time/types and market charts.
- `@radix-ui/*`, `cmdk`: UI primitives/command palette.
- `tsx`, `vitest`: scripts and tests.

Manual dependency review candidates:

- `@supabase/ssr`, `@supabase/supabase-js`: only type/runtime references found in limited scan; verify before removal.
- `exceljs`, `https-proxy-agent`, `undici`: verify full usage before removal.

## Engine Dependency Shape

```text
cmd/antigravity/main.go
  -> trading.Orchestrator
  -> strategy registry + market data feeds
  -> risk + OMS + execution gateway
  -> ledger + positions + reconciliation
  -> persistence + observability + security
```

Key Go packages:

- `github.com/gorilla/websocket`: market data streams.
- `github.com/jackc/pgx/v5`: Postgres connectivity.
- `go.mongodb.org/mongo-driver/v2`: Mongo persistence.
- `modernc.org/sqlite`: local state store.
- `github.com/prometheus/client_golang`: metrics.
- `github.com/aws/aws-sdk-go-v2`: Secrets Manager and AWS integrations.

## Service Interaction Graph

```text
Browser UI
  -> Next.js API routes
    -> MongoDB / PostgreSQL
    -> Go engine (`INTERNAL_API_URL`)
      -> Coinbase/Binance/Delta market data
      -> Delta execution bridge
      -> SQLite/file snapshots
      -> Mongo/Postgres persistence
      -> Prometheus metrics
```

## Database Access Graph

- MongoDB: mock trades, strategy authority state, AI tracker, verification track.
- PostgreSQL: trade history archive, BTC spot/options snapshots, analytics tables.
- SQLite/file: engine fallback persistence and state snapshots.

## Important Environment Groups

- Auth/session: `AUTH_JWT_SECRET`, `OWNER_ACCOUNT_KEY`, legacy `DESK_WORKER_ACCOUNT_KEY`.
- Engine proxy/security: `INTERNAL_API_URL`, `ENGINE_URL`, `ENGINE_ADMIN_SECRET`, `INTERNAL_API_SECRET`, `SERVICE_SECRETS`, `ALLOWED_ORIGINS`.
- Databases: `MONGODB_URI`, `MONGODB_DB`, `MONGODB_DB_NAME`, `DATABASE_URL`, `SQLITE_ENABLED`, `SQLITE_PATH`, `ENGINE_DATA_DIR`.
- Delta/BTC: `DELTA_API_BASE_URL`, `DELTA_API_KEY`, `DELTA_API_SECRET`, `DELTA_TESTNET`, `DELTA_BTC_FUTURES_SYMBOL`, `DELTA_BTC_CANDLE_SYMBOL`, `DELTA_OPTIONS_BTC_TICKER`.
- BTC desk flags: `NEXT_PUBLIC_DESK_*`, `NEXT_PUBLIC_BTC_FT_*`, `NEXT_PUBLIC_MOCK_TRADING_RELAX_CONFIRM`.
- AI/ML services: `OPENAI_API_KEY`, `GROQ_API_KEY`, `GEMINI_API_KEY`, `ANTHROPIC_API_KEY`, `OPENROUTER_API_KEY`, `MISTRAL_API_KEY`, `HUGGINGFACE_API_KEY`, `CLOUDFLARE_API_KEY`, `ML_SCORER_ENDPOINT`, `SENTIMENT_SERVER_URL`.
- Secrets/runtime: `VAULT_ADDR`, `VAULT_TOKEN`, `PORT`.

## Protected Import Paths

Any import chain involving these modules should be considered BTC-critical:

- `client/src/lib/trading/futures*`
- `client/src/lib/trading/btc*`
- `client/src/lib/analytics/futures*`
- `client/src/app/api/btc/*`
- `client/src/app/api/mock-trading/*`
- `engine/internal/trading`
- `engine/internal/execution*`
- `engine/internal/risk*`
- `engine/internal/omsv3`
- `engine/internal/ledger`
- `engine/internal/reconciliation*`
- `engine/internal/killswitch`
- `engine/internal/options*`
- `engine/internal/marketdata`
