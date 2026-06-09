# EXECUTION_CALL_GRAPH_FINAL

Forensic source scan — post hardening phase 2 (2026-06-09).

## Institutional core

```
executeThroughInstitutionalPath (loop.go:293)
  └─ executeThroughInstitutionalPathWithFill (loop.go:346)
       ├─ OMS ledger: EventOrderCreated → OMS NEW (loop.go:338-351)
       ├─ PMS portfolio gate (loop.go:419+) [skipped if EmergencyFlatten]
       ├─ PreTradeRiskPipeline + KillSwitch (loop.go:468+) [skipped if EmergencyFlatten]
       ├─ Elite drawdown gate (loop.go:535+)
       ├─ Risk V2 execution floor (loop.go:576+)
       └─ submitInstitutionalOrder → fillFn (loop.go:625+)
            ├─ Paper: o.exec.ExecuteSignal (loop.go:301)
            └─ Delta: bridge.SubmitOrder / SubmitReduceOnlyOrder (institutional_request.go)
```

## External HTTP entry points

| Caller | Callee | Path |
|--------|--------|------|
| `POST /api/execution/request` (client route.ts:34) | engine gateway | ProcessExecutionRequest |
| `executiongateway.Handler` (handler.go:67) | Orchestrator | ProcessExecutionRequest |
| `POST /api/delta-live/order` (main.go:1507) | Orchestrator | ProcessExecutionRequest |
| `POST /api/ai/submit` (main.go:1828) | Orchestrator | ConfirmSignal → ETP |
| `POST /api/ai/bridge-result` (main.go:1868) | Orchestrator | ConfirmSignalFromBridge → ETP |

## Delta live bridge (broker)

| Event | Handler | Broker touch |
|-------|---------|--------------|
| Paper open hook (main.go:904) | `Bridge.OnOpen` → `institutionalOpen` | `executeThroughInstitutionalPathWithFill` → `SubmitOrder` (institutional_request.go:205) |
| Paper close hook (main.go:916) | `Bridge.OnClose` → `institutionalClose` | `executeThroughInstitutionalPathWithFill` → `SubmitReduceOnlyOrder` (institutional_request.go:253) |
| Monitor auto-close (live_bridge.go:654) | `OnClose` | Same institutional close path |
| Manual UI | `submitExecutionRequest` → gateway | ProcessExecutionRequest delta path |

**Hard fail:** If `institutionalOpen` / `institutionalClose` nil → trade marked FAILED, no broker call (live_bridge.go:254-262, 346-354).

## Strategy loop (automated paper)

```
loop.go:1554 → executeThroughInstitutionalPath → ExecuteSignal
loop.go:1184  → executeThroughInstitutionalPath (AI)
loop.go:1871  → ConfirmSignal → executeThroughInstitutionalPath
```

## Kill switch flatten (paper emergency)

```
KillSwitchExecutor.FlattenPositions (killswitch_executor.go:46)
  └─ Orchestrator.ExecuteEmergencyFlatten (loop.go:311)
       └─ executeThroughInstitutionalPathWithFill(EmergencyFlatten=true)
            └─ submitInstitutionalOrder → ExecuteSignal

admin.KillSwitchController.HandleTrigger (killswitch.go:92)
  └─ orchestrator.ExecuteEmergencyFlatten (wired main.go:1321)
```

## Retired / inactive paths

| Path | Status |
|------|--------|
| `Bridge.OnOpen` direct `client.PlaceOrder` fallback | **Removed** — hard fail if handler nil |
| `Bridge.OnClose` direct `client.PlaceOrder` | **Removed** — institutional close only |
| `Bridge.PlaceManualOrder` | Returns error (live_bridge.go:663) |
| Next.js broker POST routes | 410 `blockedDirectExecutionRoute` |
| `sor/coordinator.go` PlaceOrder | Not wired in `antigravity/main.go` |
| `binance_live.go` ExecuteSignal | Not wired in prod engine |
| Browser `useBTCFuturesScalperEngine` poll | Returns immediately — no execution (line 2676) |

## Broker API leaf nodes (production)

Only these reach Delta REST `PlaceOrder`:

1. `Bridge.SubmitOrder` (live_bridge.go:135) — called from ETP fillFn only
2. `Bridge.SubmitReduceOnlyOrder` (live_bridge.go:149) — called from ETP fillFn only
