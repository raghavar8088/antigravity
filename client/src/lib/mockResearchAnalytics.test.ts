import { describe, expect, it } from "vitest";
import {
  computeAdvancedResearchAnalytics,
  computeDailyPnlPoints,
  computeStrategyComparisonSeries,
  createEquitySnapshot,
} from "@/lib/mockResearchAnalytics";
import type { MockAccountState, MockTrade } from "@/lib/mockTradingEngine";

const T0 = 1_700_000_000_000;

function trade(id: number, strategyId: number, side: "BUY" | "SELL", pnl: number, family: string): MockTrade {
  return {
    id: `t-${id}`,
    traceId: `trace-${id}`,
    strategyId,
    strategyName: `Strategy ${strategyId}`,
    symbol: "BTCUSD",
    side,
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
    currentPrice: 60_500,
    unrealizedPnl: 0,
    realizedPnl: pnl,
    fees: 10,
    exitReason: pnl >= 0 ? "TAKE_PROFIT" : "STOP_LOSS",
    exitPrice: 60_500,
    strategyFamily: family,
    confidenceScore: 80,
    strategyParams: {},
    regimeAtEntry: "TRENDING",
    researchPack: true,
  };
}

const account: MockAccountState = {
  startingBalance: 1_000_000,
  cashBalance: 1_000_100,
  equity: 1_000_100,
  realizedPnl: 100,
  unrealizedPnl: 0,
  exposure: 0,
  longExposure: 0,
  shortExposure: 0,
  marginUsed: 0,
  availableBalance: 1_000_100,
  returnPct: 0.01,
  peakEquity: 1_000_200,
  maxDrawdownPct: 0.01,
  openCount: 0,
  closedCount: 4,
};

describe("mock research analytics", () => {
  it("builds daily PnL, equity snapshots, comparisons, and risk analytics", () => {
    const trades = [
      trade(1, 2000, "BUY", 100, "VWAP"),
      trade(2, 2000, "SELL", -30, "VWAP"),
      trade(3, 2010, "BUY", 80, "Breakout"),
      trade(4, 2010, "SELL", -50, "Breakout"),
    ];

    expect(computeDailyPnlPoints(trades)[0]?.value).toBe(100);
    expect(computeStrategyComparisonSeries(trades, 2)).toHaveLength(2);

    const snapshot = createEquitySnapshot({ account, trades, regime: "TRENDING", timestamp: T0 });
    expect(snapshot.equity).toBe(1_000_100);
    expect(snapshot.regime).toBe("TRENDING");

    const analytics = computeAdvancedResearchAnalytics({ trades, scores: [], account });
    expect(analytics.familyRows).toHaveLength(2);
    expect(analytics.bias.longTrades).toBe(2);
    expect(analytics.bias.shortTrades).toBe(2);
  });
});
