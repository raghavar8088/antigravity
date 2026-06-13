#!/usr/bin/env bash
# reorganise-lib.sh — Moves client/src/lib flat files into domain sub-folders.
# Zero functionality change. Run from the project root.
set -euo pipefail

LIB="client/src/lib"

echo "Creating sub-folders..."
mkdir -p "$LIB/trading" "$LIB/risk" "$LIB/ai" "$LIB/broker" \
          "$LIB/analytics" "$LIB/portfolio" "$LIB/utils"

move() {
  local src="$LIB/$1"
  local dst="$LIB/$2/$1"
  if [ -f "$src" ]; then
    mv "$src" "$dst"
    echo "  $1 → $2/"
  fi
}

# ── ai/ ───────────────────────────────────────────────────────────────────────
move QuantAIAgent.ts             ai
move aiAppTrackerMongo.ts        ai
move institutionalResearchEngine.ts ai
move marketRegimeClassifier.ts   ai
move strategyScoringEngine.ts    ai
move strategyHealthAnalyzer.ts   ai
move strategyHealthEngine.ts     ai
move strategyPerformanceEngine.ts ai
move strategySignalTrace.ts      ai
move mockResearchAnalytics.ts    ai
move mockResearchFeedback.ts     ai
move mockResearchIndicators.ts   ai
move mockResearchPortfolioAllocation.ts ai
move mockResearchStrategies.ts   ai
move mockResearchWalkForward.ts  ai
move mockStrategyRankingEngine.ts ai
move mockTradeQualityScorer.ts   ai
move replayWalkForwardRanker.ts  ai
move researchEdgeScore.ts        ai
move shadowTradeIntentMapper.ts  ai
move shadowTradeIntentSync.ts    ai
move shadowTradeIntentTypes.ts   ai

# ── risk/ ─────────────────────────────────────────────────────────────────────
move RiskEngine.ts               risk
move blockedExecutionRoute.ts    risk
move promotionGate.ts            risk
move futuresGoLiveGates.ts       risk
move futuresProductionReadiness.ts risk
move futuresUnifiedReadiness.ts  risk
move deskStrategySafety.ts       risk
move entrySkipReason.ts          risk
move noTradeRootCause.ts         risk

# ── broker/ ───────────────────────────────────────────────────────────────────
move angelAuth.ts                broker
move deltaAuth.ts                broker
move deltaSign.ts                broker
move mongoAuthClient.ts          broker
move mongoTradesClient.ts        broker
move engineApi.ts                broker
move engineProxy.ts              broker
move engineAuthority.ts          broker
move getAuthenticatedApiSession.ts broker
move apiSessionAuth.ts           broker
move jwtSession.ts               broker
move ownerAuth.ts                broker
move ownerAccountKey.ts          broker
move anonAccountKey.ts           broker
move authoritativeAccountKey.ts  broker

# ── analytics/ ────────────────────────────────────────────────────────────────
move analyticsReporter.ts        analytics
move DiagnosticsEngine.ts        analytics
move PineScriptEngine.ts         analytics
move futuresReplayEngine.ts      analytics
move futuresReplayCompare.ts     analytics
move futuresReplayFixtures.ts    analytics
move futuresReplayGate.ts        analytics
move futuresReplayUi.ts          analytics
move futuresReplayAutoDisable.ts analytics
move futuresValidationReport.ts  analytics
move futuresSessionMetrics.ts    analytics
move futuresHoldTuningAnalysis.ts analytics
move futuresScorecardActions.ts  analytics
move futuresEdgeCandidates.ts    analytics
move futuresSignalQuality.ts     analytics
move futuresSoakTracker.ts       analytics
move futuresAttribution.ts       analytics
move futuresHealthReport.ts      analytics
move futuresDataHealth.types.ts  analytics
move mockExtendedMetrics.ts      analytics
move mockBacktestEngine.ts       analytics
move mockMarketSimulator.ts      analytics
move mockMonteCarloEngine.ts     analytics
move mockStressTest.ts           analytics
move walkForwardValidation.ts    analytics
move replayCandleUtils.ts        analytics
move regimeRosterBuilder.ts      analytics

# ── portfolio/ ────────────────────────────────────────────────────────────────
move portfolioAccountingFees.ts  portfolio
move portfolioAccountingService.ts portfolio
move portfolioAccountingTypes.ts portfolio
move futuresDeskPnLTracker.ts    portfolio
move dailyPnl.ts                 portfolio
move money.ts                    portfolio
move optionsPaperLedger.ts       portfolio
move optionsSnapshotCache.ts     portfolio
move tradeHistoryService.ts      portfolio
move paperTradesAnalytics.ts     portfolio
move paperTradeAnalyticsApi.ts   portfolio
move paperTradesMapper.ts        portfolio
move paperTradesTypes.ts         portfolio
move dualPersistenceService.ts   portfolio
move backupManager.ts            portfolio
move backupManager.server.ts     portfolio
move writeQueue.ts               portfolio

