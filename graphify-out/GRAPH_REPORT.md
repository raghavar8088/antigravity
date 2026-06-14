# Graph Report - antigravity-main  (2026-06-14)

## Corpus Check
- 4596 files · ~26,763,771 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 19 nodes · 18 edges · 3 communities
- Extraction: 100% EXTRACTED · 0% INFERRED · 0% AMBIGUOUS
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `509a31a1`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 0|Community 0]]
- [[_COMMUNITY_Community 1|Community 1]]
- [[_COMMUNITY_Community 2|Community 2]]

## God Nodes (most connected - your core abstractions)
1. `File Summary` - 5 edges
2. `StrategyDiagnosticRow` - 2 edges
3. `Purpose` - 1 edges
4. `File Format` - 1 edges
5. `Usage Guidelines` - 1 edges
6. `Notes` - 1 edges
7. `Directory Structure` - 1 edges
8. `HealthCheckResult` - 1 edges
9. `DiagnosticSummary` - 1 edges
10. `RotationStatus` - 1 edges

## Surprising Connections (you probably didn't know these)
- None detected - all connections are within the same source files.

## Import Cycles
- None detected.

## Communities (3 total, 0 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.25
Nodes (3): RotationReport, RotationScore, RotationStatus

### Community 1 - "Community 1"
Cohesion: 0.50
Nodes (3): DiagnosticSummary, HealthCheckResult, StrategyDiagnosticRow

### Community 2 - "Community 2"
Cohesion: 0.29
Nodes (6): Directory Structure, File Format, File Summary, Notes, Purpose, Usage Guidelines

## Knowledge Gaps
- **10 isolated node(s):** `Purpose`, `File Format`, `Usage Guidelines`, `Notes`, `Directory Structure` (+5 more)
  These have ≤1 connection - possible missing edges or undocumented components.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **What connects `Purpose`, `File Format`, `Usage Guidelines` to the rest of the system?**
  _10 weakly-connected nodes found - possible documentation gaps or missing edges._