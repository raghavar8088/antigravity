# OBJECTIVE 1 — TERMINAL AUTHORITY FIX

## Before

```
initialTerminalSnapshot (synthetic BTC/positions)
  → WS disconnected
  → stale fake values rendered forever
```

## After

```
initialTerminalSnapshot (empty zeros)
  → useTerminalSnapshot()
      → WS if NEXT_PUBLIC_TERMINAL_WS_URL
      → else REST: snapshot + strategy-intelligence + equity + /api/btc/price
  → TerminalAuthorityGuard blocks if no authority
  → Components show NO DATA AVAILABLE / LOADING
```

## Code Changes

| File | Lines | Reason |
|------|-------|--------|
| `terminalSnapshot.ts` | 1-42 | Empty authority defaults (already zeroed) |
| `mapSnapshotToTerminalDelta.ts` | NEW | Maps Mongo snapshot → terminal types |
| `terminalStore.tsx` | 1-220 | Multi-source REST polling + circuit breaker |
| `TerminalAuthorityGuard.tsx` | 11-35 | Blocks render without `hasAuthority` |
| `ExecutionCenter.tsx` | QuickTrade removed | No synthetic 0.05 BTC / 3x |
| `ResearchCenter.tsx` | 46-73 hardcoded removed | Live strategy-intelligence |
| `RiskModule.tsx` | fake heatmap removed | Live correlation API or empty |

## Runtime Proof

- `terminalHasAuthority()` returns true only when `authoritySource` is `ws`+connected or `rest`+`updatedAt` set
- Vitest: `iccrpImplementation.test.ts` — mapping + authority tests pass
- Build: `npm run build` succeeds
