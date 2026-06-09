# Order Lifecycle Report

**Audit date:** 2026-06-09

---

## OMS State Machine (Proven)

**File:** `engine/internal/omsv3/aggregate.go`, `order_lifecycle_test.go`

### States
| State | Constant | Trigger Event |
|-------|----------|---------------|
| PENDING | (pre-SUBMITTED) | `EventOrderCreated` |
| SUBMITTED | `StateSubmitted` | `EventOrderSubmitted` |
| ACKNOWLEDGED | `StateAcknowledged` | `EventOrderAcked` |
| PARTIAL | `StatePartiallyFilled` | `EventOrderPartial` |
| FILLED | `StateFilled` | `EventOrderFilled` |
| CANCELLED | `StateCancelled` | `EventOrderCancelled` |
| REJECTED | `StateRejected` | `EventOrderRejected` |

**Ledger projection:** `engine/internal/ledger/order_projection.go:14–19`

---

## Order Type Support Matrix

### Market Orders

| Lifecycle Stage | Go Paper | Go Delta | Client Paper OMS | Verdict |
|-----------------|----------|----------|------------------|---------|
| Create | ✅ `EventOrderCreated` | ✅ | ✅ `createPaperOmsOrder` | **PASS** |
| Submit | ✅ `EventOrderSubmitted` | ✅ | ✅ `markAccepted` | **PASS** |
| Accept/Ack | ✅ synthetic ack | ✅ synthetic ack | ✅ simulated | **FAIL** (not broker-attested) |
| Fill | ✅ instant full | ✅ REST assumed full | ✅ `markSimulatedFill` | **FAIL** (no partial) |
| Partial Fill | ❌ | ❌ | ❌ | **FAIL** |
| Cancel | ✅ `EventOrderCancelled` on reject | ❌ no gateway | ❌ | **FAIL** |
| Reject | ✅ `EventOrderRejected` | ✅ | N/A | **PASS** |
| Expire | ❌ | ❌ | ❌ | **FAIL** |

### Limit Orders

| Stage | Go | Delta | Client | Verdict |
|-------|-----|-------|--------|---------|
| Create | `OrderMode` param exists | `TypeLimit` in Delta types | ❌ market only | **FAIL** |
| Submit | Paper path only | API supports | ❌ | **FAIL** |
| Fill on touch | ❌ | ❌ no polling | ❌ | **FAIL** |
| Cancel | ❌ live path | `CancelOrder` exists | ❌ | **FAIL** |
| Expire | ❌ | ❌ | ❌ | **FAIL** |

### Stop / Stop-Market / Stop-Limit

| Type | Code Evidence | Verdict |
|------|---------------|---------|
| Stop (exchange) | Not found in Delta `PlaceOrderRequest` usage | **FAIL** |
| Stop-Market | Not implemented | **FAIL** |
| Stop-Limit | Not implemented | **FAIL** |
| Software SL/TP | `positions.Manager.CheckStopLossAndTakeProfit` | **PASS** (in-process only) |

**Critical:** Stop-loss for Go BTC scalper is **software-monitored on ticks**, not exchange-native stop orders. If engine stops, SL is not enforced on exchange.

### Reduce-Only

| Path | Evidence | Verdict |
|------|----------|---------|
| Delta close | `SubmitReduceOnlyOrder` `live_bridge.go:145–157` with `ReduceOnly: true` | **PASS** |
| Delta open | Not reduce-only (correct) | **PASS** |
| Go paper | No reduce-only flag | N/A |
| Client paper | No reduce-only concept | N/A |

---

## Lifecycle Test Coverage

| Test File | Coverage |
|-----------|----------|
| `omsv3/order_lifecycle_test.go` | Full state transitions including partial |
| `certification/flow_certification_test.go` | Invalid transition rejection |
| `certification/exchange_failure_test.go` | Cancel after submit, duplicate fills |
| `certification/stress_certification_test.go` | Mass cancel |

**Tests prove OMS logic. Live path does not exercise partial/cancel/expiry.**

---

## Client Paper OMS States

**File:** `client/src/lib/paperOms.ts`

| Transition | Function |
|------------|----------|
| NEW → RISK_CHECKED | `markRiskChecked` |
| → ACCEPTED | `markAccepted` |
| → SIMULATED_FILL | `markSimulatedFill` |
| → POSITION_OPENED | `markPositionOpened` |
| → POSITION_CLOSED | `markPositionClosed` |

No CANCELLED, REJECTED, or PARTIAL states in client OMS.

**Verdict:** **FAIL** for full lifecycle parity with Go OMS.

---

## ExchangeAdapter Interface

**File:** `engine/internal/execution/exchange_adapter.go`

Defines: `PlaceOrder`, `CancelOrder`, `GetOrder`  
**No `ModifyOrder`** anywhere in codebase.

---

## Phase 6 PASS/FAIL Matrix

| Order Type | Create | Submit | Accept | Fill | Partial | Cancel | Reject | Expire | Overall |
|------------|--------|--------|--------|------|---------|--------|--------|--------|---------|
| Market (paper) | PASS | PASS | FAIL | PARTIAL | FAIL | FAIL | PASS | FAIL | **FAIL** |
| Market (delta) | PASS | PASS | FAIL | FAIL | FAIL | FAIL | PASS | FAIL | **FAIL** |
| Limit | FAIL | FAIL | FAIL | FAIL | FAIL | FAIL | FAIL | FAIL | **FAIL** |
| Stop | FAIL | FAIL | FAIL | FAIL | FAIL | FAIL | FAIL | FAIL | **FAIL** |
| Stop-Market | FAIL | FAIL | FAIL | FAIL | FAIL | FAIL | FAIL | FAIL | **FAIL** |
| Stop-Limit | FAIL | FAIL | FAIL | FAIL | FAIL | FAIL | FAIL | FAIL | **FAIL** |
| Reduce-Only (delta close) | PASS | PASS | FAIL | FAIL | FAIL | N/A | PASS | FAIL | **FAIL** |
| Software SL/TP | N/A | N/A | N/A | PASS | FAIL | N/A | N/A | N/A | **PARTIAL** |

**Overall Phase 6:** **FAIL** — only market order create/submit/reject proven; fill attestation, partial fills, cancel, modify, and exchange stops all fail.
