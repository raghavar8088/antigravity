# RISK_COVERAGE_REPORT

## Normal execution (strategy, manual request, delta open/close)

All flow through `executeThroughInstitutionalPathWithFill`:

| Control | Location | Blocks when |
|---------|----------|-------------|
| ProcessExecutionRequest kill check | institutional_request.go:16 | Kill switch active (external requests) |
| PMS portfolio gate | loop.go:419 | Heat/VaR/drawdown/daily loss |
| PreTradeRiskPipeline | loop.go:468 | Kill switch + Risk V2 validation |
| Elite drawdown gate | loop.go:535 | Non-elite during drawdown regime |
| Risk V2 execution floor | loop.go:576 | Size below minimum |

Delta close uses identical stack via `institutionalClose` handler (institutional_request.go:219-261).

## Emergency flatten (kill switch)

`InstitutionalPathOpts{EmergencyFlatten: true}` (loop.go:293-296):

- **Bypasses:** PMS gate, PreTradeRiskPipeline (including active kill-switch block), sizing floor
- **Retains:** OMS ledger, EventRiskApproved, audit transitions with reason `emergency_flatten`
- **Rationale:** Kill switch activation must flatten despite active kill flag and sizing constraints

## Bridge enable toggle

`/api/delta-live/enable` does not submit trades. Risk controls apply when paper signal triggers `OnOpen`/`OnClose`.

## Verdict

**PASS** for broker orders under normal operation.  
**PASS*** for emergency flatten with documented intentional sizing/PMS bypass.
