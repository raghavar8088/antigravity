# Graph Report - antigravity-main  (2026-06-24)

## Corpus Check
- 4824 files · ~26,913,509 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 218 nodes · 332 edges · 19 communities
- Extraction: 79% EXTRACTED · 21% INFERRED · 0% AMBIGUOUS · INFERRED: 70 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `4fbc4866`
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

## God Nodes (most connected - your core abstractions)
1. `Candle` - 28 edges
2. `ATR()` - 23 edges
3. `AvgVolume()` - 10 edges
4. `SwingHigh()` - 10 edges
5. `SwingLow()` - 10 edges
6. `EMA()` - 8 edges
7. `RSI()` - 8 edges
8. `SqueezeDetector()` - 7 edges
9. `BB()` - 6 edges
10. `MACD()` - 6 edges

## Surprising Connections (you probably didn't know these)
- `BuildCuratedScalpers()` --calls--> `buildImmortalEditionPack()`  [INFERRED]
  engine/internal/strategy/scalpers/curated_registry.go → engine/internal/strategy/scalpers/s80_s99_immortal_edition.go

## Import Cycles
- None detected.

## Communities (19 total, 0 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.16
Nodes (23): Candle, AroonResult, BollingerBands, ADX(), Aroon(), BB(), BBWidthPercentile(), EMA() (+15 more)

### Community 1 - "Community 1"
Cohesion: 0.09
Nodes (14): MarketContext, Regime, Signal, MarketContext, Regime, Signal, MarketContext, Regime (+6 more)

### Community 2 - "Community 2"
Cohesion: 0.13
Nodes (9): MarketContext, Regime, Signal, MarketContext, Regime, Signal, ElderTripleScreen, RSI() (+1 more)

### Community 3 - "Community 3"
Cohesion: 0.25
Nodes (5): MarketContext, Regime, Signal, ChandeMomentumOscillator, CMO()

### Community 4 - "Community 4"
Cohesion: 0.25
Nodes (9): RegistryEntry, RegistryEntry, Performance, AllPerformance(), BuildCuratedScalpers(), FilterWinnersOnly(), GetPerformance(), UpdatePerformance() (+1 more)

### Community 5 - "Community 5"
Cohesion: 0.25
Nodes (5): MarketContext, Regime, Signal, CoppockCurveMomentum, CoppockCurve()

### Community 6 - "Community 6"
Cohesion: 0.25
Nodes (5): MarketContext, Regime, Signal, FibRetracement(), TrendContinuationPullback

### Community 7 - "Community 7"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, KnowSureThingMomentum

### Community 8 - "Community 8"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, AroonTrendConfirmation

### Community 9 - "Community 9"
Cohesion: 0.13
Nodes (9): MarketContext, Regime, Signal, MarketContext, Regime, Signal, AvgVolume(), InsideBarMomentum (+1 more)

### Community 10 - "Community 10"
Cohesion: 0.25
Nodes (5): MarketContext, Regime, Signal, NarrowRangeN(), NR7VolatilityCompression

### Community 11 - "Community 11"
Cohesion: 0.22
Nodes (7): MarketContext, Regime, Signal, ATR(), SqueezeDetector(), SqueezeMomentumBreakout, SqueezeResult

### Community 12 - "Community 12"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, AdaptiveTrendFollowing

### Community 13 - "Community 13"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, MorningEveningStar

### Community 14 - "Community 14"
Cohesion: 0.25
Nodes (5): MarketContext, Regime, Signal, VolumeProfilePOC(), VolumePOCMagnet

### Community 15 - "Community 15"
Cohesion: 0.25
Nodes (5): MarketContext, Regime, Signal, OBV(), OBVDivergence

### Community 16 - "Community 16"
Cohesion: 0.25
Nodes (5): MarketContext, Regime, Signal, ChaikinMoneyFlowSignal, ChaikinMoneyFlow()

### Community 17 - "Community 17"
Cohesion: 0.29
Nodes (4): MarketContext, Regime, Signal, MultiIndicatorConsensus

### Community 18 - "Community 18"
Cohesion: 0.29
Nodes (6): Directory Structure, File Format, File Summary, Notes, Purpose, Usage Guidelines

## Knowledge Gaps
- **66 isolated node(s):** `Purpose`, `File Format`, `Usage Guidelines`, `Notes`, `Directory Structure` (+61 more)
  These have ≤1 connection - possible missing edges or undocumented components.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `ATR()` connect `Community 11` to `Community 0`, `Community 1`, `Community 2`, `Community 3`, `Community 5`, `Community 6`, `Community 7`, `Community 8`, `Community 9`, `Community 10`, `Community 12`, `Community 13`, `Community 14`, `Community 15`, `Community 16`, `Community 17`?**
  _High betweenness centrality (0.503) - this node is a cross-community bridge._
- **Why does `Candle` connect `Community 0` to `Community 1`, `Community 2`, `Community 3`, `Community 5`, `Community 9`, `Community 10`, `Community 11`, `Community 14`, `Community 15`, `Community 16`?**
  _High betweenness centrality (0.073) - this node is a cross-community bridge._
- **Why does `AvgVolume()` connect `Community 9` to `Community 0`, `Community 1`, `Community 2`, `Community 3`, `Community 10`, `Community 13`, `Community 14`?**
  _High betweenness centrality (0.060) - this node is a cross-community bridge._
- **Are the 20 inferred relationships involving `ATR()` (e.g. with `.Evaluate()` and `.Evaluate()`) actually correct?**
  _`ATR()` has 20 INFERRED edges - model-reasoned connections that need verification._
- **Are the 8 inferred relationships involving `AvgVolume()` (e.g. with `.Evaluate()` and `.Evaluate()`) actually correct?**
  _`AvgVolume()` has 8 INFERRED edges - model-reasoned connections that need verification._
- **Are the 8 inferred relationships involving `SwingHigh()` (e.g. with `.Evaluate()` and `.Evaluate()`) actually correct?**
  _`SwingHigh()` has 8 INFERRED edges - model-reasoned connections that need verification._
- **Are the 8 inferred relationships involving `SwingLow()` (e.g. with `.Evaluate()` and `.Evaluate()`) actually correct?**
  _`SwingLow()` has 8 INFERRED edges - model-reasoned connections that need verification._