# FAILURE MODE CERTIFICATION — ICCF-LDAP Phase 12

**Method:** Code-path analysis of degraded-state behavior (no live simulation run in this audit environment).

---

## Scenario Matrix

| Failure | Expected Behavior | Actual (Code Proof) | Pass? |
|---------|-------------------|---------------------|-------|
| WebSocket offline | REST fallback; no fake data | WS absent → poll REST (`terminalStore.tsx:179-195`); REST fail → circuit breaker | **PARTIAL** |
| WebSocket online, no frames | Should not show authority | `WS_OPEN` sets authority immediately (`48-56`) | **FAIL** |
| SEP offline | Mongo fallback | `rankings/route.ts:55-105` | PASS |
| Mongo offline | Error states, no zeros | `mongoUnconfigured()` / `mongoUnavailable()` on APIs | PASS |
| OMS offline | Degraded indicator | RiskRibbon OMS → RED on catch L117-119; Risk page silent | PARTIAL |
| Market data offline | No fake price | Shell shows `—` when price 0 (`TerminalShell.tsx:46`); RiskRibbon MARKET DATA RED L47-50 | PASS |
| Risk API offline | No fake VaR | RiskModule shows `—` for zero VaR L37-39 | PASS |
| REST circuit open | Block UI | `TerminalAuthorityGuard` shows UNAVAILABLE L23-30 | PASS |
| Portfolio API fail | Error banner | `BACKEND AUTHORITY UNAVAILABLE` L137-144 | PASS |
| Strategy intel fail | Error banner | Dashboard L220-224 | PASS |
| Event center fail | Error banner | EventCenter L165-168 | PASS |

---

## WebSocket Offline — Detailed

1. `wsUrl` unset → WS effect no-ops (`terminalStore.tsx:137`)
2. REST polls every 3s (`179-211`)
3. After 3 failures → `REST_UNAVAILABLE`, `hasAuthority: false` (`185-187`)
4. Guard blocks content pages (`TerminalAuthorityGuard.tsx:23-30`)

**Gap:** Shell header still renders (unguarded) with `—` for most fields when price=0.

---

## Mongo Offline

All terminal APIs return structured errors via `mongoUnconfigured()` / `mongoUnavailable()`.

Portfolio/Strategies/Events show explicit error — **no synthetic balances injected**.

---

## SEP Offline

No `strategy_evidence.json` → `readSepStrategyEvidence()` returns null → Mongo path (`rankings/route.ts:55`).

Terminal doesn't call SEP APIs directly — uses strategy-intelligence (Mongo).

---

## Fake Value Display Under Failure

| Component | Shows fake on failure? | Evidence |
|-----------|------------------------|----------|
| TerminalAuthorityGuard pages | NO (blocked) | Guard L23-30 |
| TerminalShell | Zeros possible before REST | initial snapshot |
| Portfolio dashboard | NO | error state L137-144 |
| Strategy dashboard | NO | error L220-224 |
| RiskRibbon | NO | unavailable banner L78-85 |
| mapSnapshot num() | Zeros for missing fields | Always — **risk on partial API success** |

---

## Phase 12 Verdict

**FAIL** due to WS pre-data authority and partial OMS/market visibility on Risk page.

Degraded states are **generally well implemented** on standalone dashboards; **shared terminal store WS path is unsafe**.

---

## Remediation

See TERMINAL_AUTHORITY_CERTIFICATION.md P0 items + add integration test simulating WS open without message.
