import type { MarketRegime } from "@/lib/ai/marketRegimeClassifier";
import type { StrategyPerformanceMetrics } from "@/lib/ai/strategyPerformanceEngine";

export const DEFAULT_SCORING_WEIGHTS = {
  pnl: 0.25,
  profitFactor: 0.2,
  winRate: 0.15,
  drawdown: 0.15,
  sharpe: 0.1,
  recency: 0.1,
  sampleSize: 0.05,
} as const;

export interface StrategyScore {
  strategyId: number;
  strategyName: string;
  metrics: StrategyPerformanceMetrics;
  pnlScore: number;
  profitFactorScore: number;
  winRateScore: number;
  drawdownScore: number;
  sharpeScore: number;
  recencyScore: number;
  sampleSizeScore: number;
  overallScore: number;
  currentRegimeScore: number;
  confidenceRating: "HIGH" | "MEDIUM" | "LOW" | "INSUFFICIENT";
  rank: number;
  regimeRank: number;
}

type Weights = typeof DEFAULT_SCORING_WEIGHTS;

function clamp(value: number, min = 0, max = 100): number {
  if (!Number.isFinite(value)) return min;
  return Math.min(max, Math.max(min, value));
}

function percentileScores(values: readonly number[]): number[] {
  if (values.length === 0) return [];
  if (values.length === 1) return [100];

  const sorted = [...values].sort((a, b) => a - b);
  const min = sorted[0];
  const max = sorted[sorted.length - 1];
  if (min === max) return values.map(() => 50);

  return values.map((value) => {
    const below = sorted.filter((candidate) => candidate < value).length;
    const equal = sorted.filter((candidate) => candidate === value).length;
    return clamp(((below + equal * 0.5) / values.length) * 100);
  });
}

function confidenceRating(metrics: StrategyPerformanceMetrics): StrategyScore["confidenceRating"] {
  if (metrics.sampleSizeConfidence >= 70 && metrics.closedTrades >= 20) return "HIGH";
  if (metrics.sampleSizeConfidence >= 40) return "MEDIUM";
  if (metrics.closedTrades >= 5) return "LOW";
  return "INSUFFICIENT";
}

function regimeBaseScore(metrics: StrategyPerformanceMetrics, regime: MarketRegime): number {
  const stats = metrics.regimeBreakdown[regime];
  if (!stats || stats.trades < 3) return metrics.sampleSizeConfidence / 2;
  return stats.expectancy;
}

export function scoreAllStrategies(
  performances: ReadonlyMap<number, StrategyPerformanceMetrics>,
  currentRegime: MarketRegime,
  weights: Weights = DEFAULT_SCORING_WEIGHTS,
): StrategyScore[] {
  const metrics = [...performances.values()];
  if (metrics.length === 0) return [];

  const pnlScores = percentileScores(metrics.map((item) => item.netPnl));
  const profitFactorScores = percentileScores(metrics.map((item) => clamp(item.profitFactor, 0, 10)));
  const winRateScores = percentileScores(metrics.map((item) => item.winRate));
  const drawdownScores = percentileScores(metrics.map((item) => -item.maxDrawdownPct));
  const sharpeScores = percentileScores(metrics.map((item) => clamp(item.sharpeRatio, -3, 10)));
  const recencyScores = percentileScores(metrics.map((item) => item.recencyScore));
  const sampleScores = metrics.map((item) => clamp(item.sampleSizeConfidence));
  const regimeScores = percentileScores(metrics.map((item) => regimeBaseScore(item, currentRegime)));

  const scored = metrics.map((item, index): StrategyScore => {
    const overallScore = clamp(
      pnlScores[index] * weights.pnl +
        profitFactorScores[index] * weights.profitFactor +
        winRateScores[index] * weights.winRate +
        drawdownScores[index] * weights.drawdown +
        sharpeScores[index] * weights.sharpe +
        recencyScores[index] * weights.recency +
        sampleScores[index] * weights.sampleSize,
    );

    return {
      strategyId: item.strategyId,
      strategyName: item.strategyName,
      metrics: item,
      pnlScore: pnlScores[index],
      profitFactorScore: profitFactorScores[index],
      winRateScore: winRateScores[index],
      drawdownScore: drawdownScores[index],
      sharpeScore: sharpeScores[index],
      recencyScore: recencyScores[index],
      sampleSizeScore: sampleScores[index],
      overallScore,
      currentRegimeScore: clamp(regimeScores[index]),
      confidenceRating: confidenceRating(item),
      rank: 0,
      regimeRank: 0,
    };
  });

  const byOverall = [...scored].sort((a, b) => b.overallScore - a.overallScore || b.metrics.netPnl - a.metrics.netPnl);
  byOverall.forEach((score, index) => {
    score.rank = index + 1;
  });

  const byRegime = [...scored].sort(
    (a, b) => b.currentRegimeScore - a.currentRegimeScore || b.overallScore - a.overallScore,
  );
  byRegime.forEach((score, index) => {
    score.regimeRank = index + 1;
  });

  return byOverall;
}
