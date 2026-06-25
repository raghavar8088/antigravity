# Graph Report - antigravity-main  (2026-06-25)

## Corpus Check
- 4844 files · ~26,933,192 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 629 nodes · 1268 edges · 26 communities (21 shown, 5 thin omitted)
- Extraction: 89% EXTRACTED · 11% INFERRED · 0% AMBIGUOUS · INFERRED: 143 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `2694216a`
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
- [[_COMMUNITY_Community 15|Community 15]]
- [[_COMMUNITY_Community 16|Community 16]]
- [[_COMMUNITY_Community 17|Community 17]]
- [[_COMMUNITY_Community 18|Community 18]]
- [[_COMMUNITY_Community 19|Community 19]]
- [[_COMMUNITY_Community 20|Community 20]]
- [[_COMMUNITY_Community 21|Community 21]]
- [[_COMMUNITY_Community 22|Community 22]]
- [[_COMMUNITY_Community 23|Community 23]]
- [[_COMMUNITY_Community 24|Community 24]]
- [[_COMMUNITY_Community 25|Community 25]]

## God Nodes (most connected - your core abstractions)
1. `ATR()` - 58 edges
2. `Candle` - 43 edges
3. `ScalerBundle` - 40 edges
4. `main()` - 27 edges
5. `BacktestAPI` - 13 edges
6. `Runner` - 13 edges
7. `ADX()` - 13 edges
8. `BinanceHistoricalFetcher` - 12 edges
9. `EMA()` - 12 edges
10. `ResultsStore` - 10 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `DBPath()`  [INFERRED]
  engine/cmd/antigravity/main.go → engine/internal/backtest/http_handlers.go
- `main()` --calls--> `NewBacktestAPI()`  [INFERRED]
  engine/cmd/antigravity/main.go → engine/internal/backtest/http_handlers.go
- `main()` --calls--> `OpenResultsStore()`  [INFERRED]
  engine/cmd/antigravity/main.go → engine/internal/backtest/results_store.go
- `main()` --calls--> `Int64`  [INFERRED]
  engine/cmd/antigravity/main.go → engine/internal/backtest/scaler_v3_adapter.go
- `main()` --calls--> `BuildCuratedScalpers()`  [INFERRED]
  engine/cmd/antigravity/main.go → engine/internal/strategy/scalpers/curated_registry.go

## Import Cycles
- None detected.

## Communities (26 total, 5 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.07
Nodes (61): Candle, MarketContext, Signal, MarketContext, Signal, PivotLevels, AroonResult, BollingerBands (+53 more)

### Community 1 - "Community 1"
Cohesion: 0.05
Nodes (31): AggregatedSignal, AggTrade, Candle, Context, MarketContext, Mutex, Regime, RegistryEntry (+23 more)

### Community 2 - "Community 2"
Cohesion: 0.08
Nodes (13): MarketContext, Regime, Signal, DVOLExtremeRegime, ETHCorrelationDivergence, FundingOIConfirm, FundingRateExtremeLong, FundingRateExtremeShort (+5 more)

### Community 3 - "Community 3"
Cohesion: 0.08
Nodes (13): MarketContext, Regime, Signal, DonchianSqueeze, EFIMomentum, FourHMomentumBurst, HMASlopeShift, IchimokuCloudBreak (+5 more)

### Community 4 - "Community 4"
Cohesion: 0.08
Nodes (13): MarketContext, Regime, Signal, DVOLFundingMacroLong, DVOLFundingMacroShort, ETHStrengthBTCLag, LiquidationHuntLong, LiquidationHuntShort (+5 more)

### Community 5 - "Community 5"
Cohesion: 0.09
Nodes (36): configSource(), fetchBinanceBTCSpot(), fetchDeltaBTCSpotForOptions(), formatHealthTime(), getEnvOrDefault(), getInitialPaperBalanceUSD(), handleDeltaBTCProbe(), keepAlive() (+28 more)

### Community 6 - "Community 6"
Cohesion: 0.11
Nodes (27): BacktestAPI, DBPath(), jsonError(), NewBacktestAPI(), setCORSJSON(), ApplyAutoDemotion(), DemoteFromResult(), EvaluatePromotion() (+19 more)

### Community 7 - "Community 7"
Cohesion: 0.12
Nodes (28): BatchRunner, RunConfig, Runner, buildStrategyResult(), calcSharpe(), DefaultRunConfig(), NewBatchRunner(), NewRunner() (+20 more)

