# OBJECTIVE 3 — REST FALLBACK ARCHITECTURE

## Implementation: `terminalStore.tsx`

| Parameter | Value |
|-----------|-------|
| REST poll (WS down) | 3s |
| REST poll (WS up, backup) | 5s |
| Circuit breaker threshold | 3 failures |
| Circuit breaker backoff | 30s |
| WS reconnect | 2s after close |

## Data Sources (parallel fetch)

1. `/api/paper-desk/snapshot` — positions, journal, risk, health
2. `/api/strategy-intelligence?view=all&limit=100` — strategies
3. `/api/paper-desk/equity?points=30` — equity curve
4. `/api/btc/price` — mark price

## Failure Handling

| State | UI |
|-------|-----|
| `loading && !hasAuthority` | LOADING spinner |
| `restUnavailable` | BACKEND AUTHORITY UNAVAILABLE |
| `hasAuthority` | Render terminal pages |

## Mapping Layer

`mapSnapshotToTerminalDelta.ts` — single function, unit tested.
