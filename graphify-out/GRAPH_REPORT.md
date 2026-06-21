# Graph Report - antigravity-main  (2026-06-21)

## Corpus Check
- 4728 files · ~26,844,610 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 477 nodes · 729 edges · 25 communities
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 85 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `d1d30e38`
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

## God Nodes (most connected - your core abstractions)
1. `ScalerBundle` - 38 edges
2. `LoopDeps` - 28 edges
3. `main()` - 27 edges
4. `ATR()` - 23 edges
5. `NoSignal()` - 23 edges
6. `MacroFeedHolder` - 21 edges
7. `BinanceLiquidationHolder` - 18 edges
8. `Candle` - 17 edges
9. `DeribitDVOLHolder` - 13 edges
10. `BinancePerpPriceHolder` - 11 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `NewBinanceLiquidationHolder()`  [INFERRED]
  engine/cmd/antigravity/main.go → engine/internal/marketdata/binance_liquidations.go
- `main()` --calls--> `NewBinancePerpPriceHolder()`  [INFERRED]
  engine/cmd/antigravity/main.go → engine/internal/marketdata/binance_perp_price.go
- `main()` --calls--> `NewDeribitDVOLHolder()`  [INFERRED]
  engine/cmd/antigravity/main.go → engine/internal/marketdata/deribit_dvol.go
- `main()` --calls--> `NewMacroFeedHolder()`  [INFERRED]
  engine/cmd/antigravity/main.go → engine/internal/marketdata/macro_feed.go
- `main()` --calls--> `SetStrategyRolloutPhase()`  [INFERRED]
  engine/cmd/antigravity/main.go → engine/internal/observability/metrics.go

## Import Cycles
- None detected.

## Communities (25 total, 0 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.06
Nodes (33): AggregatedSignal, AggTrade, Candle, Context, Duration, MarketContext, Mutex, Regime (+25 more)

### Community 1 - "Community 1"
Cohesion: 0.07
Nodes (39): Candle, MarketContext, Regime, Signal, Duration, MarketContext, Regime, Signal (+31 more)

### Community 2 - "Community 2"
Cohesion: 0.09
Nodes (35): configSource(), fetchBinanceBTCSpot(), fetchDeltaBTCSpotForOptions(), formatHealthTime(), getEnvOrDefault(), getInitialPaperBalanceUSD(), handleDeltaBTCProbe(), keepAlive() (+27 more)

### Community 3 - "Community 3"
Cohesion: 0.08
Nodes (25): RegistryEntry, RegistryEntry, RegistryEntry, RegistryEntry, MarketContext, Regime, Signal, RegistryEntry (+17 more)

### Community 4 - "Community 4"
Cohesion: 0.09
Nodes (14): MarketContext, Regime, Signal, MarketContext, Regime, Signal, MarketContext, Regime (+6 more)

### Community 5 - "Community 5"
Cohesion: 0.07
Nodes (25): AsyncScorer, BinanceLiquidationHolder, BinancePerpPriceHolder, CalibrationResult, Classifier, CycleGuard, DepthSubscriber, DeribitDVOLHolder (+17 more)

### Community 6 - "Community 6"
Cohesion: 0.11
Nodes (12): Client, Context, RWMutex, Time, appendMacroPointCapped(), NewMacroFeedHolder(), pearsonCorrelation(), rollingHigh() (+4 more)

### Community 7 - "Community 7"
Cohesion: 0.14
Nodes (9): Conn, Context, Duration, RWMutex, Time, NewBinanceLiquidationHolder(), binanceForceOrderEvent, BinanceLiquidationHolder (+1 more)

### Community 8 - "Community 8"
Cohesion: 0.13
Nodes (16): MarketContext, Regime, Signal, Time, Candle, Direction, MarketContext, OrderBookDepthLevels (+8 more)

### Community 9 - "Community 9"
Cohesion: 0.17
Nodes (7): Client, Context, RWMutex, Time, NewDeribitDVOLHolder(), DeribitDVOLHolder, deribitVolIndexResponse

