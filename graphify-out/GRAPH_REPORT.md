# Graph Report - antigravity-main  (2026-06-16)

## Corpus Check
- 4626 files · ~26,781,575 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 199 nodes · 343 edges · 14 communities
- Extraction: 92% EXTRACTED · 8% INFERRED · 0% AMBIGUOUS · INFERRED: 26 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `e705e5e1`
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
6. `ATR()` - 8 edges
7. `mdToScalerCandle()` - 7 edges
8. `appendCapped()` - 7 edges
9. `Context` - 7 edges
10. `MACD()` - 6 edges

## Surprising Connections (you probably didn't know these)
- `AvgVolume()` --references--> `Candle`  [EXTRACTED]
  engine/internal/strategy/scalpers/indicators.go → engine/internal/strategy/scalpers/indicators.go  _Bridges community 8 → community 11_
- `SwingHigh()` --references--> `Candle`  [EXTRACTED]
  engine/internal/strategy/scalpers/indicators.go → engine/internal/strategy/scalpers/indicators.go  _Bridges community 8 → community 7_
- `ScalerBundle` --references--> `Candle`  [EXTRACTED]
  engine/internal/trading/scalers_eval.go → engine/internal/trading/scalers_eval.go  _Bridges community 10 → community 2_
- `newScalerBundle()` --references--> `ScalerBundle`  [EXTRACTED]
  engine/internal/trading/scalers_eval.go → engine/internal/trading/scalers_eval.go  _Bridges community 10 → community 9_
- `StrategyPerformance` --references--> `Time`  [EXTRACTED]
  engine/internal/trading/scalers_eval.go → engine/internal/trading/scalers_eval.go  _Bridges community 10 → community 13_

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
Cohesion: 0.24
Nodes (8): MarketContext, Regime, Signal, SwingHigh(), SwingLow(), LiquiditySweepReversal, sweepCVDBearishDiv(), sweepCVDBullishDiv()

### Community 8 - "Community 8"
Cohesion: 0.10
Nodes (22): Candle, MarketContext, Regime, Signal, MarketContext, Regime, Signal, BollingerBands (+14 more)

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
Cohesion: 0.28
Nodes (5): MarketContext, Regime, Signal, FundingRateFade, abs64()

### Community 13 - "Community 13"
Cohesion: 0.29
Nodes (5): Regime, Orchestrator, mapRegimeClassToScalersRegime(), mapToScalersRegime(), StrategyPerformance

## Knowledge Gaps
- **41 isolated node(s):** `Regime`, `Signal`, `Regime`, `MarketContext`, `Signal` (+36 more)
  These have ≤1 connection - possible missing edges or undocumented components.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Duration` connect `Community 9` to `Community 11`?**
  _High betweenness centrality (0.304) - this node is a cross-community bridge._
- **Why does `NoSignal()` connect `Community 3` to `Community 4`, `Community 5`, `Community 6`, `Community 7`, `Community 8`, `Community 11`, `Community 12`?**
  _High betweenness centrality (0.300) - this node is a cross-community bridge._
- **Why does `ScalerBundle` connect `Community 10` to `Community 1`, `Community 2`, `Community 13`, `Community 9`?**
  _High betweenness centrality (0.151) - this node is a cross-community bridge._
- **Are the 8 inferred relationships involving `NoSignal()` (e.g. with `.Evaluate()` and `.Evaluate()`) actually correct?**
  _`NoSignal()` has 8 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Regime`, `Signal`, `Regime` to the rest of the system?**
  _41 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.09841269841269841 - nodes in this community are weakly interconnected._
- **Should `Community 8` be split into smaller, more focused modules?**
  _Cohesion score 0.09659090909090909 - nodes in this community are weakly interconnected._