# Graph Report - antigravity-main  (2026-07-03)

## Corpus Check
- 4904 files · ~27,062,988 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 12 nodes · 17 edges · 3 communities (2 shown, 1 thin omitted)
- Extraction: 94% EXTRACTED · 6% INFERRED · 0% AMBIGUOUS · INFERRED: 1 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `b272aae3`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 0|Community 0]]
- [[_COMMUNITY_Community 1|Community 1]]
- [[_COMMUNITY_Community 2|Community 2]]

## God Nodes (most connected - your core abstractions)
1. `BuildCuratedScalpers()` - 5 edges
2. `Performance` - 3 edges
3. `filterTradeEngineEnabled()` - 3 edges
4. `RegistryEntry` - 3 edges
5. `FilterWinnersOnly()` - 3 edges
6. `TestCuratedRegistryMatchesTradeEngineWhitelist()` - 3 edges
7. `UpdatePerformance()` - 2 edges
8. `GetPerformance()` - 2 edges
9. `AllPerformance()` - 2 edges
10. `T` - 1 edges

## Surprising Connections (you probably didn't know these)
- `TestCuratedRegistryMatchesTradeEngineWhitelist()` --calls--> `BuildCuratedScalpers()`  [INFERRED]
  engine/internal/strategy/scalpers/s30_s79_registry_shadow_test.go → engine/internal/strategy/scalpers/curated_registry.go

## Import Cycles
- None detected.

## Communities (3 total, 1 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.60
Nodes (4): Performance, AllPerformance(), GetPerformance(), UpdatePerformance()

### Community 1 - "Community 1"
Cohesion: 0.83
Nodes (4): RegistryEntry, BuildCuratedScalpers(), filterTradeEngineEnabled(), FilterWinnersOnly()

## Knowledge Gaps
- **1 isolated node(s):** `T`
  These have ≤1 connection - possible missing edges or undocumented components.
- **1 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `BuildCuratedScalpers()` connect `Community 1` to `Community 0`, `Community 2`?**
  _High betweenness centrality (0.473) - this node is a cross-community bridge._
- **Why does `TestCuratedRegistryMatchesTradeEngineWhitelist()` connect `Community 2` to `Community 1`?**
  _High betweenness centrality (0.345) - this node is a cross-community bridge._
- **What connects `T` to the rest of the system?**
  _1 weakly-connected nodes found - possible documentation gaps or missing edges._