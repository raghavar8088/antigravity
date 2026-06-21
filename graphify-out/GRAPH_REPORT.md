# Graph Report - .  (2026-06-21)

## Corpus Check
- cluster-only mode — file stats not available

## Summary
- 10 nodes · 8 edges · 2 communities (0 shown, 2 thin omitted)
- Extraction: 100% EXTRACTED · 0% INFERRED · 0% AMBIGUOUS
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `75ecb3df`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 0|Community 0]]
- [[_COMMUNITY_Community 1|Community 1]]

## God Nodes (most connected - your core abstractions)
1. `ENGINE_BASE` - 1 edges
2. `SetRequestBody` - 1 edges
3. `ENGINE_BASE` - 1 edges
4. `SetRequestBody` - 1 edges

## Surprising Connections (you probably didn't know these)
- None detected - all connections are within the same source files.

## Import Cycles
- None detected.

## Communities (2 total, 2 thin omitted)

## Knowledge Gaps
- **4 isolated node(s):** `ENGINE_BASE`, `SetRequestBody`, `ENGINE_BASE`, `SetRequestBody`
  These have ≤1 connection - possible missing edges or undocumented components.
- **2 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **What connects `ENGINE_BASE`, `SetRequestBody`, `ENGINE_BASE` to the rest of the system?**
  _4 weakly-connected nodes found - possible documentation gaps or missing edges._