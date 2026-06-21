# Graph Report - antigravity-main  (2026-06-21)

## Corpus Check
- 4741 files · ~26,860,670 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 246 nodes · 485 edges · 24 communities (11 shown, 13 thin omitted)
- Extraction: 99% EXTRACTED · 1% INFERRED · 0% AMBIGUOUS · INFERRED: 4 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `e133e574`
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

## God Nodes (most connected - your core abstractions)
1. `Orchestrator` - 100 edges
2. `ScalerBundle` - 38 edges
3. `Context` - 23 edges
4. `NewOrchestrator()` - 12 edges
5. `Candle` - 10 edges
6. `Signal` - 9 edges
7. `ApplyStrictnessDial()` - 7 edges
8. `mdToScalerCandle()` - 7 edges
9. `appendCapped()` - 7 edges
10. `interpolate()` - 6 edges

## Surprising Connections (you probably didn't know these)
- `NewOrchestrator()` --calls--> `newScalerBundle()`  [INFERRED]
  engine/internal/trading/loop.go → engine/internal/trading/scalers_eval.go
- `TestSanitizeScalerSignalUsesConfiguredRRFloor()` --calls--> `sanitizeScalerSignal()`  [INFERRED]
  engine/internal/trading/strictness_wiring_test.go → engine/internal/trading/scalers_eval.go
- `TestSanitizeScalerSignalUsesConfiguredRRFloor()` --calls--> `RefreshThresholdsFromRegistry()`  [INFERRED]
  engine/internal/trading/strictness_wiring_test.go → engine/internal/trading/loop.go
- `TestStrictnessDialHotReloadsWiredThresholds()` --calls--> `RefreshThresholdsFromRegistry()`  [INFERRED]
  engine/internal/trading/strictness_wiring_test.go → engine/internal/trading/loop.go

## Import Cycles
- None detected.

## Communities (24 total, 13 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.11
Nodes (7): CloseEvent, Context, Tick, IndicatorSnapshot, Snapshot, buildIndicatorSnapshotFromPrices(), computeRSI()

### Community 1 - "Community 1"
Cohesion: 0.08
Nodes (26): Action, MarketContext, Regime, Side, T, AgentSignalForExec, BridgeDecision, InstitutionalPathOpts (+18 more)

### Community 2 - "Community 2"
Cohesion: 0.09
Nodes (16): AdaptiveConfidenceFloor, CandleSummary, ConcentrationGate, Mutex, RWMutex, Orchestrator, KillSwitch, MultiAgentOrchestrator (+8 more)

### Community 3 - "Community 3"
Cohesion: 0.10
Nodes (9): Context, Duration, Mutex, RegistryEntry, RWMutex, Tick, MetaLabelFilter, OrderBookSnapshot (+1 more)

### Community 4 - "Community 4"
Cohesion: 0.13
Nodes (14): Signal, Event, EventType, FillResult, MarketRegime, MarketState, OrderMode, OrderPayload (+6 more)

### Community 5 - "Community 5"
Cohesion: 0.16
Nodes (8): MarketContext, Regime, Time, Orchestrator, ScalersSignalSnapshot, mapRegimeClassToScalersRegime(), tradingSession(), StrategyPerformance

### Community 6 - "Community 6"
Cohesion: 0.20
Nodes (11): AggregatedSignal, Signal, containsAny(), newScalerBundle(), sanitizeScalerSignal(), scalerSignalToLegacy(), scalersSignalSnapshots(), scalerStrategyCooldown() (+3 more)

### Community 7 - "Community 7"
Cohesion: 0.39
Nodes (11): ApplyStrictnessDial(), BuildStrictnessProfiles(), clampF(), ComputeStrictnessValues(), DetectDialPosition(), EstimatedImpactPct(), interpolate(), StrictnessProfile (+3 more)

### Community 8 - "Community 8"
Cohesion: 0.20
Nodes (10): CandleAggregator, RegistryEntry, Manager, MarketDataClient, PaperClient, RiskEngine, SignalAggregator, StrategyTracker (+2 more)

### Community 9 - "Community 9"
Cohesion: 0.49
Nodes (5): Candle, aggregate4h(), aggregate5mBars(), appendCapped(), mdToScalerCandle()

### Community 13 - "Community 13"
Cohesion: 0.29
Nodes (6): Directory Structure, File Format, File Summary, Notes, Purpose, Usage Guidelines

## Knowledge Gaps
- **40 isolated node(s):** `Purpose`, `File Format`, `Usage Guidelines`, `Notes`, `Directory Structure` (+35 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **13 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Orchestrator` connect `Community 2` to `Community 0`, `Community 1`, `Community 4`, `Community 8`, `Community 10`, `Community 11`, `Community 12`, `Community 14`, `Community 16`, `Community 17`, `Community 18`, `Community 19`, `Community 20`, `Community 21`, `Community 22`, `Community 23`?**
  _High betweenness centrality (0.565) - this node is a cross-community bridge._
- **Why does `NewOrchestrator()` connect `Community 8` to `Community 1`, `Community 2`, `Community 6`?**
  _High betweenness centrality (0.342) - this node is a cross-community bridge._
- **Why does `newScalerBundle()` connect `Community 6` to `Community 8`, `Community 3`?**
  _High betweenness centrality (0.334) - this node is a cross-community bridge._
- **What connects `Purpose`, `File Format`, `Usage Guidelines` to the rest of the system?**
  _40 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.10606060606060606 - nodes in this community are weakly interconnected._
- **Should `Community 1` be split into smaller, more focused modules?**
  _Cohesion score 0.08067226890756303 - nodes in this community are weakly interconnected._
- **Should `Community 2` be split into smaller, more focused modules?**
  _Cohesion score 0.09486166007905138 - nodes in this community are weakly interconnected._