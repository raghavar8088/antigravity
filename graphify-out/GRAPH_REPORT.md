# Graph Report - antigravity-main  (2026-06-25)

## Corpus Check
- 4824 files · ~26,913,724 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1004 nodes · 1363 edges · 87 communities (84 shown, 3 thin omitted)
- Extraction: 97% EXTRACTED · 3% INFERRED · 0% AMBIGUOUS · INFERRED: 36 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `2a2dd9e4`
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
- [[_COMMUNITY_Community 26|Community 26]]
- [[_COMMUNITY_Community 27|Community 27]]
- [[_COMMUNITY_Community 28|Community 28]]
- [[_COMMUNITY_Community 29|Community 29]]
- [[_COMMUNITY_Community 30|Community 30]]
- [[_COMMUNITY_Community 31|Community 31]]
- [[_COMMUNITY_Community 32|Community 32]]
- [[_COMMUNITY_Community 33|Community 33]]
- [[_COMMUNITY_Community 34|Community 34]]
- [[_COMMUNITY_Community 35|Community 35]]
- [[_COMMUNITY_Community 36|Community 36]]
- [[_COMMUNITY_Community 37|Community 37]]
- [[_COMMUNITY_Community 38|Community 38]]
- [[_COMMUNITY_Community 39|Community 39]]
- [[_COMMUNITY_Community 40|Community 40]]
- [[_COMMUNITY_Community 41|Community 41]]
- [[_COMMUNITY_Community 42|Community 42]]
- [[_COMMUNITY_Community 43|Community 43]]
- [[_COMMUNITY_Community 44|Community 44]]
- [[_COMMUNITY_Community 45|Community 45]]
- [[_COMMUNITY_Community 46|Community 46]]
- [[_COMMUNITY_Community 47|Community 47]]
- [[_COMMUNITY_Community 48|Community 48]]
- [[_COMMUNITY_Community 49|Community 49]]
- [[_COMMUNITY_Community 50|Community 50]]
- [[_COMMUNITY_Community 51|Community 51]]
- [[_COMMUNITY_Community 52|Community 52]]
- [[_COMMUNITY_Community 53|Community 53]]
- [[_COMMUNITY_Community 54|Community 54]]
- [[_COMMUNITY_Community 55|Community 55]]
- [[_COMMUNITY_Community 56|Community 56]]
- [[_COMMUNITY_Community 57|Community 57]]
- [[_COMMUNITY_Community 58|Community 58]]
- [[_COMMUNITY_Community 59|Community 59]]
- [[_COMMUNITY_Community 60|Community 60]]
- [[_COMMUNITY_Community 61|Community 61]]
- [[_COMMUNITY_Community 62|Community 62]]
- [[_COMMUNITY_Community 63|Community 63]]
- [[_COMMUNITY_Community 64|Community 64]]
- [[_COMMUNITY_Community 65|Community 65]]
- [[_COMMUNITY_Community 66|Community 66]]
- [[_COMMUNITY_Community 67|Community 67]]
- [[_COMMUNITY_Community 68|Community 68]]
- [[_COMMUNITY_Community 69|Community 69]]
- [[_COMMUNITY_Community 70|Community 70]]
- [[_COMMUNITY_Community 71|Community 71]]
- [[_COMMUNITY_Community 72|Community 72]]
- [[_COMMUNITY_Community 73|Community 73]]
- [[_COMMUNITY_Community 74|Community 74]]
- [[_COMMUNITY_Community 75|Community 75]]
- [[_COMMUNITY_Community 76|Community 76]]
- [[_COMMUNITY_Community 77|Community 77]]
- [[_COMMUNITY_Community 78|Community 78]]
- [[_COMMUNITY_Community 79|Community 79]]
- [[_COMMUNITY_Community 80|Community 80]]
- [[_COMMUNITY_Community 81|Community 81]]
- [[_COMMUNITY_Community 82|Community 82]]
- [[_COMMUNITY_Community 83|Community 83]]
- [[_COMMUNITY_Community 84|Community 84]]
- [[_COMMUNITY_Community 85|Community 85]]
- [[_COMMUNITY_Community 86|Community 86]]

