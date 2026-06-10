# OUTAGE TIMELINE REPORT

## Reconstructed Timeline

| Event | Est. Time | File:Function | Evidence |
|-------|-----------|---------------|----------|
| Last successful trade (typical) | Before 2026-06-08 | Mongo `paper_trades` | Requires prod DB query |
| Commit `33c614a8` merged | 2026-06-10 20:11 IST | git log | Recon v2 production wiring |
| Engine boot + orchestrator start | T+0 | `main.go:880` | `go orchestrator.Run` |
| Recon v2 WireProduction | T+0 | `main.go:887`, `wiring.go:68` | Runtime authority started |
| First balance recon cycle | T+≤10s | `scheduler.go:66,88` | BalancesInterval 10s |
| **FIRST FAILURE** equity_drift CRITICAL | T+≤10s | `detectors.go:74–87` | OMS equity ≈ PnL vs runtime ≈ $1M |
| Kill switch triggered | T+≤10s | `killswitch_hook.go:24–35` | OMS_DESYNC |
| PreTradeRiskPipeline blocks orders | T+≤10s | `pipeline.go:51–54` | All new fills stopped |
| Signals continue generating | Ongoing | `loop.go:1334` | Strategies still OnTick |
| Orders/fills cease | After kill switch | `loop.go:503–536` | RiskBlocked path |

## FIRST FAILURE POINT

```
Timestamp:  ≤10 seconds after engine boot (first DomainBalance cycle)
Function:   BalanceDriftDetector.Detect
File:       engine/internal/reconciliationv2/detectors.go:74–87
Reason:     equity_drift CRITICAL — exchange=1000000.00 OMS=0.00 (or small PnL)
```

## Secondary failure (open positions)

```
Function:   PositionDriftDetector.Detect
File:       engine/internal/reconciliationv2/detectors.go:177–191, 243–257
Reason:     ghost_position + missing_position (BUY vs LONG side keys)
```
