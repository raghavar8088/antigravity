# OMS_COVERAGE_REPORT

## Standard institutional path

Every `executeThroughInstitutionalPathWithFill` invocation records:

1. `EventOrderCreated` + OMS `NEW` (loop.go:338-351)
2. `EventOrderValidated` (loop.go:399)
3. PMS / risk transitions or rejection with OMS state (loop.go:419-571)
4. `submitInstitutionalOrder`:
   - `EventRiskApproved`
   - OMS `ACCEPTED` (loop.go:625-640)
   - `EventOrderSubmitted`, `EventOrderAcked`
   - fillFn execution
   - `EventOrderFilled` + OMS `SIMULATED_FILL` (loop.go:641-668)

Extracted helper `submitInstitutionalOrder` (loop.go:625) ensures consistent OMS coverage for normal and emergency paths.

## Delta broker orders

| Flow | OMS source |
|------|------------|
| Delta open | ETP before `SubmitOrder` fillFn |
| Delta close | ETP before `SubmitReduceOnlyOrder` fillFn |
| Manual delta request | ProcessExecutionRequest → ETP |

Broker fill occurs **after** OMS ACCEPTED transition.

## Emergency flatten

`ExecuteEmergencyFlatten` (loop.go:311):
- Records OMS NEW → RISK_CHECKED (emergency) → ACCEPTED → SIMULATED_FILL
- Source tag: `"emergency_flatten"`

## Kill switch position close (Mode B)

`CancelOpenOrders` updates position manager only — no new broker order (paper mark close).

## Gaps

| Case | OMS |
|------|-----|
| Angel One venue request | Rejected before OMS — no broker order |
| admin HandleCloseAll | Position manager close only — no order aggregate |

## Verdict

**PASS** for all broker-touching production paths — OMS transitions precede Delta `PlaceOrder`.