### Community 10 - "Community 10"
Cohesion: 0.20
Nodes (7): Client, Context, RWMutex, Time, NewBinancePerpPriceHolder(), BinancePerpPriceHolder, binancePremiumIndexResponse

### Community 11 - "Community 11"
Cohesion: 0.24
Nodes (7): DominanceFetcher, MarketContext, Regime, Signal, DominanceRelativeStrength, getDominanceFetcher(), SetDominanceFetcher()

### Community 12 - "Community 12"
Cohesion: 0.24
Nodes (6): Candle, MarketContext, Regime, Signal, CMEGapFill, findWeekendAnchors()

### Community 13 - "Community 13"
Cohesion: 0.27
Nodes (6): MarketContext, Regime, Signal, OIFundingCrowdingComposite, normalizeFunding(), normalizeOIChange()

### Community 14 - "Community 14"
Cohesion: 0.24
Nodes (6): Candle, MarketContext, Regime, Signal, computeVWMFactor(), VolumeWeightedMomentumFactor

### Community 15 - "Community 15"
Cohesion: 0.28
Nodes (5): MarketContext, Regime, Signal, FundingResetMeanReversion, minutesToNearestSettlement()

### Community 16 - "Community 16"
Cohesion: 0.25
Nodes (5): MarketContext, Mutex, Regime, Signal, MTFZScoreConfluence

### Community 17 - "Community 17"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, DXYInverseMomentum

### Community 18 - "Community 18"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, LiquidationCascadeFade

### Community 19 - "Community 19"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, MacroDecouplingMomentum

### Community 20 - "Community 20"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, OrderbookImbalancePersistence

### Community 21 - "Community 21"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, PerpSpotBasisMomentum

### Community 22 - "Community 22"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, BTCEquitiesCorrelationBreak

### Community 23 - "Community 23"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, RiskOnOffRegimeProxy

### Community 24 - "Community 24"
Cohesion: 0.29
Nodes (6): Directory Structure, File Format, File Summary, Notes, Purpose, Usage Guidelines

## Knowledge Gaps
- **135 isolated node(s):** `Purpose`, `File Format`, `Usage Guidelines`, `Notes`, `Directory Structure` (+130 more)
  These have ≤1 connection - possible missing edges or undocumented components.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `main()` connect `Community 2` to `Community 3`, `Community 6`, `Community 7`, `Community 9`, `Community 10`, `Community 11`?**
  _High betweenness centrality (0.397) - this node is a cross-community bridge._
- **Why does `ATR()` connect `Community 1` to `Community 0`, `Community 4`, `Community 8`, `Community 11`, `Community 12`, `Community 13`, `Community 14`, `Community 15`, `Community 16`, `Community 17`, `Community 18`, `Community 19`, `Community 20`, `Community 21`, `Community 22`, `Community 23`?**
  _High betweenness centrality (0.328) - this node is a cross-community bridge._
- **Why does `NoSignal()` connect `Community 8` to `Community 1`, `Community 3`, `Community 4`, `Community 11`, `Community 12`, `Community 13`, `Community 14`, `Community 15`, `Community 16`, `Community 17`, `Community 18`, `Community 19`, `Community 20`, `Community 21`, `Community 22`, `Community 23`?**
  _High betweenness centrality (0.245) - this node is a cross-community bridge._
- **Are the 7 inferred relationships involving `main()` (e.g. with `NewBinanceLiquidationHolder()` and `NewBinancePerpPriceHolder()`) actually correct?**
  _`main()` has 7 INFERRED edges - model-reasoned connections that need verification._
- **Are the 21 inferred relationships involving `ATR()` (e.g. with `.Evaluate()` and `.Evaluate()`) actually correct?**
  _`ATR()` has 21 INFERRED edges - model-reasoned connections that need verification._
- **Are the 21 inferred relationships involving `NoSignal()` (e.g. with `.Evaluate()` and `.Evaluate()`) actually correct?**
  _`NoSignal()` has 21 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Purpose`, `File Format`, `Usage Guidelines` to the rest of the system?**
  _135 weakly-connected nodes found - possible documentation gaps or missing edges._