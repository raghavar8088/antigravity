import { describe, expect, it } from "vitest";
import { computeStrategyHealth } from "@/lib/ai/strategyHealthEngine";
import type { MockTrade } from "@/lib/trading/mockTradingEngine";
import type { StrategyScore } from "@/lib/ai/strategyScoringEngine";
import type { StrategyPerformanceMetrics } from "@/lib/ai/strategyPerformanceEngine";

const T0 = 1_700_000_000_000;

function metrics(overrides: Partial<StrategyPerformanceMetrics>): StrategyPerformanceMetrics {
  return {
    strategyId: 2000,
    strategyName: "Health Test",
    totalTrades: 25,
    closedTrades: 25,
    grossPnl: 100,
    netPnl: 100,
    winRate: 0.6,
    profitFactor: 1.4,
    avgWin: 20,
    avgLoss: -10,
    expectancy: 4,
    sharpeRatio: 1,
    sortinoRatio: 1,
    maxDrawdown: 50,
    maxDrawdownPct: 5,
    recoveryFactor: 2,
    avgHoldMinutes: 30,
    recencyScore: 70,
    last7DaysPnl: 50,
    last7DaysWinRate: 0.6,
    last7DaysTrades: 5,
    regimeBreakdown: {
      STRONG_TREND: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      WEAK_TREND: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      RANGE: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      HIGH_VOLATILITY: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      LOW_VOLATILITY: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      BREAKOUT: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      REVERSAL: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      TRENDING: { trades: 5, winRate: 0.6, expectancy: 4, netPnl: 20 },
      RANGING: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      HIGH_VOLATILITY_BREAKOUT: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      LOW_VOLATILITY_CHOP: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
      UNKNOWN: { trades: 0, winRate: 0, expectancy: 0, netPnl: 0 },
    },
    sampleSizeConfidence: 80,
    ...overrides,
  };
}

function score(overrides: Partial<StrategyPerformanceMetrics>): StrategyScore {
  const m = metrics(overrides);
  return {
    strategyId: m.strategyId,
    strategyName: m.strategyName,
    metrics: m,
    pnlScore: 70,
    profitFactorScore: 70,
    winRateScore: 70,
    drawdownScore: 70,
    sharpeScore: 70,
    recencyScore: 70,
    sampleSizeScore: 80,
    overallScore: 70,
    currentRegimeScore: 70,
    confidenceRating: "HIGH",
    rank: 1,
    regimeRank: 1,
  };
}

function closedLoss(id: number): MockTrade {
  return {
    id: `t-${id}`,
    traceId: `trace-${id}`,
    strategyId: 2000,
    strategyName: "Health Test",
    symbol: "BTCUSD",
    side: "BUY",
    notional: 10_000,
    quantity: 0.15,
    leverage: 25,
    marginUsed: 400,
    signalPrice: 60_000,
    entryPrice: 60_000,
    takeProfitPrice: 61_000,
    stopLossPrice: 59_500,
    takeProfitUsd: 100,
    stopLossUsd: 50,
    riskRewardRatio: 2,
    signalScore: 80,
    requiredThreshold: 0,
    blockers: [],
    status: "CLOSED",
    openedAt: T0 + id * 60_000,
    closedAt: T0 + id * 60_000 + 30_000,
    currentPrice: 59_500,
    unrealizedPnl: 0,
    realizedPnl: -20,
    fees: 10,
    exitReason: "STOP_LOSS",
    exitPrice: 59_500,
  };
}

describe("computeStrategyHealth", () => {
  it("marks insufficient samples as watchlist", () => {
    const rows = computeStrategyHealth([score({ closedTrades: 4, totalTrades: 4, sampleSizeConfidence: 20 })], []);
    expect(rows[0].state).toBe("WATCHLIST");
  });

  it("auto-disables negative expectancy and low profit factor strategies", () => {
    const rows = computeStrategyHealth([score({ expectancy: -2, profitFactor: 0.7 })], []);
    expect(rows[0].state).toBe("DISABLED");
    expect(rows[0].reasons.join(" ")).toContain("Negative expectancy");
    expect(rows[0].reasons.join(" ")).toContain("Profit factor");
  });

  it("auto-disables persistent losing streaks", () => {
    const rows = computeStrategyHealth([score({})], [1, 2, 3, 4, 5].map(closedLoss));
    expect(rows[0].state).toBe("DISABLED");
    expect(rows[0].consecutiveLosses).toBe(5);
  });
});
