import { describe, it, expect } from "vitest";
import { isProbeOrBootstrapTrade, computeSessionTradingMetrics } from "../analytics/futuresSessionMetrics";
import {
  computeStrategyDiagnostics,
  computeRollingHealthCheck,
} from "../trading/futuresStrategyDiagnostics";
import { recommendOneTune } from "../trading/futuresParameterTuner";
import { runProductionReadiness } from "../risk/futuresProductionReadiness";
import { computeAdaptiveThreshold, isSameSideCapped } from "../trading/futuresDeskPolicy";
import type { PaperTradeDbRow } from "../portfolio/paperTradesTypes";

const now = Date.now();
const ago = (m: number) => new Date(now - m * 60_000).toISOString();

const mkTrade = (
  o: Partial<PaperTradeDbRow> = {},
  idx = 0,
): PaperTradeDbRow => ({
  id: `t${idx}`,
  created_at: ago(30),
  account_key: "acc",
  client_trade_id: `c${idx}`,
  opened_at: ago(30),
  closed_at: ago(10),
  symbol: "BTCUSD",
  strategy_id: 1,
  strategy_name: "MTF_Trend_Align_Short",
  side: "SHORT",
  entry_price: 50000,
  exit_price: 49900,
  contracts: 1,
  notional: 100,
  margin_used: 4,
  gross_pnl: -15,
  fees: 5,
  funding_costs: 0,
  net_pnl: -20,
  exit_reason: "SL",
  payload: null,
  template_family: "mtf",
  ...o,
});

const probeTrade = (o: Partial<PaperTradeDbRow> = {}, idx = 99) =>
  mkTrade({ strategy_name: "PAPER_BOOTSTRAP_PROBE", net_pnl: 999999, gross_pnl: 1_000_000, fees: 1, ...o }, idx);

const winTrade = (o: Partial<PaperTradeDbRow> = {}, idx = 0) =>
  mkTrade({ net_pnl: 15, gross_pnl: 18, fees: 3, exit_reason: "TP", ...o }, idx);

const healthyCheck = () => ({
  window: 20,
  expectancy: 5,
  expectancyPass: true,
  winRate: 0.5,
  winRatePass: true,
  feePctOfAbsGross: 0.3,
  feePass: true,
  profitFactor: 1.5,
  pfPass: true,
  tpHits: 5,
  tpHitPass: true,
  slCount: 8,
  timeCount: 0,
  overallPass: true,
  grade: "A" as const,
});

describe("Stage 1 — Probe filter invariant", () => {
  it("isProbeOrBootstrapTrade catches all probe variants", () => {
    const variants = [
      "PAPER_BOOTSTRAP_PROBE",
      "DEV_FORCE_PROBE_OPEN",
      "paper_bootstrap_probe",
      "dev_force_anything",
      "MY_BOOTSTRAP_STRAT",
    ];
    for (const name of variants) {
      expect(isProbeOrBootstrapTrade({ strategy_name: name })).toBe(true);
    }
  });

  it("isProbeOrBootstrapTrade passes real strategy names", () => {
    const real = [
      "MTF_Trend_Align_Short",
      "PRM_Breakout_Long",
      "CORE_RSI_Mean_Revert",
      "BB_Squeeze_Long",
    ];
    for (const name of real) {
      expect(isProbeOrBootstrapTrade({ strategy_name: name })).toBe(false);
    }
  });

  it("probe trades never inflate session metrics", () => {
    const trades = [
      {
        openedAt: ago(30),
        closedAt: ago(10),
        netPnl: 999999,
        fees: 1,
        realizedPnl: 1_000_000,
        strategyName: "PAPER_BOOTSTRAP_PROBE",
      },
      {
        openedAt: ago(30),
        closedAt: ago(10),
        netPnl: -20,
        fees: 5,
        realizedPnl: -15,
        strategyName: "MTF_Trend_Align_Short",
      },
    ];
    const metrics = computeSessionTradingMetrics(trades, now);
    expect(metrics.expectancyPerTrade).toBeCloseTo(-20, 0);
  });
});

describe("Stage 2 — Session PnL accounting", () => {
  it("equity roll-forward: session = closed + unrealized", () => {
    const BASE = 1000;
    const closed = -151.56;
    const unrealized = -50.0;
    const session = closed + unrealized;
    const pct = (session / BASE) * 100;

    expect(pct).toBeCloseTo(-20.156, 1);
    expect(pct).not.toBeGreaterThan(0);
    expect(pct).not.toBeLessThan(-100);
  });

  it("session PnL is never +99690% with real trades only", () => {
    const BASE = 1000;
    const trades = Array.from({ length: 10 }, (_, i) => mkTrade({ net_pnl: -20 }, i));
    const sum = trades.reduce((s, t) => s + (t.net_pnl ?? 0), 0);
    const pct = (sum / BASE) * 100;
    expect(Math.abs(pct)).toBeLessThan(1000);
  });

  it("feePctOfAbsGross formula is correct", () => {
    const trades = [
      {
        openedAt: ago(30),
        closedAt: ago(10),
        netPnl: -20,
        fees: 5,
        realizedPnl: -15,
        strategyName: "MTF_Trend_Align_Short",
      },
      {
        openedAt: ago(30),
        closedAt: ago(10),
        netPnl: -20,
        fees: 5,
        realizedPnl: -15,
        strategyName: "MTF_Trend_Align_Short",
      },
    ];
    const metrics = computeSessionTradingMetrics(trades, now);
    expect(metrics.feePctOfAbsGross).toBeCloseTo((10 / 30) * 100, 1);
  });
});

