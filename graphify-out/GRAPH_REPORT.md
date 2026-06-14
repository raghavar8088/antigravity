# Graph Report - antigravity-main  (2026-06-14)

## Corpus Check
- 4583 files · ~26,759,370 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 730 nodes · 1165 edges · 56 communities (51 shown, 5 thin omitted)
- Extraction: 96% EXTRACTED · 4% INFERRED · 0% AMBIGUOUS · INFERRED: 49 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `e11aa89e`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 0|Community 0]]
- [[_COMMUNITY_Community 1|Community 1]]
- [[_COMMUNITY_Community 2|Community 2]]
- [[_COMMUNITY_Community 3|Community 3]]
- [[_COMMUNITY_Community 4|Community 4]]
- [[_COMMUNITY_Community 5|Community 5]]
- [[_COMMUNITY_Community 6|Community 6]]
- [[_COMMUNITY_Community 7|Community 7]]
- [[_COMMUNITY_Community 8|Community 8]]
- [[_COMMUNITY_Community 9|Community 9]]
- [[_COMMUNITY_Community 10|Community 10]]
- [[_COMMUNITY_Community 11|Community 11]]
- [[_COMMUNITY_Community 12|Community 12]]
- [[_COMMUNITY_Community 13|Community 13]]
- [[_COMMUNITY_Community 14|Community 14]]
- [[_COMMUNITY_Community 15|Community 15]]
- [[_COMMUNITY_Community 16|Community 16]]
- [[_COMMUNITY_Community 17|Community 17]]
- [[_COMMUNITY_Community 18|Community 18]]
- [[_COMMUNITY_Community 19|Community 19]]
- [[_COMMUNITY_Community 20|Community 20]]
- [[_COMMUNITY_Community 21|Community 21]]
- [[_COMMUNITY_Community 22|Community 22]]
- [[_COMMUNITY_Community 23|Community 23]]
- [[_COMMUNITY_Community 24|Community 24]]
- [[_COMMUNITY_Community 25|Community 25]]
- [[_COMMUNITY_Community 26|Community 26]]
- [[_COMMUNITY_Community 27|Community 27]]
- [[_COMMUNITY_Community 28|Community 28]]
- [[_COMMUNITY_Community 29|Community 29]]
- [[_COMMUNITY_Community 30|Community 30]]
- [[_COMMUNITY_Community 31|Community 31]]
- [[_COMMUNITY_Community 32|Community 32]]
- [[_COMMUNITY_Community 33|Community 33]]
- [[_COMMUNITY_Community 34|Community 34]]
- [[_COMMUNITY_Community 35|Community 35]]
- [[_COMMUNITY_Community 36|Community 36]]
- [[_COMMUNITY_Community 37|Community 37]]
- [[_COMMUNITY_Community 38|Community 38]]
- [[_COMMUNITY_Community 40|Community 40]]
- [[_COMMUNITY_Community 41|Community 41]]
- [[_COMMUNITY_Community 42|Community 42]]
- [[_COMMUNITY_Community 43|Community 43]]
- [[_COMMUNITY_Community 44|Community 44]]
- [[_COMMUNITY_Community 45|Community 45]]
- [[_COMMUNITY_Community 46|Community 46]]
- [[_COMMUNITY_Community 47|Community 47]]
- [[_COMMUNITY_Community 48|Community 48]]
- [[_COMMUNITY_Community 49|Community 49]]
- [[_COMMUNITY_Community 50|Community 50]]
- [[_COMMUNITY_Community 51|Community 51]]
- [[_COMMUNITY_Community 52|Community 52]]
- [[_COMMUNITY_Community 53|Community 53]]
- [[_COMMUNITY_Community 54|Community 54]]

