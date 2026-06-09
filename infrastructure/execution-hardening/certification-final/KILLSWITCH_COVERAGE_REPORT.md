# KILLSWITCH_COVERAGE_REPORT

## Pre-trade gate (normal orders)

`PreTradeRiskPipeline.Check` blocks when `killSvc.IsActive()` (gate/pipeline.go:51-54).

Wired via `orchestrator.SetKillSwitch` (main.go:723).

## External execution requests

`ProcessExecutionRequest` returns BLOCKED when kill active (institutional_request.go:16-20).

## Delta bridge

`WireDeltaBridge` sets `killCheck` on bridge (institutional_request.go:149-154).  
OnClose now routes through ETP which includes pipeline kill check (normal path).

## Kill switch actions

| Action | Path | Institutional |
|--------|------|---------------|
| Mode A block | ksSvc.IsActive | ✓ |
| Mode B flatten | KillSwitchExecutor.FlattenPositions → ExecuteEmergencyFlatten | ✓ OMS path |
| Mode C nuclear | admin HandleTrigger → ExecuteEmergencyFlatten | ✓ OMS path |
| CancelOpenOrders | posMgr.CloseAllPositions | No broker order |

## Verdict

**PASS** — Kill switch governs all normal broker submission paths. Emergency flatten intentionally bypasses active-kill pipeline block to complete flatten while recording OMS/audit trail.
