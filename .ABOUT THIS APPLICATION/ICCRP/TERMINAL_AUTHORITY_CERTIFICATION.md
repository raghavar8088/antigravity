# TERMINAL AUTHORITY CERTIFICATION — ICCF-LDAP Phase 3

---

## Components Audited

| Component | Path | Role |
|-----------|------|------|
| `terminalStore.tsx` | `client/src/lib/terminal/terminalStore.tsx` | Reducer + WS + REST polling |
| `mapSnapshotToTerminalDelta.ts` | `client/src/lib/terminal/mapSnapshotToTerminalDelta.ts` | REST → terminal delta |
| `terminalAuthority.ts` | `client/src/lib/terminal/terminalAuthority.ts` | Authority predicates |
| `TerminalAuthorityGuard.tsx` | `client/src/components/terminal/TerminalAuthorityGuard.tsx` | Render gate |

---

## Authority Predicate

```12:17:client/src/lib/terminal/terminalAuthority.ts
export function terminalHasAuthority(
  state: Pick<TerminalAuthorityState, "authoritySource" | "connected" | "updatedAt" | "restUnavailable">,
): boolean {
  if (state.authoritySource === "ws" && state.connected) return true;
  if (state.authoritySource === "rest" && state.updatedAt !== "" && !state.restUnavailable) return true;
  return false;
}
```

**Proof:** REST authority requires non-empty `updatedAt` (`terminalStore.tsx:68-75` sets this via `mapSnapshotToTerminalDelta` L238).

**Gap:** WS authority requires only `connected === true` — **no payload required**.

---

## Path Analysis

### WebSocket Path

1. `useTerminalSnapshot` reads `NEXT_PUBLIC_TERMINAL_WS_URL` (`terminalStore.tsx:126`)
2. `connect()` → `onopen` → `WS_OPEN` (`141-144`)
3. `onmessage` → parse JSON → `WS_DELTA` (`150-154`)

**Failure:** `WS_OPEN` immediately sets `hasAuthority: true`, `loading: false` (`48-56`) while state still holds `initialTerminalSnapshot` zeros (`terminalSnapshot.ts:3-42`).

**Reachability proof:** Any page with `TerminalAuthorityGuard` renders children with zero snapshot until first `WS_DELTA`.

### REST Fallback Path

1. Poll when `!wsConnectedRef.current` (`terminalStore.tsx:179`)
2. `fetchRestAuthority()` parallel fetch 4 APIs (`91-122`)
3. Success → `REST_OK` + `mapSnapshotToTerminalDelta` (`194`)
4. Fail ×3 → circuit breaker → `REST_UNAVAILABLE` (`185-187, 199-201`)

**Proof:** REST never sets authority without successful snapshot parse (`99-102` returns null on failure).

### Authority Guard Path

```12:33:client/src/components/terminal/TerminalAuthorityGuard.tsx
export function TerminalAuthorityGuard({ snapshot, children }: Props) {
  if (snapshot.loading && !snapshot.hasAuthority) { /* LOADING */ }
  if (snapshot.restUnavailable || !terminalHasAuthority(snapshot)) { /* UNAVAILABLE */ }
  return <>{children}</>;
}
```

**Pages protected:** execution, risk, research, analytics, journal.

**Pages NOT protected:** portfolio, strategies, events.

**Shell NOT protected:** `TerminalShell.tsx` always renders header metrics.

### Disconnected State

- WS close → `WS_CLOSE` (`145-147`); authority retained if `authoritySource === "rest"` (`58`)
- Circuit open → `REST_UNAVAILABLE` → guard blocks (`79-80`, guard L23-30)

### Loading State

- Initial: `loading: true`, `hasAuthority: false` (`terminalStore.tsx:39-44`)
- REST success clears loading (`71-72`)
- WS open clears loading **before data** (`52`) — **BUG**

---

## Synthetic State Render Paths

| Path | Can show synthetic? | Evidence |
|------|---------------------|----------|
| REST + Guard | Zeros possible via `num(v,0)` fallback | `mapSnapshotToTerminalDelta.ts:47-49` |
| WS + Guard | **Yes** — zero initial snapshot | `terminalStore.tsx:48-56` |
| Shell header | **Yes** — no guard | `TerminalShell.tsx:21-57` |
| Portfolio/Strategies/Events | Error on fail; no zero flash from shared store | Own fetch handlers |

---

## Phase 3 Verdict

**FAIL.** Guard exists on 5/8 terminal content pages. WS path grants authority before backend payload. Shell header bypasses guard entirely. `num(..., 0)` allows zero-filled "authoritative" displays.

---

## Remediation Plan

1. **P0** — `WS_OPEN`: set `hasAuthority: false` until first `WS_DELTA` with `updatedAt !== ""`.
2. **P0** — Wrap `TerminalShell` metrics in authority check or hide until `updatedAt` set.
3. **P1** — Add `TerminalAuthorityGuard` to portfolio, strategies, events (or shared layout wrapper).
4. **P1** — Replace `num(v, 0)` with `num(v, NaN)` + UI `—` for missing fields.
5. **P2** — Validate WS delta schema before merge (reject empty payloads).