## God Nodes (most connected - your core abstractions)
1. `Orchestrator` - 92 edges
2. `main()` - 25 edges
3. `Context` - 23 edges
4. `ScalerBundle` - 19 edges
5. `StrategyTracker` - 17 edges
6. `SignalAggregator` - 15 edges
7. `NewOrchestrator()` - 15 edges
8. `Candle` - 12 edges
9. `Candle` - 12 edges
10. `StrategyBudgetEngine` - 11 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `NewBinanceKlineClient()`  [INFERRED]
  engine/cmd/antigravity/main.go → engine/internal/marketdata/binance_klines.go
- `main()` --calls--> `NewStrategyHealthMonitor()`  [INFERRED]
  engine/cmd/antigravity/main.go → engine/internal/paperpersist/strategy_health_monitor.go
- `main()` --calls--> `NewStrategyGate()`  [INFERRED]
  engine/cmd/antigravity/main.go → engine/internal/regime/strategy_gate.go
- `main()` --calls--> `NewStrategyTracker()`  [INFERRED]
  engine/cmd/antigravity/main.go → engine/internal/risk/strategy_tracker.go
- `main()` --calls--> `NormalizeCategory()`  [INFERRED]
  engine/cmd/antigravity/main.go → engine/internal/strategy/types.go

## Import Cycles
- None detected.

## Communities (56 total, 5 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.06
Nodes (34): AdaptiveConfidenceFloor, Bridge, CandleAggregator, CandleSummary, Candle, Mutex, RegimeClassification, RegistryEntry (+26 more)

### Community 1 - "Community 1"
Cohesion: 0.33
Nodes (11): Candle, BollingerBands, ADX(), ATR(), BB(), BBWidthPercentile(), EMA(), EMASlice() (+3 more)

### Community 2 - "Community 2"
Cohesion: 0.10
Nodes (23): Candle, Context, MarketContext, Mutex, Regime, RegistryEntry, Signal, Tick (+15 more)

### Community 3 - "Community 3"
Cohesion: 0.10
Nodes (31): fetchBinanceBTCSpot(), fetchDeltaBTCSpotForOptions(), formatHealthTime(), getEnvOrDefault(), handleDeltaBTCProbe(), keepAlive(), loadDotEnv(), loadOptionsSellingSnapshot() (+23 more)

### Community 4 - "Community 4"
Cohesion: 0.11
Nodes (22): MarketContext, Regime, IndicatorSnapshot, RollingVWAP(), BridgeDecision, InstitutionalPathOpts, adjustConfidenceByExecutionWeight(), averageFloat() (+14 more)

### Community 5 - "Community 5"
Cohesion: 0.11
Nodes (21): Signal, Strategy, Tick, Time, Action, noopStrategy, StrategyFamily, CategoryTier() (+13 more)

### Community 6 - "Community 6"
Cohesion: 0.16
Nodes (4): CloseEvent, Context, Tick, Snapshot

### Community 7 - "Community 7"
Cohesion: 0.21
Nodes (13): AlphaType, Signal, EnrichedSignal, FeatureSnapshot, MicrostructureStrategy, EnrichSignal(), FilterCandidate(), NewStrategy() (+5 more)

### Community 8 - "Community 8"
Cohesion: 0.12
Nodes (11): Duration, Mutex, Signal, Time, SignalAggregator, SignalFlowDiagnostics, SignalFlowMetrics, SignalFlowSnapshot (+3 more)

### Community 9 - "Community 9"
Cohesion: 0.19
Nodes (10): Event, Signal, EventType, FillResult, OrderMode, OrderPayload, Position, brokerFillFunc (+2 more)

### Community 10 - "Community 10"
Cohesion: 0.17
Nodes (10): atomicBool, Candle, Context, atomicBool, NewBinanceKlineClient(), parseF(), BinanceKlineClient, binanceKlineEvent (+2 more)

### Community 11 - "Community 11"
Cohesion: 0.13
Nodes (5): RWMutex, NewStrategyTracker(), StrategyStats, StrategyTracker, StrategyMetrics

### Community 12 - "Community 12"
Cohesion: 0.20
Nodes (12): RWMutex, Time, StrategyStatus, TradeResult, NewWalkForwardValidator(), wfAvg(), wfComputeSharpe(), wfComputeWinRate() (+4 more)

