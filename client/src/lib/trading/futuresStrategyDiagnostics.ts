import { isProbeOrBootstrapTrade } from "../analytics/futuresSessionMetrics";
import type { PaperTradeDbRow } from "../portfolio/paperTradesTypes";

export interface StrategyDiagnosticRow {
  strategyId: number;
  strategyName: string;
  templateFamily: string;
  totalTrades: number;
  wins: number;
  losses: number;
  winRate: number;
  totalNetPnl: number;
  avgNetPnl: number;
  avgWin: number;
  avgLoss: number;
  feePctOfAbsGross: number;
  profitFactor: number;
  totalFees: number;
  avgHoldMinutes: number;
  exitReasonCounts: Record<string, number>;
  slCount: number;
  tpCount: number;
  timeCount: number;
  trailCount: number;
  profitLockCount: number;
  worstTrade: number;
  bestTrade: number;
  lastTradeAt: string;
  isProbe: boolean;
}

export interface HealthCheckResult {
  window: number;
  expectancy: number;
  expectancyPass: boolean;
  winRate: number;
  winRatePass: boolean;
  feePctOfAbsGross: number;
  feePass: boolean;
  profitFactor: number;
  pfPass: boolean;
  tpHits: number;
  tpHitPass: boolean;
  slCount: number;
  timeCount: number;
  overallPass: boolean;
  grade: "A" | "B" | "C" | "D" | "F";
}

export interface DiagnosticSummary {
  totalProduction: number;
  rows: StrategyDiagnosticRow[];
  topByExpectancy: StrategyDiagnosticRow[];
  bottomByExpectancy: StrategyDiagnosticRow[];
  slDominatedStrats: StrategyDiagnosticRow[];
  highFeeStrategies: StrategyDiagnosticRow[];
}

type StrategyAccumulator = {
  strategyId: number;
  strategyName: string;
  templateFamily: string;
  trades: PaperTradeDbRow[];
};

export function computeStrategyDiagnostics(trades: readonly PaperTradeDbRow[]): DiagnosticSummary {
  const productionTrades = trades.filter((trade) => !isProbeOrBootstrapTrade({ strategy_name: trade.strategy_name }));
  const byStrategy = new Map<number, StrategyAccumulator>();

  for (const trade of productionTrades) {
    const existing = byStrategy.get(trade.strategy_id);
    if (existing) {
      existing.trades.push(trade);
      continue;
    }

    byStrategy.set(trade.strategy_id, {
      strategyId: trade.strategy_id,
      strategyName: trade.strategy_name,
      templateFamily: trade.template_family ?? "UNKNOWN",
      trades: [trade],
    });
  }

  const rows = [...byStrategy.values()]
    .map((acc) => buildDiagnosticRow(acc))
    .sort((a, b) => b.avgNetPnl - a.avgNetPnl);

  return {
    totalProduction: productionTrades.length,
    rows,
    topByExpectancy: rows.slice(0, 10),
    bottomByExpectancy: [...rows].sort((a, b) => a.avgNetPnl - b.avgNetPnl).slice(0, 10),
    slDominatedStrats: rows.filter((row) => row.totalTrades > 0 && row.slCount / row.totalTrades > 0.6),
    highFeeStrategies: rows.filter((row) => row.feePctOfAbsGross > 0.5),
  };
}

export function computeRollingHealthCheck(
  trades: readonly PaperTradeDbRow[],
  window = 50,
): HealthCheckResult {
  const productionTrades = trades
    .filter((trade) => !isProbeOrBootstrapTrade({ strategy_name: trade.strategy_name }) && Boolean(trade.closed_at))
    .sort((a, b) => new Date(b.closed_at).getTime() - new Date(a.closed_at).getTime())
    .slice(0, Math.max(0, window));

  const n = productionTrades.length;
  if (n === 0) {
    return {
      window: 0,
      expectancy: 0,
      expectancyPass: false,
      winRate: 0,
      winRatePass: false,
      feePctOfAbsGross: 0,
      feePass: true,
      profitFactor: 0,
      pfPass: false,
      tpHits: 0,
      tpHitPass: false,
      slCount: 0,
      timeCount: 0,
      overallPass: false,
      grade: "F",
    };
  }

  const wins = productionTrades.filter((trade) => (trade.net_pnl ?? 0) > 0);
  const losses = productionTrades.filter((trade) => (trade.net_pnl ?? 0) <= 0);
  const netPnl = productionTrades.reduce((sum, trade) => sum + (trade.net_pnl ?? 0), 0);
  const totalFees = productionTrades.reduce((sum, trade) => sum + (trade.fees ?? 0), 0);
  const absGross = productionTrades.reduce((sum, trade) => sum + Math.abs(trade.gross_pnl ?? 0), 0);
  const grossWins = wins.reduce((sum, trade) => sum + (trade.net_pnl ?? 0), 0);
  const grossLosses = Math.abs(losses.reduce((sum, trade) => sum + (trade.net_pnl ?? 0), 0));
  const tpHits = productionTrades.filter((trade) => trade.exit_reason === "TP").length;
  const slCount = productionTrades.filter((trade) => trade.exit_reason === "SL").length;
  const timeCount = productionTrades.filter((trade) => trade.exit_reason === "TIME").length;

  const expectancy = netPnl / n;
  const winRate = wins.length / n;
  const feePctOfAbsGross = absGross > 0 ? totalFees / absGross : 0;
  const profitFactor = grossLosses > 0 ? grossWins / grossLosses : grossWins > 0 ? Infinity : 0;

  const expectancyPass = expectancy > 0;
  const winRatePass = winRate >= 0.45;
  const feePass = feePctOfAbsGross <= 0.5;
  const pfPass = profitFactor >= 1.1;
  const tpHitPass = tpHits > 0 && tpHits >= slCount;
  const passCount = [expectancyPass, winRatePass, feePass, pfPass, tpHitPass].filter(Boolean).length;

  return {
    window: n,
    expectancy,
    expectancyPass,
    winRate,
    winRatePass,
    feePctOfAbsGross,
    feePass,
    profitFactor,
    pfPass,
    tpHits,
    tpHitPass,
    slCount,
    timeCount,
    overallPass: passCount >= 4,
    grade: gradeFromPassCount(passCount),
  };
}

