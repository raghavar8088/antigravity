# Graph Report - antigravity-main  (2026-06-16)

## Corpus Check
- 4626 files · ~26,780,645 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 140 nodes · 233 edges · 10 communities (9 shown, 1 thin omitted)
- Extraction: 98% EXTRACTED · 2% INFERRED · 0% AMBIGUOUS · INFERRED: 5 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `64b4fee5`
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

## God Nodes (most connected - your core abstractions)
1. `ScalerBundle` - 21 edges
2. `main()` - 17 edges
3. `Candle` - 10 edges
4. `Context` - 7 edges
5. `NoSignal()` - 7 edges
6. `mdToScalerCandle()` - 7 edges
7. `appendCapped()` - 7 edges
8. `Time` - 6 edges
9. `saveOptionsSnapshot()` - 5 edges
10. `saveOptionsSellingSnapshot()` - 5 edges

## Surprising Connections (you probably didn't know these)
- `ScalerBundle` --references--> `Time`  [EXTRACTED]
  engine/internal/trading/scalers_eval.go → engine/internal/trading/scalers_eval.go  _Bridges community 2 → community 1_

## Import Cycles
- None detected.

## Communities (10 total, 1 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.11
Nodes (29): fetchBinanceBTCSpot(), fetchDeltaBTCSpotForOptions(), formatHealthTime(), getEnvOrDefault(), handleDeltaBTCProbe(), keepAlive(), loadDotEnv(), loadOptionsSellingSnapshot() (+21 more)

### Community 1 - "Community 1"
Cohesion: 0.11
Nodes (18): AggregatedSignal, Context, MarketContext, Regime, Signal, Time, Tick, Orchestrator (+10 more)

### Community 2 - "Community 2"
Cohesion: 0.26
Nodes (11): Candle, Mutex, RegistryEntry, OrderBookSnapshot, ScalersSignalSnapshot, ScalerBundle, aggregate4h(), aggregate5mBars() (+3 more)

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
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, LiquiditySweepReversal

### Community 8 - "Community 8"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, VWAPInstitutionalFade

## Knowledge Gaps
- **31 isolated node(s):** `Mutex`, `OptionsBuyPaperPersistence`, `OptionsState`, `OptionsSellPaperPersistence`, `OptionsSellingState` (+26 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **1 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `NoSignal()` connect `Community 3` to `Community 4`, `Community 5`, `Community 6`, `Community 7`, `Community 8`?**
  _High betweenness centrality (0.131) - this node is a cross-community bridge._
- **Why does `ScalerBundle` connect `Community 2` to `Community 1`?**
  _High betweenness centrality (0.051) - this node is a cross-community bridge._
- **Are the 5 inferred relationships involving `NoSignal()` (e.g. with `.Evaluate()` and `.Evaluate()`) actually correct?**
  _`NoSignal()` has 5 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Mutex`, `OptionsBuyPaperPersistence`, `OptionsState` to the rest of the system?**
  _31 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.11174242424242424 - nodes in this community are weakly interconnected._
- **Should `Community 1` be split into smaller, more focused modules?**
  _Cohesion score 0.1053763440860215 - nodes in this community are weakly interconnected._