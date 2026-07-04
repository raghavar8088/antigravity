# Graph Report - antigravity-main  (2026-07-03)

## Corpus Check
- 4904 files · ~27,063,181 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 28 nodes · 32 edges · 3 communities
- Extraction: 100% EXTRACTED · 0% INFERRED · 0% AMBIGUOUS
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `89ccd93e`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 0|Community 0]]
- [[_COMMUNITY_Community 1|Community 1]]
- [[_COMMUNITY_Community 2|Community 2]]

## God Nodes (most connected - your core abstractions)
1. `PreLiveEngineCenter()` - 4 edges
2. `toMs()` - 3 edges
3. `fmtUsd()` - 2 edges
4. `fmtPrice()` - 2 edges
5. `fmtPct()` - 2 edges
6. `fmtAge()` - 2 edges
7. `fmtTime()` - 2 edges
8. `OpenPosition` - 1 edges
9. `ClosedTrade` - 1 edges
10. `Stats` - 1 edges

## Surprising Connections (you probably didn't know these)
- None detected - all connections are within the same source files.

## Import Cycles
- None detected.

## Communities (3 total, 0 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.10
Nodes (9): ClosedTrade, monoCellStyle, OpenPosition, ScalersStats, Signal, Stats, tableStyle, tdStyle (+1 more)

### Community 1 - "Community 1"
Cohesion: 0.50
Nodes (4): fmtPct(), fmtPrice(), fmtUsd(), PreLiveEngineCenter()

### Community 2 - "Community 2"
Cohesion: 0.67
Nodes (3): fmtAge(), fmtTime(), toMs()

## Knowledge Gaps
- **9 isolated node(s):** `OpenPosition`, `ClosedTrade`, `Stats`, `Signal`, `ScalersStats` (+4 more)
  These have ≤1 connection - possible missing edges or undocumented components.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `PreLiveEngineCenter()` connect `Community 1` to `Community 0`?**
  _High betweenness centrality (0.004) - this node is a cross-community bridge._
- **Why does `toMs()` connect `Community 2` to `Community 0`?**
  _High betweenness centrality (0.001) - this node is a cross-community bridge._
- **What connects `OpenPosition`, `ClosedTrade`, `Stats` to the rest of the system?**
  _9 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.09523809523809523 - nodes in this community are weakly interconnected._