import { describe, expect, it } from "vitest";
import { computeMockWalkForwardRows } from "@/lib/mockResearchWalkForward";
import type { MockTrade } from "@/lib/mockTradingEngine";

const T0 = Date.UTC(2026, 0, 1);

function trade(id: number, day: number, pnl: number): MockTrade {
  return {
    id: `t-${id}`,
    traceId: `trace-${id}`,
    strategyId: 2000,
    strategyName: "Walk Forward Test",
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
    closedAt: T0 + day * 86_400_000 + 60_000,
    currentPrice: 60_500,
    unrealizedPnl: 0,
    realizedPnl: pnl,
    fees: 10,
    exitReason: pnl >= 0 ? "TAKE_PROFIT" : "STOP_LOSS",
    exitPrice: 60_500,
  };
}

describe("computeMockWalkForwardRows", () => {
  it("computes out-of-sample metrics and walk-forward score", () => {
    const trades = Array.from({ length: 40 }, (_, index) => trade(index, index, index < 28 ? 20 : 12));
    const rows = computeMockWalkForwardRows(trades, {
      trainDays: 14,
      validationDays: 7,
      minTrainTrades: 5,
      minValidationTrades: 3,
    });

    expect(rows[0].strategyId).toBe(2000);
    expect(rows[0].windows).toBeGreaterThan(0);
    expect(rows[0].outOfSampleTrades).toBeGreaterThan(0);
    expect(rows[0].walkForwardScore).toBeGreaterThan(0);
  });
});
