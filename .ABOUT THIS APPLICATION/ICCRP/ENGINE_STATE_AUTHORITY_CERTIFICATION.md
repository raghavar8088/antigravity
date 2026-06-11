# ENGINE STATE AUTHORITY CERTIFICATION — ICCF-LDAP Phase 4

---

## Component: `useEngineState.ts`

**Path:** `client/src/hooks/useEngineState.ts`

### Fields Exposed

| Field | Source | Lines | Backend Authority |
|-------|--------|-------|-------------------|
| `engineOnline` | `GET ${resolveEngineApiUrl()}/health` | L26-33 | Go engine `/health` |
| `balance` | `GET /api/paper-desk/state` → `data.state.balance` | L37-43 | Mongo `paper_state` |
| `equity` | Same → `state.equity` | L44 | Mongo |
| `unrealizedPnl` | Same → `state.unrealized_pnl` | L45 | Mongo |
| `drawdownPct` | Same → `state.current_drawdown` | L46 | Mongo |
| `loading` | Cleared after first state fetch | L49-51 | N/A |

### Fields NOT Exposed (audit requirement gap)

| Required Field | Status |
|----------------|--------|
| Open positions | **NOT IN HOOK** |
| Trade count | **NOT IN HOOK** |
| Strategy count | **NOT IN HOOK** |
| Risk status | **NOT IN HOOK** |

### Hardcoded Fallback Check

**Previous claim:** `FALLBACK_BALANCE = 1_000_000`  
**Current code:** No hardcoded balance. Returns `null` when missing (`L43-46`).

```36:46:client/src/hooks/useEngineState.ts
    const fetchState = () => {
      fetch("/api/paper-desk/state")
        .then((res) => (res.ok ? res.json() : null))
        .then((data) => {
          if (cancelled || !data?.ok) return;
          const s = data.state;
          if (!s) return;
          setBalance(typeof s.balance === "number" ? s.balance : null);
          setEquity(typeof s.equity === "number" ? s.equity : null);
```

### Reachability / Runtime Wiring

```bash
# Grep result: useEngineState imported nowhere in client/src
```

**Verdict:** Hook is **correctly implemented** but **UNWIRED** — zero runtime consumers in terminal or dashboard paths as of this audit.

Terminal account state is instead sourced via:
- `useTerminalSnapshot()` → `/api/paper-desk/snapshot`
- `PortfolioAnalyticsDashboard` → `/api/paper-desk/portfolio`
- `RiskRibbon` → `/api/risk-ribbon` + Mongo direct in route

---

## Backend Authority: `/api/paper-desk/state`

Consumed by `useEngineState` and available to any caller.

Trace: `state/route.ts` → `getPaperState(accountKey)` from `paperDeskClient`.

---

## Phase 4 Verdict

| Question | Answer |
|----------|--------|
| Balance from backend? | YES in hook implementation |
| Hook used in terminal? | **NO — dead code path** |
| Full engine state coverage? | **NO — 4 of 8 required fields missing** |

**FAIL** for institutional certification scope — required fields not exposed; hook not integrated into command center UI.

---

## Remediation

1. **P1** — Either wire `useEngineState` into terminal shell OR delete and document `useTerminalSnapshot` as sole authority.
2. **P1** — Extend hook with `openPositionCount`, `totalTrades`, `strategyCount`, `riskStatus` from snapshot API.
3. **P2** — Add Vitest proving null display when `/api/paper-desk/state` returns 503.
