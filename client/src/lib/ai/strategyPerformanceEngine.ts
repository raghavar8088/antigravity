import type { MarketRegime } from "@/lib/ai/marketRegimeClassifier";
import type { MockTrade } from "@/lib/trading/mockTradingEngine";

export interface RegimeStats {
  trades: number;
  winRate: number;
  expectancy: number;
  netPnl: number;
}

export interface StrategyPerformanceMetrics {
  strategyId: number;
  strategyName: string;
  totalTrades: number;
  closedTrades: number;
  grossPnl: number;
  netPnl: number;
  winRate: number;
  profitFactor: number;
  avgWin: number;
  avgLoss: number;
  expectancy: number;
  sharpeRatio: number;
  sortinoRatio: number;
  maxDrawdown: number;
  maxDrawdownPct: number;
  recoveryFactor: number;
  avgHoldMinutes: number;
  recencyScore: number;
  last7DaysPnl: number;
  last7DaysWinRate: number;
  last7DaysTrades: number;
  regimeBreakdown: Record<MarketRegime | "UNKNOWN", RegimeStats>;
  sampleSizeConfidence: number;
}

const DAY_MS = 86_400_000;
const WEEK_MS = 7 * DAY_MS;
const REGIMES: Array<MarketRegime | "UNKNOWN"> = [
  "STRONG_TREND",
  "WEAK_TREND",
  "RANGE",
  "HIGH_VOLATILITY",
  "LOW_VOLATILITY",
  "BREAKOUT",
  "REVERSAL",
  "TRENDING",
  "RANGING",
  "HIGH_VOLATILITY_BREAKOUT",
  "LOW_VOLATILITY_CHOP",
  "UNKNOWN",
];

function emptyRegimeBreakdown(): Record<MarketRegime | "UNKNOWN", RegimeStats> {
  return {
    STRONG_TREND: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
    WEAK_TREND: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
    RANGE: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
    HIGH_VOLATILITY: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
    LOW_VOLATILITY: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
    BREAKOUT: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
    REVERSAL: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
    TRENDING: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
    RANGING: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
    HIGH_VOLATILITY_BREAKOUT: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
    LOW_VOLATILITY_CHOP: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
    UNKNOWN: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
  };
}

function mean(values: readonly number[]): number {
  if (values.length === 0) return 0;
  return values.reduce((sum, value) => sum + value, 0) / values.length;
}

function std(values: readonly number[]): number {
  if (values.length < 2) return 0;
  const avg = mean(values);
  return Math.sqrt(values.reduce((sum, value) => sum + (value - avg) ** 2, 0) / (values.length - 1));
}

function dailyPnls(trades: readonly MockTrade[]): number[] {
  const buckets = new Map<number, number>();
  for (const trade of trades) {
    if (trade.closedAt == null) continue;
    const day = Math.floor(trade.closedAt / DAY_MS);
    buckets.set(day, (buckets.get(day) ?? 0) + trade.realizedPnl);
  }
  return [...buckets.entries()].sort((a, b) => a[0] - b[0]).map(([, pnl]) => pnl);
}

function sharpe(days: readonly number[]): number {
  if (days.length < 3) return 0;
  const dayMean = mean(days);
  const dayStd = std(days);
  if (dayStd === 0) return dayMean > 0 ? 10 : 0;
  return (dayMean / dayStd) * Math.sqrt(365);
}

function sortino(days: readonly number[], fallbackSharpe: number): number {
  if (days.length < 3) return 0;
  const dayMean = mean(days);
  const negatives = days.filter((value) => value < 0);
  if (negatives.length === 0) return fallbackSharpe;
  const downside = Math.sqrt(negatives.reduce((sum, value) => sum + value ** 2, 0) / negatives.length);
  return downside > 0 ? (dayMean / downside) * Math.sqrt(365) : fallbackSharpe;
}

function isKnownRegime(value: string | undefined): value is MarketRegime {
  return value === "TRENDING" || value === "RANGING" || value === "HIGH_VOLATILITY_BREAKOUT" || value === "LOW_VOLATILITY_CHOP";
}

function maxDrawdown(closed: readonly MockTrade[]): { maxDrawdown: number; maxDrawdownPct: number } {
  let equity = 0;
  let peak = 0;
  let maxDd = 0;
  let maxDdPct = 0;

  for (const trade of [...closed].sort((a, b) => (a.closedAt ?? 0) - (b.closedAt ?? 0))) {
    equity += trade.realizedPnl;
    peak = Math.max(peak, equity);
    const dd = peak - equity;
    if (dd > maxDd) maxDd = dd;
    if (peak > 0) maxDdPct = Math.max(maxDdPct, (dd / peak) * 100);
  }

  return { maxDrawdown: maxDd, maxDrawdownPct: maxDdPct };
}

