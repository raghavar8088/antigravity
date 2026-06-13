# Risk Module

## Purpose
Prevent invalid, oversized, stale, session-invalid, or unsafe trades from reaching execution.

## Dependencies
- Account and portfolio state.
- Position state.
- Strategy/signal metadata.
- Session and market-hours rules.
- Kill switch state.
- Loss and exposure limits.

## Entry Points
- `engine/internal/risk/`
- `engine/internal/risk/gate/`
- `engine/internal/riskv3/`
- `client/src/lib/trading/futuresDeskPolicy.ts`
- `client/src/components/RiskRibbon.tsx`
- `client/src/app/api/desk-entry-funnel/*`

## Public APIs
- Risk gate package APIs.
- Desk policy helpers.
- Risk dashboard and diagnostics routes.

## Major Concepts
- Risk gate.
- Position sizing.
- Loss limit.
- Session gate.
- Correlation/exposure gate.
- Kill switch.
- Rejection reason.

## Files
Do not modify execution paths before identifying risk gate callers with Graphify. Risk must remain before live or paper execution.