### Community 8 - "Community 8"
Cohesion: 0.09
Nodes (10): Regime, AggressorExhaustion, CMFBBTouch, FisherTransformSignal, HullMAScalp, NarrowRangeBreakout, StochRSICross, VolatilitySqueezeEntry (+2 more)

### Community 9 - "Community 9"
Cohesion: 0.09
Nodes (10): Regime, ChandelierFollow, DEMAPullback, EffectiveSpreadCompress, KeltnerBreakout, LastTradeSizeWhale, MultiTFZScoreLong, MultiTFZScoreShort (+2 more)

### Community 10 - "Community 10"
Cohesion: 0.17
Nodes (11): migrate(), OpenResultsStore(), scanRows(), ResultsStore, scanner, singleRow, StoredResult, DB (+3 more)

### Community 11 - "Community 11"
Cohesion: 0.11
Nodes (14): COMMAND_CENTER_NAV, CommandCenterNavItem, GRADE_ROUTE_STATUS, isPaperDeskTabKey(), LEGACY_TAB_REDIRECTS, legacyPaperDeskRedirect(), MONITOR_NAV, NavActiveItem (+6 more)

### Community 12 - "Community 12"
Cohesion: 0.23
Nodes (17): Request, T, Time, main(), Int64, NewBinanceHistoricalFetcher(), makeFakePage(), TestCacheWrittenAfterFetch() (+9 more)

### Community 13 - "Community 13"
Cohesion: 0.19
Nodes (15): approximateCVD(), NewContextBuilder(), tail(), toScalerCandles(), toScalerCandlesBefore(), ContextBuilder, Candle, HistoricalCandle (+7 more)

### Community 14 - "Community 14"
Cohesion: 0.26
Nodes (11): BinanceHistoricalFetcher, HistoricalCandle, HistoricalFundingEntry, HistoricalOIEntry, Time, filterCandles(), filterFunding(), filterOI() (+3 more)

### Community 15 - "Community 15"
Cohesion: 0.30
Nodes (7): Client, Time, BinanceHistoricalFetcher, parseBinanceKlines(), HistoricalCandle, HistoricalFundingEntry, HistoricalOIEntry

### Community 16 - "Community 16"
Cohesion: 0.15
Nodes (4): Job, LeaderboardRow, StrategyInfo, Tab

### Community 17 - "Community 17"
Cohesion: 0.23
Nodes (12): ADMIN_PATH_PREFIXES, BLOCKED_PATH_PREFIXES, classifyPath(), deny(), handle(), passthrough(), PathTier, proxyToEngine() (+4 more)

### Community 18 - "Community 18"
Cohesion: 0.29
Nodes (6): Job, NewJobStore(), JobStatus, JobStore, RWMutex, Time

### Community 19 - "Community 19"
Cohesion: 0.29
Nodes (6): Directory Structure, File Format, File Summary, Notes, Purpose, Usage Guidelines

### Community 25 - "Community 25"
Cohesion: 0.40
Nodes (4): External Integrations & Research Workspace, How to use, Layout (organized by functionality), Recommended but NOT cloned (evaluate next)

## Knowledge Gaps
- **72 isolated node(s):** `Purpose`, `File Format`, `Usage Guidelines`, `Notes`, `Directory Structure` (+67 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **5 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `ATR()` connect `Community 0` to `Community 1`, `Community 2`, `Community 3`, `Community 4`?**
  _High betweenness centrality (0.539) - this node is a cross-community bridge._
- **Why does `main()` connect `Community 5` to `Community 10`, `Community 12`, `Community 6`?**
  _High betweenness centrality (0.275) - this node is a cross-community bridge._
- **Why does `BuildCuratedScalpers()` connect `Community 6` to `Community 5`?**
  _High betweenness centrality (0.247) - this node is a cross-community bridge._
- **Are the 51 inferred relationships involving `ATR()` (e.g. with `.Evaluate()` and `.Evaluate()`) actually correct?**
  _`ATR()` has 51 INFERRED edges - model-reasoned connections that need verification._
- **Are the 5 inferred relationships involving `main()` (e.g. with `DBPath()` and `NewBacktestAPI()`) actually correct?**
  _`main()` has 5 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Purpose`, `File Format`, `Usage Guidelines` to the rest of the system?**
  _72 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.06568986568986569 - nodes in this community are weakly interconnected._