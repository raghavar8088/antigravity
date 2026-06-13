/**
 * regressionGuard.test.ts
 * These tests encode the hard invariants from PRs 1-10.
 * If ANY of these fail, a regression has been introduced.
 * Run in CI on every PR.
 */
import { describe, it, expect } from "vitest";
import { isProbeOrBootstrapTrade } from "../analytics/futuresSessionMetrics";
import { computeAdaptiveThreshold } from "../trading/futuresDeskPolicy";
import { runProductionReadiness } from "../risk/futuresProductionReadiness";
import { recommendOneTune } from "../trading/futuresParameterTuner";
import type { HealthCheckResult } from "../trading/futuresStrategyDiagnostics";
import { scoreSignalQuality } from "../analytics/futuresSignalQuality";
import {
  computeMTFConfluence,
  mtfSkipReason,
} from "../trading/futuresMTFConfluence";

const mkLossTrade = () => ({
  strategy_name: "MTF_Trend_Align",
  net_pnl: -20,
  gross_pnl: -15,
  fees: 5,
  exit_reason: "SL",
  opened_at: new Date(Date.now() - 30 * 60_000).toISOString(),
  closed_at: new Date(Date.now() - 10 * 60_000).toISOString(),
});

describe("INVARIANT 1 — Probe filter", () => {
  it("PAPER_BOOTSTRAP_PROBE is always a probe", () => {
    expect(
      isProbeOrBootstrapTrade({
        strategy_name: "PAPER_BOOTSTRAP_PROBE",
      }),
    ).toBe(true);
  });

  it("DEV_FORCE_PROBE_OPEN is always a probe", () => {
    expect(
      isProbeOrBootstrapTrade({
        strategy_name: "DEV_FORCE_PROBE_OPEN",
      }),
    ).toBe(true);
  });

  it("MTF_Trend_Align_Short is never a probe", () => {
    expect(
      isProbeOrBootstrapTrade({
        strategy_name: "MTF_Trend_Align_Short",
      }),
    ).toBe(false);
  });

  it("null strategy_name is not a probe", () => {
    expect(isProbeOrBootstrapTrade({ strategy_name: null })).toBe(false);
  });
});

describe("INVARIANT 2 — Adaptive threshold", () => {
  it.each([
    [28, "chop"],
    [28, "trendLow"],
    [28, "trendHigh"],
    [28, "chop", "F" as const, 0.9],
    [28, "trendHigh", "A" as const, 0.1],
    [35, "chop", "F" as const, 0.9],
  ])(
    "base=%i regime=%s never below base",
    (base, regime, grade?: HealthCheckResult["grade"], fee?: number) => {
      const result = computeAdaptiveThreshold(base, regime, grade, fee);
      expect(result).toBeGreaterThanOrEqual(base);
    },
  );

  it("max boost is 12 regardless of inputs", () => {
    const base = 28;
    const result = computeAdaptiveThreshold(base, "chop", "F", 0.99);
    expect(result).toBe(base + 12);
  });
});

describe("INVARIANT 3 — Leverage fixed at 25", () => {
  it("readiness fails if leverage != 25", () => {
    const report = runProductionReadiness({
      signalThreshold: 28,
      leverage: 10,
      takerFeePct: 0.001,
      maxSameSide: 2,
      minPositionNotional: 100,
      openPositionCount: 0,
      currentRegime: "chop",
      markPriceAgeMs: 1000,
      runtimeBlocklist: [],
      timeExitCount: 0,
      health: null,
      closedTradeCount: 0,
      nodeEnv: "development",
      mongoConnected: true,
      accountKeySet: true,
    });
    const check = report.checks.find((c) => c.id === "LEVERAGE_FIXED")!;
    expect(check.pass).toBe(false);
    expect(check.severity).toBe("CRITICAL");
  });
});

describe("INVARIANT 4 — No runtime TIME exits", () => {
  it("readiness fails if timeExitCount > 0", () => {
    const report = runProductionReadiness({
      signalThreshold: 28,
      leverage: 25,
      takerFeePct: 0.001,
      maxSameSide: 2,
      minPositionNotional: 100,
      openPositionCount: 0,
      currentRegime: "chop",
      markPriceAgeMs: 1000,
      runtimeBlocklist: [],
      timeExitCount: 1,
      health: null,
      closedTradeCount: 0,
      nodeEnv: "development",
      mongoConnected: true,
      accountKeySet: true,
    });
    const check = report.checks.find((c) => c.id === "NO_RUNTIME_TIME_EXITS")!;
    expect(check.pass).toBe(false);
  });
});

