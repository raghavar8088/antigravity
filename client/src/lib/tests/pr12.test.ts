import { describe, it, expect } from "vitest";
import {
  computeStrategyRotation,
  getSuspendedStrategyIds,
} from "../futuresStrategyRotation";
import { generateHealthReport } from "../futuresHealthReport";

const mkRow = (
  o: Partial<{
    strategyId: number;
    strategyName: string;
    templateFamily: string;
    totalTrades: number;
    wins: number;
    losses: number;
    winRate: number;
    avgNetPnl: number;
    feePctOfAbsGross: number;
    profitFactor: number;
    slCount: number;
    tpCount: number;
    isProbe: boolean;
  }> = {},
) => ({
  strategyId: 1,
  strategyName: "MTF_Trend_Align_Short",
  templateFamily: "mtf",
  totalTrades: 10,
  wins: 5,
  losses: 5,
  winRate: 0.5,
  totalNetPnl: 50,
  avgNetPnl: 5,
  avgWin: 20,
  avgLoss: -10,
  feePctOfAbsGross: 0.3,
  profitFactor: 2.0,
  totalFees: 15,
  avgHoldMinutes: 12,
  exitReasonCounts: { SL: 5, TP: 5 },
  slCount: 5,
  tpCount: 5,
  timeCount: 0,
  trailCount: 0,
  profitLockCount: 0,
  worstTrade: -10,
  bestTrade: 20,
  lastTradeAt: new Date().toISOString(),
  isProbe: false,
  ...o,
});

describe("computeStrategyRotation", () => {
  it("promotes high-scoring strategy", () => {
    const rows = [
      mkRow({
        winRate: 0.75,
        avgNetPnl: 15,
        feePctOfAbsGross: 0.15,
        profitFactor: 3.0,
        slCount: 2,
        tpCount: 8,
      }),
    ];
    const report = computeStrategyRotation(rows);
    expect(report.promoted.length).toBeGreaterThan(0);
    expect(report.scores[0].status).toBe("PROMOTED");
  });

  it("suspends low-scoring strategy", () => {
    const rows = [
      mkRow({
        winRate: 0.1,
        avgNetPnl: -25,
        feePctOfAbsGross: 1.5,
        profitFactor: 0.1,
        slCount: 9,
        tpCount: 1,
      }),
    ];
    const report = computeStrategyRotation(rows);
    expect(report.suspended.length).toBeGreaterThan(0);
    expect(report.scores[0].status).toBe("SUSPENDED");
  });

  it("marks insufficient when trades < 5", () => {
    const rows = [mkRow({ totalTrades: 3 })];
    const report = computeStrategyRotation(rows);
    expect(report.insufficient.length).toBe(1);
    expect(report.scores[0].status).toBe("INSUFFICIENT");
  });

  it("excludes probe rows from rotation", () => {
    const rows = [
      mkRow({ isProbe: true, avgNetPnl: 9999 }),
      mkRow({ strategyId: 2, avgNetPnl: 5 }),
    ];
    const report = computeStrategyRotation(rows);
    expect(report.scores.every((s) => s.strategyId !== 1)).toBe(true);
    expect(report.scores).toHaveLength(1);
  });

  it("ranks strategies by score descending", () => {
    const rows = [
      mkRow({ strategyId: 1, avgNetPnl: 2 }),
      mkRow({ strategyId: 2, avgNetPnl: 10, winRate: 0.7 }),
      mkRow({ strategyId: 3, avgNetPnl: -5 }),
    ];
    const report = computeStrategyRotation(rows);
    const ids = report.scores.map((s) => s.strategyId);
    expect(ids[0]).toBe(2);
  });

  it("score is between 0 and 100", () => {
    const rows = [mkRow()];
    const report = computeStrategyRotation(rows);
    report.scores.forEach((s) => {
      expect(s.score).toBeGreaterThanOrEqual(0);
      expect(s.score).toBeLessThanOrEqual(100);
    });
  });

  it("rank 1 is assigned to top strategy", () => {
    const rows = [
      mkRow({ strategyId: 1, avgNetPnl: 5 }),
      mkRow({ strategyId: 2, avgNetPnl: 15, winRate: 0.8 }),
    ];
    const report = computeStrategyRotation(rows);
    expect(report.scores.find((s) => s.rank === 1)?.strategyId).toBe(2);
  });

  it("topStrategyId is the rank-1 strategy", () => {
    const rows = [
      mkRow({ strategyId: 1, avgNetPnl: 2 }),
      mkRow({ strategyId: 2, avgNetPnl: 12, winRate: 0.7 }),
    ];
    const report = computeStrategyRotation(rows);
    expect(report.topStrategyId).toBe(report.scores[0].strategyId);
  });

  it("reasoning string contains score and status", () => {
    const rows = [mkRow()];
    const report = computeStrategyRotation(rows);
    expect(report.scores[0].reasoning).toContain("Score:");
    expect(report.scores[0].reasoning).toContain("Status:");
  });
});