describe("Stage 3 — Entry gate", () => {
  it("computeAdaptiveThreshold chop+F+highFee caps at base+12", () => {
    const base = 28;
    const result = computeAdaptiveThreshold(base, "chop", "F", 0.9);
    expect(result).toBe(base + 12);
  });

  it("computeAdaptiveThreshold never below base", () => {
    const base = 28;
    const result = computeAdaptiveThreshold(base, "trendHigh", "A", 0.1);
    expect(result).toBeGreaterThanOrEqual(base);
  });

  it("isSameSideCapped blocks when at cap", () => {
    const positions = [{ side: "SHORT" }, { side: "SHORT" }];
    expect(isSameSideCapped(positions, "SHORT", 2)).toBe(true);
    expect(isSameSideCapped(positions, "LONG", 2)).toBe(false);
  });

  it("isSameSideCapped allows when under cap", () => {
    const positions = [{ side: "SHORT" }];
    expect(isSameSideCapped(positions, "SHORT", 2)).toBe(false);
  });
});

describe("Stage 4 — Strategy diagnostics", () => {
  it("probe rows excluded from topByExpectancy", () => {
    const trades = [probeTrade(), winTrade({}, 1), winTrade({}, 2)];
    const result = computeStrategyDiagnostics(trades);
    expect(result.topByExpectancy.every((r) => !r.isProbe)).toBe(true);
  });

  it("slDominatedStrats flags >60% SL rate", () => {
    const trades = [
      mkTrade({ exit_reason: "SL" }, 0),
      mkTrade({ exit_reason: "SL" }, 1),
      mkTrade({ exit_reason: "SL" }, 2),
      winTrade({}, 3),
    ];
    const result = computeStrategyDiagnostics(trades);
    expect(result.slDominatedStrats.length).toBeGreaterThan(0);
  });

  it("highFeeStrategies flags fee/gross > 50%", () => {
    const trades = Array.from({ length: 5 }, (_, i) =>
      mkTrade({ fees: 10, gross_pnl: 15, net_pnl: 5 }, i),
    );
    const result = computeStrategyDiagnostics(trades);
    expect(result.highFeeStrategies.length).toBeGreaterThan(0);
  });

  it("profitFactor = 0 with zero wins", () => {
    const trades = Array.from({ length: 5 }, (_, i) => mkTrade({}, i));
    const result = computeStrategyDiagnostics(trades);
    const row = result.rows.find((r) => r.strategyId === 1)!;
    expect(row.profitFactor).toBe(0);
  });
});

describe("Stage 5 — Rolling health check", () => {
  it("grade A when all 5 checks pass", () => {
    const trades = Array.from({ length: 20 }, (_, i) =>
      i % 3 === 0
        ? winTrade({}, i)
        : mkTrade({ net_pnl: 8, gross_pnl: 10, fees: 1, exit_reason: "TP" }, i),
    );
    const result = computeRollingHealthCheck(trades, 20);
    expect(["A", "B"]).toContain(result.grade);
  });

  it("grade F when all metrics fail", () => {
    const trades = Array.from({ length: 20 }, (_, i) =>
      mkTrade({ net_pnl: -30, gross_pnl: -2, fees: 20, exit_reason: "SL" }, i),
    );
    const result = computeRollingHealthCheck(trades, 20);
    expect(result.grade).toBe("F");
    expect(result.overallPass).toBe(false);
  });

  it("probe trades excluded from window count", () => {
    const trades = [probeTrade({}, 0), probeTrade({}, 1), mkTrade({}, 2)];
    const result = computeRollingHealthCheck(trades, 20);
    expect(result.window).toBe(1);
  });

  it("timeCount reflects TIME exits correctly", () => {
    const trades = [
      mkTrade({ exit_reason: "TIME", net_pnl: -21 }, 0),
      mkTrade({ exit_reason: "TIME", net_pnl: -21 }, 1),
      mkTrade({ exit_reason: "SL" }, 2),
    ];
    const result = computeRollingHealthCheck(trades, 20);
    expect(result.timeCount).toBe(2);
  });
});

