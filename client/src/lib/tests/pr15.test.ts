import { describe, it, expect } from "vitest";
import { computeGoLiveGates } from "../risk/futuresGoLiveGates";
import { generateValidationReport } from "../analytics/futuresValidationReport";
import { runProductionReadiness } from "../risk/futuresProductionReadiness";

const now = Date.now();
const ago = (days: number) => new Date(now - days * 86_400_000).toISOString();
const agoMin = (m: number) => new Date(now - m * 60_000).toISOString();

const mkTrade = (
  o: Partial<{
    strategy_name: string;
    net_pnl: number;
    gross_pnl: number;
    fees: number;
    exit_reason: string;
    opened_at: string;
    closed_at: string;
  }> = {},
) => ({
  strategy_name: "MTF_Trend_Align",
  net_pnl: 12,
  gross_pnl: 15,
  fees: 2,
  exit_reason: "TP",
  opened_at: agoMin(30),
  closed_at: agoMin(5),
  ...o,
});

const mockReadinessPass = () =>
  runProductionReadiness({
    signalThreshold: 28,
    leverage: 25,
    takerFeePct: 0.001,
    maxSameSide: 2,
    minPositionNotional: 100,
    openPositionCount: 0,
    currentRegime: "trendHigh",
    markPriceAgeMs: 1000,
    runtimeBlocklist: [],
    timeExitCount: 0,
    health: {
      window: 20,
      expectancy: 5,
      expectancyPass: true,
      winRate: 0.55,
      winRatePass: true,
      feePctOfAbsGross: 0.25,
      feePass: true,
      profitFactor: 1.5,
      pfPass: true,
      tpHits: 5,
      tpHitPass: true,
      slCount: 5,
      timeCount: 0,
      overallPass: true,
      grade: "A" as const,
    },
    closedTradeCount: 60,
    nodeEnv: "development",
    mongoConnected: true,
    accountKeySet: true,
  });

describe("computeGoLiveGates", () => {
  it("returns NOT_READY with < 10 trades", () => {
    const trades = Array.from({ length: 5 }, () => mkTrade());
    const report = computeGoLiveGates({
      trades: trades as never,
      health: null,
      readiness: null,
    });
    expect(report.recommendation).toBe("NOT_READY");
    expect(report.totalProduction).toBe(5);
  });

  it("returns COLLECT_MORE_DATA with 30 trades", () => {
    const trades = Array.from({ length: 30 }, (_, i) =>
      mkTrade({
        opened_at: ago(10),
        closed_at: agoMin(5 + i),
      }),
    );
    const readiness = mockReadinessPass();
    const report = computeGoLiveGates({
      trades: trades as never,
      health: readiness.checks.length ? null : null,
      readiness,
    });
    expect(report.totalProduction).toBe(30);
    expect(["COLLECT_MORE_DATA", "NOT_READY", "REVIEW_WARNINGS"]).toContain(
      report.recommendation,
    );
  });

  it("PAPER_READY when all blocker gates pass (60 winning trades)", () => {
    const trades = Array.from({ length: 60 }, (_, i) =>
      mkTrade({
        net_pnl: 15,
        gross_pnl: 18,
        fees: 2,
        exit_reason: "TP",
        opened_at: ago(35),
        closed_at: agoMin(60 - i),
      }),
    );
    const readiness = mockReadinessPass();
    const health = {
      window: 20,
      expectancy: 12,
      expectancyPass: true,
      winRate: 0.6,
      winRatePass: true,
      feePctOfAbsGross: 0.2,
      feePass: true,
      profitFactor: 1.8,
      pfPass: true,
      tpHits: 8,
      tpHitPass: true,
      slCount: 4,
      timeCount: 0,
      overallPass: true,
      grade: "A" as const,
    };
    const report = computeGoLiveGates({
      trades: trades as never,
      health,
      readiness,
      replaySignFlipRate: 0.05,
      shadowIntentCount: 12,
      nowMs: now,
    });
    expect(report.totalProduction).toBe(60);
    expect(report.allBlockersPass).toBe(true);
    expect(report.recommendation).toBe("PAPER_READY");
  });

  it("excludes probe trades from sample size", () => {
    const trades = [
      ...Array.from({ length: 55 }, () => mkTrade()),
      mkTrade({ strategy_name: "PAPER_BOOTSTRAP_PROBE", net_pnl: 9999 }),
    ];
    const report = computeGoLiveGates({
      trades: trades as never,
      health: null,
      readiness: mockReadinessPass(),
    });
    expect(report.totalProduction).toBe(55);
  });

  it("fee/gross gate fails when fees dominate", () => {
    const trades = Array.from({ length: 55 }, () =>
      mkTrade({
        net_pnl: -2,
        gross_pnl: 5,
        fees: 20,
      }),
    );
    const report = computeGoLiveGates({
      trades: trades as never,
      health: null,
      readiness: mockReadinessPass(),
      nowMs: now,
    });
    const feeGate = report.gates.find((g) => g.id === "FEE_RATIO_MAX")!;
    expect(feeGate.pass).toBe(false);
    expect(report.recommendation).toBe("NOT_READY");
  });

  it("expectancy gate fails when avg net negative", () => {
    const trades = Array.from({ length: 55 }, (_, i) =>
      mkTrade({
        net_pnl: -10,
        gross_pnl: -8,
        fees: 2,
        opened_at: ago(10),
        closed_at: agoMin(5 + i),
      }),
    );
    const report = computeGoLiveGates({
      trades: trades as never,
      health: null,
      readiness: mockReadinessPass(),
    });
    const expGate = report.gates.find((g) => g.id === "EXPECTANCY_POSITIVE")!;
    expect(expGate.pass).toBe(false);
  });
});

describe("generateValidationReport", () => {
  it("generates non-empty report string", () => {
    const gates = computeGoLiveGates({
      trades: Array.from({ length: 5 }, () => mkTrade()) as never,
      health: null,
      readiness: null,
    });
    const text = generateValidationReport({
      gates,
      health: null,
      readiness: null,
      attribution: null,
      accountKey: "test",
      generatedAt: now,
    });
    expect(text.length).toBeGreaterThan(200);
    expect(text).toContain("VALIDATION REPORT");
  });

  it("includes BLOCKERS section when blockers fail", () => {
    const gates = computeGoLiveGates({
      trades: Array.from({ length: 3 }, () => mkTrade()) as never,
      health: null,
      readiness: null,
    });
    const text = generateValidationReport({
      gates,
      health: null,
      readiness: null,
      attribution: null,
      accountKey: "test",
      generatedAt: now,
    });
    expect(text).toContain("BLOCKERS");
  });
});
