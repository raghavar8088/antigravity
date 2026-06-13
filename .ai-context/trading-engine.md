# Trading Engine

## Purpose
The Go engine coordinates market data, strategy execution, risk controls, order management, ledger state, reconciliation, persistence, and production safety.

## Core Flow
```text
Market Data
-> Strategy Registry
-> Signal Evaluation
-> Risk Gate
-> OMS v3 Command/Event Handling
-> Execution
-> Fill Handling
-> Ledger
-> Reconciliation
-> Kill Switch Check
-> Persistence / Metrics / API
```

## Key Packages
- `engine/internal/marketdata/`: market data adapters and fallback providers.
- `engine/internal/strategy/`: curated registry and strategy families.
- `engine/internal/risk/`: loss limits, sizing, gates, and policy checks.
- `engine/internal/omsv3/`: order lifecycle command and event invariants.
- `engine/internal/execution/`: execution path and exchange-facing operations.
- `engine/internal/ledger/`: financial event recording.
- `engine/internal/reconciliation/`: broker/internal state repair.
- `engine/internal/killswitch/`: hard safety stop controls.
- `engine/internal/persistence/`: engine state storage.

## Do Not Break
- Kill switch must remain wired in prod paths.
- Risk gate must precede execution.
- Ledger must reflect fills and position changes consistently.
- Reconciliation must not silently overwrite authoritative broker state.
- NSE/BSE logic must be session gated.
