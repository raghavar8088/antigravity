import { describe, expect, it } from "vitest";
import { recommendOneTune } from "../futuresParameterTuner";
import { runProductionReadiness } from "../futuresProductionReadiness";
import type { PaperTradeDbRow } from "../paperTradesTypes";
import type { HealthCheckResult } from "../futuresStrategyDiagnostics";

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
  strategy_id: 91,
  strategy_name: "MTF_Trend_Align_Long",
  side: "LONG",
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
  ...o,
});

const healthy = (): HealthCheckResult => ({
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
  grade: "A",
});

describe("recommendOneTune", () => {
  it("returns NO_CHANGE when fewer than 10 trades", () => {
    const trades = Array.from({ length: 5 }, (_, i) => mkTrade({}, i));
    const result = recommendOneTune(trades, 28, 1.5, 0.5, 2);
    expect(result.target).toBe("NO_CHANGE");
    expect(result.confidence).toBe("LOW");
  });

  it("recommends SIGNAL_THRESHOLD when fee/gross > 60%", () => {
    const trades = Array.from({ length: 15 }, (_, i) =>
      mkTrade({ fees: 10, gross_pnl: 10, net_pnl: -10 }, i),
    );
    const result = recommendOneTune(trades, 28, 1.5, 0.5, 2);
    expect(result.target).toBe("SIGNAL_THRESHOLD");
    expect(result.suggestedValue).toBeGreaterThan(28);
  });

  it("recommends SIGNAL_THRESHOLD when SL>70% and hold<5m", () => {
    const trades = Array.from({ length: 15 }, (_, i) =>
      mkTrade(
        {
          exit_reason: "SL",
          net_pnl: -20,
          gross_pnl: -15,
          fees: 2,
          opened_at: ago(3),
          closed_at: ago(1),
        },
        i,
      ),
    );
    const result = recommendOneTune(trades, 28, 1.5, 0.5, 2);
    expect(result.target).toBe("SIGNAL_THRESHOLD");
  });

  it("recommends SL_PCT when SL>70% and hold>=5m", () => {
    const trades = Array.from({ length: 15 }, (_, i) =>
      mkTrade(
        {
          exit_reason: "SL",
          net_pnl: -20,
          gross_pnl: -15,
          fees: 2,
          opened_at: ago(20),
          closed_at: ago(5),
        },
        i,
      ),
    );
    const result = recommendOneTune(trades, 28, 1.5, 0.5, 2);
    expect(result.target).toBe("SL_PCT");
    expect(result.suggestedValue).toBeGreaterThan(0.5);
  });

  it("recommends TP_PCT when TP rate < 5% over 20 trades", () => {
    const trades = Array.from({ length: 25 }, (_, i) =>
      mkTrade(
        {
          exit_reason: "TIME",
          net_pnl: 1,
          gross_pnl: 2,
          fees: 1,
          opened_at: ago(25),
          closed_at: ago(20),
        },
        i,
      ),
    );
    const result = recommendOneTune(trades, 28, 1.5, 0.5, 2);
    expect(result.target).toBe("TP_PCT");
  });

  it("excludes probe trades from analysis", () => {
    const trades = [
      ...Array.from({ length: 12 }, (_, i) =>
        mkTrade({ net_pnl: 20, gross_pnl: 25, fees: 2, exit_reason: "TP" }, i),
      ),
      mkTrade({ strategy_id: 99, strategy_name: "PAPER_BOOTSTRAP_PROBE", net_pnl: -99999 }, 99),
    ];
    const result = recommendOneTune(trades, 28, 1.5, 0.5, 2);
    expect(result.target).toBe("NO_CHANGE");
  });

  it("beforeSim and afterSim are always present", () => {
    const trades = Array.from({ length: 15 }, (_, i) => mkTrade({}, i));
    const result = recommendOneTune(trades, 28, 1.5, 0.5, 2);
    expect(result.beforeSim).toBeDefined();
    expect(result.afterSim).toBeDefined();
    expect(result.beforeSim.label).toBeTruthy();
    expect(result.afterSim.label).toBeTruthy();
  });

  it("delta equals suggestedValue minus currentValue", () => {
    const trades = Array.from({ length: 15 }, (_, i) =>
      mkTrade({ fees: 10, gross_pnl: 10, net_pnl: -10 }, i),
    );
    const result = recommendOneTune(trades, 28, 1.5, 0.5, 2);
    if (result.target !== "NO_CHANGE") {
      expect(result.delta).toBeCloseTo(result.suggestedValue - result.currentValue, 5);
    }
  });
});

describe("runProductionReadiness", () => {
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
    health: healthy(),
    closedTradeCount: 50,
    nodeEnv: "development",
    mongoConnected: true,
    accountKeySet: true,
  };

  it("passes all checks with healthy inputs", () => {
    const report = runProductionReadiness(baseInputs);
    expect(report.criticalFails).toHaveLength(0);
    expect(report.productionReady).toBe(true);
  });

  it("fails MARK_PRICE_FRESH when mark age > 10s", () => {
    const report = runProductionReadiness({
      ...baseInputs,
      markPriceAgeMs: 15_000,
    });
    const check = report.checks.find((c) => c.id === "MARK_PRICE_FRESH")!;
    expect(check.pass).toBe(false);
    expect(check.severity).toBe("CRITICAL");
  });

  it("fails LEVERAGE_FIXED when leverage != 25", () => {
    const report = runProductionReadiness({
      ...baseInputs,
      leverage: 10,
    });
    const check = report.checks.find((c) => c.id === "LEVERAGE_FIXED")!;
    expect(check.pass).toBe(false);
  });

  it("fails NO_RUNTIME_TIME_EXITS when timeExitCount > 0", () => {
    const report = runProductionReadiness({
      ...baseInputs,
      timeExitCount: 3,
    });
    const check = report.checks.find((c) => c.id === "NO_RUNTIME_TIME_EXITS")!;
    expect(check.pass).toBe(false);
    expect(check.severity).toBe("CRITICAL");
  });

  it("fails ACCOUNT_KEY_SET when accountKeySet is false", () => {
    const report = runProductionReadiness({
      ...baseInputs,
      accountKeySet: false,
    });
    const check = report.checks.find((c) => c.id === "ACCOUNT_KEY_SET")!;
    expect(check.pass).toBe(false);
  });

  it("productionReady=false when any CRITICAL fails", () => {
    const report = runProductionReadiness({
      ...baseInputs,
      mongoConnected: false,
    });
    expect(report.productionReady).toBe(false);
  });

  it("score is between 0 and 1", () => {
    const report = runProductionReadiness(baseInputs);
    expect(report.score).toBeGreaterThan(0);
    expect(report.score).toBeLessThanOrEqual(1);
  });

  it("fails THRESHOLD_MINIMUM when threshold < 28", () => {
    const report = runProductionReadiness({
      ...baseInputs,
      signalThreshold: 20,
    });
    const check = report.checks.find((c) => c.id === "THRESHOLD_MINIMUM")!;
    expect(check.pass).toBe(false);
  });

  it("fails HEALTH_NOT_F when grade is F", () => {
    const report = runProductionReadiness({
      ...baseInputs,
      health: { ...healthy(), grade: "F", overallPass: false },
    });
    const check = report.checks.find((c) => c.id === "HEALTH_NOT_F")!;
    expect(check.pass).toBe(false);
    expect(check.severity).toBe("WARN");
  });
});
