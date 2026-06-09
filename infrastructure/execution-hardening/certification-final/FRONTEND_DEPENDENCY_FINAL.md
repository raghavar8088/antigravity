# FRONTEND_DEPENDENCY_FINAL

## Order placement UI

All live order UI components call `submitExecutionRequest()` only:

| Component | File | Line | Target |
|-----------|------|------|--------|
| TestnetOpsPanel | TestnetOpsPanel.tsx | 90 | `/api/execution/request` |
| DeltaLiveScalper | DeltaLiveScalper.tsx | 706 | `/api/execution/request` |
| DeltaSpotBuy | DeltaSpotBuy.tsx | 238 | `/api/execution/request` |
| useAngelOneOrders | useAngelOneOrders.ts | 137 | `/api/execution/request` |

Proxy: `client/src/lib/executionRequest.ts:28` → `POST /api/execution/request` → Go gateway.

## Cancel / modify

| Surface | Behaviour | Evidence |
|---------|-----------|----------|
| TestnetOpsPanel cancel | Stub message only | TestnetOpsPanel.tsx — no fetch |
| useAngelOneOrders cancel | Returns `{ ok: false, error: "..." }` | useAngelOneOrders.ts:146-150 |

## Browser paper execution — REMOVED

```2675:2678:client/src/hooks/useBTCFuturesScalperEngine.ts
const poll = async () => {
  // Institutional hardening: browser-side paper execution removed permanently.
  return;
```

```1363:1365:client/src/hooks/useBTCFuturesScalperEngine.ts
const saveToMongo = useCallback(...) => {
  return; // browser never persists execution state
```

Poll body containing `openPositionRef.current` is **unreachable dead code** after early return.

## WebSocket

| Hook | Sends | Receives |
|------|-------|----------|
| useLiveBTCMarket.ts | `{ op: "ping" }` only | price/candles |
| terminalStore.tsx | — | snapshots |
| DeltaSpotBuy.tsx | — | Binance trade price |

No execution verbs on inbound messages.

## Enable execution influence

`useDeltaLive.ts:196` → `POST /api/delta-live/enable` — sets bridge flag only; requires `PermStrategyEnable` (rbac.go:126). Does not call broker APIs.

## Verdict

**PASS** — Frontend cannot directly or indirectly place broker orders except via authenticated `/api/execution/request` proxy to institutional gateway.