describe("getSuspendedStrategyIds", () => {
  it("returns set of suspended strategy ids", () => {
    const rows = [
      mkRow({ strategyId: 1, avgNetPnl: 10 }),
      mkRow({
        strategyId: 2,
        avgNetPnl: -30,
        winRate: 0.1,
        feePctOfAbsGross: 2.0,
        profitFactor: 0,
        slCount: 9,
        tpCount: 1,
      }),
    ];
    const report = computeStrategyRotation(rows);
    const suspended = getSuspendedStrategyIds(report);
    expect(suspended.has(2)).toBe(true);
    expect(suspended.has(1)).toBe(false);
  });

  it("returns empty set when nothing suspended", () => {
    const rows = [mkRow({ avgNetPnl: 10, winRate: 0.6 })];
    const report = computeStrategyRotation(rows);
    const suspended = getSuspendedStrategyIds(report);
    expect(suspended.size).toBe(0);
  });
});

describe("generateHealthReport", () => {
  const baseHealth = {
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
  };

  it("generates non-empty report string", () => {
    const report = generateHealthReport({
      health: baseHealth,
      diagnostics: null,
      rotation: null,
      tune: null,
      readiness: null,
      accountKey: "test-key",
      generatedAt: Date.now(),
    });
    expect(report.length).toBeGreaterThan(100);
    expect(report).toContain("HEALTH REPORT");
  });

  it("includes grade in report", () => {
    const report = generateHealthReport({
      health: baseHealth,
      diagnostics: null,
      rotation: null,
      tune: null,
      readiness: null,
      accountKey: "test-key",
      generatedAt: Date.now(),
    });
    expect(report).toContain("Grade:");
    expect(report).toContain("A");
  });

  it("handles all null inputs gracefully", () => {
    expect(() =>
      generateHealthReport({
        health: null,
        diagnostics: null,
        rotation: null,
        tune: null,
        readiness: null,
        accountKey: "test",
        generatedAt: Date.now(),
      }),
    ).not.toThrow();
  });

  it("flags TIME count > 0 with warning", () => {
    const report = generateHealthReport({
      health: { ...baseHealth, timeCount: 3 },
      diagnostics: null,
      rotation: null,
      tune: null,
      readiness: null,
      accountKey: "test-key",
      generatedAt: Date.now(),
    });
    expect(report).toContain("SHOULD BE 0");
  });

  it("shows NO_CHANGE when tuner is null", () => {
    const report = generateHealthReport({
      health: null,
      diagnostics: null,
      rotation: null,
      tune: null,
      readiness: null,
      accountKey: "test",
      generatedAt: Date.now(),
    });
    expect(report).toContain("PARAMETER TUNER");
  });

  it("includes account key in report", () => {
    const report = generateHealthReport({
      health: null,
      diagnostics: null,
      rotation: null,
      tune: null,
      readiness: null,
      accountKey: "my-account-123",
      generatedAt: Date.now(),
    });
    expect(report).toContain("my-account-123");
  });
});
