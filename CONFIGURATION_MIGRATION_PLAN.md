# Configuration Migration Plan

Generated from repository discovery on 2026-06-02. Secret values are intentionally omitted. Migrate values only through a secret manager or secured operator channel.

## Configuration Sources

| Source | Location | Purpose | Migration Method |
| --- | --- | --- | --- |
| Example environment | `.env.example` | Documents engine, broker, database, AI env names | Copy names only; do not reuse sample values |
| Root Vercel config | `vercel.json` | Next build/install commands and two cron jobs | Clone and update project env in Vercel |
| Client Vercel config | `client/vercel.json` | Empty object | No active config |
| PM2 worker config | `client/ecosystem.worker.config.cjs` | Worker env template and log paths | Create clone-specific PM2 env |
| Docker Compose env | `docker-compose.prod.yml` | Engine env file and mounted data path | Use clone `.env` with isolated DB/broker endpoints |
| K8s ConfigMap/Secret | `infrastructure/kubernetes/trading-engine.yaml` | Engine config and secret keys | Inject from Vault/CI into clone namespace |
| Observability env | `grafana/docker-compose.yml`, `grafana/alertmanager/alertmanager.yml` | Grafana admin, Slack, PagerDuty | Rotate or clone secret refs in target |
| GitHub Actions secrets | `.github/workflows/*.yml` | GHCR, Lightsail deploy, keepalive | Create new repo/environment secrets |

## Environment Variable Inventory

### Databases And Storage

| Name | Purpose | Location | Migration Method |
| --- | --- | --- | --- |
| `DATABASE_URL` | PostgreSQL/Timescale connection | Go ledger/tests, client API routes, Supabase/Postgres routes, K8s secret | Create clone DB URL after restore |
| `REDIS_URL` | Redis cache/coordination | `.env.example`, K8s secret | Point to isolated clone Redis |
| `MONGODB_URI` | MongoDB Atlas connection | Client Mongo, worker, cron, scripts, K8s secret | Restore dump into clone cluster and set clone URI |
| `MONGODB_DB` | Mongo database name | Client Mongo, worker, cron, scripts | Use clone DB name |
| `MONGODB_DB_NAME` | Alternate Mongo DB name | `mongoTradesClient.ts`, `.env.example` | Prefer one canonical clone DB name |
| `SQLITE_PATH` | Engine SQLite path | Go persistence, Docker/K8s config | Point to cloned mounted path |
| `ENGINE_DATA_DIR` | File snapshot fallback directory | Go file snapshots, K8s config | Point to cloned mounted volume |
| `LOCAL_DATA_DIR` | Next server filesystem data root | `localStorageService.ts` | Point to cloned `data` root |

### Broker And Market Data

