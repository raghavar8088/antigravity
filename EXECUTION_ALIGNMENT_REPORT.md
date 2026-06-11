# EXECUTION ALIGNMENT REPORT — Forensic Audit Phase 10

**Date:** 2026-06-11  
**Scope:** All execution paths in Go engine — broker order submission  
**Method:** Source code reading only. No assumptions.

---

## All Execution Paths

### Path 1: BTC Paper Execution (PRIMARY PATH)
**File:** `engine/internal/execution/paper.go:137`  
**Entry:** `PaperClient.ExecuteSignal(sig, mode)`  
**Called from:** `executeThroughInstitutionalPathWithFill()` via the `fillFn` closure (loop.go:328-336)  
**Goes through OMS?** YES — full OMS v3 event sequence (NEW→VALIDATED→RISK_APPROVED→SUBMITTED→ACKNOWLEDGED→FILLED)  
**Persisted?** YES — events to ledger + MongoDB `paper_orders` + `paper_positions` + `paper_trades`  
**UI Visible?** YES — via paper_desk/snapshot polling  
**Updates positions?** YES — `openAndTrackPosition()` registers in `positions.Manager`  
**Fill confirmation to strategy?** Implicit via `risk.NotifyFill()` and position open. No callback to strategy.

**Operation:** Pure in-memory simulation. `PaperClient.applyFill()` debits `balanceUSD` by `notional + fee` for BUY, credits `balanceUSD` by `notional - fee` for SELL. Sets `positionBTC` accordingly.

---

### Path 2: Delta Live Order Submission
**File:** `engine/internal/delta/live_bridge.go:131-142`  
**Entry:** `bridge.SubmitOrder(ctx, productID, side, contracts)` → `client.PlaceOrder()`  
**Called from:** `processDeltaExecutionRequest()` via `fillFn` closure (institutional_request.go:117-132)  
**Goes through OMS?** YES — via `executeThroughInstitutionalPathWithFill()` (institutional_request.go:133)  
**Persisted?** YES — same OMS v3/paperpersist path as paper execution  
**UI Visible?** Partially — persisted to same MongoDB collections but Delta-specific UI at `/api/delta-live/*`  
**Updates positions?** YES — result from broker is mapped to `FillResult`, `openAndTrackPosition()` is NOT called from `processDeltaExecutionRequest()` (gap found — see below)

**Delta API call:** `POST https://api.delta.exchange/v2/orders` with HMAC-SHA256 authentication  
**HMAC implementation:** `engine/internal/delta/client.go` — full implementation with proper crypto/hmac/sha256

**WARNING:** The `deltaSign()` function in `execution/delta_adapter.go:191-197` is a STUB:
```go
return fmt.Sprintf("hmac_%s_%s", method, timestamp[:8])
```
However, this stub is in `DeltaAdapter` which is the newer ExchangeAdapter interface. The actual live Delta execution uses `delta.Client` (live_bridge.go:131-142) which has the proper HMAC implementation. The `DeltaAdapter` stub in `execution/delta_adapter.go` is not the path used for live trading.

---

### Path 3: Delta Bridge OnOpen (Option Opens)
**File:** `engine/internal/delta/live_bridge.go:116-128` (SetInstitutionalOpenHandler)  
**Called from:** `WireDeltaBridge()` (institutional_request.go:155+)  
**Goes through OMS?** YES — routes through `executeThroughInstitutionalPathWithFill()` via institutionalOpen handler  
**Persisted?** YES  
**UI Visible?** Via `/api/delta-live/*` routes  

---

### Path 4: Binance Live Client (LEGACY PATH)
**File:** `engine/internal/execution/binance_live.go:31-80`  
**Entry:** `BinanceLiveClient.ExecuteSignal(sig, mode)` → `POST https://api.binance.com/api/v3/order`  
**Goes through OMS?** NO — `BinanceLiveClient.ExecuteSignal()` calls the Binance REST API directly with no OMS wiring  
**Persisted?** NO — no ledger events, no MongoDB writes  
**UI Visible?** NO  
**Fill confirmation to strategy?** NO  
**Kill switch check?** NO  