## God Nodes (most connected - your core abstractions)
1. `Orchestrator` - 100 edges
2. `main()` - 27 edges
3. `Context` - 23 edges
4. `ShadowLedger` - 22 edges
5. `NewOrchestrator()` - 12 edges
6. `ShadowTrade` - 11 edges
7. `sanitizeSignalForProfit()` - 10 edges
8. `NewMetaLabelFilter()` - 10 edges
9. `Signal` - 9 edges
10. `longSig()` - 9 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `BuildCuratedScalpers()`  [INFERRED]
  engine/cmd/antigravity/main.go → engine/internal/strategy/scalpers/curated_registry.go
- `main()` --calls--> `SetDominanceFetcher()`  [INFERRED]
  engine/cmd/antigravity/main.go → engine/internal/strategy/scalpers/s_eth_btc_relative_strength.go
- `main()` --calls--> `NewShadowLedger()`  [INFERRED]
  engine/cmd/antigravity/main.go → engine/internal/shadow/ledger.go
- `main()` --calls--> `NewOrchestrator()`  [INFERRED]
  engine/cmd/antigravity/main.go → engine/internal/trading/loop.go
- `TestSMCOrderBlockFVG_ConfidenceNotBoostedWithoutConfirmation()` --calls--> `detectBullishFVG()`  [INFERRED]
  engine/internal/strategy/scalpers/s10_smc_order_block_fvg_test.go → engine/internal/strategy/scalpers/s10_smc_order_block_fvg.go

## Import Cycles
- None detected.

## Communities (87 total, 3 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.05
Nodes (37): AdaptiveConfidenceFloor, AggTrade, Bridge, CandleAggregator, CandleSummary, Candle, Mutex, RegistryEntry (+29 more)

### Community 1 - "Community 1"
Cohesion: 0.06
Nodes (23): MarketContext, Regime, Signal, MarketContext, Regime, Signal, MarketContext, Regime (+15 more)

### Community 2 - "Community 2"
Cohesion: 0.06
Nodes (23): DominanceFetcher, MarketContext, Regime, Signal, MarketContext, Regime, Signal, MarketContext (+15 more)

### Community 3 - "Community 3"
Cohesion: 0.15
Nodes (25): Candle, MarketContext, Regime, Signal, Candle, T, Time, fvgZone (+17 more)

### Community 4 - "Community 4"
Cohesion: 0.09
Nodes (23): Action, MarketContext, Regime, IndicatorSnapshot, Side, AgentSignalForExec, BridgeDecision, InstitutionalPathOpts (+15 more)

### Community 5 - "Community 5"
Cohesion: 0.20
Nodes (20): Direction, AggregatedSignal, MarketContext, Regime, WalkForwardSummary, AggregatedSignal, T, NewMetaLabelFilter() (+12 more)

### Community 6 - "Community 6"
Cohesion: 0.14
Nodes (5): CloseEvent, Context, Snapshot, Tick, isTrustedStrategy()

### Community 7 - "Community 7"
Cohesion: 0.19
Nodes (10): Signal, Event, EventType, FillResult, OrderMode, OrderPayload, Position, brokerFillFunc (+2 more)

### Community 8 - "Community 8"
Cohesion: 0.13
Nodes (10): Candle, MarketContext, Regime, Signal, MarketContext, Regime, Signal, DualMomentumTrend (+2 more)

### Community 9 - "Community 9"
Cohesion: 0.23
Nodes (16): T, adjustConfidenceByExecutionWeight(), isCategoryAlignedWithRegime(), makeConstantSeries(), makeLinearSeries(), makeRangeSeries(), makeVolatileSeries(), TestAdjustConfidenceByExecutionWeight() (+8 more)

### Community 10 - "Community 10"
Cohesion: 0.09
Nodes (36): configSource(), fetchBinanceBTCSpot(), fetchDeltaBTCSpotForOptions(), formatHealthTime(), getEnvOrDefault(), getInitialPaperBalanceUSD(), handleDeltaBTCProbe(), keepAlive() (+28 more)

### Community 11 - "Community 11"
Cohesion: 0.27
Nodes (8): Candle, MarketContext, Regime, Signal, RSIDivergenceMultiframe, rsiBearishDiv3Bar(), rsiBullishDiv3Bar(), rsiSeries()

