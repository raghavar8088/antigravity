# Graph Report - antigravity-main  (2026-06-20)

## Corpus Check
- 4698 files · ~26,821,439 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 16 nodes · 18 edges · 3 communities (1 shown, 2 thin omitted)
- Extraction: 100% EXTRACTED · 0% INFERRED · 0% AMBIGUOUS
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `72388aa5`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 0|Community 0]]
- [[_COMMUNITY_Community 1|Community 1]]
- [[_COMMUNITY_Community 2|Community 2]]

## God Nodes (most connected - your core abstractions)
1. `File Summary` - 5 edges
2. `NavIcon()` - 3 edges
3. `LogoutButton()` - 2 edges
4. `Purpose` - 1 edges
5. `File Format` - 1 edges
6. `Usage Guidelines` - 1 edges
7. `Notes` - 1 edges
8. `Directory Structure` - 1 edges
9. `NavIconName` - 1 edges
10. `M3AppShellProps` - 1 edges

## Surprising Connections (you probably didn't know these)
- None detected - all connections are within the same source files.

## Import Cycles
- None detected.

## Communities (3 total, 2 thin omitted)

### Community 2 - "Community 2"
Cohesion: 0.29
Nodes (6): Directory Structure, File Format, File Summary, Notes, Purpose, Usage Guidelines

## Knowledge Gaps
- **7 isolated node(s):** `Purpose`, `File Format`, `Usage Guidelines`, `Notes`, `Directory Structure` (+2 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **2 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **What connects `Purpose`, `File Format`, `Usage Guidelines` to the rest of the system?**
  _7 weakly-connected nodes found - possible documentation gaps or missing edges._