# TERMINAL AUTHORITY FINAL CERTIFICATION

**Status:** PASS  
**Scope:** `/terminal/*` Institutional Command Center

---

## Authority Stack

```
TerminalSnapshotProvider [terminalStore.tsx:227-233]
  → useTerminalSnapshotState() [terminalStore.tsx:125-222]
  → InstitutionalTerminalShell [TerminalShell.tsx:21-94]
  → TerminalAuthorityGuard [TerminalLayoutClient.tsx:8-14]
  → Page content
```

---

## Handshake Proof

### WS_OPEN — no premature authority

```typescript
// terminalStore.tsx:48-55
case "WS_OPEN":
  return { ...state, connected: true, hasAuthority: false, ... };
```

### Authority gate function

```typescript
// terminalAuthority.ts:12-18
if (state.restUnavailable || state.updatedAt === "") return false;
if (state.authoritySource === "ws" && state.connected) return true;
if (state.authoritySource === "rest") return true;
```

### First delta required

```typescript
// terminalStore.tsx:59-66, 68-75
WS_DELTA / REST_OK → mergeSnapshot → terminalHasAuthority(next)
```

---

## Guard Coverage

| Route | Guard | File |
|-------|-------|------|
| `/terminal/execution` | Layout | `TerminalLayoutClient.tsx:12` |
| `/terminal/risk` | Layout | same |
| `/terminal/research` | Layout | same |
| `/terminal/strategies` | Layout | same |
| `/terminal/analytics` | Layout | same |
| `/terminal/portfolio` | Layout | same |
| `/terminal/events` | Layout | same |
| `/terminal/journal` | Layout | same |

Shell header renders **outside** guard (intentional) but shows `—` when `!hasAuthority` (`TerminalShell.tsx:46-49, 52-56`).

---

## Synthetic Data Audit

| Field | Initial | After delta | Display when no auth |
|-------|---------|-------------|----------------------|
| Analytics metrics | `null` | Mongo/API | `—` |
| Price | 0 | `/api/btc/price` | `—` |
| spreadBps | 0 | not wired | `—` (check `> 0`) |
| fundingRate | 0 | not wired | `—` (check `!== 0`) |
| Strategy sharpe | null | null | `—` |

Initial snapshot: `terminalSnapshot.ts:30-37` — analytics all `null`.

---

## REST Fallback Path

```
fetchRestAuthority() [terminalStore.tsx:91-122]
  → /api/paper-desk/snapshot
  → /api/strategy-intelligence
  → /api/paper-desk/equity
  → /api/btc/price
  → mapSnapshotToTerminalDelta()
  → REST_OK dispatch
```

Circuit breaker: 3 failures → `REST_UNAVAILABLE` → `hasAuthority: false` (`terminalStore.tsx:185-187`).

---

## Test Evidence

`iccrpImplementation.test.ts`:
- `rejects WS connected before first delta`
- `accepts WS after first delta`
- `strategies[0].sharpe === null`

**Certification:** No terminal page renders authoritative metrics before first delta.
