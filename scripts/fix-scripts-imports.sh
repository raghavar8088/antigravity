#!/usr/bin/env bash
# fix-scripts-imports.sh — Updates ../src/lib/{filename} references in client/scripts/
# after lib reorganisation. Run from the project root.
set -euo pipefail

SCRIPTS="client/scripts"

# mapping: filename -> subfolder
declare -A MAP=(
  # trading/
  [btcFtDeskBuild]=trading [btcFtPremiumStrategies]=trading [btcFtResearch]=trading
  [btcFtRoster]=trading [btcFtStrategyGenerator]=trading [btcFtStrategySignals]=trading
  [btcFtStrategyTemplates]=trading [btcFutureTradingEntryProbe.test]=trading
  [btcFutureTradingRoster]=trading [btcFuturesTrade.types]=trading
  [btcInstitutionalStrategies]=trading [btcResearchStrategyRegistry]=trading
  [btcSpotPrice]=trading [canonicalMarkPrice]=trading [cryptoTop20]=trading
  [deskEntryFunnelSnapshot]=trading [deskFormat]=trading [deskModuleChrome]=trading
  [deskOperatorHealth]=trading [deskSelfHealing]=trading [deskSelfHealingExecutor]=trading
  [deskUiCompact]=trading [deskWorkerLease]=trading [executionRequest]=trading
  [futuresCategoryRegistry]=trading [futuresCategoryStrategies]=trading
  [futuresDeskPolicy]=trading [futuresDeskRecommendation]=trading
  [futuresDeskRuntime]=trading [futuresEntryDebug]=trading [futuresKlinesFetch]=trading
  [futuresMTFConfluence]=trading [futuresMarketData]=trading [futuresPaperMath]=trading
  [futuresParameterTuner]=trading [futuresProfitMode]=trading [futuresSignalScoring]=trading
  [futuresSignals]=trading [futuresStratTypes]=trading [futuresStrategies]=trading
  [futuresStrategyDiagnostics]=trading [futuresStrategyRotation]=trading
  [futuresWinnersRefresh]=trading [marketSignal]=trading
  [mockInstitutionalStrategies]=trading [mockOrderManagement]=trading
  [mockPortfolioOptimizer]=trading [mockPositionLifecycle]=trading
  [mockTradingApiErrors]=trading [mockTradingEngine]=trading
  [mockTradingMarketData]=trading [mockTradingMongo]=trading
  [mockTradingPersistenceTypes]=trading [mockTradingSignalEvaluator]=trading
  [mockTradingSnapshotService]=trading [paperDeskPositionMath]=trading
  [perpMarketFeatures]=trading [playbookRegistry]=trading [randomTrader]=trading
  [strategyAllocation]=trading [strategyProfitabilityAnalysis]=trading
  [strategySessionStats]=trading [styleRoster]=trading [tradingStyleRegistry]=trading
  [tradingStyleTypes]=trading [workspaceModuleDescription]=trading
  [mcxCommodities]=trading [mcxTokenResolver]=trading [nifty50Stocks]=trading
  [niftyBeesTokenResolver]=trading [platformEvents]=trading

  # risk/
  [RiskEngine]=risk [blockedExecutionRoute]=risk [promotionGate]=risk
  [futuresGoLiveGates]=risk [futuresProductionReadiness]=risk
  [futuresUnifiedReadiness]=risk [deskStrategySafety]=risk
  [entrySkipReason]=risk [noTradeRootCause]=risk

  # ai/
  [QuantAIAgent]=ai [aiAppTrackerMongo]=ai [institutionalResearchEngine]=ai
  [marketRegimeClassifier]=ai [strategyScoringEngine]=ai [strategyHealthAnalyzer]=ai
  [strategyHealthEngine]=ai [strategyPerformanceEngine]=ai [strategySignalTrace]=ai
  [mockResearchAnalytics]=ai [mockResearchFeedback]=ai [mockResearchIndicators]=ai
  [mockResearchPortfolioAllocation]=ai [mockResearchStrategies]=ai
  [mockResearchWalkForward]=ai [mockStrategyRankingEngine]=ai [mockTradeQualityScorer]=ai
  [replayWalkForwardRanker]=ai [researchEdgeScore]=ai [shadowTradeIntentMapper]=ai
  [shadowTradeIntentSync]=ai [shadowTradeIntentTypes]=ai

  # broker/
  [angelAuth]=broker [deltaAuth]=broker [deltaSign]=broker [mongoAuthClient]=broker
  [mongoTradesClient]=broker [engineApi]=broker [engineProxy]=broker
  [engineAuthority]=broker [getAuthenticatedApiSession]=broker
  [apiSessionAuth]=broker [jwtSession]=broker [ownerAuth]=broker
  [ownerAccountKey]=broker [anonAccountKey]=broker [authoritativeAccountKey]=broker

  # analytics/
  [analyticsReporter]=analytics [DiagnosticsEngine]=analytics [PineScriptEngine]=analytics
  [futuresReplayEngine]=analytics [futuresReplayCompare]=analytics
  [futuresReplayFixtures]=analytics [futuresReplayGate]=analytics
  [futuresReplayUi]=analytics [futuresReplayAutoDisable]=analytics
  [futuresValidationReport]=analytics [futuresSessionMetrics]=analytics
  [futuresHoldTuningAnalysis]=analytics [futuresScorecardActions]=analytics
  [futuresEdgeCandidates]=analytics [futuresSignalQuality]=analytics
  [futuresSoakTracker]=analytics [futuresAttribution]=analytics
  [futuresHealthReport]=analytics [futuresDataHealth.types]=analytics
  [mockExtendedMetrics]=analytics [mockBacktestEngine]=analytics
  [mockMarketSimulator]=analytics [mockMonteCarloEngine]=analytics
  [mockStressTest]=analytics [walkForwardValidation]=analytics
  [replayCandleUtils]=analytics [regimeRosterBuilder]=analytics

  # portfolio/
  [portfolioAccountingFees]=portfolio [portfolioAccountingService]=portfolio
  [portfolioAccountingTypes]=portfolio [futuresDeskPnLTracker]=portfolio
  [dailyPnl]=portfolio [money]=portfolio [optionsPaperLedger]=portfolio
  [optionsSnapshotCache]=portfolio [tradeHistoryService]=portfolio
  [paperTradesAnalytics]=portfolio [paperTradeAnalyticsApi]=portfolio
  [paperTradesMapper]=portfolio [paperTradesTypes]=portfolio
  [dualPersistenceService]=portfolio [backupManager]=portfolio
  [backupManager.server]=portfolio [writeQueue]=portfolio

  # utils/
  [chartTime]=utils [commandPaletteItems]=utils [envCheck]=utils
  [localStorageService]=utils [navRoutes]=utils [time]=utils [utils]=utils
)

echo "Fixing scripts/ relative imports..."

find "$SCRIPTS" \( -name "*.ts" -o -name "*.tsx" \) | while read -r tsfile; do
  for base in "${!MAP[@]}"; do
    subfolder="${MAP[$base]}"
    # Match ../src/lib/{base} with optional .ts extension and quote variants
    if grep -q "\.\./src/lib/${base}" "$tsfile" 2>/dev/null; then
      sed -i \
        -e "s|../src/lib/${base}'|../src/lib/${subfolder}/${base}'|g" \
        -e "s|../src/lib/${base}\"|../src/lib/${subfolder}/${base}\"|g" \
        -e "s|../src/lib/${base}.ts'|../src/lib/${subfolder}/${base}.ts'|g" \
        -e "s|../src/lib/${base}.ts\"|../src/lib/${subfolder}/${base}.ts\"|g" \
        "$tsfile"
    fi
  done
done

echo "Done. Verify with: cd client && npx tsc --noEmit"
