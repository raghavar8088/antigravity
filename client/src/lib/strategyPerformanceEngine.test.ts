import { describe, expect, it } from "vitest";
import { computeStrategyPerformance } from "@/lib/strategyPerformanceEngine";
import type { MockTrade } from "@/lib/mockTradingEngine";

const T0 = 1_700_000_000_000;

function trade(id: number, pnl: number, day: number, regimeAtEntry = "TRENDING"): MockTrade {
  return {
    id: `t-${id}`,
    traceId: `trace-${id}`,
    strategyId: 2000,
    strategyName: "Test Strategy",
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
    openedAt: T0 + day * 86_400_000,
    closedAt: T0 + day * 86_400_000 + 30 * 60_000,
    currentPrice: 60_500,
    unrealizedPnl: 0,
    realizedPnl: pnl,
    fees: 10,
    exitReason: pnl >= 0 ? "TAKE_PROFIT" : "STOP_LOSS",
    exitPrice: 60_500,
    strategyFamily: "Test",
    confidenceScore: 80,
    strategyParams: {},
    regimeAtEntry,
    researchPack: true,
  };
}

describe("computeStrategyPerformance", () => {
  it("computes net PnL, profit factor, expectancy, drawdown, and regime stats", () => {
    const metrics = computeStrategyPerformance([
      trade(1, 100, 0),
      trade(2, -50, 1),
      trade(3, 150, 2, "RANGING"),
      trade(4, -25, 3),
    ], T0 + 5 * 86_400_000);

    expect(metrics.netPnl).toBe(175);
    expect(metrics.grossPnl).toBe(215);
    expect(metrics.winRate).toBe(0.5);
    expect(metrics.profitFactor).toBeCloseTo(250 / 75, 4);
    expect(metrics.expectancy).toBeCloseTo(43.75, 4);
    expect(metrics.maxDrawdown).toBe(50);
    expect(metrics.avgHoldMinutes).toBe(30);
    expect(metrics.regimeBreakdown.TRENDING.trades).toBe(3);
    expect(metrics.regimeBreakdown.RANGING.netPnl).toBe(150);
    expect(metrics.sampleSizeConfidence).toBe(20);
  });
});
