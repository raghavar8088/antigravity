# Broker Validation Report

**Audit date:** 2026-06-09  
**Brokers audited:** Delta, AngelOne, Binance, Coinbase, bridge/ (non-broker)

---

## Summary Matrix

| Capability | Delta | AngelOne | Binance | Coinbase | bridge/ |
|------------|-------|----------|---------|----------|---------|
| A. Order submission | PARTIAL | FAIL | PARTIAL | FAIL | FAIL |
| B. Order acknowledgement | PARTIAL | FAIL | PARTIAL | FAIL | FAIL |
| C. Fill handling | PARTIAL | FAIL | FAIL | FAIL | FAIL |
| D. Partial fills | FAIL | FAIL | FAIL | FAIL | FAIL |
| E. Order rejection | PASS | PARTIAL | PARTIAL | FAIL | FAIL |
| F. Cancel order | PARTIAL | FAIL | PARTIAL | FAIL | FAIL |
| G. Modify order | FAIL | FAIL | FAIL | FAIL | FAIL |
| H. Disconnect recovery | PARTIAL | PARTIAL | PARTIAL | PASS (MD) | PARTIAL |
| I. Rate limit handling | PARTIAL | FAIL | PARTIAL | N/A | FAIL |
| J. Duplicate event handling | PARTIAL | FAIL | FAIL | PARTIAL | PARTIAL |

---

## 1. Delta Exchange

### Files
- `engine/internal/delta/client.go` — REST client
- `engine/internal/delta/live_bridge.go` — options mirror bridge
- `engine/internal/trading/institutional_request.go` — gateway routing
- `client/src/server/delta/deltaClient.ts` — testnet client
- `client/src/app/api/execution/request/route.ts` — proxy to engine

### Capability Detail

| Capability | Verdict | Source Evidence |
|------------|---------|-----------------|
| **A. Order submission** | **PARTIAL PASS** | `Client.PlaceOrder` (`client.go:182`); `Bridge.SubmitOrder` (`live_bridge.go:131`); `processDeltaExecutionRequest` (`institutional_request.go:81–140`). Direct Next.js routes return 410 (`blockedExecutionRoute.ts`). |
| **B. Order acknowledgement** | **FAIL** | REST returns order ID (`client.go:220–228`). Institutional path writes synthetic ack `"paper-" + clientOrderID` before broker call (`loop.go:671–673`). Real `ExchangeOrderID` not in `EventOrderAcked`. |
| **C. Fill handling** | **FAIL** | Assumes synchronous full fill from `average_fill_price` on submit response (`client.go:215–217`). No fill WebSocket. No open-order polling loop. |
| **D. Partial fills** | **FAIL** | OMS supports `EventOrderPartial` but live path always emits `EventOrderFilled` with full `sig.TargetSize` (`loop.go:707–710`). |
| **E. Order rejection** | **PASS** | HTTP non-200 / `!resp.Success` → error (`client.go:192–213`). Broker failure → `EventOrderRejected` (`loop.go:677–705`). Bridge marks `FAILED` (`live_bridge.go:305–310`). |
| **F. Cancel order** | **FAIL** | `Client.CancelOrder` exists (`client.go:233`). `SubmitReduceOnlyOrder` for closes (`live_bridge.go:145–157`). **No institutional cancel endpoint.** UI cancel routes return 410. |
| **G. Modify order** | **FAIL** | No amend/modify function in codebase. |
| **H. Disconnect recovery** | **PARTIAL** | REST 10s timeout (`client.go:103`). Position monitor polls every 5 min (`StartMonitor`). Client `fetchWithRetry` for klines. No order-session reconnect. |
| **I. Rate limits** | **PARTIAL** | Client `checkTestnetPlaceOrderRateLimit` 10/hr — but place-order route retired. Go client has no 429/backoff. |
| **J. Duplicate events** | **PARTIAL** | Ledger idempotency keys (`loop.go:369`, `ledger/store.go:55–67`). No broker fill-stream dedup. |

### Delta Critical Defect

`engine/internal/execution/delta_adapter.go` contains placeholder HMAC (`deltaSign` returns `hmac_%s_%s`) — **not production-safe** if used as exchange adapter.