| Name | Purpose | Location | Migration Method |
| --- | --- | --- | --- |
| `BINANCE_API_KEY` | Binance API credential | `.env.example`, K8s secret | Use clone/test credentials |
| `BINANCE_API_SECRET` | Binance secret | `.env.example`, K8s secret | Use clone/test credentials |
| `DELTA_API_KEY` | Delta credential | Go/client Delta code, K8s secret | Use clone/testnet key |
| `DELTA_API_SECRET` | Delta secret | Go/client Delta code, K8s secret | Use clone/testnet secret |
| `DELTA_API_BASE_URL` | Delta endpoint | Probe/worker/client config | Set clone endpoint explicitly |
| `DELTA_TESTNET` | Delta testnet mode | Go/client Delta code | Force true for dry run |
| `DELTA_PROXY_URL` | Delta proxy endpoint | Go/client Delta code | Point to clone proxy if used |
| `DELTA_OPTIONS_BTC_TICKER` | BTC options ticker | Go engine | Copy or set clone symbol |
| `DELTA_BTC_FUTURES_SYMBOL` | BTC futures symbol for worker | Worker poll tick | Copy if same market; isolate account |
| `COINBASE_WS_URL` | Coinbase websocket override | Go market data | Copy if custom |
| `NSE_BASE_URL` | NSE REST override | Go market data | Copy if custom |
| `ANGELONE_API_KEY` | AngelOne API credential | Go/client Angel code, K8s secret | Use clone/test credential |
| `ANGELONE_CLIENT_CODE` | AngelOne client code | Go/client Angel code | Clone/test account |
| `ANGELONE_PIN` | AngelOne PIN | Go/client Angel code | Secret manager only |
| `ANGELONE_TOTP_SECRET` | AngelOne TOTP seed | Go/client Angel code | Secret manager only |
| `ANGELONE_TOTP_CODE` | AngelOne override TOTP | Go/client Angel code | Avoid persistent config unless necessary |
| `ANGELONE_JWT_TOKEN` | AngelOne JWT override | Go/client Angel code | Avoid copying; regenerate |
| `ANGELONE_NIFTY_EXCHANGE` | NIFTY exchange | Go market data | Copy |
| `ANGELONE_NIFTY_TRADING_SYMBOL` | NIFTY symbol | Go market data | Copy |
| `ANGELONE_NIFTY_SYMBOL_TOKEN` | NIFTY token | Go market data | Copy |
| `ANGELONE_BASE_URL` | AngelOne base URL | Go market data | Copy |
| `ANGELONE_CLIENT_LOCAL_IP` | AngelOne client metadata | Go market data | Set for clone host |
| `ANGELONE_CLIENT_PUBLIC_IP` | AngelOne public IP | Go market data | Set for clone host |
| `ANGELONE_MAC_ADDRESS` | AngelOne device metadata | Go market data | Set for clone host |

### AI Providers

| Name | Purpose | Location | Migration Method |
| --- | --- | --- | --- |
| `OPENAI_API_KEY` | OpenAI models | Go AI clients, `.env.example` | New clone secret or shared read-only budget |
| `GEMINI_API_KEY` | Gemini models | Go AI clients, `.env.example` | New clone secret |
| `GROQ_API_KEY` | Groq fallback | `.env.example` | New clone secret |
| `ANTHROPIC_API_KEY` | Anthropic bot/client | Python bot, docs | New clone secret |
| `MISTRAL_API_KEY` | Mistral client | `engine/internal/ai/mistral.go` | New clone secret |
| `CLOUDFLARE_API_KEY` | Cloudflare Workers AI | `engine/internal/ai/cloudflare.go` | New clone secret |
| `CLOUDFLARE_ACCOUNT_ID` | Cloudflare account | `engine/internal/ai/cloudflare.go` | Clone account reference |
| `OPENROUTER_API_KEY` | OpenRouter client | `engine/internal/ai/openrouter.go` | New clone secret |
| `AI_GRPC_HOST` | Python AI gRPC host | `.env.example` | Point to clone AI service |

### Auth And Security

| Name | Purpose | Location | Migration Method |
| --- | --- | --- | --- |
| `AUTH_JWT_SECRET` | App/engine JWT signing | Client auth, engine policies, K8s secret | Generate clone-specific secret; do not reuse unless sessions must migrate |
| `ENGINE_ADMIN_SECRET` | Engine admin auth | `engine/internal/security/policies.go` | Generate clone-specific secret |
| `SECURITY_ENFORCE_AUTH` | Engine auth enforcement | Engine policies | Copy desired setting |
| `SECURITY_ENFORCE_ADMIN_IP` | Admin IP enforcement | Engine policies | Set for clone network |
| `ALLOWED_ORIGINS` | CORS allowlist | Engine policies | Replace with clone origins |
| `ENGINE_ADMIN_CIDR` | Admin CIDR allowlist | Engine policies | Replace with clone network |
| `SERVICE_SECRETS` | Service-to-service secrets | Engine policies | Generate clone-specific values |
| `INTERNAL_API_SECRET` | Next-to-engine shared secret | Engine policies | Generate clone-specific value |
| `CRON_SECRET` | Cron route auth | Vercel cron routes, K8s secret | Generate clone-specific secret |
| `ALLOW_PAPER_TRADES_ANON` | Paper trade anonymous mode | Client API | Copy if intended |
| `ALLOW_ANON_PAPER_TRADES` | Paper trade anonymous mode | Client API | Copy if intended |
| `NEXT_PUBLIC_ALLOW_ANON_PAPER_TRADES` | Browser anonymous mode | Client | Copy if intended |

