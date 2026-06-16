# Graph Report - antigravity-main  (2026-06-16)

## Corpus Check
- 4626 files · ~26,779,298 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 107 nodes · 244 edges · 10 communities (8 shown, 2 thin omitted)
- Extraction: 100% EXTRACTED · 0% INFERRED · 0% AMBIGUOUS · INFERRED: 1 edges (avg confidence: 0.8)
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
1. `evaluateMockTradeOpenRisk()` - 13 edges
2. `buildMockTradeFromTrace()` - 11 edges
3. `UseMockTradingEngineResult` - 8 edges
4. `applyPriceTickToTrade()` - 8 edges
5. `toPaperSide()` - 7 edges
6. `maxOpenMockTradesFromConfig()` - 7 edges
7. `buildMockTradeFromResearchSignal()` - 7 edges
8. `markMockTradeAtPrice()` - 7 edges
9. `normalizeMockTradingConfig()` - 7 edges
10. `computeMockFundingCost()` - 6 edges

## Surprising Connections (you probably didn't know these)
- `useMockTradingEngine()` --calls--> `normalizeMockTradingConfig()`  [INFERRED]
  client/src/hooks/useMockTradingEngine.ts → client/src/lib/trading/mockTradingEngine.ts
- `trimTradeCache()` --calls--> `maxOpenMockTradesFromConfig()`  [EXTRACTED]
  client/src/hooks/useMockTradingEngine.ts → client/src/lib/trading/mockTradingEngine.ts
- `UseMockTradingEngineOptions` --references--> `MockTradingConfig`  [EXTRACTED]
  client/src/hooks/useMockTradingEngine.ts → client/src/lib/trading/mockTradingEngine.ts
- `UseMockTradingEngineResult` --references--> `MockTradingConfig`  [EXTRACTED]
  client/src/hooks/useMockTradingEngine.ts → client/src/lib/trading/mockTradingEngine.ts
- `build()` --calls--> `buildMockTradeFromTrace()`  [EXTRACTED]
  client/src/lib/mockTradingEngine.test.ts → client/src/lib/trading/mockTradingEngine.ts

## Import Cycles
- None detected.

## Communities (10 total, 2 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.10
Nodes (22): candidateFamilyKey(), finiteNonNegative(), isExecutableTraceRow(), isStrategySignalRaised(), MOCK_REJECTION_CATEGORIES, MockBlockerAggregate, MockExitReason, MockFamilyAggregate (+14 more)

### Community 1 - "Community 1"
Cohesion: 0.19
Nodes (19): applyPriceTickToTrade(), blockersFromTraceRow(), buildMockTradeFromResearchSignal(), buildMockTradeFromTrace(), closeMockTrade(), computeMockExitLevels(), computeMockFundingCost(), computeMockNetPnlAtExitMark() (+11 more)

### Community 2 - "Community 2"
Cohesion: 0.16
Nodes (15): baseConfig, build(), traceRow(), tradeWithPnl(), wideExitConfig, computeAccountState(), computeAnalytics(), DEFAULT_MOCK_TRADING_CONFIG (+7 more)

### Community 3 - "Community 3"
Cohesion: 0.14
Nodes (10): STRATEGY_EXIT_OVERRIDES, logForMockTradeClosed(), logForMockTradeCreated(), MockDiagnosticFunnel, MockRejectionCategory, MockRejectionCounts, MockRejectionEvent, scoreMockResearchSignal() (+2 more)

### Community 4 - "Community 4"
Cohesion: 0.18
Nodes (14): trimTradeCache(), useMockTradingEngine(), canOpenAdditionalMockTrade(), countOpenMockTrades(), countOpenMockTradesBySide(), evaluateMockTradeOpenRisk(), isGradeDiscoveryStage(), maxOpenLongTradesFromConfig() (+6 more)

### Community 5 - "Community 5"
Cohesion: 0.29
Nodes (7): UseMockTradingEngineResult, MockAccountState, MockResearchSignalInput, MockTrade, MockTradeAnalytics, MockTradeLog, MockTradingDiagnostics

### Community 6 - "Community 6"
Cohesion: 0.67
Nodes (4): diagnosticsDelta(), emptyMockDiagnosticFunnel(), emptyMockRejectionCounts(), emptyMockTradingDiagnostics()

### Community 9 - "Community 9"
Cohesion: 0.60
Nodes (4): TestKillSwitchHook_SkipsBalanceEquityDrift(), TestKillSwitchHook_SkipsBalanceEquityDriftOnFullAudit(), TestKillSwitchHook_TriggersOnCriticalPositionDrift(), T

## Knowledge Gaps
- **18 isolated node(s):** `STRATEGY_EXIT_OVERRIDES`, `baseConfig`, `wideExitConfig`, `MockTradeStatus`, `MockSide` (+13 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **2 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `buildMockTradeFromTrace()` connect `Community 1` to `Community 0`, `Community 2`, `Community 3`?**
  _High betweenness centrality (0.015) - this node is a cross-community bridge._
- **Why does `evaluateMockTradeOpenRisk()` connect `Community 4` to `Community 0`, `Community 2`, `Community 3`?**
  _High betweenness centrality (0.012) - this node is a cross-community bridge._
- **Why does `maxOpenMockTradesFromConfig()` connect `Community 4` to `Community 0`, `Community 2`, `Community 3`?**
  _High betweenness centrality (0.007) - this node is a cross-community bridge._
- **What connects `STRATEGY_EXIT_OVERRIDES`, `baseConfig`, `wideExitConfig` to the rest of the system?**
  _18 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.09782608695652174 - nodes in this community are weakly interconnected._
- **Should `Community 3` be split into smaller, more focused modules?**
  _Cohesion score 0.14285714285714285 - nodes in this community are weakly interconnected._