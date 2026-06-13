import { describe, expect, it } from "vitest";
import {
  isProbeOrBootstrapTrade,
  computeSessionEquityFromProduction,
  computeSessionTradingMetrics,
} from "../analytics/futuresSessionMetrics";

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

  it("prefers strategy_name (DB snake_case field) over strategyName when both are set", () => {
    expect(isProbeOrBootstrapTrade({ strategyName: "btc_scalp_v1", strategy_name: "bootstrap" })).toBe(true);
    expect(isProbeOrBootstrapTrade({ strategyName: null, strategy_name: "bootstrap" })).toBe(true);
    expect(isProbeOrBootstrapTrade({ strategyName: "btc_scalp_v1", strategy_name: null })).toBe(false);
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

describe("computeSessionEquityFromProduction", () => {
  const INITIAL = 1000;

  it("returns $0 session PnL when 0 production trades and 0 unrealized", () => {
    const result = computeSessionEquityFromProduction({
      initialBalance: INITIAL,
      productionClosedNetPnl: 0,
      productionUnrealizedPnl: 0,
    });
    expect(result.sessionPnL).toBe(0);
    expect(result.equity).toBe(INITIAL);
    expect(result.totalReturnPct).toBe(0);
  });

  it("returns correct session PnL from 3 production trades summing +$50", () => {
    const result = computeSessionEquityFromProduction({
      initialBalance: INITIAL,
      productionClosedNetPnl: 50,
      productionUnrealizedPnl: 0,
    });
    expect(result.sessionPnL).toBe(50);
    expect(result.equity).toBe(1050);
    expect(result.totalReturnPct).toBeCloseTo(5, 5);
  });

  it("ignores raw balance inflation — only uses production sums", () => {
    // Raw balance might be $999,421 from probe trades, but production PnL is -$10.
    const result = computeSessionEquityFromProduction({
      initialBalance: INITIAL,
      productionClosedNetPnl: -10,
      productionUnrealizedPnl: 0,
    });
    expect(result.sessionPnL).toBe(-10);
    expect(result.equity).toBe(990);
    // NOT 999421 - 1000 = 998421
  });

  it("includes unrealized PnL from open production positions", () => {
    const result = computeSessionEquityFromProduction({
      initialBalance: INITIAL,
      productionClosedNetPnl: 20,
      productionUnrealizedPnl: 5,
    });
    expect(result.sessionPnL).toBe(25);
    expect(result.equity).toBe(1025);
  });
});

describe("isProbeOrBootstrapTrade safety cap — logic contract", () => {
  it("identifies probe trades that should be capped", () => {
    const absurdNetPnl = 15_000; // 15× $1000 initial
    const INITIAL_BALANCE = 1000;
    const isProbe = isProbeOrBootstrapTrade({ strategyName: "BOOTSTRAP_force_open" });
    const shouldCap = isProbe && Math.abs(absurdNetPnl) > 10 * INITIAL_BALANCE;
    expect(isProbe).toBe(true);
    expect(shouldCap).toBe(true);
  });

  it("does not cap production trades even with large PnL", () => {
    const largePnl = 15_000;
    const INITIAL_BALANCE = 1000;
    const isProbe = isProbeOrBootstrapTrade({ strategyName: "btc_scalp_v1" });
    const shouldCap = isProbe && Math.abs(largePnl) > 10 * INITIAL_BALANCE;
    expect(isProbe).toBe(false);
    expect(shouldCap).toBe(false);
  });

  it("does not cap probe trades within normal range", () => {
    const smallPnl = 5; // well under 10× initial
    const INITIAL_BALANCE = 1000;
    const isProbe = isProbeOrBootstrapTrade({ strategyName: "probe_entry" });
    const shouldCap = isProbe && Math.abs(smallPnl) > 10 * INITIAL_BALANCE;
    expect(isProbe).toBe(true);
    expect(shouldCap).toBe(false);
  });
});

describe("computeSessionTradingMetrics — probe filtering", () => {
  it("excludes probe trades from expectancy calculation", () => {
    const trades = [
      makeTrade(10, 1, 11, "btc_scalp_v1"),
      makeTrade(-50, 2, -48, "bootstrap_probe"),
      makeTrade(10, 1, 11, "btc_scalp_v2"),
    ];
    const metrics = computeSessionTradingMetrics(trades, Date.now());
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
    expect(metrics.feePctOfAbsGross).toBeCloseTo(20, 1);
  });

  it("excludes probe holds from hold-time percentiles", () => {
    const trades = [
      makeTrade(5, 1, 6, "btc_scalp_v1", 600_000),
      makeTrade(-2, 1, -1, "bootstrap", 1_000),
    ];
    const metrics = computeSessionTradingMetrics(trades, Date.now());
    expect(metrics.medianHoldMinutes).toBeCloseTo(10, 0);
  });
});
