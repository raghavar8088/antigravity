# ADR 002: Risk Engine Before Execution

## Status
Accepted

## Context
The platform supports automated trading paths where stale data, oversized exposure, session violations, or strategy defects can create financial risk. Both paper and live paths must preserve the same safety model.

## Decision
Risk gates must run before OMS/execution submission. Risk checks include sizing, exposure, daily loss, session, stale data, and kill switch state. Rejections should carry explicit reasons for diagnostics.

## Consequences
- Execution paths must not create orders before risk approval.
- UI and API diagnostics can explain why a trade did not happen.
- Paper trading remains useful as a rehearsal for live safety behavior.
- Tests should target gate decisions and rejection reasons for changed risk behavior.

## AI Guidance
Do not move execution earlier in the flow to simplify code. If a feature needs a new order path, wire it through policy/risk checks first.
