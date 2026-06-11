# DATA CONSISTENCY AUDIT — ICCF-LDAP Phase 11

---

## Cross-Check Matrix

| Comparison | Path A | Path B | Consistent? | Notes |
|------------|--------|--------|-------------|-------|
| UI Balance | Portfolio dashboard `snapshot.balance` | Mongo via `/api/paper-desk/portfolio` | **YES** (same API) | Single authority |
| UI Balance | Terminal shell exposure header | Portfolio balance | **NO** | Shell shows exposure not balance (`TerminalShell.tsx:26-30`) |
| UI Balance | RiskRibbon EXPOSURE | Portfolio equity | **PARTIAL** | Ribbon uses raw `state.equity` (`risk-ribbon/route.ts:101`) — should match portfolio if same Mongo doc |
| UI Positions | ExecutionCenter `snapshot.positions` | OMS/Mongo open positions | **YES** | Both from `listOpenPositions` via snapshot (`snapshot/route.ts:34`) |
| UI PnL | Portfolio realized | Strategy intel `total_realized_pnl` | **YES** | Both from `getClosedTradeStats` |
| UI PnL | RiskRibbon TODAY PnL | Actual today PnL | **NO** | Ribbon uses lifetime realized L103 |
| UI Strategy Count | Strategy dashboard `total_strategies` | Research summary `total` | **YES** | Same API family |
| UI Strategy Count | Research snapshot strategies length | Full 600 limit intel | **PARTIAL** | Snapshot poll limits 100 (`terminalStore.tsx:94`) vs Research 600 |
| UI Trades | Portfolio `total_trades` | Closed stats | **YES** | |
| Engine vs UI Balance | Go engine state | Mongo paper_state | **NOT VERIFIED RUNTIME** | Requires live curl — code paths separate |
| Snapshot PF/Sharpe | Analytics terminal page | Portfolio page | **NO** | Snapshot sync build nulls vs async extended metrics |

---

## Code-Level Consistency Proofs

### Positions — Single Mongo Query Chain

```
listOpenPositions(accountKey)
  → snapshot/route.ts:34
  → mapPositions() mapSnapshotToTerminalDelta.ts:63-79
  → ExecutionCenter.tsx:83-94
```

### Balance — Portfolio Authority

```
getPortfolioAccountingSnapshot()
  → portfolio/route.ts:17
  → PortfolioAnalyticsDashboard.tsx:106
```

### Strategy Count Divergence

| Source | Limit | File:Line |
|--------|-------|-----------|
| Terminal REST poll | 100 | `terminalStore.tsx:94` |
| ResearchCenter summary | 600 | `ResearchCenter.tsx:23` |
| Strategy dashboard | 600 | `StrategyIntelligenceDashboard.tsx:97` |

**Impact:** Terminal snapshot strategy table may show subset vs Strategies page.

---

## mergeClosedTradeStatsIntoState

Snapshot route merges accounting into state (`snapshot/route.ts:49-53`) — improves consistency between state block and portfolio block within same response.

---

## Phase 11 Verdict

**FAIL** — Material inconsistencies:
1. Shell header metric semantics (exposure vs balance/equity)
2. Risk ribbon TODAY PnL mislabel
3. Strategy count limit mismatch (100 vs 600)
4. Analytics vs Portfolio Sharpe/PF divergence

---

## Remediation

1. **P0** — Unify strategy-intelligence limit to 600 in `fetchRestAuthority`.
2. **P0** — Fix RiskRibbon today PnL calculation.
3. **P1** — TerminalShell: show equity from snapshot.state via dedicated field, not exposure proxy.
4. **P1** — Snapshot route call `getPortfolioAccountingSnapshot` async for full metrics.