### Deployment And Runtime

| Name | Purpose | Location | Migration Method |
| --- | --- | --- | --- |
| `INTERNAL_API_URL` | Next to Go engine URL | Engine proxy, seed route, Docker comments | Point to clone engine |
| `ENGINE_URL` | Engine fallback URL | Engine proxy, bridge | Point to clone engine |
| `NEXT_PUBLIC_API_URL` | Browser API URL | `engineApi.ts` | Point to clone public API |
| `NEXT_PUBLIC_VERCEL_URL` | Worker app URL | Paper worker | Point to clone Vercel URL |
| `VERCEL_URL` | Vercel runtime URL | Paper worker | Clone project-provided |
| `NEXT_PUBLIC_VERCEL_GIT_COMMIT_SHA` | Build SHA | Client health/build | Clone deploy metadata |
| `PORT` | Engine port | Go engine, Docker | Set per runtime |
| `APP_VERSION` | Logging version | Go logging | Set clone release |
| `LOG_LEVEL` | Logging verbosity | Go observability/K8s | Copy desired setting |
| `METRICS_PATH` | Metrics endpoint path | Go exporter | Copy desired setting |
| `SERVICE_NAME` | Metrics service name | Go exporter | Use clone service name |
| `GOMEMLIMIT` | Go memory limit | Docker compose | Set per host |
| `GOGC` | Go GC tuning | Docker compose | Set per host |
| `HA_ENABLED` | HA engine mode | K8s config | Set per deployment |
| `BACKUP_DIR` | Backup path | K8s config | Clone volume |
| `BACKUP_LEDGER_INTERVAL` | Ledger backup interval | K8s config | Copy desired setting |
| `BACKUP_DB_INTERVAL` | DB backup interval | K8s config | Copy desired setting |
| `NODE_ID` | HA node id | K8s downward API | Auto per clone pod |
| `POD_NAMESPACE` | K8s namespace | K8s downward API | Auto per clone namespace |

### Paper Desk Policy And Worker

| Name | Purpose | Location | Migration Method |
| --- | --- | --- | --- |
| `DESK_WORKER_ACCOUNT_KEY` | Worker account identity | Worker, health route, scripts | Clone-specific account key unless reproducing same account offline |
| `DESK_WORKER_STORAGE_NAMESPACE` | Paper state namespace | Worker | Copy or clone-specific |
| `POLL_MS` | Worker poll interval | Worker | Copy/tune |
| `DESK_WORKER_STRATEGY_IDS` | Explicit worker roster | Worker | Copy only if intended |
| `DESK_WORKER_ROSTER_FALLBACK` | Worker roster fallback | Poll tick | Copy |
| `DESK_WORKER_NOTIONAL` | Worker notional | Poll tick | Copy/tune |
| `NEXT_PUBLIC_DESK_INITIAL_BALANCE_USD` | Initial balance | Worker/browser | Copy if reproducing paper state |
| `NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD` | BTC FT threshold | Worker/browser | Copy |
| `NEXT_PUBLIC_BTC_FT_RELAX_CONFIRM` | Relax confirmation | Worker/browser | Copy; usually off in production |
| `NEXT_PUBLIC_DESK_WORKER_ENABLED` | UI worker mode | Client hook | Copy |
| `NEXT_PUBLIC_DESK_WAKE_LOCK` | Browser wake lock | Client hook | Copy |
| `NEXT_PUBLIC_DESK_ENTRY_DEBUG` | Entry debug | Client debug | Copy only for debug |
| `NEXT_PUBLIC_BTC_FT_ENTRY_DEBUG` | Entry debug | Tests/components | Copy only for debug |
| `NEXT_PUBLIC_DESK_REPLAY_GATE` | Replay gate | Client | Copy |
| `NEXT_PUBLIC_DESK_SHADOW_INTENTS` | Shadow intents | Client | Copy |
| `NEXT_PUBLIC_DESK_SHADOW_LOG_OPEN` | Shadow open logging | Client | Copy |
| `NEXT_PUBLIC_DESK_TESTNET_OPS` | Testnet operation schemas | Client Delta | Enable for clone dry run |
| `NEXT_PUBLIC_BTC_FT_DESK_BUILD` | Desk build label | Client | Clone release label |