**CRITICAL FINDING:** `BinanceLiveClient` bypasses all institutional controls. However, checking main.go for whether this is wired into production:
`engine/internal/reconciliation/sync.go:20` — `Reconciler` uses `BinanceLiveClient` for reconciliation (position querying), not order execution. From grep results, `BinanceLiveClient` appears not to be wired into the main trading execution path currently — it is only used for reconciliation queries.

---

### Path 5: BinanceAdapter (ExchangeAdapter interface)
**File:** `engine/internal/execution/binance_adapter.go:37-85`  
**Entry:** `BinanceAdapter.PlaceOrder(ctx, req)`  
**Goes through OMS?** Designed to be called from OMS layer  
**Persisted?** GetOrder() and GetPosition() currently return stub/placeholder responses (lines 150-167)  
**UI Visible?** Would be via OMS events  
**Status:** PARTIALLY IMPLEMENTED — PlaceOrder is wired, GetOrder/GetPosition are stubs  

---

### Path 6: DeltaAdapter (ExchangeAdapter interface)
**File:** `engine/internal/execution/delta_adapter.go:39-82`  
**Entry:** `DeltaAdapter.PlaceOrder(ctx, req)`  
**Goes through OMS?** Designed to be called from OMS layer  
**HMAC signing:** STUB (`deltaSign()` returns fake `hmac_METHOD_TIMESTAMP`)  
**Status:** STUB — not production-ready. The live Delta execution uses `delta.Bridge` directly.

---

### Path 7: Emergency Flatten
**File:** `engine/internal/trading/loop.go:339-372`  
**Entry:** `orchestrator.ExecuteEmergencyFlatten(ctx, sig, reason)`  
**Goes through OMS?** YES — calls `executeThroughInstitutionalPathWithFill()` with `EmergencyFlatten: true`  
**Persisted?** YES — same path as normal execution  
**UI Visible?** YES — appears in paper_orders as `KILL_SWITCH_*` strategy name  
**Bypasses:** PMS gate and sizing floors (pathOpts.EmergencyFlatten = true, line 437)  
**Kill switch:** Does NOT check kill switch again (would be circular — this IS the flatten response)

---

### Path 8: Manual Order via HTTP API
**File:** `engine/cmd/antigravity/main.go` + `engine/internal/executiongateway/handler.go`  
**Entry:** External execution request via HTTP  
**Goes through OMS?** YES — routes to `orchestrator.ProcessExecutionRequest()` → `processPaperExecutionRequest()` or `processDeltaExecutionRequest()`  
**Kill switch:** YES — first check in `ProcessExecutionRequest()` (institutional_request.go:16)

---

### Path 9: PaperOMS via HTTP admin override
**File:** `engine/internal/execution/paper_oms_handler.go:42-54`  
**Entry:** `POST /paper/open`, `POST /paper/close/{id}`  
**Requires:** `PAPER_OMS_ADMIN_OVERRIDE` header + env var match  
**Goes through OMS?** NO — uses `PaperOMS` directly, not the institutional path  
**Persisted?** NO — PaperOMS is in-memory, separate from institutional path  
**Status:** Admin override only, disabled in production unless env var is set

---

## Execution Paths — Summary Table

| Path | Goes Through OMS? | Kill Switch? | Persisted? | UI Visible? | Position Updated? |
|------|---|---|---|---|---|
| BTC Paper (primary) | YES | YES | YES | YES | YES |
| Delta Live (institutional) | YES | YES | YES | Partially | YES |
| Delta Bridge OnOpen | YES | YES | YES | Partially | YES |
| BinanceLiveClient (legacy) | NO | NO | NO | NO | NO |
| BinanceAdapter | Designed-YES | Unknown | Stub | Stub | Unknown |
| DeltaAdapter | Designed-YES | Unknown | NO (HMAC stub) | NO | NO |
| Emergency Flatten | YES (partial) | Bypassed | YES | YES | YES |
| HTTP Manual Request | YES | YES | YES | YES | YES |
| PaperOMS admin override | NO | NO | NO | NO | NO |

---

## Hidden Execution Paths That Bypass Controls

