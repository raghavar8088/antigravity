import { describe, expect, it } from "vitest";
import { buildTradeEngineStrategyRows } from "./StrategiesCenter";

describe("buildTradeEngineStrategyRows", () => {
  it("renders the Trade Engine roster when analytics are empty", () => {
    const rows = buildTradeEngineStrategyRows([]);

    expect(rows.length).toBeGreaterThan(0);
    expect(rows.some((row) => row.strategyId === 91 && row.strategyName === "Trend_Continuation_Long")).toBe(true);
    expect(rows.every((row) => row.total === 0)).toBe(true);
  });

  it("overlays ledger analytics onto roster rows", () => {
    const rows = buildTradeEngineStrategyRows([
      {
        strategyId: 91,
        strategyName: "Trend_Continuation_Long",
        total: 3,
        open: 1,
        closed: 2,
        wins: 1,
        losses: 1,
        winRate: 0.5,
        totalPnl: 42,
        realizedPnl: 30,
        unrealizedPnl: 12,
        exposure: 5000,
      },
    ]);

    const row = rows.find((item) => item.strategyId === 91);
    expect(row).toMatchObject({
      total: 3,
      open: 1,
      closed: 2,
      winRate: 0.5,
      totalPnl: 42,
      hasAnalytics: true,
    });
  });
});