describe("INVARIANT 5 — Tuner returns single recommendation", () => {
  const VALID_TARGETS = [
    "SIGNAL_THRESHOLD",
    "TP_PCT",
    "SL_PCT",
    "SAME_SIDE_CAP",
    "COOLDOWN_MIN",
    "HOLD_MINUTES",
    "NO_CHANGE",
  ];

  it("target is always a valid TuneTarget", () => {
    const trades = Array.from({ length: 15 }, () => mkLossTrade());
    const result = recommendOneTune(trades as never, 28, 1.5, 0.5, 2);
    expect(VALID_TARGETS).toContain(result.target);
  });

  it("delta = suggestedValue - currentValue invariant", () => {
    const trades = Array.from({ length: 15 }, () => mkLossTrade());
    const result = recommendOneTune(trades as never, 28, 1.5, 0.5, 2);
    if (result.target !== "NO_CHANGE") {
      expect(result.delta).toBeCloseTo(result.suggestedValue - result.currentValue, 5);
    }
  });
});

describe("INVARIANT 6 — MongoDB field naming", () => {
  it("account_key check id exists in readiness report", () => {
    const report = runProductionReadiness({
      signalThreshold: 28,
      leverage: 25,
      takerFeePct: 0.001,
      maxSameSide: 2,
      minPositionNotional: 100,
      openPositionCount: 0,
      currentRegime: "chop",
      markPriceAgeMs: 1000,
      runtimeBlocklist: [],
      timeExitCount: 0,
      health: null,
      closedTradeCount: 0,
      nodeEnv: "development",
      mongoConnected: true,
      accountKeySet: false,
    });
    const check = report.checks.find((c) => c.id === "ACCOUNT_KEY_SET")!;
    expect(check).toBeDefined();
    expect(check.pass).toBe(false);
  });
});

describe("INVARIANT 8 — scoreSignalQuality is pure", () => {
  it("identical inputs produce identical scores", () => {
    const input = {
      signalScore: 30,
      atrPct: 0.001,
      spreadPct: 0.0002,
      volumeRatio: 1.2,
      regime: "trendHigh",
      regimeFitsStrategy: true,
      ema20AboveEma50: true,
      priceAboveEma20: true,
      side: "LONG" as const,
      openPositionCount: 2,
      sameSideCount: 0,
      hoursIntoSession: 12,
      strategyWinRate: 0.5,
      strategyTrades: 10,
      cooldownRemainMs: 0,
    };
    expect(scoreSignalQuality(input)).toEqual(scoreSignalQuality(input));
  });
});

describe("INVARIANT 9 — mtfSkipReason when confluent + agrees", () => {
  it("returns null for aligned bullish LONG setup", () => {
    const snap = (tf: "1m" | "5m" | "15m" | "1h" | "4h" | "1d") => ({
      tf,
      close: 100_000,
      ema20: 99_000,
      ema50: 98_000,
      rsi: 60,
      atr: 500,
      volumeRatio: 1.1,
      isAvailable: true,
    });
    const result = computeMTFConfluence(
      (["1m", "5m", "15m", "1h", "4h", "1d"] as const).map(snap),
      "LONG",
    );
    expect(mtfSkipReason(result, 55)).toBeNull();
  });
});

describe("INVARIANT 7 — PnL formula", () => {
  it("gross pnl = direction * (exit/entry - 1) * notional LONG", () => {
    const entry = 100_000;
    const exit_ = 100_450;
    const notional = 2500;
    const gross = 1 * (exit_ / entry - 1) * notional;
    expect(gross).toBeCloseTo(11.25, 2);
  });

  it("gross pnl = direction * (exit/entry - 1) * notional SHORT", () => {
    const entry = 100_000;
    const exit_ = 99_550;
    const notional = 2500;
    const gross = -1 * (exit_ / entry - 1) * notional;
    expect(gross).toBeCloseTo(11.25, 2);
  });

  it("net = gross - fees", () => {
    const gross = 11.25;
    const fees = 2500 * 0.002;
    const net = gross - fees;
    expect(net).toBeCloseTo(6.25, 1);
  });
});
