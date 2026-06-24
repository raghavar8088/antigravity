# Graph Report - antigravity-main  (2026-06-23)

## Corpus Check
- 4746 files · ~26,868,447 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 165 nodes · 333 edges · 19 communities (6 shown, 13 thin omitted)
- Extraction: 100% EXTRACTED · 0% INFERRED · 0% AMBIGUOUS · INFERRED: 1 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `31725aa2`
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
1. `Orchestrator` - 106 edges
2. `Context` - 23 edges
3. `NewOrchestrator()` - 11 edges
4. `Signal` - 9 edges
5. `targetSizeForCapital()` - 6 edges
6. `PendingSignal` - 5 edges
7. `FillResult` - 5 edges
8. `computeRSI()` - 5 edges
9. `sanitizeSignalForProfit()` - 5 edges
10. `generateAutoPrompt()` - 5 edges

## Surprising Connections (you probably didn't know these)
- `NewOrchestrator()` --references--> `Orchestrator`  [EXTRACTED]
  engine/internal/trading/loop.go → engine/internal/trading/loop.go  _Bridges community 2 → community 5_
- `Orchestrator` --references--> `Bridge`  [EXTRACTED]
  engine/internal/trading/loop.go → engine/internal/trading/loop.go  _Bridges community 2 → community 11_
- `Orchestrator` --references--> `Candle`  [EXTRACTED]
  engine/internal/trading/loop.go → engine/internal/trading/loop.go  _Bridges community 2 → community 7_
- `Orchestrator` --references--> `MultiAgentOrchestrator`  [EXTRACTED]
  engine/internal/trading/loop.go → engine/internal/trading/loop.go  _Bridges community 2 → community 12_
- `Orchestrator` --references--> `PortfolioLedger`  [EXTRACTED]
  engine/internal/trading/loop.go → engine/internal/trading/loop.go  _Bridges community 2 → community 13_

## Import Cycles
- None detected.

## Communities (19 total, 13 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.08
Nodes (25): Action, IndicatorSnapshot, MarketContext, Regime, Side, Time, AgentSignalForExec, BridgeStatus (+17 more)

### Community 1 - "Community 1"
Cohesion: 0.13
Nodes (15): Event, EventType, FillResult, OrderMode, OrderPayload, Position, Signal, BridgeDecision (+7 more)

### Community 2 - "Community 2"
Cohesion: 0.09
Nodes (16): AdaptiveConfidenceFloor, CandleSummary, ConcentrationGate, ExecutionWatchdog, KillSwitch, LoopDeps, Mutex, PaperPersistBundle (+8 more)

### Community 3 - "Community 3"
Cohesion: 0.17
Nodes (3): CloseEvent, Context, Tick

### Community 4 - "Community 4"
Cohesion: 0.16
Nodes (5): Duration, FillResult, OrderMode, RouteModeForCategory(), signalMaxAge()

### Community 5 - "Community 5"
Cohesion: 0.20
Nodes (10): CandleAggregator, Manager, MarketDataClient, PaperClient, RegistryEntry, RiskEngine, SignalAggregator, StrategyTracker (+2 more)

## Knowledge Gaps
- **24 isolated node(s):** `StrategyGroups`, `Tracker`, `CandleSummary`, `Mutex`, `RWMutex` (+19 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **13 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Orchestrator` connect `Community 2` to `Community 0`, `Community 1`, `Community 3`, `Community 4`, `Community 5`, `Community 6`, `Community 7`, `Community 8`, `Community 9`, `Community 10`, `Community 11`, `Community 12`, `Community 13`, `Community 14`, `Community 15`, `Community 16`, `Community 17`, `Community 18`?**
  _High betweenness centrality (0.798) - this node is a cross-community bridge._
- **Why does `Context` connect `Community 3` to `Community 1`, `Community 4`, `Community 9`?**
  _High betweenness centrality (0.025) - this node is a cross-community bridge._
- **What connects `StrategyGroups`, `Tracker`, `CandleSummary` to the rest of the system?**
  _24 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.07954545454545454 - nodes in this community are weakly interconnected._
- **Should `Community 1` be split into smaller, more focused modules?**
  _Cohesion score 0.12535612535612536 - nodes in this community are weakly interconnected._
- **Should `Community 2` be split into smaller, more focused modules?**
  _Cohesion score 0.09420289855072464 - nodes in this community are weakly interconnected._