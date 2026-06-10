# SELF-HEALING UPGRADE REPORT

## Implemented

| Component | File | Behavior |
|-----------|------|----------|
| Execution watchdog | `trading/execution_watchdog.go` | Alerts on stale ticks, kill switch active, no fills 45m |
| Watchdog wiring | `main.go` + `loop.go` | Records tick/signal/fill timestamps |
| Kill switch restore | `killswitch/service.go:65–110` | Replays ledger on boot |
| Auto-release false positives | `killswitch/service.go:shouldAutoReleaseReconFalsePositive` | Releases stale OMS_DESYNC from recon drift |
| Recon startup grace | `killswitch_hook.go:12,22–24` | 90s before hook can trigger |
| Margin drift suppression | `killswitch_hook.go:isKnownFalsePositiveMismatch` | Paper mode has no margin |

## Alerts (Log-Based)

```
[WATCHDOG] ALERT kill_switch_active reason="..."
[WATCHDOG] ALERT stale_market_data last_tick=...
[WATCHDOG] ALERT no_fills last_fill=...
[KILL SWITCH] auto-releasing stale kill switch from reconciliation false positive
```

## Recommended Follow-Up (Not Implemented)

- Prometheus counters: `mock_trading_last_fill_timestamp`
- PagerDuty webhook on `[WATCHDOG] ALERT no_fills`
- Staged recon deploy with canary account before production kill-switch wiring
