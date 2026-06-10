# FINAL MOCK TRADING RECOVERY CERTIFICATION

**Audit date:** 2026-06-10  
**Standard:** Forensic — source code, wiring, tests only; no trust in prior docs  
**Production commit (HEAD):** `33c614a8` — **still contains outage bug**  
**Fixes:** Present in **uncommitted working tree only** (`git status` shows 10+ modified files)

---

## FINAL VERDICT: **VERDICT 4 — UNSAFE (production)**

**VERDICT 3 — CERTIFIED WITH MATERIAL RISKS (local working tree, if deployed)**

Production cannot be certified until fixes are committed, built, deployed, and validated on Lightsail with live fill evidence.

---

## Phase 12 Answers (Evidence-Based)

| # | Question | Status | Evidence |
|---|----------|--------|----------|
| 1 | Can mock trading generate signals? | **PASS** (code) / **UNKNOWN** (prod) | `loop.go:1042` `recordWatchdogTick`; `processStrategyGroup` → `OnTick` |
| 2 | Can mock trading open positions? | **PASS** (code) / **UNKNOWN** (prod) | `loop.go:1660` → `executeThroughInstitutionalPath` → `paper.go:137` |
| 3 | Can mock trading close positions? | **PASS** (code) | `loop.go:processCloseEvents` → `SettlePosition` |
| 4 | Can risk approve trades? | **PASS** when kill switch inactive | `loop.go:512-528` `PreTradeRiskPipeline.Check` |
| 5 | Can OMS create orders? | **PASS** | `loop.go:374+` ledger events + `omsv3.Replay` |
| 6 | Can mock broker fill orders? | **PASS** | `paper.go:137-152` `ExecuteSignal` |
| 7 | Can positions update correctly? | **PASS** | `positions/manager.go:126` `OpenPosition` |
| 8 | Can portfolio state update? | **PASS** | `paperpersist_hooks.go` Mongo writes |
| 9 | Can PnL update? | **PASS** | `paper.go:SettlePosition`, `processCloseEvents` |
| 10 | Can reconciliation operate safely? | **PARTIAL** | Fixes in working tree; HEAD still broken; open-position equity not fully tested |
| 11 | Can watchdog detect outages? | **PARTIAL** | Logs alerts; bitmask dedup bug `execution_watchdog.go:174-191`; no alert before first fill |
| 12 | Can kill switch recover? | **PASS** (working tree) | `restore_test.go` PASS; `RestoreFromLedger` wired `main.go:723` |
| 13 | Can restart restore trading? | **PARTIAL** | Auto-release for recon false positives only; legitimate KS persists |
| 14 | 72h unattended? | **FAIL** | No long-run test, no prod deploy proof |

---

## Scores

| Metric | Score | Rationale |
|--------|-------|-----------|
| Production Readiness | **4/10** | HEAD still has kill-switch outage path; fixes undeployed |
| Mock Trading Readiness | **6/10** | Working-tree fixes + unit tests pass; no live replay / 72h proof |

---

## Top Remaining Risks

1. **HEAD commit still uses `TotalPnLUSD` as equity** — `git show HEAD:ledger_oms_reader.go`
2. **Fixes not committed or deployed** — `git status` modified, not pushed
3. **Kill switch may still be active in prod Postgres ledger** — requires `GET /api/admin/ks/status`
4. **EscalateCount > 0 triggers kill switch without false-positive filter** — `killswitch_hook.go:50-65`
5. **Delta reconciliation authority** (if env creds set) can trigger real OMS_DESYNC — `wiring.go:73-87`
6. **Equity model with open positions** not certified at >1% drift threshold — only empty-book + side tests
7. **ETH symbol normalization missing** — `normalizeReconSymbol` only maps BTC variants — `position_manager_adapter.go:136-141`
8. **Watchdog no-trade bitmask bug** — stale `fired` variable in loop — `execution_watchdog.go:174-191`
9. **No no-trade detection before first fill since boot** — `execution_watchdog.go:165-167`
10. **Health endpoint omits position count, strategy count, risk state** — `main.go:1661-1691`
11. **`trading_allowed: true` while `no_trades_24h` status** — health logic `main.go:1665-1666`
12. **Legacy `omsv3/snapshot_provider.go` still uses TotalPnLUSD as equity** — not on hot path but drift if used
13. **90s recon grace then cyclic kill-switch re-trigger** if real drift persists
14. **No automated recovery from kill switch during runtime** (only startup auto-release)
15. **PMS / risk gates can still block all signals** independent of recon fix

---

## Top Remediation Actions

1. Commit and deploy working-tree fixes to Lightsail
2. `GET /api/admin/ks/status` — release if active
3. Fix watchdog bitmask: `fired = w.noTradeAlertFired.Load()` inside loop
4. Add integration test: open position + real `GetEquityUSD` vs ledger equity
5. Add ETH (and other) symbol normalization if multi-asset recon needed
6. Gate `EscalateCount` kill-switch trigger with same false-positive filters
7. Extend health endpoint with open positions, kill switch, recon drift score
8. Alert on zero fills since boot (not only since last fill)
9. Run 24h+ staging soak with `/api/health/mock-trading` polling
10. Verify Mongo `paper_trades` insert rate post-deploy

---

## Test Evidence (executed 2026-06-10)

```
go test ./internal/reconciliationv2/ -run TestPostFix     → PASS (3 tests)
go test ./internal/killswitch/ -run TestRestore           → PASS (2 tests)
go test ./internal/certification/ -run TestProductionFlow → PASS
go test ./internal/trading/ -run TestExecutionWatchdog    → FAIL (bitmask bug)
```

---

## Production vs Working Tree

| Item | HEAD `33c614a8` | Working tree |
|------|-----------------|--------------|
| Equity = initial + PnL | **NO** (TotalPnLUSD) | **YES** `ledger_oms_reader.go:130-147` |
| Side normalization | **NO** | **YES** `detectors.go:160,171` |
| Kill switch restore | **NO** | **YES** `main.go:723`, `service.go:68-110` |
| Watchdog | **NO** | **YES** `main.go:728-730` |
| Health endpoint | **NO** | **YES** `main.go:1661-1691` |

**Burden of proof:** Production mock trading is **UNSAFE** until deploy + live fill proof.
