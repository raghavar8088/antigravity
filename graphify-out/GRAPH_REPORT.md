# Graph Report - antigravity-main  (2026-06-14)

## Corpus Check
- 4597 files · ~26,764,289 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 13 nodes · 12 edges · 3 communities (2 shown, 1 thin omitted)
- Extraction: 100% EXTRACTED · 0% INFERRED · 0% AMBIGUOUS
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `d8492570`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 0|Community 0]]
- [[_COMMUNITY_Community 1|Community 1]]
- [[_COMMUNITY_Community 2|Community 2]]

## God Nodes (most connected - your core abstractions)
1. `File Summary` - 5 edges
2. `fmt()` - 2 edges
3. `ScoreRow()` - 2 edges
4. `Purpose` - 1 edges
5. `File Format` - 1 edges
6. `Usage Guidelines` - 1 edges
7. `Notes` - 1 edges
8. `Directory Structure` - 1 edges
9. `StrategyRotationPanelProps` - 1 edges
10. `STATUS_COLOR` - 1 edges

## Surprising Connections (you probably didn't know these)
- None detected - all connections are within the same source files.

## Import Cycles
- None detected.

## Communities (3 total, 1 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.40
Nodes (4): fmt(), ScoreRow(), STATUS_COLOR, StrategyRotationPanelProps

### Community 1 - "Community 1"
Cohesion: 0.40
Nodes (5): File Format, File Summary, Notes, Purpose, Usage Guidelines

## Knowledge Gaps
- **7 isolated node(s):** `Purpose`, `File Format`, `Usage Guidelines`, `Notes`, `Directory Structure` (+2 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **1 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `File Summary` connect `Community 1` to `Community 2`?**
  _High betweenness centrality (0.212) - this node is a cross-community bridge._
- **What connects `Purpose`, `File Format`, `Usage Guidelines` to the rest of the system?**
  _7 weakly-connected nodes found - possible documentation gaps or missing edges._