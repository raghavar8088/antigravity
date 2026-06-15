# Graph Report - antigravity-main  (2026-06-15)

## Corpus Check
- 4620 files · ~26,777,883 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 19 nodes · 22 edges · 3 communities
- Extraction: 91% EXTRACTED · 9% INFERRED · 0% AMBIGUOUS · INFERRED: 2 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `6ea208cd`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 0|Community 0]]
- [[_COMMUNITY_Community 1|Community 1]]
- [[_COMMUNITY_Community 2|Community 2]]

## God Nodes (most connected - your core abstractions)
1. `CriticalDriftKillSwitchHook()` - 7 edges
2. `File Summary` - 5 edges
3. `isKillSwitchWorthyMismatch()` - 4 edges
4. `isKnownFalsePositiveMismatch()` - 3 edges
5. `TestKillSwitchHook_SkipsBalanceEquityDrift()` - 3 edges
6. `TestKillSwitchHook_TriggersOnCriticalPositionDrift()` - 3 edges
7. `Mismatch` - 2 edges
8. `T` - 2 edges
9. `Purpose` - 1 edges
10. `File Format` - 1 edges

## Surprising Connections (you probably didn't know these)
- `TestKillSwitchHook_SkipsBalanceEquityDrift()` --calls--> `CriticalDriftKillSwitchHook()`  [INFERRED]
  engine/internal/reconciliationv2/killswitch_hook_test.go → engine/internal/reconciliationv2/killswitch_hook.go
- `TestKillSwitchHook_TriggersOnCriticalPositionDrift()` --calls--> `CriticalDriftKillSwitchHook()`  [INFERRED]
  engine/internal/reconciliationv2/killswitch_hook_test.go → engine/internal/reconciliationv2/killswitch_hook.go

## Import Cycles
- None detected.

## Communities (3 total, 0 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.36
Nodes (7): CycleHook, Mismatch, MismatchDomain, CriticalDriftKillSwitchHook(), isKillSwitchWorthyMismatch(), isKnownFalsePositiveMismatch(), Service

### Community 1 - "Community 1"
Cohesion: 0.67
Nodes (3): TestKillSwitchHook_SkipsBalanceEquityDrift(), TestKillSwitchHook_TriggersOnCriticalPositionDrift(), T

### Community 2 - "Community 2"
Cohesion: 0.29
Nodes (6): Directory Structure, File Format, File Summary, Notes, Purpose, Usage Guidelines

## Knowledge Gaps
- **8 isolated node(s):** `Purpose`, `File Format`, `Usage Guidelines`, `Notes`, `Directory Structure` (+3 more)
  These have ≤1 connection - possible missing edges or undocumented components.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `CriticalDriftKillSwitchHook()` connect `Community 0` to `Community 1`?**
  _High betweenness centrality (0.261) - this node is a cross-community bridge._
- **Are the 2 inferred relationships involving `CriticalDriftKillSwitchHook()` (e.g. with `TestKillSwitchHook_SkipsBalanceEquityDrift()` and `TestKillSwitchHook_TriggersOnCriticalPositionDrift()`) actually correct?**
  _`CriticalDriftKillSwitchHook()` has 2 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Purpose`, `File Format`, `Usage Guidelines` to the rest of the system?**
  _8 weakly-connected nodes found - possible documentation gaps or missing edges._