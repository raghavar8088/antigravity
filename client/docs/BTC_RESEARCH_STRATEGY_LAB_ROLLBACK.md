# BTC Research Strategy Lab Rollback Summary

## Rollback Scope

This upgrade is contained to the mock-trading research lab surface. It does not modify paper/live execution order placement paths. A rollback should remove the research-lab additions and restore the previous mock trading dashboard/runner/persistence behavior.

## Modified Existing Files

- `CHANGELOG.md`
- `client/src/components/MockTradingDashboard.tsx`
- `client/src/components/MockTradingDashboard.test.tsx`
- `client/src/hooks/useMockResearchRunner.ts`
- `client/src/hooks/useMockTradingEngine.ts`
- `client/src/lib/mockCandleBuilder.ts`
- `client/src/lib/mockResearchIndicators.ts`
- `client/src/lib/mockResearchStrategies.test.ts`
- `client/src/lib/mockTradingEngine.ts`
- `client/src/lib/mockTradingEngine.test.ts`
- `client/src/lib/mockTradingMongo.ts`

## Added Files

### API Routes

- `client/src/app/api/mock-trading/signals/route.ts`
- `client/src/app/api/mock-trading/regime/route.ts`
- `client/src/app/api/mock-trading/equity/route.ts`
- `client/src/app/api/mock-trading/scores/route.ts`
- `client/src/app/api/mock-trading/daily-pnl/route.ts`

### Components and Hooks

- `client/src/components/MockResearchChartsPanel.tsx`
- `client/src/hooks/useMarketRegime.ts`
- `client/src/hooks/useStrategyScoring.ts`

### Research Lab Libraries

- `client/src/lib/btcResearchStrategyRegistry.ts`
- `client/src/lib/marketRegimeClassifier.ts`
- `client/src/lib/strategyPerformanceEngine.ts`
- `client/src/lib/strategyScoringEngine.ts`
- `client/src/lib/mockResearchAnalytics.ts`
- `client/src/lib/strategyHealthEngine.ts`
- `client/src/lib/mockResearchWalkForward.ts`
- `client/src/lib/mockResearchPortfolioAllocation.ts`

### Tests

- `client/src/lib/btcResearchStrategyRegistry.test.ts`
- `client/src/lib/marketRegimeClassifier.test.ts`
- `client/src/lib/strategyPerformanceEngine.test.ts`
- `client/src/lib/strategyScoringEngine.test.ts`
- `client/src/lib/mockResearchAnalytics.test.ts`
- `client/src/lib/strategyHealthEngine.test.ts`
- `client/src/lib/mockResearchWalkForward.test.ts`
- `client/src/lib/mockResearchPortfolioAllocation.test.ts`

### Documentation

- `client/docs/BTC_RESEARCH_STRATEGY_LAB_RELEASE_NOTES.md`
- `client/docs/BTC_RESEARCH_STRATEGY_LAB_ROLLBACK.md`

## New Mongo Collections to Drop on Full Rollback

If a full data rollback is needed, drop these mock research collections:

- `strategy_signals`
- `regime_snapshots`
- `strategy_scores`
- `strategy_score_history`
- `equity_curve`
- `daily_pnl_history`

Existing mock collections can remain unless the rollback also needs to erase all mock trading history:

- `mock_trades`
- `mock_account_snapshots`
- `mock_strategy_analytics`
- `mock_trade_logs`
- `mock_engine_config`

## Major Modules to Revert

1. Strategy registry and signal generation
   - Remove `btcResearchStrategyRegistry.ts`.
   - Revert `useMockResearchRunner.ts` to evaluating only `RESEARCH_STRATEGIES`.
   - Remove Profit/Regime mode gating additions.

2. Regime, scoring, health, walk-forward, and allocation engines
   - Remove the new pure research lab libraries and tests.
   - Remove their dashboard panels.

3. Persistence and API routes
   - Remove new API route directories.
   - Remove new collection constants, doc types, indexes, and CRUD helpers from `mockTradingMongo.ts`.

4. Dashboard
   - Remove `MockResearchChartsPanel.tsx`.
   - Revert `MockTradingDashboard.tsx` to the previous research controls and analytics cards.
   - Remove strategy health, walk-forward, allocation, correlation, exposure, and chart panels.

5. Mock execution metadata
   - Remove optional `regimeAtEntry` from `MockTrade` and `MockResearchSignalInput` if historical regime attribution is no longer needed.

## Safety Notes

- Rolling back these files should not affect live broker/exchange code.
- Do not remove or modify paper/live execution modules while rolling back the mock research lab.
- If Mongo collections are dropped, historical research lab analytics will be lost, but mock trades can remain if `mock_trades` is preserved.
