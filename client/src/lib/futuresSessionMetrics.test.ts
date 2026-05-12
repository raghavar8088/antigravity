import { describe, expect, it } from "vitest";
import {
  computeSessionTradingMetrics,
  FUTURES_STRATEGY_PROFILES,
  resolveStrategyProfile,
} from "./futuresSessionMetrics";

describe("computeSessionTradingMetrics", () => {
  const t0 = "2026-05-11T10:00:00.000Z";
  const t1 = "2026-05-11T10:05:00.000Z";
  const t2 = "2026-05-11T10:20:00.000Z";

  it("returns zeros for empty trades", () => {
    const m = computeSessionTradingMetrics([], Date.now());
    expect(m.tradesPerHour).toBe(0);
    expect(m.expectancyPerTrade).toBe(0);
    expect(m.feePctOfAbsGross).toBe(0);
  });

  it("computes expectancy and fee % of abs gross", () => {
    const trades = [
      { openedAt: t0, closedAt: t1, netPnl: 2, fees: 0.2, realizedPnl: 2.2 },
      { openedAt: t0, closedAt: t1, netPnl: -1, fees: 0.2, realizedPnl: -0.8 },
    ];
    const m = computeSessionTradingMetrics(trades, new Date(t2).getTime());
    expect(m.expectancyPerTrade).toBeCloseTo(0.5, 5);
    const absGross = 2.2 + 0.8;
    const feePct = (0.4 / absGross) * 100;
    expect(m.feePctOfAbsGross).toBeCloseTo(feePct, 5);
  });

  it("computes hold distribution and trades/hour floor", () => {
    const trades = [
      { openedAt: t0, closedAt: t1, netPnl: 0, fees: 0.1, realizedPnl: 0.1 },
    ];
    const now = new Date(t1).getTime();
    const m = computeSessionTradingMetrics(trades, now);
    expect(m.avgHoldMinutes).toBeCloseTo(5, 5);
    expect(m.medianHoldMinutes).toBeCloseTo(5, 5);
    expect(m.holdP95Minutes).toBeCloseTo(5, 5);
    expect(m.tradesPerHour).toBeGreaterThan(0);
  });
});

describe("strategy profiles", () => {
  it("resolves known profiles", () => {
    expect(resolveStrategyProfile(undefined)).toBe("baseline");
    expect(resolveStrategyProfile("scalp_aggro_v1")).toBe("scalp_aggro_v1");
    expect(resolveStrategyProfile("unknown")).toBe("baseline");
  });

  it("keeps invariant-friendly multipliers", () => {
    expect(FUTURES_STRATEGY_PROFILES.baseline.cooldownMul).toBe(1);
    expect(FUTURES_STRATEGY_PROFILES.scalp_aggro_v1.cooldownMul).toBeLessThan(1);
    expect(FUTURES_STRATEGY_PROFILES.scalp_aggro_v1.holdTimeMul).toBeLessThanOrEqual(1);
  });
});
