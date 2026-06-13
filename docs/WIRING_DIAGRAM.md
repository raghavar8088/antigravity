# BTC-PILOT SOVEREIGN — Wiring Diagram
Generated: 2026-06-13 | Version: v2.0

## Startup Sequence (main.go → LoopDeps)

```
main.go
  ├─ tracing.InitTracer()               → global OTel tracer
  ├─ secrets.NewSecretClient()          → used inline (not in LoopDeps)
  ├─ mongopersist.EnsureIndexes()       → runs at startup (not in LoopDeps)
  ├─ dataquality.NewValidator()         → LoopDeps.DataValidator
  ├─ reconciliationv2.ReconcileOnRestart() → runs at startup (not in LoopDeps)
  ├─ killswitch.RestoreStateOnStartup() → restores halted state if needed
  ├─ regime.NewClassifier()             → LoopDeps.RegimeClassifier
  ├─ regime.NewStrategyGate()           → LoopDeps.StrategyGate
  ├─ trading.NewCycleGuard()            → LoopDeps.CycleGuard
  ├─ aiscoring.NewAsyncScorer()         → LoopDeps.AsyncScorer
  ├─ aiscoring.NewFallbackScorer()      → LoopDeps.FallbackScorer
  ├─ derivatives.NewFundingFetcher()    → LoopDeps.FundingFetcher
  ├─ derivatives.NewOIFetcher()         → LoopDeps.OIFetcher
  ├─ orderbook.NewDepthSubscriber()     → LoopDeps.DepthSubscriber
  ├─ etf.NewETFFetcher()                → LoopDeps.ETFFetcher
  ├─ dominance.NewDominanceFetcher()    → LoopDeps.DominanceFetcher
  ├─ macro.NewMacroFetcher()            → LoopDeps.MacroFetcher
  ├─ sentiment.NewSentimentFetcher()    → LoopDeps.SentimentFetcher
  ├─ temporal.NewTemporalAnalyser()     → LoopDeps.TemporalAnalyser
  ├─ calibration.LoadLatest()           → LoopDeps.CalibrationResult
  ├─ learning.NewLessonGenerator()      → LoopDeps.LessonGenerator
  ├─ eventstore.NewEventWriter()        → LoopDeps.EventStore
  └─ ml.NewMLPrescorer()                → LoopDeps.MLPrescorer
```

## loop.go Execution Path (per 15-min cycle)

```
[market tick arrives]
  │
  ├─ Change A: DataValidator.ValidateCandle()
  │     ActionHalt → critical quality halt — skip candle
  │     ActionProceedReduced → currentSizingModifier = 0.5 — continue with halved sizes
  │     ActionSkipCandle → skip candle
  │     ActionProceed → currentSizingModifier = 1.0
  │
  ├─ CycleGuard.TryStart() → skip if overlap
  │
  ├─ Change B: RegimeClassifier.Classify()
  │     AllowNewEntries=false → return (HIGH_VOLATILITY — all entries suspended)
  │     StrategyGate.GetActiveStrategies() → build allowedStrategySet
  │     Store lastRegimeClass + lastAllowedStrategySet on Orchestrator
  │     FundingFetcher.GetLatest() + OIFetcher.GetLatest()
  │     DepthSubscriber.GetLatestAnalysis()
  │     ETFFetcher + DominanceFetcher + MacroFetcher + SentimentFetcher
  │     TemporalAnalyser.GetLatest()
  │
  └─ [for each approved signal]:
        Change C Gate 1: allowedStrategySet[strategyName] → skip if blocked by regime gate
        AsyncScorer.GetCachedScore() (or FallbackScorer.Score())
        ApplyMicrostructureWeight()
        MLPrescorer.ShouldProceed() → skip if blocked
        montecarlo.Simulate() → skip if ShouldTrade=false
        Change C Kelly: kelly.GetKellyInputs() + kelly.Compute()
                        finalPositionUSD = kellyResult.FinalPositionUSD × currentSizingModifier
        calibration.Apply() → adjusted confidence
        o.risk.Validate(sig) → existing risk gate
        [execution via bridge or direct]
```

## OMS v3 Order Lifecycle (Fixed)

```
ORDER_CREATED   → StateNew
ORDER_VALIDATED → StateValidated
RISK_APPROVED   → StateRiskApproved
ORDER_SUBMITTED → StateSubmitted
ORDER_ACCEPTED  → StateAccepted        ← NEW: broker receipt confirmed
ORDER_FILLED    → StateFilled
                  (VALIDATED → FILLED direct jump is REJECTED — invalid transition)

Legacy path still supported:
ORDER_SUBMITTED → StateSubmitted → ORDER_ACKED → StateAcknowledged → StateFilled
```

## Kill Switch Flow

```
[activation trigger]
  ├─ KillSwitch.Trigger(activation)
  │     sets in-memory active=true
  │     persist(ctx, activation) → ledger.EventKillSwitchTriggered
  │
[engine crash + restart]
  ├─ KillSwitch.RestoreStateOnStartup(ctx)
  │     wraps RestoreFromLedger(ctx)
  │     replays ledger events to find triggered/released state
  │     if active: logs HALTED state before trading loop starts
  │     returns bool: true = engine starts halted
  │
[manual resume]
  └─ KillSwitch.Release(ctx, ...)
        sets in-memory active=false
        appends ledger.EventKillSwitchReleased
```

## Strategy Registry (606 total)

```
BuildCuratedScalpers():
  ├─ Base curated pack:  ~305 strategies (original + elite variants + alpha modules)
  ├─ Expansion pack:     301 strategies (buildExpansionPack())
  └─ FilterWinnersOnly() → removes strategies with negative net expectancy
                           (WINNERS_ONLY gate active since 2026-05-01)
```

## Shutdown Sequence

```
[SIGTERM received]
  ├─ CycleGuard.Finish() (via defer)
  ├─ AsyncScorer.Stop()
  ├─ DepthSubscriber disconnect (via ctx cancel)
  ├─ EventStore.Stop()
  ├─ tracingShutdown() (via defer)
  └─ ctx cancel → all polling goroutines stop
```
