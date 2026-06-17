import type { StrategySignalTraceRow } from "@/lib/ai/strategySignalTrace";
import { isExecutableTraceRow } from "@/lib/trading/mockTradingEngine";
import { evaluateMockTradingTick } from "@/lib/mockTradingExecutor/evaluateMockTradingTick";
import {
  clampMinSignalScore,
  clampSignalThreshold,
} from "@/lib/mockTradingExecutor/executorConfigConstants";
import { getExecutorConfigView } from "@/lib/mockTradingExecutor/executorConfig";
import type { SignalImpactReport, SignalImpactStrategyRow } from "@/lib/mockTradingExecutor/signalImpactTypes";
import {
  fetchMockTradingKlines,
  sanitizeMockTradingSymbol,
} from "@/lib/trading/mockTradingMarketData";
import { DEFAULT_MOCK_ACCOUNT_KEY } from "@/lib/trading/mockTradingPersistenceTypes";

function impactFromRows(args: {
  rows: StrategySignalTraceRow[];
  currentThreshold: number;
  testThreshold: number;
  currentMinSignalScore: number;
  testMinSignalScore: number;
  evaluatedStrategies: number;
  accountKey: string;
}): SignalImpactReport {
  const byStrategy = new Map<number, SignalImpactStrategyRow>();

  for (const row of args.rows) {
    if (!row.strategyId || row.strategyName === "NO_STRATEGIES") continue;
    const score = Number.isFinite(row.signalScore) ? row.signalScore : 0;
    const existing = byStrategy.get(row.strategyId);
    if (existing && existing.currentScore >= score) continue;

    const signalThresholdPass = score >= args.testThreshold;
    const openThresholdPass = score >= args.testMinSignalScore;
    const wouldQualify =
      signalThresholdPass &&
      openThresholdPass &&
      row.regimeAllowed !== false &&
      row.confirmPassed !== false &&
      (row.gate === "OPENED" || row.status === "CANDIDATE" || isExecutableTraceRow(row));

    byStrategy.set(row.strategyId, {
      name: row.strategyName,
      strategyId: row.strategyId,
      currentScore: score,
      signalThresholdPass,
      openThresholdPass,
      wouldQualify,
      gate: row.gate,
    });
  }

  const strategies = [...byStrategy.values()].sort((a, b) => b.currentScore - a.currentScore);
  return {
    accountKey: args.accountKey,
    currentThreshold: args.currentThreshold,
    testThreshold: args.testThreshold,
    currentMinSignalScore: args.currentMinSignalScore,
    testMinSignalScore: args.testMinSignalScore,
    evaluatedStrategies: args.evaluatedStrategies,
    strategiesAboveSignalThreshold: strategies.filter((s) => s.signalThresholdPass).length,
    strategiesAboveOpenThreshold: strategies.filter((s) => s.openThresholdPass).length,
    strategiesFullyQualified: strategies.filter((s) => s.wouldQualify).length,
    strategies: strategies.slice(0, 10),
  };
}

export async function buildSignalImpactReport(args?: {
  accountKey?: string;
  symbol?: string;
  testThreshold?: number;
  testMinSignalScore?: number;
}): Promise<SignalImpactReport> {
  const accountKey = args?.accountKey?.trim() || DEFAULT_MOCK_ACCOUNT_KEY;
  const symbol = sanitizeMockTradingSymbol(args?.symbol ?? "BTCUSD");
  const current = await getExecutorConfigView(accountKey);
  const testThreshold = clampSignalThreshold(args?.testThreshold ?? current.signalThreshold);
  const testMinSignalScore = clampMinSignalScore(
    args?.testMinSignalScore ?? current.minSignalScore,
  );

  const market = await fetchMockTradingKlines(symbol);
  const evalResult = evaluateMockTradingTick({
    bars: market.bars,
    markPrice: market.markPrice,
    symbol,
    signalThreshold: testThreshold,
  });

  return impactFromRows({
    rows: evalResult.rows,
    currentThreshold: current.signalThreshold,
    testThreshold,
    currentMinSignalScore: current.minSignalScore,
    testMinSignalScore,
    evaluatedStrategies: evalResult.evaluatedStrategies,
    accountKey,
  });
}
