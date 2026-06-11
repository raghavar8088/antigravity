# SYNTHETIC DATA CERTIFICATION — ICCF-LDAP Phase 2

**Method:** Full codebase search + reachability analysis for `/terminal/*` display paths.

---

## ACTIVE — Visible in Terminal UI

| Item | File | Function / Lines | Description | Status |
|------|------|------------------|-------------|--------|
| Pseudo-Sharpe | `mapSnapshotToTerminalDelta.ts` | `mapStrategies` L108 | `sharpe: s.evidence_score / 50` shown as "Sharpe" in ResearchCenter | **ACTIVE** |
| Zero-default numeric coercion | `mapSnapshotToTerminalDelta.ts` | `num()` L47-49 | Missing fields become `0`, not null/undefined | **ACTIVE** |
| Hardcoded spread/funding | `mapSnapshotToTerminalDelta.ts` | `mapSnapshotToTerminalDelta` L226-227 | `spreadBps: 0`, `fundingRate: 0` always on REST | **ACTIVE** (shows as `—` in shell) |
| Hardcoded funding received | `mapSnapshotToTerminalDelta.ts` | `mapRisk` L143 | `fundingReceivedUsd: 0` | **ACTIVE** |
| Duplicate Sharpe labels | `mapSnapshotToTerminalDelta.ts` | `mapAnalytics` L161-162 | 30D and 90D both use `portfolio.sharpe` | **ACTIVE** (misleading) |
| Empty R-multiple buckets | `mapSnapshotToTerminalDelta.ts` | L166 | Always `[]` — UI shows NO DATA | **ACTIVE** (safe) |
| Initial zero snapshot | `terminalSnapshot.ts` | L3-42 | Zeros in all numeric fields before first fetch | **ACTIVE** — reachable via WS_OPEN |
| Portfolio PF null → 0.00 | `strategy-intelligence/route.ts` | L123 | `profit_factor: null` | **ACTIVE** — `fmt(null)` → "0.00" in dashboard L164 |
| BTC benchmark = equity | `mapSnapshotToTerminalDelta.ts` | L156 | `btcBenchmark: num(pt.equity)` — not BTC benchmark | **ACTIVE** (mislabeled) |
| Risk ribbon "TODAY PnL" | `risk-ribbon/route.ts` | L103 | Uses lifetime `closedStats.realized_pnl`, not today | **ACTIVE** (mislabeled) |

---

## ACTIVE — Outside `/terminal/*` but in App

| Item | File | Lines | Status |
|------|------|-------|--------|
| Mock candle builder | `TerminalDashboard.tsx` | L7, L257 | **ACTIVE** on `/` (root page), not institutional terminal |
| Mock trading module | `mockMarketSimulator.ts`, `MockTradingDashboard.tsx` | entire modules | **ACTIVE** at `/mock-trading` (separate product surface) |
| NIFTY synthetic chain | `api/nifty/option-chain/route.ts` | L164-243 | **ACTIVE** — not in terminal scope |
| Fake diversity strategies | `useBTCFuturesScalperEngine.ts` | L1017-1019 | **ACTIVE** in paper desk engine, not terminal display |

---

## REMOVED / FIXED (verified in source)

| Item | File | Evidence | Status |
|------|------|----------|--------|
| Hardcoded $1M balance in useEngineState | `useEngineState.ts` | L43-46 polls `/api/paper-desk/state`, returns `null` on missing | **REMOVED** |
| Browser mock trading authority | `useBTCFuturesScalperEngine.ts` | L1366 comment: "permanently disabled" | **REMOVED** |

---

## UNREACHABLE — Test / Mock Only

| Item | File | Status |
|------|------|--------|
| `sampleTrade` fixtures | `paperTradesMapper.test.ts` | UNREACHABLE (test) |
| vi.mock in delta testnet | `deltaTestnetRoutes.test.ts` | UNREACHABLE (test) |
| MockTradingDashboard test mocks | `MockTradingDashboard.test.tsx` | UNREACHABLE (test) |

---

## WS Pre-Authority Zero Flash

| Item | File | Lines | Status |
|------|------|-------|--------|
| Authority before data | `terminalStore.tsx` | `WS_OPEN` L48-56 sets `hasAuthority: true` with initial zeros | **ACTIVE** — synthetic zero state reachable |

---

## Phase 2 Verdict

**FAIL.** At least one **ACTIVE** synthetic metric (pseudo-Sharpe) is displayed with a real metric label in production terminal UI. Zero-default coercion and WS pre-authority flash remain material risks.
