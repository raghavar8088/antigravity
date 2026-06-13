#!/usr/bin/env bash
# fix-lib-imports.sh — Updates all TypeScript import paths after lib reorganisation.
# Finds every @/lib/{filename} reference and rewrites to @/lib/{subfolder}/{filename}.
# Run from project root AFTER reorganise-lib.sh.
set -euo pipefail

SRC="client/src"

fix() {
  local file="$1"
  local subfolder="$2"
  local base="${file%.*}"   # strip .ts or .tsx for import match

  # Match both single and double quotes; handle .ts extension presence/absence.
  find "$SRC" \( -name "*.ts" -o -name "*.tsx" \) | while read -r tsfile; do
    if grep -q "@/lib/${base}" "$tsfile" 2>/dev/null; then
      sed -i \
        -e "s|@/lib/${base}'|@/lib/${subfolder}/${base}'|g" \
        -e "s|@/lib/${base}\"|@/lib/${subfolder}/${base}\"|g" \
        -e "s|@/lib/${file}'|@/lib/${subfolder}/${file}'|g" \
        -e "s|@/lib/${file}\"|@/lib/${subfolder}/${file}\"|g" \
        "$tsfile"
    fi
  done
}

echo "Updating import paths..."

# ai/
fix QuantAIAgent.ts              ai
fix aiAppTrackerMongo.ts         ai
fix institutionalResearchEngine.ts ai
fix marketRegimeClassifier.ts    ai
fix strategyScoringEngine.ts     ai
fix strategyHealthAnalyzer.ts    ai
fix strategyHealthEngine.ts      ai
fix strategyPerformanceEngine.ts ai
fix strategySignalTrace.ts       ai
fix mockResearchAnalytics.ts     ai
fix mockResearchFeedback.ts      ai
fix mockResearchIndicators.ts    ai
fix mockResearchPortfolioAllocation.ts ai
fix mockResearchStrategies.ts    ai
fix mockResearchWalkForward.ts   ai
fix mockStrategyRankingEngine.ts ai
fix mockTradeQualityScorer.ts    ai
fix replayWalkForwardRanker.ts   ai
fix researchEdgeScore.ts         ai
fix shadowTradeIntentMapper.ts   ai
fix shadowTradeIntentSync.ts     ai
fix shadowTradeIntentTypes.ts    ai

# risk/
fix RiskEngine.ts                risk
fix blockedExecutionRoute.ts     risk
fix promotionGate.ts             risk
fix futuresGoLiveGates.ts        risk
fix futuresProductionReadiness.ts risk
fix futuresUnifiedReadiness.ts   risk
fix deskStrategySafety.ts        risk
fix entrySkipReason.ts           risk
fix noTradeRootCause.ts          risk

# broker/
fix angelAuth.ts                 broker
fix deltaAuth.ts                 broker
fix deltaSign.ts                 broker
fix mongoAuthClient.ts           broker
fix mongoTradesClient.ts         broker
fix engineApi.ts                 broker
fix engineProxy.ts               broker
fix engineAuthority.ts           broker
fix getAuthenticatedApiSession.ts broker
fix apiSessionAuth.ts            broker
fix jwtSession.ts                broker
fix ownerAuth.ts                 broker
fix ownerAccountKey.ts           broker
fix anonAccountKey.ts            broker
fix authoritativeAccountKey.ts   broker

# analytics/
fix analyticsReporter.ts         analytics
fix DiagnosticsEngine.ts         analytics
fix PineScriptEngine.ts          analytics
fix futuresReplayEngine.ts       analytics
fix futuresReplayCompare.ts      analytics
fix futuresReplayFixtures.ts     analytics
fix futuresReplayGate.ts         analytics
fix futuresReplayUi.ts           analytics
fix futuresReplayAutoDisable.ts  analytics
fix futuresValidationReport.ts   analytics
fix futuresSessionMetrics.ts     analytics
fix futuresHoldTuningAnalysis.ts analytics
fix futuresScorecardActions.ts   analytics
fix futuresEdgeCandidates.ts     analytics
fix futuresSignalQuality.ts      analytics
fix futuresSoakTracker.ts        analytics
fix futuresAttribution.ts        analytics
fix futuresHealthReport.ts       analytics
fix futuresDataHealth.types.ts   analytics
fix mockExtendedMetrics.ts       analytics
fix mockBacktestEngine.ts        analytics
fix mockMarketSimulator.ts       analytics
fix mockMonteCarloEngine.ts      analytics
fix mockStressTest.ts            analytics
fix walkForwardValidation.ts     analytics
fix regimeRosterBuilder.ts       analytics

