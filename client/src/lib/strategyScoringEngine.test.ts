import { describe, expect, it } from "vitest";
import { scoreAllStrategies } from "@/lib/ai/strategyScoringEngine";
import type { StrategyPerformanceMetrics } from "@/lib/ai/strategyPerformanceEngine";

function metrics(id: number, netPnl: number, winRate: number, profitFactor: number): StrategyPerformanceMetrics {
  return {
    strategyId: id,
    strategyName: `Strategy ${id}`,
    totalTrades: 20,
    closedTrades: 20,
    grossPnl: netPnl + 100,
    netPnl,
    winRate,
    profitFactor,
    avgWin: 100,
    avgLoss: -50,
    expectancy: netPnl / 20,
    sharpeRatio: netPnl / 100,
    sortinoRatio: netPnl / 120,
    maxDrawdown: Math.max(0, 300 - netPnl),
    maxDrawdownPct: Math.max(0, 30 - netPnl / 10),
    recoveryFactor: 2,
    avgHoldMinutes: 30,
    recencyScore: netPnl > 0 ? 80 : 30,
    last7DaysPnl: netPnl / 2,
    last7DaysWinRate: winRate,
    last7DaysTrades: 5,
    regimeBreakdown: {
      STRONG_TREND: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      WEAK_TREND: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      RANGE: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      HIGH_VOLATILITY: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      LOW_VOLATILITY: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      BREAKOUT: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      REVERSAL: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      TRENDING: { trades: 5, winRate, expectancy: netPnl / 20, netPnl },
      RANGING: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      HIGH_VOLATILITY_BREAKOUT: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      LOW_VOLATILITY_CHOP: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      UNKNOWN: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
    },
    sampleSizeConfidence: 80,
  };
}

function withRangingExpectancy(metric: StrategyPerformanceMetrics, expectancy: number): StrategyPerformanceMetrics {
  return {
    ...metric,
    regimeBreakdown: {
      ...metric.regimeBreakdown,
      RANGING: { trades: 5, winRate: 0.6, expectancy, netPnl: expectancy * 5 },
    },
  };
}

describe("scoreAllStrategies", () => {
  it("ranks stronger strategy metrics above weaker ones", () => {
    const performances = new Map([
      [2000, metrics(2000, 500, 0.65, 2.5)],
      [2001, metrics(2001, -100, 0.35, 0.8)],
      [2002, metrics(2002, 200, 0.55, 1.5)],
    ]);

    const scores = scoreAllStrategies(performances, "TRENDING");

    expect(scores[0].strategyId).toBe(2000);
    expect(scores[0].rank).toBe(1);
    expect(scores.every((score) => score.overallScore >= 0 && score.overallScore <= 100)).toBe(true);
    expect(scores.every((score) => score.currentRegimeScore >= 0 && score.currentRegimeScore <= 100)).toBe(true);
    expect(scores[0].confidenceRating).toBe("HIGH");
  });

  it("updates regime-specific rankings from current-regime expectancy", () => {
    const performances = new Map([
      [2000, withRangingExpectancy(metrics(2000, 500, 0.65, 2.5), -10)],
      [2001, withRangingExpectancy(metrics(2001, 100, 0.5, 1.2), 50)],
      [2002, withRangingExpectancy(metrics(2002, 200, 0.55, 1.5), 15)],
    ]);

    const scores = scoreAllStrategies(performances, "RANGING");
    const regimeLeader = scores.find((score) => score.regimeRank === 1);

    expect(regimeLeader?.strategyId).toBe(2001);
  });
});
