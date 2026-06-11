# AUTHORITY HANDSHAKE CERTIFICATION

**Status:** PASS  
**Audited:** 2026-06-11

## Finding (Before)

```
WS_OPEN → hasAuthority=true → UI renders before first WS_DELTA
```

`terminalStore.tsx:48-55` set `hasAuthority: true` on socket open.  
`terminalAuthority.ts:15` treated `ws + connected` as authoritative without data.

## Remediation

### Store reducer (`terminalStore.tsx`)

| Action | connected | hasAuthority | updatedAt required |
|--------|-----------|--------------|-------------------|
| WS_OPEN | true | **false** | — |
| WS_DELTA | true | via `terminalHasAuthority()` | yes |
| REST_OK | — | via `terminalHasAuthority()` | yes |
| REST_UNAVAILABLE | — | false | — |

### Authority function (`terminalAuthority.ts:12-18`)

```typescript
if (state.restUnavailable || state.updatedAt === "") return false;
if (state.authoritySource === "ws" && state.connected) return true;
if (state.authoritySource === "rest") return true;
```

### Unified provider (`terminalStore.tsx:225-239`)

`TerminalSnapshotProvider` shares one authority state across all terminal routes.

### Unified guard (`TerminalLayoutClient.tsx`)

All `/terminal/*` pages wrapped in `TerminalAuthorityGuard` at layout level.

## Reachability Proof

```
WebSocket onopen
  → dispatch WS_OPEN [terminalStore.tsx:144]
  → hasAuthority=false, connected=true
  → TerminalAuthorityGuard shows LOADING [TerminalAuthorityGuard.tsx:13-20]

First REST/WS delta with updatedAt
  → terminalHasAuthority() true [terminalAuthority.ts:12]
  → Guard renders children
```

## Test Proof

`iccrpImplementation.test.ts`:
- `rejects WS connected before first delta`
- `accepts WS after first delta`

## UI Proof

- Shell price/regime/exposure show `—` when `!hasAuthority` (`TerminalShell.tsx:45-49`)
- No placeholder KPI zeros before first delta (`terminalSnapshot.ts` analytics init = null)
