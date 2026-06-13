# Graph Report - engine\cmd  (2026-06-14)

## Corpus Check
- cluster-only mode — file stats not available

## Summary
- 143 nodes · 305 edges · 12 communities (10 shown, 2 thin omitted)
- Extraction: 90% EXTRACTED · 10% INFERRED · 0% AMBIGUOUS · INFERRED: 29 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `62da936e`
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

## God Nodes (most connected - your core abstractions)
1. `main()` - 34 edges
2. `main()` - 17 edges
3. `StrategyMetrics` - 11 edges
4. `DataSources` - 10 edges
5. `truncate()` - 10 edges
6. `Context` - 9 edges
7. `ComputeMetrics()` - 9 edges
8. `StrategyMetrics` - 9 edges
9. `Trade` - 8 edges
10. `setCORS()` - 7 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `RunMonteCarlo()`  [INFERRED]
  antigravity/main.go → sep_evidence/analytics.go
- `main()` --calls--> `writeJSON()`  [INFERRED]
  sep_evidence/main.go → sep_evidence/reports.go
- `main()` --calls--> `ComputeMetrics()`  [INFERRED]
  sep_evidence/main.go → sep_evidence/analytics.go
- `classifyTier()` --calls--> `max()`  [INFERRED]
  sep_evidence/analytics.go → sep_evidence/reports.go
- `main()` --calls--> `RunMonteCarlo()`  [INFERRED]
  sep_evidence/main.go → sep_evidence/analytics.go

## Import Cycles
- None detected.

## Communities (12 total, 2 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.15
Nodes (26): AlignPnLSeries(), PearsonCorrelation(), MCResult, writeCapitalReadyPortfolio(), writeCorrelationReport(), computeAlphaScore(), MCResult, StrategyMetrics (+18 more)

### Community 1 - "Community 1"
Cohesion: 0.14
Nodes (20): Client, Database, DB, equityPoint, M, mongoScore, Connect(), Context (+12 more)

### Community 2 - "Community 2"
Cohesion: 0.23
Nodes (20): buildPortfolioPnL(), computeWalkForward(), generateMockTrades(), StrategyMetrics, Trade, WFResult, main(), statusRank() (+12 more)

### Community 3 - "Community 3"
Cohesion: 0.33
Nodes (14): classifyTier(), ComputeMetrics(), fillBasicMetrics(), fillDuration(), fillExecution(), fillRiskAdjusted(), Time, lcg() (+6 more)

### Community 4 - "Community 4"
Cohesion: 0.27
Nodes (12): fetchBinanceBTCSpot(), fetchDeltaBTCSpotForOptions(), formatHealthTime(), getEnvOrDefault(), Time, keepAlive(), loadDotEnv(), main() (+4 more)

### Community 5 - "Community 5"
Cohesion: 0.22
Nodes (9): loadOptionsSellingSnapshot(), loadOptionsSnapshot(), saveOptionsSellingSnapshot(), saveOptionsSnapshot(), OptionsBuyPaperPersistence, OptionsSellingState, OptionsSellPaperPersistence, OptionsState (+1 more)

### Community 6 - "Community 6"
Cohesion: 0.57
Nodes (8): Context, handleAngelOneNiftyProbe(), handleAngelOneProxy(), handleDeltaBTCProbe(), handleNiftyInjectCandles(), setCORS(), Request, ResponseWriter

### Community 7 - "Community 7"
Cohesion: 0.36
Nodes (3): benchStrategy, Tick, Signal

### Community 8 - "Community 8"
Cohesion: 0.50
Nodes (4): Pool, Context, main(), processCSV()

### Community 10 - "Community 10"
Cohesion: 0.67
Nodes (3): Tick, loadMockHistory(), main()

## Knowledge Gaps
- **20 isolated node(s):** `Mutex`, `OptionsBuyPaperPersistence`, `OptionsState`, `OptionsSellPaperPersistence`, `OptionsSellingState` (+15 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **2 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `main()` connect `Community 2` to `Community 0`, `Community 1`, `Community 3`?**
  _High betweenness centrality (0.533) - this node is a cross-community bridge._
- **Why does `RunMonteCarlo()` connect `Community 3` to `Community 2`, `Community 4`?**
  _High betweenness centrality (0.343) - this node is a cross-community bridge._
- **Why does `main()` connect `Community 4` to `Community 9`, `Community 3`, `Community 5`, `Community 6`?**
  _High betweenness centrality (0.331) - this node is a cross-community bridge._
- **Are the 15 inferred relationships involving `main()` (e.g. with `ComputeMetrics()` and `RunMonteCarlo()`) actually correct?**
  _`main()` has 15 INFERRED edges - model-reasoned connections that need verification._
- **Are the 3 inferred relationships involving `truncate()` (e.g. with `writeCapitalReadyPortfolio()` and `writeCorrelationReport()`) actually correct?**
  _`truncate()` has 3 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Mutex`, `OptionsBuyPaperPersistence`, `OptionsState` to the rest of the system?**
  _20 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 1` be split into smaller, more focused modules?**
  _Cohesion score 0.14461538461538462 - nodes in this community are weakly interconnected._