function buildDiagnosticRow(acc: StrategyAccumulator): StrategyDiagnosticRow {
  const trades = acc.trades;
  const wins = trades.filter((trade) => (trade.net_pnl ?? 0) > 0);
  const losses = trades.filter((trade) => (trade.net_pnl ?? 0) <= 0);
  const totalNetPnl = trades.reduce((sum, trade) => sum + (trade.net_pnl ?? 0), 0);
  const totalFees = trades.reduce((sum, trade) => sum + (trade.fees ?? 0), 0);
  const absGross = trades.reduce((sum, trade) => sum + Math.abs(trade.gross_pnl ?? 0), 0);
  const winSum = wins.reduce((sum, trade) => sum + (trade.net_pnl ?? 0), 0);
  const lossSum = Math.abs(losses.reduce((sum, trade) => sum + (trade.net_pnl ?? 0), 0));
  const exitReasonCounts = countExitReasons(trades);
  const holdMinutes = trades.map(holdMinutesForTrade).filter((value) => Number.isFinite(value));

  return {
    strategyId: acc.strategyId,
    strategyName: acc.strategyName,
    templateFamily: acc.templateFamily,
    totalTrades: trades.length,
    wins: wins.length,
    losses: losses.length,
    winRate: trades.length > 0 ? wins.length / trades.length : 0,
    totalNetPnl,
    avgNetPnl: trades.length > 0 ? totalNetPnl / trades.length : 0,
    avgWin: wins.length > 0 ? winSum / wins.length : 0,
    avgLoss: losses.length > 0 ? -lossSum / losses.length : 0,
    feePctOfAbsGross: absGross > 0 ? totalFees / absGross : 0,
    profitFactor: lossSum > 0 ? winSum / lossSum : winSum > 0 ? Infinity : 0,
    totalFees,
    avgHoldMinutes: holdMinutes.length > 0 ? holdMinutes.reduce((sum, value) => sum + value, 0) / holdMinutes.length : 0,
    exitReasonCounts,
    slCount: exitReasonCounts.SL ?? 0,
    tpCount: exitReasonCounts.TP ?? 0,
    timeCount: exitReasonCounts.TIME ?? 0,
    trailCount: exitReasonCounts.TRAIL ?? 0,
    profitLockCount: exitReasonCounts.PROFIT_LOCK ?? 0,
    worstTrade: trades.reduce((worst, trade) => Math.min(worst, trade.net_pnl ?? 0), Infinity),
    bestTrade: trades.reduce((best, trade) => Math.max(best, trade.net_pnl ?? 0), -Infinity),
    lastTradeAt: latestClosedAt(trades),
    isProbe: false,
  };
}

function countExitReasons(trades: readonly PaperTradeDbRow[]): Record<string, number> {
  const counts: Record<string, number> = {};
  for (const trade of trades) {
    const reason = trade.exit_reason || "UNKNOWN";
    counts[reason] = (counts[reason] ?? 0) + 1;
  }
  return counts;
}

function holdMinutesForTrade(trade: PaperTradeDbRow) {
  const opened = new Date(trade.opened_at).getTime();
  const closed = new Date(trade.closed_at).getTime();
  return Number.isFinite(opened) && Number.isFinite(closed) ? Math.max(0, (closed - opened) / 60_000) : Number.NaN;
}

function latestClosedAt(trades: readonly PaperTradeDbRow[]) {
  return trades
    .map((trade) => trade.closed_at)
    .filter(Boolean)
    .sort((a, b) => new Date(b).getTime() - new Date(a).getTime())[0] ?? "";
}

function gradeFromPassCount(passCount: number): HealthCheckResult["grade"] {
  if (passCount >= 5) return "A";
  if (passCount === 4) return "B";
  if (passCount === 3) return "C";
  if (passCount === 2) return "D";
  return "F";
}
