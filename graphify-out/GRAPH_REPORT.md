# Graph Report - .  (2026-06-16)

## Corpus Check
- cluster-only mode — file stats not available

## Summary
- 137 nodes · 233 edges · 10 communities (9 shown, 1 thin omitted)
- Extraction: 98% EXTRACTED · 2% INFERRED · 0% AMBIGUOUS · INFERRED: 5 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `f6559065`
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
4. `Context` - 8 edges
5. `NoSignal()` - 7 edges
6. `mdToScalerCandle()` - 7 edges
7. `appendCapped()` - 7 edges
8. `Time` - 6 edges
9. `MarketContext` - 5 edges
10. `ScalersSnapshot` - 5 edges

## Surprising Connections (you probably didn't know these)
- `RingLogger` --references--> `Mutex`  [EXTRACTED]
  D:/APPLICATION/antigravity-main/antigravity-main/engine/cmd/antigravity/main.go → D:/APPLICATION/antigravity-main/antigravity-main/engine/internal/trading/scalers_eval.go
- `strategyNamesAdapter` --references--> `RegistryEntry`  [EXTRACTED]
  D:/APPLICATION/antigravity-main/antigravity-main/engine/cmd/antigravity/main.go → D:/APPLICATION/antigravity-main/antigravity-main/engine/internal/trading/scalers_eval.go
- `fetchBinanceBTCSpot()` --references--> `Context`  [EXTRACTED]
  D:/APPLICATION/antigravity-main/antigravity-main/engine/cmd/antigravity/main.go → D:/APPLICATION/antigravity-main/antigravity-main/engine/internal/trading/scalers_eval.go
- `fetchDeltaBTCSpotForOptions()` --references--> `Context`  [EXTRACTED]
  D:/APPLICATION/antigravity-main/antigravity-main/engine/cmd/antigravity/main.go → D:/APPLICATION/antigravity-main/antigravity-main/engine/internal/trading/scalers_eval.go
- `handleDeltaBTCProbe()` --calls--> `Context`  [EXTRACTED]
  D:/APPLICATION/antigravity-main/antigravity-main/engine/cmd/antigravity/main.go → D:/APPLICATION/antigravity-main/antigravity-main/engine/internal/trading/scalers_eval.go

## Import Cycles
- None detected.

## Communities (10 total, 1 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.11
Nodes (18): AggregatedSignal, MarketContext, Regime, Signal, Time, Tick, Orchestrator, mapRegimeClassToScalersRegime() (+10 more)

### Community 1 - "Community 1"
Cohesion: 0.13
Nodes (27): fetchBinanceBTCSpot(), fetchDeltaBTCSpotForOptions(), formatHealthTime(), getEnvOrDefault(), handleDeltaBTCProbe(), keepAlive(), loadDotEnv(), loadOptionsSellingSnapshot() (+19 more)

### Community 2 - "Community 2"
Cohesion: 0.26
Nodes (10): strategyNamesAdapter, Candle, OrderBookSnapshot, RegistryEntry, ScalersSignalSnapshot, ScalerBundle, aggregate4h(), aggregate5mBars() (+2 more)

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
- **26 isolated node(s):** `Regime`, `MarketContext`, `Signal`, `Regime`, `MarketContext` (+21 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **1 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `NoSignal()` connect `Community 3` to `Community 4`, `Community 5`, `Community 6`, `Community 7`, `Community 8`?**
  _High betweenness centrality (0.136) - this node is a cross-community bridge._
- **Why does `ScalerBundle` connect `Community 2` to `Community 0`, `Community 9`?**
  _High betweenness centrality (0.123) - this node is a cross-community bridge._
- **Why does `Context` connect `Community 1` to `Community 0`?**
  _High betweenness centrality (0.100) - this node is a cross-community bridge._
- **Are the 5 inferred relationships involving `NoSignal()` (e.g. with `.Evaluate()` and `.Evaluate()`) actually correct?**
  _`NoSignal()` has 5 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Regime`, `MarketContext`, `Signal` to the rest of the system?**
  _26 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.1053763440860215 - nodes in this community are weakly interconnected._
- **Should `Community 1` be split into smaller, more focused modules?**
  _Cohesion score 0.1330049261083744 - nodes in this community are weakly interconnected._