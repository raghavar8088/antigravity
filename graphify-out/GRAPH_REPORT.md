# Graph Report - antigravity-main  (2026-07-04)

## Corpus Check
- 4913 files · ~27,182,838 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 137 nodes · 223 edges · 17 communities (10 shown, 7 thin omitted)
- Extraction: 99% EXTRACTED · 1% INFERRED · 0% AMBIGUOUS · INFERRED: 2 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `955e5fd8`
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

## God Nodes (most connected - your core abstractions)
1. `Manager` - 30 edges
2. `Position` - 14 edges
3. `Backtest → Qualification → Live Pipeline (Reference Procedure)` - 10 edges
4. `Regime` - 10 edges
5. `MarketContext` - 10 edges
6. `Signal` - 10 edges
7. `main()` - 9 edges
8. `regimePnLTracker` - 6 edges
9. `File Summary` - 5 edges
10. `CloseReason` - 5 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `NewManagerWithConfig()`  [INFERRED]
  engine/cmd/pre_live/main.go → engine/internal/positions/manager.go
- `BuildAllScalpers()` --calls--> `buildNewStrategiesBatch10()`  [INFERRED]
  engine/internal/strategy/scalpers/registry.go → engine/internal/strategy/scalpers/new10_batch.go

## Import Cycles
- None detected.

## Communities (17 total, 7 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.12
Nodes (14): Action, CloseQueue, CloseQueueMetrics, RWMutex, Signal, CloseEvent, CloseReason, Manager (+6 more)

### Community 1 - "Community 1"
Cohesion: 0.17
Nodes (15): CloseReason, RegistryEntry, RWMutex, Mutex, Orchestrator, loadDotEnv(), main(), newRegimePnLTracker() (+7 more)

### Community 2 - "Community 2"
Cohesion: 0.12
Nodes (6): Regime, EMA8CrossWMA13FisherADXLong, FisherZeroCMFADXShort, RSIDeepMidCrossShort, WMA9CrossWMA21MACDADXShort, WMA9EMA21FisherADXLong

### Community 3 - "Community 3"
Cohesion: 0.18
Nodes (10): 1. Architecture map, 2. Adding new strategies, 3. Fetch/verify historical data (only if date range not already cached), 4. Run the backtest, 5. Read qualification results, 6. Promote qualified strategies, 7. Add to the live trade-engine whitelist (final manual gate), 8. Demotion (ongoing, automatic once live) (+2 more)

### Community 4 - "Community 4"
Cohesion: 0.32
Nodes (5): Signal, MarketContext, hwSLTPMidPlusShort(), hwSLTPTightTPShort(), hwSLTPWideTP()

### Community 5 - "Community 5"
Cohesion: 0.29
Nodes (6): Directory Structure, File Format, File Summary, Notes, Purpose, Usage Guidelines

### Community 6 - "Community 6"
Cohesion: 0.43
Nodes (6): RegistryEntry, RegistryEntry, buildNewStrategiesBatch10(), BuildAllScalpers(), BuildPortedStrategies(), buildPortedStrategiesBase()

## Knowledge Gaps
- **27 isolated node(s):** `Purpose`, `File Format`, `Usage Guidelines`, `Notes`, `Directory Structure` (+22 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **7 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `main()` connect `Community 1` to `Community 0`?**
  _High betweenness centrality (0.068) - this node is a cross-community bridge._
- **Why does `NewManagerWithConfig()` connect `Community 0` to `Community 1`?**
  _High betweenness centrality (0.066) - this node is a cross-community bridge._
- **What connects `Purpose`, `File Format`, `Usage Guidelines` to the rest of the system?**
  _27 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.12310606060606061 - nodes in this community are weakly interconnected._
- **Should `Community 2` be split into smaller, more focused modules?**
  _Cohesion score 0.11764705882352941 - nodes in this community are weakly interconnected._