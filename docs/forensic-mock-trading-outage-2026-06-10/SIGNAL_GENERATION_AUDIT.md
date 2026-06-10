# SIGNAL GENERATION AUDIT

## Verdict: SIGNALS STILL GENERATING — outage is downstream of signal generation

## Strategy Engine

| Component | File:Function | Lines | Status |
|-----------|---------------|-------|--------|
| Registry | `strategy/curated_registry.go` `BuildCuratedScalpers` | 6+ | OK — 600+ strategies |
| Grouping | `loop.go` `NewOrchestrator` | 217–219 | OK — tick/1m/5m groups |
| Main loop | `loop.go` `Run` | 969–1006 | OK — reads tick channel |
| Tick pipeline | `loop.go` `processTickPipeline` | 1012–1047 | OK |
| Strategy eval | `loop.go` `processStrategyGroup` | 1334–1394 | OK — `Strategy.OnTick` |
| Aggregation | `aggregator_selective.go` `FilterSignalsSelective` | 36+ | OK — may filter weak signals |

## Blocking Conditions (pre-execution, not outage cause)

| Condition | File:Lines | Effect |
|-----------|------------|--------|
| Strategy disabled | `loop.go:1353–1355` | skip |
| Stale signal | `loop.go:1449–1457` | skip |
| Position limit | `loop.go:1461–1466` | skip |
| Regime filter | `loop.go:1474–1480` | skip |
| Aggregator empty | `aggregator_selective.go:73–75` | no approved signals |

## Schedulers / Workers

| Worker | Location | Status |
|--------|----------|--------|
| Orchestrator goroutine | `main.go:880` `safeGo("Orchestrator")` | Alive |
| Recon scheduler | `scheduler.go:62–72` | Alive — **caused kill switch** |
| TS paper worker | `btc-ft-paper-worker.ts` | Disabled when `ENGINE_EXECUTION_AUTHORITY=1` |
| Vercel cron | `paper-desk-tick/route.ts` | Skipped when engine authority |

## Conclusion

Signal generation path intact. Mock trading stopped because **approved signals blocked at kill switch** (`pipeline.go:51–54`), not because strategies stopped running.
