# RECONCILIATION FORENSIC REPORT

## Is reconciliation blocking orders?

**Indirectly YES (root cause):** Reconciliation v2 does not reject individual orders inline. It triggers kill switch on CRITICAL drift (`killswitch_hook.go:18–36`), which blocks `PreTradeRiskPipeline`.

## Loops running?

| Component | File | Interval | Status |
|-----------|------|----------|--------|
| Orders | `scheduler.go:64` | 2s | Running when authority started |
| Positions | `scheduler.go:65` | 5s | Running |
| Balances | `scheduler.go:66` | 10s | **Triggered false equity drift** |
| Exposure | `scheduler.go:67` | 10s | Running |
| Full audit | `scheduler.go:68` | 60s | Running |

## False positive chain (pre-fix)

1. `LedgerOMSStateReader.GetOMSSnapshot` — equity = `pnl.TotalPnLUSD` (~$0–500)
2. `PositionManagerExchangeAdapter.GetBalances` — equity = `GetEquityUSD()` (~$1M)
3. `BalanceDriftDetector.Detect` — drift >1% → CRITICAL
4. `CriticalDriftKillSwitchHook` — `Trigger(OMS_DESYNC)`
5. All new mock orders blocked

## Position false positive (when open positions exist)

- OMS key: `BTC-USD|BUY` (ledger side from `loop.go:842`)
- Runtime key: `BTC-USD|LONG` (`position_manager_adapter.go:78`)
- Result: ghost + missing position, both CRITICAL

## Post-fix

- Equity: `initialBalance + realized + unrealized` (`ledger_oms_reader.go:buildLedgerBalanceSnapshot`)
- Side keys normalized via `positionSideKey()` (`detectors.go`, `position_manager_adapter.go`)
- 90s grace period (`killswitch_hook.go:12`)
- Margin drift suppressed in paper mode

## Repair engine

`RepairEngine.Repair` (`engine.go:103`) — auto-repair for small drifts; escalates manual intervention → also triggered kill switch via `EscalateCount > 0` (`killswitch_hook.go:38–52`).
