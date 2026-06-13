import { describe, expect, it } from "vitest";
import {
  computeSessionExitReasonAnalytics,
  computeSessionTradingMetrics,
  effectiveMinExpectedMoveSafetyK,
  formatExitReasonSessionSummary,
  FUTURES_STRATEGY_PROFILES,
  resolveStrategyProfile,
} from "./analytics/futuresSessionMetrics";
import { effectiveSignalThreshold } from "./trading/futuresSignals";

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

describe("computeSessionExitReasonAnalytics", () => {
  const t0 = "2026-05-11T10:00:00.000Z";
  const t1 = "2026-05-11T10:05:00.000Z";

  it("groups counts and mean net by exitReason", () => {
    const trades = [
      { openedAt: t0, closedAt: t1, netPnl: 2, fees: 0.2, realizedPnl: 2.2, exitReason: "TP" },
      { openedAt: t0, closedAt: t1, netPnl: 1, fees: 0.2, realizedPnl: 1.2, exitReason: "TP" },
      { openedAt: t0, closedAt: t1, netPnl: -0.5, fees: 0.2, realizedPnl: -0.3, exitReason: "TIME" },
    ];
    const { rows, totalInWindow } = computeSessionExitReasonAnalytics(trades, 400);
    expect(totalInWindow).toBe(3);
    const tp = rows.find((r) => r.reason === "TP");
    const time = rows.find((r) => r.reason === "TIME");
    expect(tp?.count).toBe(2);
    expect(tp?.avgNet).toBeCloseTo(1.5, 5);
    expect(time?.count).toBe(1);
    expect(time?.avgNet).toBeCloseTo(-0.5, 5);
  });

  it("formats a compact summary string", () => {
    const trades = [
      { openedAt: t0, closedAt: t1, netPnl: 2, fees: 0.1, realizedPnl: 2.1, exitReason: "SL" },
    ];
    const s = formatExitReasonSessionSummary(computeSessionExitReasonAnalytics(trades).rows);
    expect(s).toContain("SL×1");
    expect(s).toContain("avg+");
  });
});

describe("strategy profiles", () => {
  it("resolves known profiles", () => {
    expect(resolveStrategyProfile(undefined)).toBe("baseline");
    expect(resolveStrategyProfile("scalp_aggro_v1")).toBe("scalp_aggro_v1");
    expect(resolveStrategyProfile("fee_aware_v1")).toBe("fee_aware_v1");
    expect(resolveStrategyProfile("unknown")).toBe("baseline");
  });

  it("keeps invariant-friendly multipliers", () => {
    expect(FUTURES_STRATEGY_PROFILES.baseline.cooldownMul).toBe(1);
    expect(FUTURES_STRATEGY_PROFILES.baseline.minExpectedMoveSafetyKMul).toBe(1);
    expect(FUTURES_STRATEGY_PROFILES.scalp_aggro_v1.cooldownMul).toBeLessThan(1);
    expect(FUTURES_STRATEGY_PROFILES.scalp_aggro_v1.holdTimeMul).toBeLessThanOrEqual(1);
    expect(FUTURES_STRATEGY_PROFILES.scalp_aggro_v1.minExpectedMoveSafetyKMul).toBe(1);
  });

  it("fee_aware_v1: +6 signal delta, 1.25× min-move K, unit hold/cooldown", () => {
    const p = FUTURES_STRATEGY_PROFILES.fee_aware_v1;
    expect(p.signalThresholdDelta).toBe(6);
    expect(p.minExpectedMoveSafetyKMul).toBe(1.25);
    expect(p.holdTimeMul).toBe(1);
    expect(p.cooldownMul).toBe(1);
    expect(effectiveSignalThreshold(26, p.signalThresholdDelta)).toBe(32);
    expect(effectiveMinExpectedMoveSafetyK(1, p)).toBe(1.25);
  });
});