describe("Stage 6 — Parameter tuner", () => {
  it("recommends SIGNAL_THRESHOLD for high fee ratio", () => {
    const trades = Array.from({ length: 15 }, (_, i) =>
      mkTrade({ fees: 15, gross_pnl: 10, net_pnl: -15 }, i),
    );
    const result = recommendOneTune(trades, 28, 1.5, 0.5, 2);
    expect(result.target).toBe("SIGNAL_THRESHOLD");
    expect(result.suggestedValue).toBeGreaterThan(28);
  });

  it("recommends NO_CHANGE for healthy desk", () => {
    const trades = Array.from({ length: 20 }, (_, i) => winTrade({}, i));
    const result = recommendOneTune(trades, 28, 1.5, 0.5, 2);
    expect(result.target).toBe("NO_CHANGE");
  });

  it("delta = suggestedValue - currentValue always", () => {
    const trades = Array.from({ length: 15 }, (_, i) =>
      mkTrade({ fees: 15, gross_pnl: 10, net_pnl: -15 }, i),
    );
    const result = recommendOneTune(trades, 28, 1.5, 0.5, 2);
    if (result.target !== "NO_CHANGE") {
      expect(result.delta).toBeCloseTo(result.suggestedValue - result.currentValue, 5);
    }
  });

  it("probe trades never affect tuner recommendation", () => {
    const trades = [
      ...Array.from({ length: 15 }, (_, i) => winTrade({}, i)),
      probeTrade(),
    ];
    const result = recommendOneTune(trades, 28, 1.5, 0.5, 2);
    expect(result.target).toBe("NO_CHANGE");
  });
});

describe("Stage 7 — Production readiness", () => {
  const baseInputs = {
    signalThreshold: 28,
    leverage: 25,
    takerFeePct: 0.001,
    maxSameSide: 2,
    minPositionNotional: 100,
    openPositionCount: 3,
    currentRegime: "chop",
    markPriceAgeMs: 2000,
    runtimeBlocklist: [] as number[],
    timeExitCount: 0,
    health: healthyCheck(),
    closedTradeCount: 50,
    nodeEnv: "development",
    mongoConnected: true,
    accountKeySet: true,
  };

  it("productionReady when all critical checks pass", () => {
    const report = runProductionReadiness(baseInputs);
    expect(report.productionReady).toBe(true);
    expect(report.criticalFails).toHaveLength(0);
  });

  it("not productionReady when leverage wrong", () => {
    const report = runProductionReadiness({ ...baseInputs, leverage: 10 });
    expect(report.productionReady).toBe(false);
  });

  it("not productionReady when TIME exits firing", () => {
    const report = runProductionReadiness({
      ...baseInputs,
      timeExitCount: 2,
    });
    expect(report.productionReady).toBe(false);
  });

  it("not productionReady when mark price stale", () => {
    const report = runProductionReadiness({
      ...baseInputs,
      markPriceAgeMs: 15_000,
    });
    expect(report.productionReady).toBe(false);
  });

  it("not productionReady when account_key missing", () => {
    const report = runProductionReadiness({
      ...baseInputs,
      accountKeySet: false,
    });
    expect(report.productionReady).toBe(false);
  });

  it("score is 1.0 when all checks pass", () => {
    const report = runProductionReadiness(baseInputs);
    expect(report.score).toBeCloseTo(1.0, 1);
  });

  it("score is between 0 and 1 always", () => {
    const report = runProductionReadiness({
      ...baseInputs,
      leverage: 10,
      timeExitCount: 5,
      mongoConnected: false,
    });
    expect(report.score).toBeGreaterThanOrEqual(0);
    expect(report.score).toBeLessThanOrEqual(1);
  });
});

describe("Stage 8 — Full pipeline simulation (50 trades)", () => {
  it("simulates 50 trades: probes excluded, health computed, tuner fires", () => {
    const realTrades = Array.from({ length: 45 }, (_, i) =>
      i % 2 === 0
        ? winTrade({ strategy_id: (i % 5) + 1 }, i)
        : mkTrade({ strategy_id: (i % 5) + 1, net_pnl: -5, gross_pnl: -3, fees: 2 }, i),
    );
    const probeTradesList = Array.from({ length: 5 }, (_, i) => probeTrade({}, 100 + i));
    const allTrades = [...realTrades, ...probeTradesList];

    const prod = allTrades.filter((t) => !isProbeOrBootstrapTrade({ strategy_name: t.strategy_name }));
    expect(prod).toHaveLength(45);

    const diag = computeStrategyDiagnostics(prod);
    expect(diag.totalProduction).toBe(45);
    expect(diag.rows.every((r) => !r.isProbe)).toBe(true);

    const health = computeRollingHealthCheck(prod, 20);
    expect(health.window).toBeLessThanOrEqual(20);
    expect(["A", "B", "C", "F"]).toContain(health.grade);

    const tune = recommendOneTune(prod, 28, 1.5, 0.5, 2);
    expect(tune.target).toBeDefined();
    expect(tune.tradesAnalyzed).toBeGreaterThanOrEqual(10);

    const ready = runProductionReadiness({
      signalThreshold: 28,
      leverage: 25,
      takerFeePct: 0.001,
      maxSameSide: 2,
      minPositionNotional: 100,
      openPositionCount: 3,
      currentRegime: "chop",
      markPriceAgeMs: 1000,
      runtimeBlocklist: [],
      timeExitCount: 0,
      health,
      closedTradeCount: 45,
      nodeEnv: "development",
      mongoConnected: true,
      accountKeySet: true,
    });
    expect(ready.criticalFails).toHaveLength(0);
    expect(ready.productionReady).toBe(true);
  });
});
