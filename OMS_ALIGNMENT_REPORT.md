# OMS ALIGNMENT REPORT — Forensic Audit Phase 7

**Date:** 2026-06-11  
**Scope:** engine/internal/omsv3/ and client/src/app/api/paper-oms/  
**Method:** Source code reading only. No assumptions.

---

## CRITICAL FINDING: Two Separate, Disconnected OMS Systems

This application has **two completely separate OMS implementations** that do not communicate:

- **Go Engine OMS v3** (`engine/internal/omsv3/`) — institutional state machine, event-sourced, in Go
- **TypeScript Paper OMS** (`client/src/lib/paperOms.ts`) — separate state machine, MongoDB-backed, in TypeScript

These systems share similar state names but are architecturally disconnected. The UI displays only TypeScript OMS state.

---

## Go Engine OMS v3 State Machine

**File:** `engine/internal/omsv3/aggregate.go`

### States Defined (lines 13-22)

| State | Constant |
|-------|----------|
| `""` (empty) | `StateEmpty` |
| `NEW` | `StateNew` |
| `VALIDATED` | `StateValidated` |
| `RISK_APPROVED` | `StateRiskApproved` |
| `SUBMITTED` | `StateSubmitted` |
| `ACKNOWLEDGED` | `StateAcknowledged` |
| `PARTIALLY_FILLED` | `StatePartiallyFilled` |
| `FILLED` | `StateFilled` |
| `CANCELLED` | `StateCancelled` |
| `REJECTED` | `StateRejected` |

### Valid Transitions (aggregate.go:27-38)

```
Empty → NEW
NEW → VALIDATED | CANCELLED | REJECTED
VALIDATED → RISK_APPROVED | CANCELLED | REJECTED
RISK_APPROVED → SUBMITTED | CANCELLED | REJECTED
SUBMITTED → ACKNOWLEDGED | CANCELLED | REJECTED
ACKNOWLEDGED → PARTIALLY_FILLED | FILLED | CANCELLED | REJECTED
PARTIALLY_FILLED → PARTIALLY_FILLED | FILLED | CANCELLED | REJECTED
FILLED → (terminal)
CANCELLED → (terminal)
REJECTED → (terminal)
```

### Event-to-State Mapping (aggregate.go:120-143)

| Ledger Event | → OMS State |
|---|---|
| `EventOrderCreated` | NEW |
| `EventOrderValidated` | VALIDATED |
| `EventRiskApproved` | RISK_APPROVED |
| `EventOrderSubmitted` | SUBMITTED |
| `EventOrderAcked` | ACKNOWLEDGED |
| `EventOrderPartial` | PARTIALLY_FILLED |
| `EventOrderFilled` | FILLED |
| `EventOrderCancelled` | CANCELLED |
| `EventOrderRejected` / `EventRiskBlocked` | REJECTED |

---

## Go Engine OMS v3 — Actual Execution Trace

**File:** `engine/internal/trading/loop.go:374-756`  
In `executeThroughInstitutionalPathWithFill()`, the actual state transitions emitted are:

1. `EventOrderCreated` → NEW (line 412)
2. `EventOrderValidated` → VALIDATED (line 433)
3. On risk approval: `EventRiskApproved` → RISK_APPROVED (line 679)
4. `EventOrderSubmitted` → SUBMITTED (line 696)
5. `EventOrderAcked` → ACKNOWLEDGED (line 701)
6. `EventOrderFilled` → FILLED (line 738)

On risk block: `EventRiskBlocked` → REJECTED  
On execution failure: `EventOrderRejected` → REJECTED (line 715)  
On fill failure: ACCEPTED → CANCELLED in MongoDB paperpersist (line 728)

**Key observation:** `StatePartiallyFilled` is defined but **never emitted** anywhere in the execution path. There is no partial fill logic in the paper client.

---

## Go Engine OMS v3 — Storage

**Default storage:** `ledger.NewMemoryStore()` (loop.go:235) — **VOLATILE, lost on restart**  
**Durable storage:** `ledger.PostgresStore` only if `DATABASE_URL` env var is set (main.go:762)

There is **no MongoDB storage** for Go engine OMS v3 events. The PostgreSQL ledger only persists kill switch events and risk events. Order events in the in-memory store are lost on engine restart.

---

## MongoDB Paper Orders (parallel tracking system)

In parallel with OMS v3, the paperpersist layer writes OMS transitions to MongoDB `paper_orders`:

**File:** `engine/internal/trading/paperpersist_hooks.go:213-226`  
`persistOMSTransition()` → `orderWriter.RecordTransition()` → MongoDB `paper_orders` collection

States written (using paperpersist constants, not omsv3 constants):
- `OMSNew` → `OMSRiskChecked` (or `OMSRejected` on block)
- `OMSNew` → `OMSAccepted` (risk approved)  
- `OMSAccepted` → `OMSSimulatedFill`
- `OMSSimulatedFill` → `OMSPositionOpened`
- `OMSPositionOpened` → `OMSPositionClosed`

