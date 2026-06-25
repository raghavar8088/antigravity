# Graph Report - .  (2026-06-25)

## Corpus Check
- cluster-only mode — file stats not available

## Summary
- 290 nodes · 662 edges · 10 communities (9 shown, 1 thin omitted)
- Extraction: 100% EXTRACTED · 0% INFERRED · 0% AMBIGUOUS
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `0f74914f`
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
1. `Engine` - 47 edges
2. `Engine` - 45 edges
3. `main()` - 22 edges
4. `Context` - 20 edges
5. `Store` - 17 edges
6. `strategyState` - 15 edges
7. `Time` - 14 edges
8. `strategyState` - 14 edges
9. `evaluateMockTradeOpenRisk()` - 13 edges
10. `Time` - 13 edges

## Surprising Connections (you probably didn't know these)
- `build()` --calls--> `buildMockTradeFromTrace()`  [EXTRACTED]
  D:/APPLICATION/antigravity-main/antigravity-main/client/src/lib/mockTradingEngine.test.ts → D:/APPLICATION/antigravity-main/antigravity-main/client/src/lib/trading/mockTradingEngine.ts
- `fetchBinanceBTCSpot()` --references--> `Context`  [EXTRACTED]
  D:/APPLICATION/antigravity-main/antigravity-main/engine/cmd/antigravity/main.go → D:/APPLICATION/antigravity-main/antigravity-main/engine/internal/persistence/store.go
- `fetchDeltaBTCSpotForOptions()` --references--> `Context`  [EXTRACTED]
  D:/APPLICATION/antigravity-main/antigravity-main/engine/cmd/antigravity/main.go → D:/APPLICATION/antigravity-main/antigravity-main/engine/internal/persistence/store.go
- `handleDeltaBTCProbe()` --calls--> `Context`  [EXTRACTED]
  D:/APPLICATION/antigravity-main/antigravity-main/engine/cmd/antigravity/main.go → D:/APPLICATION/antigravity-main/antigravity-main/engine/internal/persistence/store.go
- `keepAlive()` --references--> `Context`  [EXTRACTED]
  D:/APPLICATION/antigravity-main/antigravity-main/engine/cmd/antigravity/main.go → D:/APPLICATION/antigravity-main/antigravity-main/engine/internal/persistence/store.go

## Import Cycles
- None detected.

## Communities (10 total, 1 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.10
Nodes (21): AggregateStats, Duration, MarketProfile, OptionPosition, OptionTrade, PersistedState, Request, ResponseWriter (+13 more)

### Community 1 - "Community 1"
Cohesion: 0.10
Nodes (20): AggregateStats, Duration, MarketProfile, OptionPosition, OptionTrade, PersistedState, Request, ResponseWriter (+12 more)

### Community 2 - "Community 2"
Cohesion: 0.05
Nodes (38): computeDailyPnl(), DailyPnlResult, DailyPnlRow, DailyPnlSummary, emptyMockDiagnosticFunnel(), emptyMockRejectionCounts(), emptyMockTradingDiagnostics(), finiteNonNegative() (+30 more)

### Community 3 - "Community 3"
Cohesion: 0.08
Nodes (35): configSource(), fetchBinanceBTCSpot(), fetchDeltaBTCSpotForOptions(), formatHealthTime(), getEnvOrDefault(), getInitialPaperBalanceUSD(), handleDeltaBTCProbe(), keepAlive() (+27 more)

### Community 4 - "Community 4"
Cohesion: 0.19
Nodes (10): Context, Time, DB, EngineState, OptionsSellingState, OptionsState, Store, NewStore() (+2 more)

### Community 5 - "Community 5"
Cohesion: 0.19
Nodes (19): applyPriceTickToTrade(), blockersFromTraceRow(), buildMockTradeFromResearchSignal(), buildMockTradeFromTrace(), closeMockTrade(), computeMockExitLevels(), computeMockFundingCost(), computeMockNetPnlAtExitMark() (+11 more)

### Community 6 - "Community 6"
Cohesion: 0.14
Nodes (17): baseConfig, build(), traceRow(), tradeWithPnl(), wideExitConfig, computeAccountState(), computeAnalytics(), filterMockTrades() (+9 more)

### Community 7 - "Community 7"
Cohesion: 0.17
Nodes (15): candidateFamilyKey(), canOpenAdditionalMockTrade(), countOpenMockTrades(), countOpenMockTradesBySide(), evaluateMockTradeOpenRisk(), isGradeDiscoveryStage(), maxOpenLongTradesFromConfig(), maxOpenMockTradesFromConfig() (+7 more)

### Community 8 - "Community 8"
Cohesion: 0.25
Nodes (6): getMockConfigForPipelineStage(), mockConfigForStage(), STAGE_ACCOUNT_KEY, TRADE_ENGINE_CONFIG, DEFAULT_MOCK_TRADING_CONFIG, MockTradingConfig

## Knowledge Gaps
- **51 isolated node(s):** `baseConfig`, `wideExitConfig`, `TRADE_ENGINE_CONFIG`, `STAGE_ACCOUNT_KEY`, `MockTradeStatus` (+46 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **1 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Context` connect `Community 4` to `Community 3`?**
  _High betweenness centrality (0.022) - this node is a cross-community bridge._
- **What connects `baseConfig`, `wideExitConfig`, `TRADE_ENGINE_CONFIG` to the rest of the system?**
  _51 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.09562841530054644 - nodes in this community are weakly interconnected._
- **Should `Community 1` be split into smaller, more focused modules?**
  _Cohesion score 0.0955837870538415 - nodes in this community are weakly interconnected._
- **Should `Community 2` be split into smaller, more focused modules?**
  _Cohesion score 0.052854122621564484 - nodes in this community are weakly interconnected._
- **Should `Community 3` be split into smaller, more focused modules?**
  _Cohesion score 0.08461538461538462 - nodes in this community are weakly interconnected._
- **Should `Community 6` be split into smaller, more focused modules?**
  _Cohesion score 0.13725490196078433 - nodes in this community are weakly interconnected._