### Community 12 - "Community 12"
Cohesion: 0.23
Nodes (8): Duration, MarketContext, Regime, Signal, Time, MacroCalendarVolPositioning, macroEvent, nearestMacroEvent()

### Community 13 - "Community 13"
Cohesion: 0.25
Nodes (7): Candle, MarketContext, Regime, Signal, FractalBreakout, findLastFractalHigh(), findLastFractalLow()

### Community 14 - "Community 14"
Cohesion: 0.25
Nodes (7): Candle, MarketContext, Regime, Signal, HiddenLiquidityDetection, clusterTouchesHigh(), clusterTouchesLow()

### Community 15 - "Community 15"
Cohesion: 0.25
Nodes (7): Candle, MarketContext, Regime, Signal, RecurrenceQuantificationSignal, featureVector(), nextWindowDirection()

### Community 16 - "Community 16"
Cohesion: 0.31
Nodes (6): MarketContext, Regime, Signal, LiquiditySweepReversal, sweepCVDBearishDiv(), sweepCVDBullishDiv()

### Community 17 - "Community 17"
Cohesion: 0.24
Nodes (6): Candle, MarketContext, Regime, Signal, williamsPercentR(), WilliamsPercentRReversion

### Community 18 - "Community 18"
Cohesion: 0.24
Nodes (6): Candle, MarketContext, Regime, Signal, PivotPointReversion, isReversalCandle()

### Community 19 - "Community 19"
Cohesion: 0.27
Nodes (6): MarketContext, Regime, Signal, meanFloats(), stdevFloats(), VolatilityMeanReversion

### Community 20 - "Community 20"
Cohesion: 0.24
Nodes (6): Candle, MarketContext, Regime, Signal, GradientBoostingFeatureSignal, rocAndMomentumSign()

### Community 21 - "Community 21"
Cohesion: 0.24
Nodes (6): Candle, MarketContext, Regime, Signal, ExcessReturnMomentum, excessReturn()

### Community 22 - "Community 22"
Cohesion: 0.27
Nodes (6): MarketContext, Regime, Signal, GammaExposureMagnet, gexProxyNearestLevel(), gexProxyScore()

### Community 23 - "Community 23"
Cohesion: 0.24
Nodes (6): Candle, MarketContext, Regime, Signal, CMEGapFill, findWeekendAnchors()

### Community 24 - "Community 24"
Cohesion: 0.27
Nodes (6): MarketContext, Regime, Signal, OIFundingCrowdingComposite, normalizeFunding(), normalizeOIChange()

### Community 25 - "Community 25"
Cohesion: 0.28
Nodes (5): MarketContext, Regime, Signal, barrierHitProbability(), TripleBarrierMomentum

### Community 26 - "Community 26"
Cohesion: 0.28
Nodes (5): MarketContext, Regime, Signal, FundingResetMeanReversion, minutesToNearestSettlement()

### Community 27 - "Community 27"
Cohesion: 0.39
Nodes (7): RegistryEntry, Performance, AllPerformance(), BuildCuratedScalpers(), FilterWinnersOnly(), GetPerformance(), UpdatePerformance()

### Community 28 - "Community 28"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, EMARibbonTrendRider

### Community 29 - "Community 29"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, BollingerMeanReversion

### Community 30 - "Community 30"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, KalmanFilterTrend

### Community 31 - "Community 31"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, AdaptiveMovingAverageTrend

### Community 32 - "Community 32"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, SuperTrendMomentum

### Community 33 - "Community 33"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, IchimokuCloudBreakout

### Community 34 - "Community 34"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, DonchianChannelBreakout

### Community 35 - "Community 35"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, ParabolicSARTrend

### Community 36 - "Community 36"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, OrnsteinUhlenbeckReversion

### Community 37 - "Community 37"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, CointegrationSpreadBTCETH

### Community 38 - "Community 38"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, StochasticOscillatorReversion

### Community 39 - "Community 39"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, CommodityChannelIndexReversion

### Community 40 - "Community 40"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, PriceChannelMeanReversion

### Community 41 - "Community 41"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, LinearRegressionChannelFade

### Community 42 - "Community 42"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, ADXMomentumBreakout

### Community 43 - "Community 43"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, TradeIntensityBurst

### Community 44 - "Community 44"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, LargeTradeImpactFollow

