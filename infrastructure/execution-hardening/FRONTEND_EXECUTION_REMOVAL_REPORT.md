# Frontend Execution Removal Report

## Changes implemented

### Retired direct broker API routes (410 Gone)

| Route | File |
|-------|------|
| `/api/angelone/order` | `client/src/app/api/angelone/order/route.ts` |
| `/api/angelone/cancel-order` | `client/src/app/api/angelone/cancel-order/route.ts` |
| `/api/delta/mirror` | `client/src/app/api/delta/mirror/route.ts` |
| `/api/delta/spot` POST | `client/src/app/api/delta/spot/route.ts` |
| `/api/delta/testnet/place-order` | `client/src/app/api/delta/testnet/place-order/route.ts` |

### New execution request client

- `client/src/lib/executionRequest.ts` — `submitExecutionRequest()` sole UI entry
- `client/src/app/api/execution/request/route.ts` — proxies to Go `/api/execution/request`

### Components refactored

| Component | Before | After |
|-----------|--------|-------|
| `TestnetOpsPanel.tsx` | `fetch(/api/delta/testnet/place-order)` | `submitExecutionRequest({ venue: "delta" })` |
| `DeltaLiveScalper.tsx` | `fetch(/api/delta/mirror)` | `submitExecutionRequest` |
| `DeltaSpotBuy.tsx` | `fetch(/api/delta/spot)` | `submitExecutionRequest` |
| `useAngelOneOrders.ts` | `fetch(/api/angelone/order)` | `submitExecutionRequest({ venue: "angelone" })` |

### Read-only preserved

- `/api/angelone/orders`, `/api/angelone/funds` — order book / RMS reads only
- Browser paper desk (`useBTCFuturesScalperEngine`) — paper math only; blocked when `ENGINE_EXECUTION_AUTHORITY=1`

### Residual client modules (not mounted in UI routes)

- `client/src/internal/exchange/` — legacy v2 stubs; no production page imports
- `client/src/server/delta/deltaClient.ts` — server-only; no live route calls `placeOrder` after hardening
