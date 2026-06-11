# FAILURE MODE RECERTIFICATION

**Status:** PASS (institutional terminal)  
**Audited:** 2026-06-11

## Simulated Failures

| Failure | Expected UI | Verified Behavior |
|---------|-------------|-------------------|
| No WebSocket | REST fallback or guard | REST poll when WS down; guard if both fail |
| No Mongo | Guard / NO DATA | `REST_UNAVAILABLE` after circuit breaker; `mongoUnconfigured()` on APIs |
| No OMS | Empty orders, no synthetic fills | Event center shows empty or degraded; no fake orders |
| No Market Data | Price `—` | `TerminalShell` requires `hasAuthority && price > 0` |
| No SEP | N/A on terminal SEP panels | SEP not rendered in institutional terminal shell |
| No Risk API | Risk events from Mongo state only | Drawdown from `paper_state`; no synthetic risk scores |
| Engine unreachable | SYSTEM event + ribbon RED | `platformEvents.ts` SYSTEM warning; risk-ribbon ENGINE RED |

## No Synthetic Values Rule

Initial terminal snapshot (`terminalSnapshot.ts`):
- Analytics metrics = `null` (not 0)
- Positions = `[]`
- `updatedAt = ""` → blocks authority

## Operator Warning

`TerminalAuthorityGuard.tsx`:
- LOADING state before first delta
- `BACKEND AUTHORITY UNAVAILABLE` on circuit break
- Rose badge in shell header

## Circuit Breaker

`terminalStore.tsx:86-89, 185-187`:
- 3 REST failures → 30s backoff
- `REST_UNAVAILABLE` clears authority