# portfolio/
fix portfolioAccountingFees.ts   portfolio
fix portfolioAccountingService.ts portfolio
fix portfolioAccountingTypes.ts  portfolio
fix futuresDeskPnLTracker.ts     portfolio
fix dailyPnl.ts                  portfolio
fix money.ts                     portfolio
fix optionsPaperLedger.ts        portfolio
fix optionsSnapshotCache.ts      portfolio
fix tradeHistoryService.ts       portfolio
fix paperTradesAnalytics.ts      portfolio
fix paperTradeAnalyticsApi.ts    portfolio
fix paperTradesMapper.ts         portfolio
fix paperTradesTypes.ts          portfolio
fix dualPersistenceService.ts    portfolio
fix backupManager.ts             portfolio
fix backupManager.server.ts      portfolio
fix writeQueue.ts                portfolio

# trading/
fix btcFtDeskBuild.ts            trading
fix btcFtPremiumStrategies.ts    trading
fix btcFtResearch.ts             trading
fix btcFtRoster.ts               trading
fix btcFtStrategyGenerator.ts    trading
fix btcFtStrategySignals.ts      trading
fix btcFtStrategyTemplates.ts    trading
fix btcFutureTradingEntryProbe.test.ts trading
fix btcFutureTradingRoster.ts    trading
fix btcFuturesTrade.types.ts     trading
fix btcInstitutionalStrategies.ts trading
fix btcResearchStrategyRegistry.ts trading
fix btcSpotPrice.ts              trading
fix canonicalMarkPrice.ts        trading
fix cryptoTop20.ts               trading
fix deskEntryFunnelSnapshot.ts   trading
fix deskFormat.ts                trading
fix deskModuleChrome.ts          trading
fix deskOperatorHealth.ts        trading
fix deskSelfHealing.ts           trading
fix deskSelfHealingExecutor.ts   trading
fix deskUiCompact.ts             trading
fix deskWorkerLease.ts           trading
fix executionRequest.ts          trading
fix futuresCategoryRegistry.ts   trading
fix futuresCategoryStrategies.ts trading
fix futuresDeskPolicy.ts         trading
fix futuresDeskRecommendation.ts trading
fix futuresDeskRuntime.ts        trading
fix futuresEntryDebug.ts         trading
fix futuresKlinesFetch.ts        trading
fix futuresMTFConfluence.ts      trading
fix futuresMarketData.ts         trading
fix futuresPaperMath.ts          trading
fix futuresParameterTuner.ts     trading
fix futuresProfitMode.ts         trading
fix futuresSignalScoring.ts      trading
fix futuresSignals.ts            trading
fix futuresStratTypes.ts         trading
fix futuresStrategies.ts         trading
fix futuresStrategyDiagnostics.ts trading
fix futuresStrategyRotation.ts   trading
fix futuresWinnersRefresh.ts     trading
fix marketSignal.ts              trading
fix mockInstitutionalStrategies.ts trading
fix mockOrderManagement.ts       trading
fix mockPortfolioOptimizer.ts    trading
fix mockPositionLifecycle.ts     trading
fix mockTradingApiErrors.ts      trading
fix mockTradingEngine.ts         trading
fix mockTradingMarketData.ts     trading
fix mockTradingMongo.ts          trading
fix mockTradingPersistenceTypes.ts trading
fix mockTradingSignalEvaluator.ts trading
fix mockTradingSnapshotService.ts trading
fix paperDeskPositionMath.ts     trading
fix perpMarketFeatures.ts        trading
fix playbookRegistry.ts          trading
fix randomTrader.ts              trading
fix strategyAllocation.ts        trading
fix strategyProfitabilityAnalysis.ts trading
fix strategySessionStats.ts      trading
fix styleRoster.ts               trading
fix tradingStyleRegistry.ts      trading
fix tradingStyleTypes.ts         trading
fix workspaceModuleDescription.ts trading
fix mcxCommodities.ts            trading
fix mcxTokenResolver.ts          trading
fix nifty50Stocks.ts             trading
fix niftyBeesTokenResolver.ts    trading
fix platformEvents.ts            trading

# utils/
fix chartTime.ts                 utils
fix commandPaletteItems.ts       utils
fix envCheck.ts                  utils
fix localStorageService.ts       utils
fix navRoutes.ts                 utils
fix time.ts                      utils
fix utils.ts                     utils

echo "Import paths updated."
echo "Run: cd client && npx tsc --noEmit to verify zero TypeScript errors."
