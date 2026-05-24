import { describe, expect, it } from "vitest";
import { isProbeOrBootstrapTrade, computeSessionTradingMetrics } from "../futuresSessionMetrics";

describe("isProbeOrBootstrapTrade", () => {
  it("returns true for BOOTSTRAP strategy names (case-insensitive)", () => {
    expect(isProbeOrBootstrapTrade({ strategyName: "bootstrap_long" })).toBe(true);
    expect(isProbeOrBootstrapTrade({ strategyName: "BOOTSTRAP" })).toBe(true);
    expect(isProbeOrBootstrapTrade({ strategy_name: "Bootstrap_v1" })).toBe(true);
  });

  it("returns true for PROBE strategy names", () => {
    expect(isProbeOrBootstrapTrade({ strategyName: "probe_entry" })).toBe(true);
    expect(isProbeOrBootstrapTrade({ strategy_name: "PROBE" })).toBe(true);
  });

  it("returns true for DEV_FORCE strategy names", () => {
    expect(isProbeOrBootstrapTrade({ strategyName: "DEV_FORCE_open" })).toBe(true);
    expect(isProbeOrBootstrapTrade({ strategy_name: "dev_force_probe" })).toBe(true);
  });

  it("returns false for production strategy names", () => {
    expect(isProbeOrBootstrapTrade({ strategyName: "btc_scalp_v1" })).toBe(false);
    expect(isProbeOrBootstrapTrade({ strategyName: "trend_follow_long" })).toBe(false);
    expect(isProbeOrBootstrapTrade({ strategyName: undefined })).toBe(false);
    expect(isProbeOrBootstrapTrade({ strategy_name: null })).toBe(false);
  });

  it("prefers strategyName over strategy_name", () => {
    // strategyName is production, strategy_name is probe — should use first non-null
    expect(isProbeOrBootstrapTrade({ strategyName: "btc_scalp_v1", strategy_name: "bootstrap" })).toBe(false);
    expect(isProbeOrBootstrapTrade({ strategyName: null, strategy_name: "bootstrap" })).toBe(true);
  });
});

const makeTrade = (
  netPnl: number,
  fees: number,
  realizedPnl: number,
  strategyName?: string,
  holdMs = 300_000,
) => {
  const openedAt = new Date(Date.now() - holdMs).toISOString();
  const closedAt = new Date().toISOString();
  return { openedAt, closedAt, netPnl, fees, realizedPnl, strategyName };
};

describe("computeSessionTradingMetrics — probe filtering", () => {
  it("excludes probe trades from expectancy calculation", () => {
    const trades = [
      makeTrade(10, 1, 11, "btc_scalp_v1"),
      makeTrade(-50, 2, -48, "bootstrap_probe"),
      makeTrade(10, 1, 11, "btc_scalp_v2"),
    ];
    const metrics = computeSessionTradingMetrics(trades, Date.now());
    // Only 2 production trades: net = 10 + 10 = 20, expectancy = 10
    expect(metrics.expectancyPerTrade).toBeCloseTo(10, 5);
  });

  it("returns zeroes when all trades are probes", () => {
    const trades = [
      makeTrade(-30, 2, -28, "BOOTSTRAP_short"),
      makeTrade(-20, 1, -19, "probe_force"),
    ];
    const metrics = computeSessionTradingMetrics(trades, Date.now());
    expect(metrics.expectancyPerTrade).toBe(0);
    expect(metrics.tradesPerHour).toBe(0);
    expect(metrics.feePctOfAbsGross).toBe(0);
  });

  it("excludes probe trades from fee ratio", () => {
    const trades = [
      makeTrade(8, 2, 10, "btc_scalp_v1"),
      makeTrade(-100, 10, -90, "DEV_FORCE_open"),
    ];
    const metrics = computeSessionTradingMetrics(trades, Date.now());
    // Only production trade: fees=2, absGross=10 → 20%
    expect(metrics.feePctOfAbsGross).toBeCloseTo(20, 1);
  });

  it("excludes probe holds from hold-time percentiles", () => {
    const trades = [
      makeTrade(5, 1, 6, "btc_scalp_v1", 600_000),    // 10 min
      makeTrade(-2, 1, -1, "bootstrap", 1_000),         // 1.67 sec probe
    ];
    const metrics = computeSessionTradingMetrics(trades, Date.now());
    // Only 1 production trade → median ≈ 10 min
    expect(metrics.medianHoldMinutes).toBeCloseTo(10, 0);
  });
});
