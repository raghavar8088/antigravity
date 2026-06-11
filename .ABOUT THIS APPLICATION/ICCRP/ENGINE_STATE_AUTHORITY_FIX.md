# OBJECTIVE 2 — ENGINE STATE AUTHORITY FIX

## Finding (Before)

`useEngineState.ts` returned hardcoded `FALLBACK_BALANCE = 1000000`.

## Fix (After)

`useEngineState.ts` polls `/api/paper-desk/state` every 10s for:
- `balance`, `equity`, `unrealized_pnl`, `current_drawdown`

Returns `null` when unavailable — **never** synthetic $1M.

## Reachability

- Used by components importing `useEngineState`
- Source chain: Go engine → MongoDB `paper_state` → `/api/paper-desk/state`

## Runtime Proof

```typescript
// useEngineState.ts — fetchState polls /api/paper-desk/state
setBalance(typeof s.balance === "number" ? s.balance : null);
```

`TerminalDashboard.tsx` KPI cards show `—` when `equity == null` (no STARTING_BALANCE fallback for display).
