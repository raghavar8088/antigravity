# Phase 1–5 Wiring Diagram

```
engine/cmd/antigravity/main.go
│
├── loadDotEnv()
│
├── [Wiring 1] secrets.NewSecretClient(region, useLocal)
│   └── env fallback when USE_LOCAL_SECRETS=true or AWS unreachable
│
├── production.RunBootGate()
│
├── coinbaseClient.Connect(ctx)         ← market data
├── strategy.BuildCuratedScalpers()     ← 600+ strategies
├── risk.NewRiskEngine()
├── execution.NewPaperClient()
├── persistence.NewStore()              ← PostgreSQL
│
├── paperpersist.NewMongoManager()      ← MongoDB Atlas
│   ├── mongoMgr.EnsureIndexes()        ← paperpersist indexes
│   └── [Wiring 2] mongopersist.EnsureIndexes(ctx, mongoMgr.DB())
│       └── TTL: paper_trades(90d), audit_logs(90d),
│               engine_killswitch(180d), signal_history(60d)
│
├── [Wiring 4] reconciliationv2.ReconcileOnRestart(ctx, mongoMgr.DB(), btcPrice)
│   ├── Queries paper_trades WHERE status=OPEN
│   ├── Checks each against current BTC price for SL/TP breach
│   └── Closes retroactively; FATAL on error
│
├── trading.NewOrchestrator(...)
│   └── orchestrator.SetKillSwitch(ksSvc)
│       orchestrator.SetPMSBudget(pmsBudget)
│       orchestrator.SetPaperPersist(ppBundle)
│
├── warmup (FetchWarmupCandles)
│
├── [Wiring 3] dataquality.NewValidator()
├── [Wiring 5] regime.NewClassifier()
│             regime.NewStrategyGate(classifier, strategyNamesAdapter)
│             &trading.CycleGuard{}
│             &aiscoring.FallbackScorer{}
│             aiscoring.NewAsyncScorer(nil, 3).Start()
│
├── [Wiring 6] derivatives.NewFundingFetcher().StartPolling(ctx, 15m)
│             derivatives.NewOIFetcher().StartPolling(ctx, 15m)
│
├── [Wiring 7] orderbook.NewDepthSubscriber()
│             go depthSubscriber.Connect(ctx)   ← Binance @depth20@1000ms
│
├── loopDeps := &trading.LoopDeps{...all above deps...}
│   orchestrator.SetDeps(loopDeps)
│
└── go orchestrator.Run(ctx)
    │
    └── trading/loop.go
        │
        ├── processTickPipeline()          ← every tick
        │
        ├── process1mCandles()             ← every 1m candle
        │   └── [Change A] dataquality.Validator.ValidateCandle()
        │       HALT/SKIP → continue (drop candle)
        │       PROCEED/PROCEED_REDUCED → processStrategyGroup(M1, "1m")
        │
        ├── process5mCandles()             ← every 5m candle
        │   ├── processStrategyGroup(M5, "5m")
        │   ├── every 3rd: run15mCycle()   ← [Change B]
        │   │   ├── CycleGuard.TryStart()  ← prevents overlap
        │   │   ├── OIFetcher.UpdatePriceDirection()
        │   │   ├── DepthSubscriber.SetCurrentPrice()
        │   │   └── processStrategyGroup(M15, "15m")
        │   └── every 12th: processStrategyGroup(H1, "1h")
        │
        ├── processStrategyGroup()         ← all strategies × tick
        │   ├── aggregator.FilterSignalsSelective()
        │   ├── regime gate (isCategoryAlignedWithRegime)
        │   ├── quality filter (executionWeight)
        │   ├── profit sanitizer (sanitizeSignalForProfit)
        │   ├── [Change C] DerivativesScore × OrderBook → ApplyMicrostructureWeight
        │   │   sig.Confidence = ApplyMicrostructureWeight(conf×100, signals, dir) / 100
        │   ├── [Change C] asyncScorer.GetCachedScore() → blend if same direction
        │   ├── risk.Validate()
        │   └── executeThroughInstitutionalPath()
        │       ├── PMS gate
        │       ├── PreTradeRiskPipeline (kill switch + Kelly + Risk V2)
        │       └── paper fill → openAndTrackPosition()
        │
        └── preScoreLoop() [Change D]     ← goroutine, every 30s
            └── asyncScorer.SubmitForScoring(regime_prescore)
                └── FallbackScorer.Score() → cache for 60s
```

## Data Flow — Microstructure (Phase 3)

```
Binance fapi/v1/fundingRate (every 15m)
  → FundingFetcher.GetLatest() → FundingData{Rate, Signal}
  ↘
    derivatives.ComputeDerivativesScore(funding, oi)
    → DerivativesScore.TotalScore [-3, +3]
    → DerivativesScoreToConfidenceModifier [-0.20, +0.15]
  ↗
Binance fapi/v1/openInterest (every 15m)
  → OIFetcher.GetLatest() → OIData{OI_USD, Trend, MarketState}

Binance ws/btcusdt@depth20@1000ms (continuous)
  → DepthSubscriber → OrderBook snapshot every 2m
  → Analyse() → OrderBookAnalysis.Score [-3, +3]

Both scores → alpha.ApplyMicrostructureWeight(baseConfidence, signals, bias)
  → adjusted confidence [0, 95.0]
```

## Kill Switch & Safety (Unchanged)

```
Every new order → PreTradeRiskPipeline
  ├── ksSvc.IsActive()     → block if active
  ├── Risk V2 Kelly        → size
  ├── PMS budget gate      → portfolio heat/VaR/drawdown
  └── Elite drawdown gate  → drawdown regime
```