# ── trading/ — everything else with trading/desk/signal/strategy context ──────
move btcFtDeskBuild.ts           trading
move btcFtPremiumStrategies.ts   trading
move btcFtResearch.ts            trading
move btcFtRoster.ts              trading
move btcFtStrategyGenerator.ts   trading
move btcFtStrategySignals.ts     trading
move btcFtStrategyTemplates.ts   trading
move btcFutureTradingEntryProbe.test.ts trading
move btcFutureTradingRoster.ts   trading
move btcFuturesTrade.types.ts    trading
move btcInstitutionalStrategies.ts trading
move btcResearchStrategyRegistry.ts trading
move btcSpotPrice.ts             trading
move canonicalMarkPrice.ts       trading
move cryptoTop20.ts              trading
move deskEntryFunnelSnapshot.ts  trading
move deskFormat.ts               trading
move deskModuleChrome.ts         trading
move deskOperatorHealth.ts       trading
move deskSelfHealing.ts          trading
move deskSelfHealingExecutor.ts  trading
move deskUiCompact.ts            trading
move deskWorkerLease.ts          trading
move executionRequest.ts         trading
move futuresCategoryRegistry.ts  trading
move futuresCategoryStrategies.ts trading
move futuresDeskPolicy.ts        trading
move futuresDeskRecommendation.ts trading
move futuresDeskRuntime.ts       trading
move futuresEntryDebug.ts        trading
move futuresKlinesFetch.ts       trading
move futuresMTFConfluence.ts     trading
move futuresMarketData.ts        trading
move futuresPaperMath.ts         trading
move futuresParameterTuner.ts    trading
move futuresProfitMode.ts        trading
move futuresSignalScoring.ts     trading
move futuresSignals.ts           trading
move futuresStratTypes.ts        trading
move futuresStrategies.ts        trading
move futuresStrategyDiagnostics.ts trading
move futuresStrategyRotation.ts  trading
move futuresWinnersRefresh.ts    trading
move marketSignal.ts             trading
move mockInstitutionalStrategies.ts trading
move mockOrderManagement.ts      trading
move mockPortfolioOptimizer.ts   trading
move mockPositionLifecycle.ts    trading
move mockTradingApiErrors.ts     trading
move mockTradingEngine.ts        trading
move mockTradingMarketData.ts    trading
move mockTradingMongo.ts         trading
move mockTradingPersistenceTypes.ts trading
move mockTradingSignalEvaluator.ts trading
move mockTradingSnapshotService.ts trading
move paperDeskPositionMath.ts    trading
move perpMarketFeatures.ts       trading
move playbookRegistry.ts         trading
move randomTrader.ts             trading
move strategyAllocation.ts       trading
move strategyProfitabilityAnalysis.ts trading
move strategySessionStats.ts     trading
move styleRoster.ts              trading
move tradingStyleRegistry.ts     trading
move tradingStyleTypes.ts        trading
move workspaceModuleDescription.ts trading
move mcxCommodities.ts           trading
move mcxTokenResolver.ts         trading
move nifty50Stocks.ts            trading
move niftyBeesTokenResolver.ts   trading
move platformEvents.ts           trading
# ── utils/ — everything remaining ────────────────────────────────────────────
move chartTime.ts                utils
move commandPaletteItems.ts      utils
move envCheck.ts                 utils
move localStorageService.ts      utils
move navRoutes.ts                utils
move time.ts                     utils
move utils.ts                    utils
move mockResearchFeedback.ts     utils

echo ""
echo "Migration complete. Files moved:"
echo "trading/ : $(ls "$LIB/trading/" 2>/dev/null | wc -l) files"
echo "risk/    : $(ls "$LIB/risk/"    2>/dev/null | wc -l) files"
echo "ai/      : $(ls "$LIB/ai/"      2>/dev/null | wc -l) files"
echo "broker/  : $(ls "$LIB/broker/"  2>/dev/null | wc -l) files"
echo "analytics/: $(ls "$LIB/analytics/" 2>/dev/null | wc -l) files"
echo "portfolio/: $(ls "$LIB/portfolio/" 2>/dev/null | wc -l) files"
echo "utils/   : $(ls "$LIB/utils/"   2>/dev/null | wc -l) files"
echo ""
echo "Remaining in lib root (sub-dirs and test snapshots):"
ls "$LIB/" | grep -v "^__" | grep -v "/$" || echo "  (none)"
