# Graph Report - antigravity-main  (2026-06-14)

## Corpus Check
- 4537 files · ~26,734,228 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 140 nodes · 197 edges · 16 communities (14 shown, 2 thin omitted)
- Extraction: 96% EXTRACTED · 4% INFERRED · 0% AMBIGUOUS · INFERRED: 7 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `bbebced3`
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
- [[_COMMUNITY_Community 15|Community 15]]

## God Nodes (most connected - your core abstractions)
1. `File Summary` - 5 edges
2. `StrategyMetadata` - 5 edges
3. `RegistryEntry` - 5 edges
4. `resolveBtcFtActiveStrategyIds()` - 4 edges
5. `TestAIStrategyLibrarySummaryIsEmpty()` - 4 edges
6. `BuildCuratedScalpers()` - 4 edges
7. `MetadataFromRegistry()` - 4 edges
8. `TierFromCategory()` - 4 edges
9. `BuildAllScalpers()` - 4 edges
10. `BTC_FT_PREMIUM_STRATEGY_IDS` - 3 edges

## Surprising Connections (you probably didn't know these)
- `TestAIStrategyLibraryIsEmpty()` --calls--> `GetAIStrategyLibrary()`  [INFERRED]
  engine/internal/ai/strategy_library_test.go → engine/internal/ai/strategy_library.go
- `TestAIStrategyLibrarySummaryIsEmpty()` --calls--> `SummarizeAIStrategyLibrary()`  [INFERRED]
  engine/internal/ai/strategy_library_test.go → engine/internal/ai/strategy_library.go
- `TestAIStrategyLibrarySummaryIsEmpty()` --calls--> `GetAIStrategyCategories()`  [INFERRED]
  engine/internal/ai/strategy_library_test.go → engine/internal/ai/strategy_library.go
- `TestBuildAllScalpersHasNoAlphaStrategies()` --calls--> `BuildAllScalpers()`  [INFERRED]
  engine/internal/strategy/alpha_registry_test.go → engine/internal/strategy/registry.go
- `TestBuildExpansionPackIsEmpty()` --calls--> `buildExpansionPack()`  [INFERRED]
  engine/internal/strategy/curated_registry_test.go → engine/internal/strategy/curated_expansion_pack.go

## Import Cycles
- None detected.

## Communities (16 total, 2 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.15
Nodes (14): ALL_INSTITUTIONAL_FAMILIES, INSTITUTIONAL_FAMILY_LABELS, INSTITUTIONAL_STRATEGIES, INSTITUTIONAL_STRATEGY_BY_ID, INSTITUTIONAL_STRATEGY_IDS, InstitutionalFamily, InstitutionalStrategy, InstitutionalStrategyMeta (+6 more)

### Community 1 - "Community 1"
Cohesion: 0.20
Nodes (15): T, RegistryEntry, Strategy, TestBuildAllScalpersHasNoAlphaStrategies(), BuildAllScalpers(), GetStrategyNames(), GroupByTimeframe(), MetadataFromCategory() (+7 more)

### Community 2 - "Community 2"
Cohesion: 0.13
Nodes (8): BTC_FT_EXTENDED_STRATEGY_IDS, BTC_FT_GENERATED_STRATEGY_IDS, BTC_FT_RESEARCH_CATEGORY_IDS, BTC_FT_RESEARCH_FULL_POOL, BtcFtRosterResolution, BtcFtRosterSource, BtcFtStrategyRankingRow, CATEGORY_STRATEGY_IDS

### Community 3 - "Community 3"
Cohesion: 0.16
Nodes (11): RegistryEntry, RegistryEntry, T, T, buildExpansionPack(), BuildCuratedScalpers(), FilterWinnersOnly(), TestBuildCuratedScalpersIsEmpty() (+3 more)

### Community 4 - "Community 4"
Cohesion: 0.23
Nodes (9): AIStrategyBlueprint, GetAIStrategyCategories(), GetAIStrategyLibrary(), SummarizeAIStrategyLibrary(), TestAIStrategyLibraryIsEmpty(), TestAIStrategyLibrarySummaryIsEmpty(), StrategyLibrarySummary, StrategySupportLevel (+1 more)

### Community 5 - "Community 5"
Cohesion: 0.32
Nodes (10): BREAKOUT_TRADING_STRATEGIES, CATEGORY_POOL_160, DAY_TRADING_STRATEGIES, LEGACY_CORE_CATEGORY_MAP, MOMENTUM_TRADING_STRATEGIES, POSITION_TRADING_STRATEGIES, RANGE_TRADING_STRATEGIES, SCALPING_STRATEGIES (+2 more)

### Community 6 - "Community 6"
Cohesion: 0.36
Nodes (7): ALL_RESEARCH_FAMILIES, RESEARCH_FAMILIES_WITH_STRATEGIES, RESEARCH_FAMILY_LABELS, RESEARCH_STRATEGIES, RESEARCH_STRATEGY_BY_ID, ResearchFamily, ResearchStrategy

### Community 7 - "Community 7"
Cohesion: 0.29
Nodes (6): BTC_FUTURE_TRADING_STRATEGY_IDS, btcFtActiveCategoryFromEnv(), btcFtResearchModeEnabled(), buildCategoryRoster(), CORE_BTC_FT_STRATEGY_IDS, resolveBtcFtActiveStrategyIds()

### Community 8 - "Community 8"
Cohesion: 0.38
Nodes (4): BTC_FT_EXTENDED_STRATEGY_IDS, BTC_FUTURE_TRADING_CORE_STRATEGY_IDS, BTC_FUTURE_TRADING_STRATEGY_IDS, FUTURES_STRAT_DEFS

### Community 10 - "Community 10"
Cohesion: 0.67
Nodes (3): StrategyDef, buildAllStrategies(), BuildStrategies()

### Community 11 - "Community 11"
Cohesion: 0.67
Nodes (3): StrategyDef, buildAllStrategies(), BuildStrategies()

### Community 12 - "Community 12"
Cohesion: 0.67
Nodes (3): T, TestBuildAllStrategiesIsEmpty(), TestBuildStrategiesIsEmpty()

### Community 15 - "Community 15"
Cohesion: 0.29
Nodes (6): Directory Structure, File Format, File Summary, Notes, Purpose, Usage Guidelines

## Knowledge Gaps
- **32 isolated node(s):** `Purpose`, `File Format`, `Usage Guidelines`, `Notes`, `Directory Structure` (+27 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **2 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Are the 2 inferred relationships involving `TestAIStrategyLibrarySummaryIsEmpty()` (e.g. with `GetAIStrategyCategories()` and `SummarizeAIStrategyLibrary()`) actually correct?**
  _`TestAIStrategyLibrarySummaryIsEmpty()` has 2 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Purpose`, `File Format`, `Usage Guidelines` to the rest of the system?**
  _32 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 2` be split into smaller, more focused modules?**
  _Cohesion score 0.13333333333333333 - nodes in this community are weakly interconnected._