# Protected Modules

These modules are protected because they contain BTC futures trading safety, execution, accounting, and broker-boundary logic.

## Protected Areas
- `strategy-engine`: `engine/internal/strategy/`, `client/src/lib/trading/`, `client/src/lib/strategyAuthority/`
- `risk-engine`: `engine/internal/risk/`, `engine/internal/risk/gate/`, `engine/internal/riskv3/`, `client/src/lib/trading/futuresDeskPolicy.ts`
- `order-execution`: `engine/internal/omsv3/`, `engine/internal/oms/`, `engine/internal/execution/`, `engine/internal/trading/`, `client/src/internal/oms/`, `client/src/internal/execution/`
- `exchange-adapters`: `engine/internal/exchange/`, `engine/internal/marketdata/`, `client/src/lib/broker/`, broker API routes
- `position-management`: `engine/internal/positions/`, `engine/internal/ledger/`, `engine/internal/reconciliation/`, `engine/internal/reconciliationv2/`
- `paper-trading-math`: BTC futures paper math, fees, funding, liquidation, sizing, and PnL helpers under `client/src/lib/trading/`
- `safety-controls`: `engine/internal/killswitch/`, `engine/internal/security/`, admin kill/reset routes

## Rules
- Do not delete protected modules.
- Do not rename public interfaces without explicit user approval.
- Do not modify business rules without explicit user approval.
- Do not bypass risk gates, session gates, or kill switch checks.
- Do not change fee, funding, liquidation, PnL, or ledger math without focused tests.
- Do not replace broker adapter boundaries with direct exchange-specific logic in strategies or UI.
- Do not silently change persisted trade, order, position, ledger, or reconciliation schemas.

## Required Before Editing
1. Read `.ai-context/business-rules.md`.
2. Read `.ai-context/domain-model.md`.
3. Read the relevant ADR under `.ai-context/adr/`.
4. Query Graphify for callers and dependencies.
5. Add or update focused tests for changed behavior.

## Approval Required
Ask the user before:
- Removing a strategy family.
- Re-adding losing strategies while WINNERS_ONLY is active.
- Changing production kill switch behavior.
- Changing max loss, sizing, leverage, liquidation, funding, or fee assumptions.
- Changing live broker execution behavior.