These are written to MongoDB `paper_orders` and ARE queryable via `/api/paper-desk/orders`.

---

## TypeScript Paper OMS

**File:** `client/src/lib/paperOms.ts`

### States (lines 19-27)
```
NEW | RISK_CHECKED | REJECTED | ACCEPTED | SIMULATED_FILL | POSITION_OPENED | POSITION_CLOSED | CANCELLED
```

### Storage: `paper_oms_orders` MongoDB collection
**File:** `client/src/lib/paperOmsMongo.ts`

### Who writes to this collection?
After searching the codebase, **no active code path was found that calls `insertPaperOmsOrder()` or `updatePaperOmsOrderStatus()`** from production trading code paths. The functions exist and have tests but no invocation from the active engine flow. The TypeScript OMS appears to be a legacy or planned system that is not currently being populated by the live Go engine execution path.

---

## OMS API Routes

### `/api/paper-oms/orders` (GET)
**File:** `client/src/app/api/paper-oms/orders/route.ts`  
Reads from `paper_oms_orders` — the TypeScript OMS collection. If nothing writes to this collection, returns empty.

### `/api/paper-oms/summary` (GET)
**File:** `client/src/app/api/paper-oms/summary/route.ts`  
Aggregates stats from `paper_oms_orders`.

### `/api/paper-desk/orders` (GET)
**File:** `client/src/app/api/paper-desk/orders/route.ts`  
Reads from `paper_orders` — the Go engine OMS transition records. This IS populated by the Go engine via `persistOMSTransition()`.

---

## OMS State Visibility in UI

### Via `/api/paper-desk/orders` → `PaperOrderDoc`

| Go Engine OMS State | UI Visible via paper_orders? |
|---|---|
| NEW (OMSNew) | YES — transition_to: "NEW" |
| RISK_CHECKED (OMSRiskChecked) | YES |
| REJECTED (OMSRejected) | YES — includes reason |
| ACCEPTED (OMSAccepted/OMSRiskChecked→OMSAccepted) | YES |
| SIMULATED_FILL | YES |
| POSITION_OPENED | YES |
| POSITION_CLOSED | YES |
| CANCELLED (OMSCancelled) | YES |

All paperpersist OMS transitions are visible via `paper_orders`. The Go engine's omsv3 aggregate states (VALIDATED, RISK_APPROVED, SUBMITTED, ACKNOWLEDGED) are **NOT visible** in MongoDB paper_orders — these intermediate OMS v3 states are only in the in-memory ledger and are never persisted to any MongoDB collection.

### Via PaperOmsPanel → `/api/paper-oms/orders` → `paper_oms_orders`

This collection appears to be empty or unpopulated in the current live system. The `PaperOmsPanel` component fetching from this endpoint would show no data if the TypeScript OMS is never written to.

---

## OMS States With No UI Visibility

| State | Why Not Visible |
|---|---|
| `VALIDATED` (Go OMS v3) | Only exists in in-memory ledger |
| `RISK_APPROVED` (Go OMS v3) | Only exists in in-memory ledger |
| `SUBMITTED` (Go OMS v3) | Only exists in in-memory ledger |
| `ACKNOWLEDGED` (Go OMS v3) | Only exists in in-memory ledger |
| `PARTIALLY_FILLED` (Go OMS v3) | Never emitted; partial fills not implemented |

---

## UI Order States With No Go Engine OMS v3 Backing

| UI State (TypeScript OMS) | Go OMS v3 Equivalent | Gap |
|---|---|---|
| `SIMULATED_FILL` | `FILLED` | Different naming; populated via paperpersist not omsv3 |
| `POSITION_OPENED` | No equivalent state in omsv3 (handled by position aggregate) | Extension state specific to paper trading |
| `POSITION_CLOSED` | No equivalent state in omsv3 | Extension state |

---

## Verdict: Is OMS Aligned With UI?

**PARTIALLY ALIGNED, with major gaps:**

1. The Go engine's full OMS v3 aggregate states (9 states) are **not visible** in the UI. Only the 8 paperpersist MongoDB transition labels are visible via `paper_orders`.

2. The `PaperOmsPanel` fetches from a **separate TypeScript OMS collection** (`paper_oms_orders`) that appears to not be populated by the live engine execution path. This panel likely shows nothing useful in production.

3. The `paper_orders` collection (accessible via `/api/paper-desk/orders`) does reflect Go engine OMS transitions and IS reliable for orders that reach the execution path.

4. OMS states VALIDATED, RISK_APPROVED, SUBMITTED, ACKNOWLEDGED have **zero UI visibility** — they exist only in Go RAM.

5. `PARTIALLY_FILLED` is defined in OMS v3 but never used — partial fills are not implemented.
