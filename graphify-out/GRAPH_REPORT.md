# Graph Report - antigravity-main  (2026-06-17)

## Corpus Check
- 4667 files · ~26,802,218 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 64 nodes · 119 edges · 8 communities
- Extraction: 100% EXTRACTED · 0% INFERRED · 0% AMBIGUOUS
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `b5d43ea8`
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

## God Nodes (most connected - your core abstractions)
1. `collections()` - 25 edges
2. `nowIso()` - 13 edges
3. `upsertMockTrade()` - 7 edges
4. `paperClosedTradeToMockTrade()` - 5 edges
5. `listMockTrades()` - 5 edges
6. `paperPositionToMockTrade()` - 4 edges
7. `getMockTrade()` - 4 edges
8. `pnlValue()` - 3 edges
9. `mockTradeToDoc()` - 3 edges
10. `timeToMs()` - 3 edges

## Surprising Connections (you probably didn't know these)
- `appendEquityCurvePoint()` --calls--> `collections()`  [EXTRACTED]
  client/src/lib/trading/mockTradingMongo.ts → client/src/lib/trading/mockTradingMongo.ts  _Bridges community 2 → community 1_
- `getMockTrade()` --calls--> `collections()`  [EXTRACTED]
  client/src/lib/trading/mockTradingMongo.ts → client/src/lib/trading/mockTradingMongo.ts  _Bridges community 2 → community 3_
- `listMockTrades()` --calls--> `collections()`  [EXTRACTED]
  client/src/lib/trading/mockTradingMongo.ts → client/src/lib/trading/mockTradingMongo.ts  _Bridges community 2 → community 6_
- `mockLogToDoc()` --calls--> `nowIso()`  [EXTRACTED]
  client/src/lib/trading/mockTradingMongo.ts → client/src/lib/trading/mockTradingMongo.ts  _Bridges community 1 → community 3_

## Import Cycles
- None detected.

## Communities (8 total, 0 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.11
Nodes (17): DailyPnlPoint, EquityCurvePoint, MockAccountSnapshotDoc, MockDailyPnlHistoryDoc, MockEngineConfigDoc, MockEquityCurvePointDoc, MockRegimeSnapshotDoc, MockStrategyAnalyticsDoc (+9 more)

### Community 1 - "Community 1"
Cohesion: 0.18
Nodes (11): appendEquityCurvePoint(), batchUpsertStrategyScores(), deleteClosedMockTrade(), deleteClosedMockTrades(), getMockAnalyticsSummary(), insertMockAccountSnapshot(), insertStrategySignals(), nowIso() (+3 more)

### Community 2 - "Community 2"
Cohesion: 0.18
Nodes (11): collections(), computeAccountFromMongo(), ensureMockIndexes(), getLatestMockAccountSnapshot(), getLatestRegimeSnapshot(), listDailyPnlHistory(), listEquityCurvePoints(), listMockLogs() (+3 more)

### Community 3 - "Community 3"
Cohesion: 0.33
Nodes (7): closeMockTradeInMongo(), getMockTrade(), mockLogToDoc(), mockTradeFromDoc(), mockTradeToDoc(), pnlValue(), upsertMockTrade()

### Community 4 - "Community 4"
Cohesion: 0.47
Nodes (6): paperClosedTradeToMockTrade(), paperExitReasonToMock(), paperPositionToMockTrade(), paperSideToMockSide(), strategyNumericId(), timeToMs()

### Community 5 - "Community 5"
Cohesion: 0.50
Nodes (4): addMongoAndCondition(), mockTradeAgeConditionForQuery(), mockTradeAgeDurationMsExpression(), mockTradeMongoFilterForQuery()

### Community 6 - "Community 6"
Cohesion: 0.50
Nodes (4): listMockTrades(), listPaperPersistTrades(), queryFilterFromMockTradeQuery(), sortForQuery()

## Knowledge Gaps
- **17 isolated node(s):** `MockTradeDoc`, `MockStrategySignalDoc`, `MockRegimeSnapshotDoc`, `MockStrategyScoreDoc`, `MockStrategyScoreHistoryDoc` (+12 more)
  These have ≤1 connection - possible missing edges or undocumented components.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `collections()` connect `Community 2` to `Community 0`, `Community 1`, `Community 3`, `Community 6`?**
  _High betweenness centrality (0.066) - this node is a cross-community bridge._
- **Why does `nowIso()` connect `Community 1` to `Community 0`, `Community 3`?**
  _High betweenness centrality (0.012) - this node is a cross-community bridge._
- **Why does `upsertMockTrade()` connect `Community 3` to `Community 0`, `Community 1`, `Community 2`?**
  _High betweenness centrality (0.003) - this node is a cross-community bridge._
- **What connects `MockTradeDoc`, `MockStrategySignalDoc`, `MockRegimeSnapshotDoc` to the rest of the system?**
  _17 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.10526315789473684 - nodes in this community are weakly interconnected._