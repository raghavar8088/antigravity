# Graph Report - .  (2026-06-25)

## Corpus Check
- cluster-only mode — file stats not available

## Summary
- 107 nodes · 203 edges · 8 communities
- Extraction: 100% EXTRACTED · 0% INFERRED · 0% AMBIGUOUS
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `c46db18b`
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

## God Nodes (most connected - your core abstractions)
1. `evaluateMockTradeOpenRisk()` - 13 edges
2. `buildMockTradeFromTrace()` - 10 edges
3. `toPaperSide()` - 7 edges
4. `applyPriceTickToTrade()` - 7 edges
5. `computeMockFundingCost()` - 6 edges
6. `buildMockTradeFromResearchSignal()` - 6 edges
7. `computeMockPnl()` - 6 edges
8. `withMockExitFields()` - 6 edges
9. `markMockTradeAtPrice()` - 6 edges
10. `finalizeClose()` - 6 edges

## Surprising Connections (you probably didn't know these)
- `build()` --calls--> `buildMockTradeFromTrace()`  [EXTRACTED]
  D:/APPLICATION/antigravity-main/antigravity-main/client/src/lib/mockTradingEngine.test.ts → D:/APPLICATION/antigravity-main/antigravity-main/client/src/lib/trading/mockTradingEngine.ts

## Import Cycles
- None detected.

## Communities (8 total, 0 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.06
Nodes (28): DailyPnlResult, DailyPnlRow, DailyPnlSummary, MOCK_REJECTION_CATEGORIES, MockAccountState, MockBlockerAggregate, MockDiagnosticFunnel, MockExitReason (+20 more)

### Community 1 - "Community 1"
Cohesion: 0.16
Nodes (21): applyPriceTickToTrade(), blockersFromTraceRow(), buildMockTradeFromResearchSignal(), buildMockTradeFromTrace(), closeMockTrade(), computeMockExitLevels(), computeMockFundingCost(), computeMockNetPnlAtExitMark() (+13 more)

### Community 2 - "Community 2"
Cohesion: 0.14
Nodes (17): baseConfig, build(), traceRow(), tradeWithPnl(), wideExitConfig, computeAccountState(), computeAnalytics(), filterMockTrades() (+9 more)

### Community 3 - "Community 3"
Cohesion: 0.17
Nodes (15): candidateFamilyKey(), canOpenAdditionalMockTrade(), countOpenMockTrades(), countOpenMockTradesBySide(), evaluateMockTradeOpenRisk(), isGradeDiscoveryStage(), maxOpenLongTradesFromConfig(), maxOpenMockTradesFromConfig() (+7 more)

### Community 4 - "Community 4"
Cohesion: 0.25
Nodes (6): getMockConfigForPipelineStage(), mockConfigForStage(), STAGE_ACCOUNT_KEY, TRADE_ENGINE_CONFIG, DEFAULT_MOCK_TRADING_CONFIG, MockTradingConfig

### Community 5 - "Community 5"
Cohesion: 0.50
Nodes (4): computeDailyPnl(), istDateKey(), round2(), round3()

### Community 6 - "Community 6"
Cohesion: 0.67
Nodes (3): emptyMockDiagnosticFunnel(), emptyMockRejectionCounts(), emptyMockTradingDiagnostics()

### Community 7 - "Community 7"
Cohesion: 0.67
Nodes (3): finiteNonNegative(), mockTradeAgeMinutes(), passesMockTradeAgeFilter()

## Knowledge Gaps
- **32 isolated node(s):** `baseConfig`, `wideExitConfig`, `TRADE_ENGINE_CONFIG`, `STAGE_ACCOUNT_KEY`, `MockTradeStatus` (+27 more)
  These have ≤1 connection - possible missing edges or undocumented components.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `buildMockTradeFromTrace()` connect `Community 1` to `Community 0`, `Community 2`?**
  _High betweenness centrality (0.010) - this node is a cross-community bridge._
- **Why does `evaluateMockTradeOpenRisk()` connect `Community 3` to `Community 0`, `Community 2`?**
  _High betweenness centrality (0.008) - this node is a cross-community bridge._
- **Why does `MockTradingConfig` connect `Community 4` to `Community 0`, `Community 2`?**
  _High betweenness centrality (0.002) - this node is a cross-community bridge._
- **What connects `baseConfig`, `wideExitConfig`, `TRADE_ENGINE_CONFIG` to the rest of the system?**
  _32 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.058823529411764705 - nodes in this community are weakly interconnected._
- **Should `Community 2` be split into smaller, more focused modules?**
  _Cohesion score 0.13725490196078433 - nodes in this community are weakly interconnected._