### FINDING 1: BinanceLiveClient has no OMS, no kill switch, no persistence
**File:** `engine/internal/execution/binance_live.go:31`  
If this were wired into production order flow, it would place orders with zero institutional controls. Currently appears to be used only for reconciliation position queries.

### FINDING 2: DeltaAdapter HMAC is a stub
**File:** `engine/internal/execution/delta_adapter.go:191-197`  
The `DeltaAdapter` in the ExchangeAdapter interface has a placeholder HMAC implementation that returns a fake signature. Any attempt to place live orders through `DeltaAdapter.PlaceOrder()` would be rejected by Delta Exchange with an authentication error. The actual live Delta path uses `delta.Client` (live_bridge.go) which has proper HMAC.

### FINDING 3: PaperOMS admin override bypasses institutional path
**File:** `engine/internal/execution/paper_oms_handler.go`  
The `POST /paper/open` and `POST /paper/close/{id}` endpoints bypass the full institutional path, risk checks, and persistence. They modify the in-memory `PaperOMS` state but this is a separate in-memory structure from the main trading engine's `PaperClient`. Any balances changed here would not be reflected in MongoDB or the main position state.

---

## Orphan Positions (Positions Without Fills)

**Potential for orphan positions in crash scenario:**  
1. `openAndTrackPosition()` in loop.go:844 first calls `posMgr.OpenPosition()` (in-memory)
2. Then launches async goroutine `go o.emitPositionOpened(ctx, pos, fill, sig)`
3. Then calls `persistPositionOpen()` (another async goroutine)

If the engine crashes after step 1 but before steps 2/3 complete:
- The position exists in `positions.Manager` (RAM — lost)
- The position is NOT in MongoDB `paper_positions`
- On restart, the position does not exist anywhere

However, the paper balance WAS modified by `PaperClient.applyFill()` already (in `submitInstitutionalOrder`, before `openAndTrackPosition` is called). So balance is debited but position may not be recorded if crash occurs between fill and goroutine completion.

**The 10-second state snapshotter mitigates this** — if it fired before the crash, `paper_state` will have the new balance, and on recovery the position can be reconstructed from `paper_positions`. But there is a 10-second window of vulnerability.

---

## Fill Confirmation Back to Strategy

**No fill confirmation callback to strategy was found.**  
After `executeThroughInstitutionalPath()` returns, the loop calls `risk.NotifyFill(sig)` (reduces exposure), then `openAndTrackPosition()` (registers position). The original strategy that generated the signal is NOT notified of the fill.

**Evidence of absence:** The `strategy.RegistryEntry` struct does not have a fill-callback field. `Strategy.OnTick()` returns `[]Signal` and has no fill-notification method. The strategy does not know if its signal was executed or rejected.

This is by design for the paper trading path — strategies are stateless signal generators. They emit signals on every tick regardless of whether previous signals were filled.

---

## Verdict: Is All Execution Visible and Auditable?

**FOR BTC PAPER TRADING: YES, mostly auditable.**  
Every paper trade goes through the institutional path, OMS v3 events, paperpersist MongoDB writes. The audit trail is: ledger events (in-memory or PostgreSQL) + MongoDB paper_orders + paper_trades + paper_positions.

**FOR DELTA LIVE TRADING: PARTIALLY AUDITABLE.**  
Goes through institutional path with OMS events and paperpersist MongoDB writes. The Delta-specific LiveTrade records in the Bridge's in-memory `trades` slice are not persisted to MongoDB independently — they are available via the `/api/delta-live/stats` endpoint (which reads bridge RAM state) but are lost on engine restart.

**FOR BINANCE LIVE CLIENT: NOT AUDITABLE.**  
`BinanceLiveClient.ExecuteSignal()` places orders directly with no OMS, no kill switch, no persistence. Currently this path is not used for order execution (only reconciliation queries), but it represents a control gap if it were wired.

**FOR DELTA ADAPTER (ExchangeAdapter interface): NON-FUNCTIONAL.**  
HMAC stub means any live Delta orders attempted through this adapter would fail authentication.

**STRATEGY FILL FEEDBACK: NONE.**  
Strategies have no mechanism to learn if their signals were filled, rejected, or pending. They generate signals on every tick regardless.
