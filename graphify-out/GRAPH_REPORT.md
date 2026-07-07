# Graph Report - antigravity-main  (2026-07-07)

## Corpus Check
- 4931 files · ~27,266,081 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 114 nodes · 207 edges · 8 communities
- Extraction: 94% EXTRACTED · 6% INFERRED · 0% AMBIGUOUS · INFERRED: 12 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `a6a54f76`
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

## God Nodes (most connected - your core abstractions)
1. `newEngine()` - 16 edges
2. `Context` - 12 edges
3. `inMemoryExchangeAdapter` - 12 edges
4. `T` - 11 edges
5. `T` - 8 edges
6. `T` - 8 edges
7. `faultStore` - 7 edges
8. `restoreFromSnapshot()` - 7 edges
9. `T` - 6 edges
10. `takeDRSnapshot()` - 6 edges

## Surprising Connections (you probably didn't know these)
- `TestStress_KillSwitchActivatesUnderCrash()` --calls--> `newEngine()`  [INFERRED]
  engine/internal/certification/stress_certification_test.go → engine/internal/certification/reconciliation_certification_test.go
- `TestStress_RiskGateBlocksExtremeExposure()` --calls--> `newEngine()`  [INFERRED]
  engine/internal/certification/stress_certification_test.go → engine/internal/certification/reconciliation_certification_test.go
- `TestClearHistoryKeepsOpenPositions()` --calls--> `newEngine()`  [INFERRED]
  engine/internal/options/engine_test.go → engine/internal/certification/reconciliation_certification_test.go
- `TestMarkToMarketCanExitEarlyToProtectGrindProfit()` --calls--> `newEngine()`  [INFERRED]
  engine/internal/options/engine_test.go → engine/internal/certification/reconciliation_certification_test.go
- `TestMarkToMarketKeepsPositionOpenWhenPremiumSlightlyOff()` --calls--> `newEngine()`  [INFERRED]
  engine/internal/options/engine_test.go → engine/internal/certification/reconciliation_certification_test.go

## Import Cycles
- None detected.

## Communities (8 total, 0 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.15
Nodes (11): AccountState, AssetBalance, inMemoryExchangeAdapter, noopRepairTarget, Context, Time, ExchangeFill, ExchangeOrder (+3 more)

### Community 1 - "Community 1"
Cohesion: 0.23
Nodes (14): AggregateType, newFaultStore(), TestChaos_ConcurrentWritesNoRace(), TestChaos_EventHashTampering(), TestChaos_KillSwitchUnderLoad(), TestChaos_LedgerDiskExhaustion(), TestChaos_LedgerTransientFailure(), TestChaos_OMSv3_ConcurrentOrderProcessing() (+6 more)

### Community 2 - "Community 2"
Cohesion: 0.23
Nodes (14): inMemoryOMSReader, newEngine(), TestReconciliation_AuditEventsPersistedToLedger(), TestReconciliation_DetectsBalanceDrift(), TestReconciliation_DeterministicAfterReplay(), TestReconciliation_HighFrequency(), TestReconciliation_MultipleExchanges(), TestReconciliation_NoBalanceDrift() (+6 more)

### Community 3 - "Community 3"
Cohesion: 0.27
Nodes (13): restoreFromSnapshot(), takeDRSnapshot(), TestDR_KillSwitchSurvivesRestart(), TestDR_LedgerReplayAfterCrash(), TestDR_ProjectionRebuildAfterRestart(), TestDR_RPO_MeasureEventLoss(), TestDR_SnapshotAndFullRecovery(), drSnapshot (+5 more)

### Community 4 - "Community 4"
Cohesion: 0.28
Nodes (12): T, TestAggregateStatsIncludesOpenOptionMarketValueInEquity(), TestClassifyMarketRegimeRecognizesTrend(), TestClearHistoryKeepsOpenPositions(), TestLiveSizeMultiplierRewardsProfitableStrategies(), TestLongOptionStrategiesScaledProfitTargets(), TestMarkToMarketCanExitEarlyToProtectGrindProfit(), TestMarkToMarketKeepsPositionOpenWhenPremiumSlightlyOff() (+4 more)

### Community 5 - "Community 5"
Cohesion: 0.31
Nodes (8): TestExchangeFailure_DuplicateExchangeMessage(), TestExchangeFailure_OutagePreventsDualExecution(), TestExchangeFailure_RejectStorm(), TestExchangeFailure_TimeoutNoOrphan(), TestExchangeFailure_WebsocketReconnectNoGhostPosition(), mockExchangeAdapter, Context, T

### Community 6 - "Community 6"
Cohesion: 0.36
Nodes (9): TestSecurity_ContextCancellationPropagates(), TestSecurity_DuplicateEventIDRejected(), TestSecurity_HashTamperingRejected(), TestSecurity_IdempotencyKeyPreventsFillReplay(), TestSecurity_KillSwitchAuditTrailImmutable(), TestSecurity_KillSwitchRequiresTrigger(), TestSecurity_LedgerRejectsEmptyAggregateType(), TestSecurity_OrderTransitionGraphEnforced() (+1 more)

### Community 7 - "Community 7"
Cohesion: 0.31
Nodes (6): inactiveKS, TestStress_KillSwitchActivatesUnderCrash(), TestStress_LedgerIntegrityAfterCrash(), TestStress_RiskGateBlocksExtremeExposure(), StressScenario, T

## Knowledge Gaps
- **14 isolated node(s):** `Store`, `AggregateType`, `Time`, `Event`, `Context` (+9 more)
  These have ≤1 connection - possible missing edges or undocumented components.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `newEngine()` connect `Community 2` to `Community 0`, `Community 4`, `Community 7`?**
  _High betweenness centrality (0.244) - this node is a cross-community bridge._
- **Why does `inMemoryExchangeAdapter` connect `Community 0` to `Community 2`?**
  _High betweenness centrality (0.190) - this node is a cross-community bridge._
- **Why does `TestReconciliation_DeterministicAfterReplay()` connect `Community 2` to `Community 1`?**
  _High betweenness centrality (0.177) - this node is a cross-community bridge._
- **Are the 7 inferred relationships involving `newEngine()` (e.g. with `TestStress_KillSwitchActivatesUnderCrash()` and `TestStress_RiskGateBlocksExtremeExposure()`) actually correct?**
  _`newEngine()` has 7 INFERRED edges - model-reasoned connections that need verification._
- **What connects `Store`, `AggregateType`, `Time` to the rest of the system?**
  _14 weakly-connected nodes found - possible documentation gaps or missing edges._