### Community 45 - "Community 45"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, BidAskSpreadRegime

### Community 46 - "Community 46"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, VPINToxicitySignal

### Community 47 - "Community 47"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, OrderBookDepthImbalancePersistence

### Community 48 - "Community 48"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, AbsorptionPatternSignal

### Community 49 - "Community 49"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, SweepAndReloadPattern

### Community 50 - "Community 50"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, MarketDepthGradient

### Community 51 - "Community 51"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, AggressorRatioMomentum

### Community 52 - "Community 52"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, VWAPInstitutionalFade

### Community 53 - "Community 53"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, FeatureConfluenceScore

### Community 54 - "Community 54"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, RegimeProbabilityComposite

### Community 55 - "Community 55"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, AutocorrelationPattern

### Community 56 - "Community 56"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, ShannonEntropyRegime

### Community 57 - "Community 57"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, CrossCorrelationLeadLag

### Community 58 - "Community 58"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, WaveletTrendSignal

### Community 59 - "Community 59"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, OptionsSkewDirectionSignal

### Community 60 - "Community 60"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, PerpetualFundingBasisComposite

### Community 61 - "Community 61"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, LongShortRatioContrarian

### Community 62 - "Community 62"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, TakerBuySellVolumeRatio

### Community 63 - "Community 63"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, OpenInterestVelocity

### Community 64 - "Community 64"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, ExchangeNetflowSignal

### Community 65 - "Community 65"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, MinerHashRateMomentum

### Community 66 - "Community 66"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, StablecoinSupplyFlow

### Community 67 - "Community 67"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, FearGreedContrarian

### Community 68 - "Community 68"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, ElderTripleScreen

### Community 69 - "Community 69"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, AroonTrendConfirmation

### Community 70 - "Community 70"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, ThreeBarReversal

### Community 71 - "Community 71"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, TrendContinuationPullback

### Community 72 - "Community 72"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, EngulfingBarReversal

### Community 73 - "Community 73"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, OIDivergence

### Community 74 - "Community 74"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, BTCEquitiesCorrelationBreak

### Community 75 - "Community 75"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, DXYInverseMomentum

### Community 76 - "Community 76"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, LiquidationCascadeFade

### Community 77 - "Community 77"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, MacroDecouplingMomentum

### Community 78 - "Community 78"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, OrderbookImbalancePersistence

### Community 79 - "Community 79"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, PerpSpotBasisMomentum

### Community 80 - "Community 80"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, RiskOnOffRegimeProxy

### Community 81 - "Community 81"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, SessionHandoffMomentum

### Community 83 - "Community 83"
Cohesion: 0.33
Nodes (3): Time, BridgeStatus, PendingSignal

### Community 86 - "Community 86"
Cohesion: 0.12
Nodes (16): Collection, Database, Context, RWMutex, Time, runningPerf, Direction, avgWinLoss() (+8 more)

## Knowledge Gaps
- **293 isolated node(s):** `Mutex`, `OptionsBuyPaperPersistence`, `OptionsState`, `OptionsSellPaperPersistence`, `OptionsSellingState` (+288 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **3 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `main()` connect `Community 10` to `Community 0`, `Community 2`, `Community 27`, `Community 86`?**
  _High betweenness centrality (0.049) - this node is a cross-community bridge._
- **Why does `Orchestrator` connect `Community 0` to `Community 4`, `Community 6`, `Community 7`, `Community 82`, `Community 83`, `Community 84`?**
  _High betweenness centrality (0.042) - this node is a cross-community bridge._
- **Why does `NewOrchestrator()` connect `Community 0` to `Community 10`, `Community 4`?**
  _High betweenness centrality (0.038) - this node is a cross-community bridge._
- **Are the 5 inferred relationships involving `main()` (e.g. with `BuildCuratedScalpers()` and `SetDominanceFetcher()`) actually correct?**
  _`main()` has 5 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Mutex`, `OptionsBuyPaperPersistence`, `OptionsState` to the rest of the system?**
  _293 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.04792518994739918 - nodes in this community are weakly interconnected._
- **Should `Community 1` be split into smaller, more focused modules?**
  _Cohesion score 0.05537098560354374 - nodes in this community are weakly interconnected._