---

## 2. AngelOne

### Files
- `engine/internal/marketdata/angelone.go` — JWT login, LTP quotes
- `client/src/app/api/angelone/orders/route.ts` — read-only order book
- `engine/internal/trading/institutional_request.go` — explicit rejection

| Capability | Verdict | Evidence |
|------------|---------|----------|
| **A. Order submission** | **FAIL** | `"angelone broker adapter disabled"` (`institutional_request.go:28–32`) |
| **B–G. All trading ops** | **FAIL** | No placement integration |
| **E. Rejection** | **PARTIAL** | Login/LTP structured errors (`angelone.go:226–234`) |
| **H. Disconnect recovery** | **PARTIAL** | JWT re-login on 401/403 (`angelone.go:202–216`) |
| **I. Rate limits** | **FAIL** | No throttling |

---

## 3. Binance

### Files
- `engine/internal/execution/binance_live.go` — `ExecuteSignal` (not booted)
- `engine/internal/execution/binance_adapter.go` — adapter stub
- `engine/internal/reconciliationv2/binance_reconciliation.go` — recon only
- `client/src/hooks/useLiveBTCMarket.ts` — market-data WS only

| Capability | Verdict | Evidence |
|------------|---------|----------|
| **A. Order submission** | **FAIL** (code exists, not wired) | `BinanceLiveClient.ExecuteSignal` (`binance_live.go:31`) not in `antigravity/main.go`. Client `UnsupportedLiveAdapter` rejects orders. |
| **B. Ack** | **FAIL** | HTTP 200 treated as accepted without body parse (`binance_adapter.go:72–85`) |
| **C. Fill handling** | **FAIL** | `ExecPrice: 0` returned (`binance_live.go:81–84`). No user data stream. |
| **D. Partial fills** | **FAIL** | Recon reads `ExecutedQty` but no live OMS events |
| **E. Rejection** | **PARTIAL** | HTTP != 200 → error |
| **F. Cancel** | **FAIL** | Code exists, not on live path |
| **G. Modify** | **FAIL** | Not implemented |
| **H. Disconnect recovery** | **PARTIAL** | UI WS reconnect (`useLiveBTCMarket.ts`). Price fallback chain Delta→Binance REST. |
| **I. Rate limits** | **PARTIAL** | `rateDelay = 120ms` for historical loads only |

---

## 4. Coinbase

### Files
- `engine/internal/marketdata/coinbase.go` — WS price feed
- Booted in `antigravity/main.go` for `BTC-USD` ticks

| Capability | Verdict | Evidence |
|------------|---------|----------|
| **A–G. Trading** | **FAIL** | Market-data only |
| **H. Disconnect recovery** | **PASS** (MD) | `keepConnected` retry loop (`coinbase.go:41–60`) |
| **J. Duplicate events** | **PARTIAL** | `TradeID` in ticks but no consumer dedup |

---

## 5. bridge/ (Not a Broker)

`bridge/bridge.js` — ChatGPT Puppeteer automation. No exchange APIs.

All trading capabilities: **FAIL**.

---

## Institutional Gateway (Cross-Cutting)

| Route | File | Venues |
|-------|------|--------|
| `POST /api/execution/request` | `client/src/app/api/execution/request/route.ts` | Proxies to engine |
| `ProcessExecutionRequest` | `institutional_request.go:15` | `paper`, `delta` only |
| Retired direct routes | `blockedExecutionRoute.ts` | 410 EXECUTION_ROUTE_RETIRED |

**Live venue routing today:**
- `paper` ✅
- `delta` ✅ (REST, institutional path)
- `angelone` ❌
- `binance` ❌

---

## Phase 3 Conclusion

| Broker | Production-Ready for Capital | Verdict |
|--------|------------------------------|---------|
| Delta | No — no fill stream, no partial fills, synthetic ack | **FAIL** |
| AngelOne | No — disabled | **FAIL** |
| Binance | No — not wired | **FAIL** |
| Coinbase | N/A — data only | **N/A** |

**Overall Phase 3:** **FAIL** — no broker integration meets institutional fill-attestation requirements.
