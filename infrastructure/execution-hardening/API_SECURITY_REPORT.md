# API Security Report

## Route classification (post-hardening)

### EXECUTE (institutional gateway only)

| Route | Auth | Risk | Status |
|-------|------|------|--------|
| `POST /api/execution/request` (Next) | JWT session | proxied to engine ETP | ACTIVE |
| `POST /api/execution/request` (Go) | JWT / service | full ETP | ACTIVE |
| `POST /api/delta-live/order` (Go) | JWT + RBAC | full ETP | ACTIVE |

### RETIRED (410)

- `/api/angelone/order`, `/api/angelone/cancel-order`
- `/api/delta/mirror`, `/api/delta/spot` POST
- `/api/delta/testnet/place-order`

### READ

- `/api/angelone/orders`, `/api/angelone/funds`
- `/api/paper-desk/*`, `/api/delta-live/stats` (via engine proxy allowlist)

### ADMIN

- `/api/admin/kill`, `/api/admin/ks/*` — requires `ENGINE_ADMIN_SECRET` on engine

## Engine proxy allowlist

`client/src/app/api/engine/[...path]/route.ts` — denies non-allowlisted paths including direct execution bypass.

## RBAC changes

- New permission: `trade.request` (`PermTradeRequest`)
- Human `RoleTrader`: request only, not `trade.execute`
- `RoleService`: retains `trade.execute` for engine-internal broker fills
