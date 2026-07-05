# Graph Report - antigravity-main  (2026-07-05)

## Corpus Check
- 4927 files · ~27,264,655 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 121 nodes · 202 edges · 9 communities
- Extraction: 96% EXTRACTED · 4% INFERRED · 0% AMBIGUOUS · INFERRED: 8 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `4036d53b`
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

## God Nodes (most connected - your core abstractions)
1. `Metric()` - 11 edges
2. `TerminalCard()` - 9 edges
3. `Badge()` - 9 edges
4. `usd()` - 8 edges
5. `Card()` - 6 edges
6. `StatusChip()` - 6 edges
7. `File Summary` - 5 edges
8. `PortfolioAnalyticsDashboard()` - 5 edges
9. `ErrorBanner()` - 5 edges
10. `CommandCenterHome()` - 4 edges

## Surprising Connections (you probably didn't know these)
- `RiskModule()` --calls--> `usd()`  [INFERRED]
  client/src/components/terminal/institutional/RiskModule.tsx → client/src/components/terminal/institutional/PreLiveStrategiesCenter.tsx
- `metricUsd()` --calls--> `usd()`  [INFERRED]
  client/src/components/PortfolioAnalyticsDashboard.tsx → client/src/components/terminal/institutional/PreLiveStrategiesCenter.tsx
- `CommandCenterHome()` --calls--> `pct()`  [INFERRED]
  client/src/components/terminal/institutional/CommandCenterHome.tsx → client/src/components/terminal/institutional/PreLiveStrategiesCenter.tsx
- `CommandCenterHome()` --calls--> `usd()`  [INFERRED]
  client/src/components/terminal/institutional/CommandCenterHome.tsx → client/src/components/terminal/institutional/PreLiveStrategiesCenter.tsx
- `ExecutionCenter()` --calls--> `usd()`  [INFERRED]
  client/src/components/terminal/institutional/ExecutionCenter.tsx → client/src/components/terminal/institutional/PreLiveStrategiesCenter.tsx

## Import Cycles
- None detected.

## Communities (9 total, 0 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.12
Nodes (16): ALL_SEVERITIES, ALL_TYPES, EventSeverity, EventType, PlatformEvent, BadgeSize, Badge(), BadgeVariant (+8 more)

### Community 1 - "Community 1"
Cohesion: 0.15
Nodes (7): DiagnosticsPayload, HealthCheck, HealthPayload, CorrelationMatrix, RiskModule(), TerminalCard(), Metric()

### Community 2 - "Community 2"
Cohesion: 0.19
Nodes (12): dateLabel(), metricPct(), metricTone(), metricUsd(), PortfolioAnalyticsDashboard(), RegimeRow, EmptyState(), EmptyStateProps (+4 more)

### Community 3 - "Community 3"
Cohesion: 0.16
Nodes (6): ChangeHistorySection(), DIAL_CONTROLLED_KEYS, FilterMode, pillButtonStyle(), SaveStatus, ThresholdConfigCenter()

### Community 4 - "Community 4"
Cohesion: 0.19
Nodes (10): Card(), CardProps, CardTint, CardVariant, collectMetricTones(), getMetricAccentTone(), MetricSize, MetricTone (+2 more)

### Community 5 - "Community 5"
Cohesion: 0.25
Nodes (7): AnalyticsCenter(), ExecutionCenter(), pct(), PreLiveStrategiesCenter(), StrategyRow, usd(), ErrorBanner()

### Community 6 - "Community 6"
Cohesion: 0.28
Nodes (4): CommandCenterHome(), formatFreshness(), Sparkline(), SparklineProps

### Community 7 - "Community 7"
Cohesion: 0.28
Nodes (6): rate(), relativeTime(), ScalerSignalSnapshot, ScalersStatsResponse, ScalerStrategyStat, StrategiesCenter()

### Community 8 - "Community 8"
Cohesion: 0.29
Nodes (6): Directory Structure, File Format, File Summary, Notes, Purpose, Usage Guidelines

## Knowledge Gaps
- **38 isolated node(s):** `Purpose`, `File Format`, `Usage Guidelines`, `Notes`, `Directory Structure` (+33 more)
  These have ≤1 connection - possible missing edges or undocumented components.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Metric()` connect `Community 1` to `Community 2`, `Community 4`, `Community 5`, `Community 6`, `Community 7`?**
  _High betweenness centrality (0.103) - this node is a cross-community bridge._
- **Why does `TerminalCard()` connect `Community 1` to `Community 3`, `Community 6`, `Community 7`?**
  _High betweenness centrality (0.041) - this node is a cross-community bridge._
- **Why does `Badge()` connect `Community 0` to `Community 1`, `Community 3`, `Community 5`, `Community 6`, `Community 7`?**
  _High betweenness centrality (0.037) - this node is a cross-community bridge._
- **Are the 6 inferred relationships involving `usd()` (e.g. with `metricUsd()` and `AnalyticsCenter()`) actually correct?**
  _`usd()` has 6 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Purpose`, `File Format`, `Usage Guidelines` to the rest of the system?**
  _38 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.12380952380952381 - nodes in this community are weakly interconnected._