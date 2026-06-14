# Graph Report - antigravity-main  (2026-06-14)

## Corpus Check
- 4603 files · ~26,770,719 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 308 nodes · 421 edges · 29 communities (22 shown, 7 thin omitted)
- Extraction: 100% EXTRACTED · 0% INFERRED · 0% AMBIGUOUS · INFERRED: 2 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `3d66ade0`
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
- [[_COMMUNITY_Community 13|Community 13]]
- [[_COMMUNITY_Community 14|Community 14]]
- [[_COMMUNITY_Community 15|Community 15]]
- [[_COMMUNITY_Community 22|Community 22]]
- [[_COMMUNITY_Community 23|Community 23]]
- [[_COMMUNITY_Community 24|Community 24]]
- [[_COMMUNITY_Community 25|Community 25]]
- [[_COMMUNITY_Community 26|Community 26]]
- [[_COMMUNITY_Community 27|Community 27]]
- [[_COMMUNITY_Community 28|Community 28]]

## God Nodes (most connected - your core abstractions)
1. `parseSignal()` - 8 edges
2. `TerminalCard()` - 7 edges
3. `stringValue()` - 6 edges
4. `parseStrategyStat()` - 6 edges
5. `parseScalersStats()` - 6 edges
6. `MockStageTradingSuite()` - 6 edges
7. `File Summary` - 5 edges
8. `isRecord()` - 5 edges
9. `Metric()` - 5 edges
10. `SkeletonBlock()` - 5 edges

## Surprising Connections (you probably didn't know these)
- `M3AppShell()` --calls--> `resolvePageTitle()`  [INFERRED]
  client/src/components/ui/M3AppShell.tsx → client/src/lib/utils/commandPaletteItems.ts
- `ShellNavLink()` --calls--> `isNavItemActive()`  [INFERRED]
  client/src/components/ui/M3AppShell.tsx → client/src/lib/utils/navRoutes.ts

## Import Cycles
- None detected.

## Communities (29 total, 7 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.02
Nodes (22): DESK_CHOP_DISABLED_MR_CATEGORIES, DESK_DEFAULT_REGIMES_BY_CATEGORY, DESK_HOLD_MINUTES_MUL_BY_CATEGORY, DESK_REGIME_EXTRA_TOKENS_BY_STRAT_ID, DESK_REGIME_FALLBACK_ALLOW_ALL, DESK_REGIME_IMPULSE, DESK_REGIME_LS_PAYLOAD_VERSION, DESK_REGIME_MR (+14 more)

### Community 1 - "Community 1"
Cohesion: 0.06
Nodes (33): ClosedTradesTable(), firstArray(), firstDefined(), fmtPct(), fmtPrice(), fmtUsd(), isRecord(), MockStageLayoutSections (+25 more)

### Community 2 - "Community 2"
Cohesion: 0.11
Nodes (28): NavIcon(), NavIconName, KillSwitchIndicator(), M3AppShell(), ShellNavLink(), COMMAND_PALETTE_ITEMS, CommandPaletteItem, PAGE_TITLES (+20 more)

### Community 3 - "Community 3"
Cohesion: 0.13
Nodes (18): RegimeBadge(), Card(), CardProps, CardVariant, Metric(), MetricSize, MetricTone, variantClass (+10 more)

### Community 4 - "Community 4"
Cohesion: 0.13
Nodes (12): CommandCenterHome(), formatFreshness(), fmt(), MainEngineSurvivors(), SortKey, collectMetricTones(), getMetricAccentTone(), Metric() (+4 more)

### Community 5 - "Community 5"
Cohesion: 0.18
Nodes (11): pmOff(), pmOn(), buildDiagnosticRow(), computeRollingHealthCheck(), countExitReasons(), DiagnosticSummary, gradeFromPassCount(), HealthCheckResult (+3 more)

### Community 6 - "Community 6"
Cohesion: 0.40
Nodes (3): TRADE_ENGINE_TABS, TradeEngineCenter(), TradeEngineTabKey

### Community 7 - "Community 7"
Cohesion: 0.53
Nodes (5): baseConfig, build(), traceRow(), tradeWithPnl(), wideExitConfig

### Community 8 - "Community 8"
Cohesion: 0.40
Nodes (5): applyWinnersOnlyGate(), deskEffectiveMaxOpenPerFamily(), deskMaxOpenPerTemplateFromEnv(), deskResearchModeEnabled(), deskRollingExpectancyBlocked()

### Community 9 - "Community 9"
Cohesion: 0.40
Nodes (5): buildPaperDeskStrategies(), defaultRegimesForCategory(), deskDisableChopForMrEnabled(), deskHoldMinutesCategoryMul(), mergeDeskRegimeExtras()

### Community 13 - "Community 13"
Cohesion: 0.67
Nodes (3): appendPrunedDeskRegimePersistEvent(), parseDeskRegimePersistLsPayload(), pruneDeskRegimePersistEvents()

### Community 14 - "Community 14"
Cohesion: 0.67
Nodes (3): btcFtPaperTpPctCapFromEnv(), paperDeskEffectiveStops(), paperDeskWidenPositionStops()

### Community 15 - "Community 15"
Cohesion: 0.67
Nodes (3): deskHoldTuningAnalysisModeEnabled(), deskRegimeWatchIntervalMsFromEnv(), deskRegimeWatchPollWindowFromEnv()

### Community 28 - "Community 28"
Cohesion: 0.29
Nodes (6): Directory Structure, File Format, File Summary, Notes, Purpose, Usage Guidelines

## Knowledge Gaps
- **69 isolated node(s):** `Purpose`, `File Format`, `Usage Guidelines`, `Notes`, `Directory Structure` (+64 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **7 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `TerminalCard()` connect `Community 4` to `Community 1`, `Community 3`?**
  _High betweenness centrality (0.024) - this node is a cross-community bridge._
- **Why does `SkeletonBlock()` connect `Community 3` to `Community 1`, `Community 4`?**
  _High betweenness centrality (0.011) - this node is a cross-community bridge._
- **Why does `TERMINAL_ROUTES` connect `Community 2` to `Community 4`?**
  _High betweenness centrality (0.005) - this node is a cross-community bridge._
- **What connects `Purpose`, `File Format`, `Usage Guidelines` to the rest of the system?**
  _69 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.023255813953488372 - nodes in this community are weakly interconnected._
- **Should `Community 1` be split into smaller, more focused modules?**
  _Cohesion score 0.0573025856044724 - nodes in this community are weakly interconnected._
- **Should `Community 2` be split into smaller, more focused modules?**
  _Cohesion score 0.1051693404634581 - nodes in this community are weakly interconnected._