### Community 13 - "Community 13"
Cohesion: 0.19
Nodes (10): Context, RWMutex, Time, ledgerStore, BudgetViolation, ledgerStore, NewStrategyBudgetEngine(), StrategyBudget (+2 more)

### Community 14 - "Community 14"
Cohesion: 0.19
Nodes (11): RegistryEntry, Performance, RegistryEntry, RegistryEntry, buildExpansionPack(), AllPerformance(), BuildCuratedScalpers(), FilterWinnersOnly() (+3 more)

### Community 15 - "Community 15"
Cohesion: 0.33
Nodes (11): ClosedTrade, clamp(), Compute(), dataQualityMult(), effectiveMax(), GetKellyInputs(), max64(), min64() (+3 more)

### Community 16 - "Community 16"
Cohesion: 0.07
Nodes (24): MarketContext, Regime, Signal, MarketContext, Regime, Signal, MarketContext, Regime (+16 more)

### Community 17 - "Community 17"
Cohesion: 0.29
Nodes (5): Classifier, RegimeClassification, NewStrategyGate(), StrategyGate, StrategyRegistry

### Community 18 - "Community 18"
Cohesion: 0.27
Nodes (9): Context, Signal, Strategy, Tick, EvaluationMode, EvaluationResult, ScheduledStrategy, NewStrategyScheduler() (+1 more)

### Community 19 - "Community 19"
Cohesion: 0.24
Nodes (7): RegistryEntry, Signal, Strategy, BuildCuratedScalpers(), FilterWinnersOnly(), Tick, scalerAdapter

### Community 20 - "Community 20"
Cohesion: 0.42
Nodes (6): Context, Duration, MongoManager, NewStrategyHealthMonitor(), StrategyDataSource, StrategyHealthMonitor

### Community 21 - "Community 21"
Cohesion: 0.36
Nodes (5): Signal, Tick, NewMovingAverageCrossover(), sma(), MovingAverageCrossover

### Community 22 - "Community 22"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, BollingerMeanReversion

### Community 23 - "Community 23"
Cohesion: 0.07
Nodes (28): 1. Application Identity, 2.1 Folder Structure, 2.2 Engine Modules (All Built), 2.3 Frontend Dashboard (All Built), 2.4 Signals & Indicators Computed, 2.5 AI Decision Layer, 2.6 Paper Trading Logic, 2. Current Architecture (What Exists and Works) (+20 more)

### Community 24 - "Community 24"
Cohesion: 0.39
Nodes (4): Event, NewStrategyAggregate(), StrategyAggregate, StrategyState

### Community 25 - "Community 25"
Cohesion: 0.20
Nodes (16): BollingerBands, ADX(), ATR(), AvgVolume(), BB(), BBWidthPercentile(), EMA(), EMASlice() (+8 more)

### Community 26 - "Community 26"
Cohesion: 0.25
Nodes (5): MarketContext, Regime, Signal, VWAP(), VWAPInstitutionalFade

### Community 27 - "Community 27"
Cohesion: 0.21
Nodes (7): Candle, MarketContext, Regime, Signal, Time, AvgVolume(), OpeningRangeBreakout

### Community 28 - "Community 28"
Cohesion: 0.25
Nodes (5): MarketContext, Regime, Signal, RSI(), OIDivergence

### Community 29 - "Community 29"
Cohesion: 0.29
Nodes (4): RWMutex, Time, NewAdaptiveConfidenceFloor(), AdaptiveConfidenceFloor

### Community 30 - "Community 30"
Cohesion: 0.57
Nodes (4): RankedStrategy, NewStrategyRanker(), StrategyRanker, StrategyLiveMetrics

### Community 32 - "Community 32"
Cohesion: 0.50
Nodes (3): AggregatedSignal, SignalAggregator, strategyPriority()

### Community 33 - "Community 33"
Cohesion: 0.40
Nodes (4): ISPAPCatalogEntry, StrategyMetrics, StrategyStatus, StrategyWithMetrics

