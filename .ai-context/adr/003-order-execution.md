# ADR 003: OMS-Centered Order Execution

## Status
Accepted

## Context
Trading systems need auditable order state transitions, consistent fill handling, and clear separation between intent, validation, broker submission, and accounting. Directly mutating positions from strategy code makes reconciliation and debugging unsafe.

## Decision
Use OMS-centered execution. Strategies create intent, risk approves or rejects, OMS records command/event state, execution adapters submit or simulate orders, fills update positions and ledger, and reconciliation checks internal state against broker or persisted state.

## Consequences
- Order lifecycle is explicit and inspectable.
- Partial fills, cancellations, and rejections can be represented safely.
- Ledger and reconciliation can reason from fills/events rather than raw strategy intent.
- Execution adapters stay replaceable across paper and live trading.

## AI Guidance
When adding order behavior, update OMS, execution, fill, ledger, and reconciliation impacts together. Do not let strategies directly mutate positions or balances.
