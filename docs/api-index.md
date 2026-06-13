# API Index

## Client API Route Groups

| Prefix | Responsibility |
|---|---|
| `/api/btc/*` | BTC spot/futures market data and state. Protected. |
| `/api/mock-trading/*` | BTC paper/mock account, trades, signals, analytics, snapshots. Protected. |
| `/api/options/*` | BTC options paper snapshot/positions/trades where present. Protected. |
| `/api/execution/request` | Human/service execution intent into engine. Protected. |
| `/api/engine/*` | Authenticated proxy to Go engine allowlisted paths. |
| `/api/risk/*` | Risk summaries, positions, gates. Protected. |
| `/api/strategy-authority/*` | Strategy lifecycle, rankings, allocation, promotion/demotion. |
| `/api/strategies/*` | Strategy registry and detail access. |
| `/api/delta/*` | Delta account/testnet/probe/mirror helpers. Protected when BTC-related. |
| `/api/cron/*` | Scheduled strategy ranking/policy snapshots. |
| `/api/trade-history/*` | Trade archive and download APIs. |
| `/api/killswitch/*` | Kill switch trigger/status/resume. Protected. |
| `/api/auth/*`, `/auth/callback` | Owner/session authentication. |
| `/api/storage/*`, `/api/health/*`, `/api/system/*` | Storage and system health. |
| `/api/ai-app-tracker/*`, `/api/verification-track/*` | AI/debug observability. |

## Engine HTTP Surface

Defined primarily in `engine/cmd/antigravity/main.go`.

| Path | Responsibility |
|---|---|
| `/health`, `/ready`, `/metrics` | Health and Prometheus metrics. |
| `/api/health`, `/api/health/mock-trading` | Engine health views. |
| `/api/options/*`, `/api/options-selling/*`, `/api/option-chain` | BTC options buy/sell paper engines. |
| `/api/delta-live/*` | Delta BTC options bridge status/trades/manual controls. |
| `/api/execution/request` | Institutional execution gateway. |
| `/paper/*` | Paper OMS endpoints. |
| `/api/admin/*`, `/api/admin/ks/*` | Admin and kill switch controls. |
| `/api/strategies`, `/api/positions`, `/api/trades`, `/api/stats` | Trading state and telemetry. |
| `/api/logs`, `/api/security/*`, `/api/regime` | Ops/security/regime views. |

## Security Notes

- Mutating/admin routes require server-side auth/secret gates.
- Browser should submit execution intents only; no direct broker orders from UI.
- Engine-side kill switch and risk gates must remain in all production paths.