export function computeStrategyPerformance(
  trades: readonly MockTrade[],
  now = Date.now(),
): StrategyPerformanceMetrics {
  const closed = trades.filter((trade) => trade.status === "CLOSED" && trade.closedAt != null);
  const first = trades[0];
  const wins = closed.filter((trade) => trade.realizedPnl > 0);
  const losses = closed.filter((trade) => trade.realizedPnl < 0);
  const grossWins = wins.reduce((sum, trade) => sum + trade.realizedPnl, 0);
  const grossLosses = Math.abs(losses.reduce((sum, trade) => sum + trade.realizedPnl, 0));
  const netPnl = closed.reduce((sum, trade) => sum + trade.realizedPnl, 0);
  const grossPnl = closed.reduce((sum, trade) => sum + trade.realizedPnl + trade.fees, 0);
  const winRate = closed.length > 0 ? wins.length / closed.length : 0;
  const avgWin = wins.length > 0 ? grossWins / wins.length : 0;
  const avgLoss = losses.length > 0 ? losses.reduce((sum, trade) => sum + trade.realizedPnl, 0) / losses.length : 0;
  const lossRate = closed.length > 0 ? losses.length / closed.length : 0;
  const expectancy = winRate * avgWin + lossRate * avgLoss;
  const profitFactor = grossLosses > 0 ? grossWins / grossLosses : grossWins > 0 ? 999 : 0;
  const dayPnls = dailyPnls(closed);
  const sharpeRatio = sharpe(dayPnls);
  const sortinoRatio = sortino(dayPnls, sharpeRatio);
  const drawdown = maxDrawdown(closed);
  const holdMinutes =
    closed.length > 0
      ? closed.reduce((sum, trade) => sum + (((trade.closedAt ?? trade.openedAt) - trade.openedAt) / 60_000), 0) / closed.length
      : 0;

  const recent = closed.filter((trade) => (trade.closedAt ?? 0) >= now - WEEK_MS);
  const recentWins = recent.filter((trade) => trade.realizedPnl > 0);
  const recentPnl = recent.reduce((sum, trade) => sum + trade.realizedPnl, 0);
  const weightedRecent = closed.reduce((sum, trade) => {
    const age = Math.max(0, now - (trade.closedAt ?? now));
    const weight = Math.exp(-age / WEEK_MS);
    return sum + trade.realizedPnl * weight;
  }, 0);
  const weightedAbs = closed.reduce((sum, trade) => {
    const age = Math.max(0, now - (trade.closedAt ?? now));
    const weight = Math.exp(-age / WEEK_MS);
    return sum + Math.abs(trade.realizedPnl) * weight;
  }, 0);
  const recencyScore = weightedAbs > 0 ? Math.max(0, Math.min(100, 50 + (weightedRecent / weightedAbs) * 50)) : 0;

  const regimeBreakdown = emptyRegimeBreakdown();
  const regimeTrades = new Map<MarketRegime | "UNKNOWN", MockTrade[]>();
  for (const regime of REGIMES) regimeTrades.set(regime, []);
  for (const trade of closed) {
    const regime = isKnownRegime(trade.regimeAtEntry) ? trade.regimeAtEntry : "UNKNOWN";
    regimeTrades.get(regime)?.push(trade);
  }

  for (const regime of REGIMES) {
    const subset = regimeTrades.get(regime) ?? [];
    const subsetWins = subset.filter((trade) => trade.realizedPnl > 0);
    regimeBreakdown[regime] = {
      trades: subset.length,
      winRate: subset.length > 0 ? subsetWins.length / subset.length : 0,
      expectancy: subset.length > 0 ? subset.reduce((sum, trade) => sum + trade.realizedPnl, 0) / subset.length : 0,
      netPnl: subset.reduce((sum, trade) => sum + trade.realizedPnl, 0),
    };
  }

  return {
    strategyId: first?.strategyId ?? 0,
    strategyName: first?.strategyName ?? "Unknown Strategy",
    totalTrades: trades.length,
    closedTrades: closed.length,
    grossPnl,
    netPnl,
    winRate,
    profitFactor,
    avgWin,
    avgLoss,
    expectancy,
    sharpeRatio,
    sortinoRatio,
    maxDrawdown: drawdown.maxDrawdown,
    maxDrawdownPct: drawdown.maxDrawdownPct,
    recoveryFactor: drawdown.maxDrawdown > 0 ? netPnl / drawdown.maxDrawdown : netPnl > 0 ? 999 : 0,
    avgHoldMinutes: holdMinutes,
    recencyScore,
    last7DaysPnl: recentPnl,
    last7DaysWinRate: recent.length > 0 ? recentWins.length / recent.length : 0,
    last7DaysTrades: recent.length,
    regimeBreakdown,
    sampleSizeConfidence: Math.min(100, Math.sqrt(closed.length) * 10),
  };
}
