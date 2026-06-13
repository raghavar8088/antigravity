import { describe, expect, it } from "vitest";
import { computePortfolioAllocation } from "@/lib/ai/mockResearchPortfolioAllocation";
import type { StrategyHealthRow } from "@/lib/ai/strategyHealthEngine";
import type { StrategyScore } from "@/lib/ai/strategyScoringEngine";
import type { StrategyPerformanceMetrics } from "@/lib/ai/strategyPerformanceEngine";

function metrics(id: number): StrategyPerformanceMetrics {
  return {
    strategyId: id,
    strategyName: `Strategy ${id}`,
    totalTrades: 30,
    closedTrades: 30,
    grossPnl: 500,
    netPnl: 450,
    winRate: 0.6,
    profitFactor: 1.8,
    avgWin: 50,
    avgLoss: -25,
    expectancy: 15,
    sharpeRatio: 1.2,
    sortinoRatio: 1.4,
    maxDrawdown: 100,
    maxDrawdownPct: 4,
    recoveryFactor: 4.5,
    avgHoldMinutes: 20,
    recencyScore: 80,
    last7DaysPnl: 120,
    last7DaysWinRate: 0.7,
    last7DaysTrades: 10,
    regimeBreakdown: {
      STRONG_TREND: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      WEAK_TREND: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      RANGE: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      HIGH_VOLATILITY: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      LOW_VOLATILITY: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      BREAKOUT: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      REVERSAL: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      TRENDING: { trades: 10, winRate: 0.6, expectancy: 15, netPnl: 150 },
      RANGING: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      HIGH_VOLATILITY_BREAKOUT: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      LOW_VOLATILITY_CHOP: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      UNKNOWN: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
    },
    sampleSizeConfidence: 90,
  };
}

function score(id: number, overall: number): StrategyScore {
  return {
    strategyId: id,
    strategyName: `Strategy ${id}`,
    metrics: metrics(id),
    pnlScore: overall,
    profitFactorScore: overall,
    winRateScore: overall,
    drawdownScore: overall,
    sharpeScore: overall,
    recencyScore: overall,
    sampleSizeScore: 90,
    overallScore: overall,
    currentRegimeScore: overall,
    confidenceRating: "HIGH",
    rank: id - 1999,
    regimeRank: id - 1999,
  };
}

function health(id: number, state: StrategyHealthRow["state"]): StrategyHealthRow {
  return {
    strategyId: id,
    strategyName: `Strategy ${id}`,
    state,
    trustScore: 80,
    reasons: ["test"],
    closedTrades: 30,
    expectancy: 15,
    profitFactor: 1.8,
    consecutiveLosses: 0,
  };
}

describe("computePortfolioAllocation", () => {
  it("allocates capital only to active strategies and rolls up families", () => {
    const familyMap = new Map([
      [2000, "VWAP"],
      [2001, "Breakout"],
      [2002, "Disabled"],
    ]);
    const result = computePortfolioAllocation({
      scores: [score(2000, 90), score(2001, 70), score(2002, 95)],
      healthRows: [health(2000, "ACTIVE"), health(2001, "ACTIVE"), health(2002, "DISABLED")],
      equity: 1_000_000,
      strategyFamilyById: familyMap,
    });

    expect(result.strategyRows.map((row) => row.strategyId)).not.toContain(2002);
    expect(result.strategyRows.reduce((sum, row) => sum + row.allocationPct, 0)).toBeLessThanOrEqual(1);
    expect(result.familyRows.map((row) => row.family)).toEqual(["VWAP", "Breakout"]);
  });
});
