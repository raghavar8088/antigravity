# Execution Module

## Purpose
Convert approved trading decisions into paper or live orders while preserving OMS, fill, ledger, and reconciliation invariants.

## Dependencies
- Risk gate approval.
- OMS v3 command/event handling.
- Exchange/broker adapters.
- Ledger and position state.
- Reconciliation and kill switch state.

## Entry Points
- `engine/internal/execution/`
- `engine/internal/omsv3/`
- `engine/internal/trading/`
- `client/src/internal/execution/`
- `client/src/internal/oms/`
- `client/src/lib/trading/mockTradingEngine.ts`

## Public APIs
- Paper trading API routes under `client/src/app/api/paper-*` and `client/src/app/api/mock-trading/*`.
- Engine execution endpoints through `client/src/app/api/engine/[...path]`.
- Go execution and OMS package APIs.

## Major Concepts
- Order intent.
- OMS command.
- Fill event.
- Position update.
- Ledger event.
- Reconciliation.
- Kill switch enforcement.

## Files
Start with Graphify paths around `ExecutionEngine`, `OMS`, `Order`, `Fill`, and `Position` before opening source.
