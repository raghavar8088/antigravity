# Graph Report - antigravity-main  (2026-06-16)

## Corpus Check
- 4626 files · ~26,782,935 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 221 nodes · 394 edges · 14 communities
- Extraction: 93% EXTRACTED · 7% INFERRED · 0% AMBIGUOUS · INFERRED: 29 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `9600d98c`
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

## God Nodes (most connected - your core abstractions)
1. `ScalerBundle` - 27 edges
2. `main()` - 17 edges
3. `Candle` - 12 edges
4. `NoSignal()` - 10 edges
5. `Candle` - 10 edges
6. `recoverOpenPositions()` - 9 edges
7. `recoverAccountState()` - 8 edges
8. `ATR()` - 8 edges
9. `BootstrapJournalFromMongo()` - 7 edges
10. `RecoveryReport` - 7 edges

## Surprising Connections (you probably didn't know these)
- `BootstrapJournalFromMongo()` --calls--> `getFloat()`  [INFERRED]
  engine/internal/paperpersist/journal_bootstrap.go → engine/internal/paperpersist/recovery.go
- `BootstrapJournalFromMongo()` --calls--> `getString()`  [INFERRED]
  engine/internal/paperpersist/journal_bootstrap.go → engine/internal/paperpersist/recovery.go
- `BootstrapJournalFromMongo()` --calls--> `getTime()`  [INFERRED]
  engine/internal/paperpersist/journal_bootstrap.go → engine/internal/paperpersist/recovery.go
- `scalerStrategyCooldown()` --references--> `Duration`  [EXTRACTED]
  engine/internal/trading/scalers_eval.go → engine/internal/paperpersist/recovery.go

## Import Cycles
- None detected.

## Communities (14 total, 0 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.10
Nodes (31): fetchBinanceBTCSpot(), fetchDeltaBTCSpotForOptions(), formatHealthTime(), getEnvOrDefault(), handleDeltaBTCProbe(), keepAlive(), loadDotEnv(), loadOptionsSellingSnapshot() (+23 more)

### Community 1 - "Community 1"
Cohesion: 0.17
Nodes (5): Context, Signal, Tick, sanitizeScalerSignal(), scalerSignalToLegacy()

### Community 2 - "Community 2"
Cohesion: 0.49
Nodes (5): Candle, aggregate4h(), aggregate5mBars(), appendCapped(), mdToScalerCandle()

### Community 3 - "Community 3"
Cohesion: 0.27
Nodes (11): Time, Candle, Direction, MarketContext, OrderBookSnapshot, Performance, Regime, RegistryEntry (+3 more)

### Community 4 - "Community 4"
Cohesion: 0.25
Nodes (7): Candle, MarketContext, Regime, Signal, CVDDivergenceSniper, rollingCVDBearishDiv(), rollingCVDBullishDiv()

### Community 5 - "Community 5"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, EMARibbonTrendRider

### Community 6 - "Community 6"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, BollingerMeanReversion

### Community 7 - "Community 7"
Cohesion: 0.27
Nodes (7): MarketContext, Regime, Signal, SwingLow(), LiquiditySweepReversal, sweepCVDBearishDiv(), sweepCVDBullishDiv()

### Community 8 - "Community 8"
Cohesion: 0.07
Nodes (28): Candle, MarketContext, Regime, Signal, MarketContext, Regime, Signal, MarketContext (+20 more)

### Community 9 - "Community 9"
Cohesion: 0.21
Nodes (9): AggregatedSignal, Duration, containsAny(), newScalerBundle(), scalersSignalSnapshots(), scalerStrategyCooldown(), ScalersSignalSnapshot, ScalersSnapshot (+1 more)

### Community 10 - "Community 10"
Cohesion: 0.19
Nodes (9): MarketContext, Time, Mutex, OrderBookSnapshot, RegistryEntry, RWMutex, ScalersSignalSnapshot, ScalerBundle (+1 more)

### Community 11 - "Community 11"
Cohesion: 0.21
Nodes (7): Candle, MarketContext, Regime, Signal, Time, AvgVolume(), OpeningRangeBreakout

### Community 12 - "Community 12"
Cohesion: 0.22
Nodes (20): Context, MongoManager, Context, MongoManager, M, BootstrapJournalFromMongo(), RecoveredAccountState, RecoveredPosition (+12 more)

### Community 13 - "Community 13"
Cohesion: 0.29
Nodes (5): Regime, Orchestrator, mapRegimeClassToScalersRegime(), mapToScalersRegime(), StrategyPerformance

## Knowledge Gaps
- **44 isolated node(s):** `Context`, `MongoManager`, `TradeJournal`, `Regime`, `Signal` (+39 more)
  These have ≤1 connection - possible missing edges or undocumented components.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Duration` connect `Community 9` to `Community 11`, `Community 12`?**
  _High betweenness centrality (0.394) - this node is a cross-community bridge._
- **Why does `NoSignal()` connect `Community 3` to `Community 4`, `Community 5`, `Community 6`, `Community 7`, `Community 8`, `Community 11`?**
  _High betweenness centrality (0.287) - this node is a cross-community bridge._
- **Why does `RecoveryReport` connect `Community 12` to `Community 9`?**
  _High betweenness centrality (0.142) - this node is a cross-community bridge._
- **Are the 8 inferred relationships involving `NoSignal()` (e.g. with `.Evaluate()` and `.Evaluate()`) actually correct?**
  _`NoSignal()` has 8 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Context`, `MongoManager`, `TradeJournal` to the rest of the system?**
  _44 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.09841269841269841 - nodes in this community are weakly interconnected._
- **Should `Community 8` be split into smaller, more focused modules?**
  _Cohesion score 0.07308970099667775 - nodes in this community are weakly interconnected._