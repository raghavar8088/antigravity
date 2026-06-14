# Graph Report - antigravity-main  (2026-06-14)

## Corpus Check
- 4598 files · ~26,765,223 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 34 nodes · 57 edges · 5 communities (4 shown, 1 thin omitted)
- Extraction: 98% EXTRACTED · 2% INFERRED · 0% AMBIGUOUS · INFERRED: 1 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `2060799b`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 0|Community 0]]
- [[_COMMUNITY_Community 1|Community 1]]
- [[_COMMUNITY_Community 2|Community 2]]
- [[_COMMUNITY_Community 3|Community 3]]
- [[_COMMUNITY_Community 4|Community 4]]

## God Nodes (most connected - your core abstractions)
1. `metricsToScore()` - 9 edges
2. `computeStrategyPerformance()` - 7 edges
3. `clamp()` - 7 edges
4. `File Summary` - 5 edges
5. `StrategyScore` - 4 edges
6. `scoreStrategiesFromTrades()` - 4 edges
7. `StrategyPerformanceMetrics` - 3 edges
8. `scorePnl()` - 3 edges
9. `scoreProfitFactor()` - 3 edges
10. `scoreWinRate()` - 3 edges

## Surprising Connections (you probably didn't know these)
- `scoreStrategiesFromTrades()` --calls--> `computeStrategyPerformance()`  [INFERRED]
  client/src/lib/ai/strategyScoringEngine.ts → client/src/lib/ai/strategyPerformanceEngine.ts
- `UseStrategyScoringResult` --references--> `StrategyScore`  [EXTRACTED]
  client/src/hooks/useStrategyScoring.ts → client/src/lib/ai/strategyScoringEngine.ts
- `StrategyScore` --references--> `StrategyPerformanceMetrics`  [EXTRACTED]
  client/src/lib/ai/strategyScoringEngine.ts → client/src/lib/ai/strategyPerformanceEngine.ts

## Import Cycles
- None detected.

## Communities (5 total, 1 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.47
Nodes (9): clamp(), confidenceFromSample(), ConfidenceRating, metricsToScore(), scoreDrawdown(), scorePnl(), scoreProfitFactor(), scoreSharpe() (+1 more)

### Community 1 - "Community 1"
Cohesion: 0.33
Nodes (8): computeMaxDrawdown(), computeSharpe(), computeSortino(), computeStrategyPerformance(), EMPTY_REGIME_STATS, RegimeKey, RegimeStats, safeDiv()

### Community 2 - "Community 2"
Cohesion: 0.40
Nodes (4): StrategyPerformanceMetrics, StrategyScore, UseStrategyScoringArgs, UseStrategyScoringResult

### Community 4 - "Community 4"
Cohesion: 0.29
Nodes (6): Directory Structure, File Format, File Summary, Notes, Purpose, Usage Guidelines

## Knowledge Gaps
- **10 isolated node(s):** `Purpose`, `File Format`, `Usage Guidelines`, `Notes`, `Directory Structure` (+5 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **1 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `computeStrategyPerformance()` connect `Community 1` to `Community 0`, `Community 3`?**
  _High betweenness centrality (0.081) - this node is a cross-community bridge._
- **Why does `StrategyScore` connect `Community 2` to `Community 0`?**
  _High betweenness centrality (0.024) - this node is a cross-community bridge._
- **What connects `Purpose`, `File Format`, `Usage Guidelines` to the rest of the system?**
  _10 weakly-connected nodes found - possible documentation gaps or missing edges._