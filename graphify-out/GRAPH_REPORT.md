# Graph Report - .  (2026-07-18)

## Corpus Check
- cluster-only mode — file stats not available

## Summary
- 102 nodes · 175 edges · 15 communities (6 shown, 9 thin omitted)
- Extraction: 100% EXTRACTED · 0% INFERRED · 0% AMBIGUOUS
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `27f60e3a`
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

## God Nodes (most connected - your core abstractions)
1. `ScalerBundle` - 40 edges
2. `Candle` - 10 edges
3. `main()` - 10 edges
4. `mdToScalerCandle()` - 7 edges
5. `appendCapped()` - 7 edges
6. `Time` - 6 edges
7. `regimePnLTracker` - 6 edges
8. `Orchestrator` - 5 edges
9. `ScalersSnapshot` - 5 edges
10. `handle()` - 4 edges

## Surprising Connections (you probably didn't know these)
- `regimePnLTracker` --references--> `Mutex`  [EXTRACTED]
  D:/INDIAN MARKET/antigravity-main/antigravity-main/engine/cmd/pre_live/main.go → D:/INDIAN MARKET/antigravity-main/antigravity-main/engine/internal/trading/scalers_eval.go
- `strategyNamesAdapter` --references--> `RegistryEntry`  [EXTRACTED]
  D:/INDIAN MARKET/antigravity-main/antigravity-main/engine/cmd/pre_live/main.go → D:/INDIAN MARKET/antigravity-main/antigravity-main/engine/internal/trading/scalers_eval.go
- `strategyNamesAdapter` --references--> `RWMutex`  [EXTRACTED]
  D:/INDIAN MARKET/antigravity-main/antigravity-main/engine/cmd/pre_live/main.go → D:/INDIAN MARKET/antigravity-main/antigravity-main/engine/internal/trading/scalers_eval.go

## Import Cycles
- None detected.

## Communities (15 total, 9 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.20
Nodes (16): CloseReason, Mutex, Orchestrator, applyEnvFile(), loadApplicationProperties(), loadDotEnv(), main(), newRegimePnLTracker() (+8 more)

### Community 1 - "Community 1"
Cohesion: 0.14
Nodes (14): AggregatedSignal, strategyNamesAdapter, RegistryEntry, RWMutex, Signal, bundleStrategies(), fixedTradeSizeBTC(), newScalerBundle() (+6 more)

### Community 2 - "Community 2"
Cohesion: 0.16
Nodes (7): MarketContext, Regime, Time, Orchestrator, mapRegimeClassToScalersRegime(), tradingSession(), StrategyPerformance

### Community 3 - "Community 3"
Cohesion: 0.14
Nodes (3): MetaLabelFilter, OrderBookSnapshot, ScalerBundle

### Community 4 - "Community 4"
Cohesion: 0.49
Nodes (5): Candle, aggregate4h(), aggregate5mBars(), appendCapped(), mdToScalerCandle()

### Community 6 - "Community 6"
Cohesion: 0.43
Nodes (6): ALLOWED_PATHS, handle(), isAllowed(), RouteCtx, upstreamBase(), verifySessionToken()

## Knowledge Gaps
- **15 isolated node(s):** `ALLOWED_PATHS`, `RouteCtx`, `GET`, `POST`, `PUT` (+10 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **9 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `ScalerBundle` connect `Community 3` to `Community 0`, `Community 1`, `Community 2`, `Community 4`, `Community 5`, `Community 7`, `Community 8`, `Community 9`?**
  _High betweenness centrality (0.565) - this node is a cross-community bridge._
- **Why does `Mutex` connect `Community 0` to `Community 3`?**
  _High betweenness centrality (0.136) - this node is a cross-community bridge._
- **What connects `ALLOWED_PATHS`, `RouteCtx`, `GET` to the rest of the system?**
  _15 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 1` be split into smaller, more focused modules?**
  _Cohesion score 0.1437908496732026 - nodes in this community are weakly interconnected._
- **Should `Community 3` be split into smaller, more focused modules?**
  _Cohesion score 0.14285714285714285 - nodes in this community are weakly interconnected._