# API_SECURITY_FINAL

## Execution routes

| Route | Method | Broker call | Institutional controls |
|-------|--------|-------------|------------------------|
| `/api/execution/request` | POST | Via engine ProcessExecutionRequest | ✓ RBAC + ETP |
| Engine `/api/execution/request` | POST | Same | ✓ PermTradeRequest (handler.go:59) |
| `/api/delta-live/order` | POST | ProcessExecutionRequest | ✓ ETP |
| `/api/ai/submit` | POST | ConfirmSignal → ETP | ✓ (engine-side) |
| `/api/delta-live/enable` | POST | None (SetEnabled flag) | ✓ PermStrategyEnable |

## Retired routes (410)

All use `blockedDirectExecutionRoute` (`blockedExecutionRoute.ts:4`):

- `/api/angelone/order`
- `/api/angelone/cancel-order`
- `/api/delta/mirror`
- `/api/delta/spot`
- `/api/delta/testnet/place-order`
- `/api/delta/testnet/cancel-order` ← **newly retired**

## Read-only broker queries (no execution)

- `GET /api/angelone/orders` — order book read
- `GET /api/delta/account` — GET open orders
- `GET /api/delta/testnet/positions` — read positions

## Angel One proxy (engine)

`handleAngelOneProxy` (main.go:2329-2340) allowlist excludes place/cancel/modify paths.

## Cron / worker

`GET /api/cron/paper-desk-tick` skips when `ENGINE_EXECUTION_AUTHORITY=1` (paper-desk-tick/route.ts:58).

Browser poll permanently disabled (useBTCFuturesScalperEngine.ts:2676).

## Verdict

**PASS** — No Next.js route performs direct broker order placement or cancellation. All execution intents route through engine institutional gateway or ETP.
