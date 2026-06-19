# Graph Report - antigravity-main  (2026-06-18)

## Corpus Check
- 4669 files · ~26,804,294 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 130 nodes · 196 edges · 10 communities (8 shown, 2 thin omitted)
- Extraction: 99% EXTRACTED · 1% INFERRED · 0% AMBIGUOUS · INFERRED: 1 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `7a224e47`
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
1. `evaluateMockTradeOpenRisk()` - 11 edges
2. `buildMockTradeFromTrace()` - 8 edges
3. `toPaperSide()` - 7 edges
4. `computeMockFundingCost()` - 6 edges
5. `withMockExitFields()` - 6 edges
6. `applyPriceTickToTrade()` - 6 edges
7. `finalizeClose()` - 6 edges
8. `computeDailyPnl()` - 6 edges
9. `File Summary` - 5 edges
10. `resolveExit()` - 5 edges

## Surprising Connections (you probably didn't know these)
- `GET()` --calls--> `computeDailyPnl()`  [INFERRED]
  client/src/app/api/mock-trading/daily-pnl-summary/route.ts → client/src/lib/trading/mockTradingEngine.ts

## Import Cycles
- None detected.

## Communities (10 total, 2 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.05
Nodes (31): DailyPnlResult, DailyPnlRow, DailyPnlSummary, MOCK_REJECTION_CATEGORIES, MOCK_TRADE_SORT_OPTIONS, MockAccountState, MockBlockerAggregate, MockDiagnosticFunnel (+23 more)

### Community 1 - "Community 1"
Cohesion: 0.09
Nodes (14): DailyPnlApiResponse, DailyPnlRow, DailyPnlSummary, DailyPnLTable(), DailyPnLTableProps, fmtSignedPct(), fmtSignedUsd(), monoCellStyle (+6 more)

### Community 2 - "Community 2"
Cohesion: 0.19
Nodes (19): applyPriceTickToTrade(), blockersFromTraceRow(), buildMockTradeFromResearchSignal(), buildMockTradeFromTrace(), closeMockTrade(), computeMockExitLevels(), computeMockFundingCost(), computeMockNetPnlAtExitMark() (+11 more)

### Community 3 - "Community 3"
Cohesion: 0.15
Nodes (16): candidateFamilyKey(), canOpenAdditionalMockTrade(), computeAccountState(), computeAnalytics(), countOpenMockTrades(), countOpenMockTradesBySide(), evaluateMockTradeOpenRisk(), isGradeDiscoveryStage() (+8 more)

### Community 4 - "Community 4"
Cohesion: 0.32
Nodes (7): GET(), mongoNotConfigured(), computeDailyPnl(), DEFAULT_MOCK_TRADING_CONFIG, istDateKey(), round2(), round3()

### Community 5 - "Community 5"
Cohesion: 0.67
Nodes (3): emptyMockDiagnosticFunnel(), emptyMockRejectionCounts(), emptyMockTradingDiagnostics()

### Community 6 - "Community 6"
Cohesion: 0.67
Nodes (3): finiteNonNegative(), mockTradeAgeMinutes(), passesMockTradeAgeFilter()

### Community 9 - "Community 9"
Cohesion: 0.29
Nodes (6): Directory Structure, File Format, File Summary, Notes, Purpose, Usage Guidelines

## Knowledge Gaps
- **46 isolated node(s):** `Purpose`, `File Format`, `Usage Guidelines`, `Notes`, `Directory Structure` (+41 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **2 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `computeDailyPnl()` connect `Community 4` to `Community 0`?**
  _High betweenness centrality (0.006) - this node is a cross-community bridge._
- **Why does `evaluateMockTradeOpenRisk()` connect `Community 3` to `Community 0`?**
  _High betweenness centrality (0.003) - this node is a cross-community bridge._
- **What connects `Purpose`, `File Format`, `Usage Guidelines` to the rest of the system?**
  _46 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.046511627906976744 - nodes in this community are weakly interconnected._
- **Should `Community 1` be split into smaller, more focused modules?**
  _Cohesion score 0.08831908831908832 - nodes in this community are weakly interconnected._