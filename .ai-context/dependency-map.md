# Dependency Map

## High-Level Dependencies
```text
client UI
-> client API routes
-> client domain libs / engine proxy
-> Go engine HTTP/API surface
-> engine strategy/risk/OMS/execution packages
-> broker/market data adapters
-> persistence and observability
```

## Client Dependencies
- UI components depend on hooks and `client/src/lib/` domain helpers.
- API routes depend on broker clients, persistence helpers, trading domain logic, auth/session helpers, and engine proxy utilities.
- Trading hooks depend on API routes and normalize route responses for dashboard state.
- Analytics and replay helpers depend on persisted trades and signal trace state.

## Engine Dependencies
- `cmd/antigravity` wires environment, strategy registry, market data, risk, OMS, execution, persistence, observability, and health endpoints.
- Strategy packages depend on normalized market data and shared strategy/risk types.
- Risk gates depend on positions, account state, loss limits, session rules, and policy configuration.
- OMS v3 depends on command/event invariants and feeds execution/ledger state.
- Ledger and reconciliation depend on fills, positions, broker state, and persistence.
- Kill switch depends on risk, admin controls, and production safety state.

## Structured Maps
Use these files for precise dependency lookup:
- `.ai-context/dependencies.json`
- `.ai-context/call-graph.json`
- `.ai-context/repo-map.json`

Regenerate with:

```bash
npm run ai-context:maps
```