### Community 34 - "Community 34"
Cohesion: 0.67
Nodes (4): Time, Session, DetectSession(), SessionKellyMultiplier()

### Community 35 - "Community 35"
Cohesion: 0.33
Nodes (3): Time, BridgeStatus, PendingSignal

### Community 40 - "Community 40"
Cohesion: 0.30
Nodes (11): Candle, Direction, MarketContext, OrderBookSnapshot, Performance, Regime, RegistryEntry, Signal (+3 more)

### Community 41 - "Community 41"
Cohesion: 0.22
Nodes (6): MarketContext, Regime, Signal, CVDDivergesBearish(), CVDDivergesBullish(), LiquiditySweepReversal

### Community 42 - "Community 42"
Cohesion: 0.22
Nodes (6): MarketContext, Regime, Signal, ADXMomentumBreakout, SwingHigh(), SwingLow()

### Community 43 - "Community 43"
Cohesion: 0.29
Nodes (4): ADXMomentumBreakout, MarketContext, Regime, Signal

### Community 44 - "Community 44"
Cohesion: 0.29
Nodes (4): BollingerMeanReversion, MarketContext, Regime, Signal

### Community 45 - "Community 45"
Cohesion: 0.39
Nodes (7): AllPerformance(), GetPerformance(), Performance, RegistryEntry, BuildCuratedScalpers(), FilterWinnersOnly(), UpdatePerformance()

### Community 46 - "Community 46"
Cohesion: 0.29
Nodes (4): CVDDivergenceSniper, MarketContext, Regime, Signal

### Community 47 - "Community 47"
Cohesion: 0.29
Nodes (4): EMARibbonTrendRider, MarketContext, Regime, Signal

### Community 48 - "Community 48"
Cohesion: 0.29
Nodes (4): LiquiditySweepReversal, MarketContext, Regime, Signal

### Community 49 - "Community 49"
Cohesion: 0.29
Nodes (4): OpeningRangeBreakout, MarketContext, Regime, Signal

### Community 50 - "Community 50"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, VWAPInstitutionalFade

### Community 51 - "Community 51"
Cohesion: 0.29
Nodes (6): Directory Structure, File Format, File Summary, Notes, Purpose, Usage Guidelines

### Community 52 - "Community 52"
Cohesion: 0.50
Nodes (4): Action, Side, AgentSignalForExec, riskSideFromAction()

### Community 53 - "Community 53"
Cohesion: 0.83
Nodes (3): buildExpansionPack(), BuildFullRegistry(), RegistryEntry

## Knowledge Gaps
- **156 isolated node(s):** `Purpose`, `File Format`, `Usage Guidelines`, `Notes`, `Directory Structure` (+151 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **5 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `main()` connect `Community 3` to `Community 0`, `Community 5`, `Community 8`, `Community 10`, `Community 11`, `Community 17`, `Community 20`?**
  _High betweenness centrality (0.163) - this node is a cross-community bridge._
- **Why does `NewOrchestrator()` connect `Community 0` to `Community 2`, `Community 3`, `Community 4`, `Community 12`, `Community 29`?**
  _High betweenness centrality (0.155) - this node is a cross-community bridge._
- **Why does `Orchestrator` connect `Community 0` to `Community 35`, `Community 4`, `Community 36`, `Community 6`, `Community 9`, `Community 31`?**
  _High betweenness centrality (0.126) - this node is a cross-community bridge._
- **Are the 8 inferred relationships involving `main()` (e.g. with `NewBinanceKlineClient()` and `NewStrategyHealthMonitor()`) actually correct?**
  _`main()` has 8 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Purpose`, `File Format`, `Usage Guidelines` to the rest of the system?**
  _156 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.05551020408163265 - nodes in this community are weakly interconnected._
- **Should `Community 2` be split into smaller, more focused modules?**
  _Cohesion score 0.10128205128205128 - nodes in this community are weakly interconnected._