Additional `NEXT_PUBLIC_DESK_*` policy variables are referenced across futures policy/profit mode code. Export all existing Vercel env names from the original project and recreate them in the clone after review.

### Supabase

| Name | Purpose | Location | Migration Method |
| --- | --- | --- | --- |
| `NEXT_PUBLIC_SUPABASE_URL` | Supabase project URL | Client Supabase server/research scripts | Point to clone Supabase project |
| `NEXT_PUBLIC_SUPABASE_ANON_KEY` | Supabase anon key | Client Supabase server | Clone project key |
| `SUPABASE_SERVICE_ROLE_KEY` | Supabase service role | Server scripts/routes | Clone service role secret |

### Observability

| Name | Purpose | Location | Migration Method |
| --- | --- | --- | --- |
| `GRAFANA_ADMIN_PASSWORD` | Grafana admin | `grafana/docker-compose.yml` | Generate clone-specific password |
| `SLACK_WEBHOOK_URL` | AlertManager Slack | `grafana/alertmanager/alertmanager.yml` | Clone/ops webhook |
| `PAGERDUTY_INTEGRATION_KEY` | AlertManager PagerDuty | `grafana/alertmanager/alertmanager.yml` | Clone/ops integration |

### GitHub Actions

| Secret Name | Purpose | Location | Migration Method |
| --- | --- | --- | --- |
| `GITHUB_TOKEN` | GHCR push/login | `.github/workflows/deploy.yml` | GitHub-provided in clone repo |
| `LIGHTSAIL_HOST` | Engine deploy target | `.github/workflows/deploy.yml` | Clone host |
| `LIGHTSAIL_SSH_KEY` | SSH deploy key | `.github/workflows/deploy.yml` | Clone deploy key |

## Migration Rules

1. Export only env names from original systems for documentation.
2. Move secret values through Vercel/GitHub/Kubernetes/Vault secret import workflows.
3. Generate new secrets for clone identity: `AUTH_JWT_SECRET`, `ENGINE_ADMIN_SECRET`, `CRON_SECRET`, `INTERNAL_API_SECRET`, `SERVICE_SECRETS`.
4. Use paper/testnet broker credentials until cutover validation passes.
5. Replace every URL that points to original infrastructure:
   - `INTERNAL_API_URL`
   - `ENGINE_URL`
   - `NEXT_PUBLIC_API_URL`
   - `NEXT_PUBLIC_VERCEL_URL`
   - GitHub keep-alive target
6. Confirm no `NEXT_PUBLIC_*` variable contains a secret.

## Validation

Run these after configuration import:

```bash
env | sort | sed 's/=.*/=<redacted>/'
```

Application checks:

- `/api/health/storage`
- `/api/health/desk-worker`
- `/api/engine/health` through proxy
- Engine `/health`, `/ready`, `/metrics`

Security checks:

- Unauthorized cron call must fail.
- Authorized cron call in clone must target clone MongoDB only.
- Engine admin endpoint must reject missing/old original secrets.
- Broker order endpoints must be in testnet/paper mode for dry run.
