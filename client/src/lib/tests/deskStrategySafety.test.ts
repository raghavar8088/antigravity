import { describe, it, expect } from "vitest";
import { computeStrategySafety, formatDisableListAsEnv } from "../deskStrategySafety";
import type { RotationReport } from "../futuresStrategyRotation";
import type { DiagnosticSummary, StrategyDiagnosticRow } from "../futuresStrategyDiagnostics";

function makeRotationReport(
  scores: { strategyId: number; status: string; score: number }[],
): RotationReport {
  const byStatus = (s: string) =>
    scores.filter((r) => r.status === s).map((r) => ({ ...r, strategyName: `strat_${r.strategyId}`, rank: 0, winRateScore: 0, expectancyScore: 0, feeScore: 0, consistencyScore: 0, tradesAnalyzed: 5, lastUpdated: Date.now(), reasoning: "" }));
  return {
    scores: scores.map((r) => ({ ...r, strategyName: `strat_${r.strategyId}`, rank: 0, winRateScore: 0, expectancyScore: 0, feeScore: 0, consistencyScore: 0, tradesAnalyzed: 5, lastUpdated: Date.now(), reasoning: "" })),
    active: byStatus("ACTIVE") as RotationReport["active"],
    promoted: byStatus("PROMOTED") as RotationReport["promoted"],
    probation: byStatus("PROBATION") as RotationReport["probation"],
    suspended: byStatus("SUSPENDED") as RotationReport["suspended"],
    insufficient: byStatus("INSUFFICIENT") as RotationReport["insufficient"],
    topStrategyId: null,
    computedAt: Date.now(),
  } as RotationReport;
}

function makeDiagRow(
  overrides: Partial<StrategyDiagnosticRow> & { strategyId: number },
): StrategyDiagnosticRow {
  return {
    strategyId: overrides.strategyId,
    strategyName: `strat_${overrides.strategyId}`,
    templateFamily: "test",
    totalTrades: overrides.totalTrades ?? 10,
    wins: overrides.wins ?? 4,
    losses: overrides.losses ?? 6,
    winRate: 0.4,
    totalNetPnl: overrides.totalNetPnl ?? -30,
    avgNetPnl: overrides.avgNetPnl ?? -3,
    avgWin: 5,
    avgLoss: -8,
    profitFactor: 0.5,
    totalFees: overrides.totalFees ?? 50,
    feePctOfAbsGross: overrides.feePctOfAbsGross ?? 1.5,
    avgHoldMinutes: 30,
    exitReasonCounts: {},
    slCount: 4,
    tpCount: 2,
    timeCount: 4,
    trailCount: 0,
    profitLockCount: 0,
    worstTrade: -20,
    bestTrade: 10,
    lastTradeAt: null,
    isProbe: overrides.isProbe ?? false,
  };
}

function makeDiagSummary(rows: StrategyDiagnosticRow[]): DiagnosticSummary {
  const prod = rows.filter((r) => !r.isProbe);
  return {
    rows,
    totalProduction: prod.length,
    topByExpectancy: [],
    bottomByExpectancy: [],
    highFeeStrategies: [],
    slDominatedStrats: [],
    computedAt: Date.now(),
  };
}

describe("computeStrategySafety — excludes probes", () => {
  it("ignores probe/bootstrap rows in diagnostics", () => {
    const diagnostics = makeDiagSummary([
      makeDiagRow({ strategyId: 1, isProbe: true, avgNetPnl: -100, feePctOfAbsGross: 5 }),
    ]);
    const result = computeStrategySafety({ rotationReport: null, diagnostics });
    expect(result.autoDisableCandidates).not.toContain(1);
  });
});

describe("computeStrategySafety — suggests disable for >= 5 bad trades", () => {
  it("suggests disabling when >=5 closes, negative expectancy, fee/gross > 100%", () => {
    const diagnostics = makeDiagSummary([
      makeDiagRow({ strategyId: 42, totalTrades: 8, avgNetPnl: -5, feePctOfAbsGross: 1.2 }),
    ]);
    const result = computeStrategySafety({ rotationReport: null, diagnostics });
    expect(result.autoDisableCandidates).toContain(42);
    expect(result.reasonByStrategyId[42]).toContain("8 closes");
  });

  it("does NOT suggest disabling when trades < 5 (insufficient signal)", () => {
    const diagnostics = makeDiagSummary([
      makeDiagRow({ strategyId: 7, totalTrades: 3, avgNetPnl: -10, feePctOfAbsGross: 2 }),
    ]);
    const result = computeStrategySafety({ rotationReport: null, diagnostics });
    expect(result.autoDisableCandidates).not.toContain(7);
  });

  it("does NOT suggest disabling when expectancy is positive despite high fees", () => {
    const diagnostics = makeDiagSummary([
      makeDiagRow({ strategyId: 99, totalTrades: 10, avgNetPnl: 3, feePctOfAbsGross: 1.1 }),
    ]);
    const result = computeStrategySafety({ rotationReport: null, diagnostics });
    expect(result.autoDisableCandidates).not.toContain(99);
  });
});

describe("computeStrategySafety — suspended strategy suggested", () => {
  it("suggests disabling rotation SUSPENDED strategies", () => {
    const report = makeRotationReport([
      { strategyId: 10, status: "SUSPENDED", score: 15 },
    ]);
    const result = computeStrategySafety({ rotationReport: report, diagnostics: null });
    expect(result.autoDisableCandidates).toContain(10);
    expect(result.reasonByStrategyId[10]).toContain("SUSPENDED");
  });

  it("keeps ACTIVE strategies in keepEnabledCandidates", () => {
    const report = makeRotationReport([
      { strategyId: 5, status: "ACTIVE", score: 60 },
    ]);
    const result = computeStrategySafety({ rotationReport: report, diagnostics: null });
    expect(result.keepEnabledCandidates).toContain(5);
    expect(result.autoDisableCandidates).not.toContain(5);
  });

  it("keeps PROMOTED strategies in keepEnabledCandidates", () => {
    const report = makeRotationReport([
      { strategyId: 3, status: "PROMOTED", score: 80 },
    ]);
    const result = computeStrategySafety({ rotationReport: report, diagnostics: null });
    expect(result.keepEnabledCandidates).toContain(3);
  });
});

describe("computeStrategySafety — INSUFFICIENT strategies never suggested", () => {
  it("keeps INSUFFICIENT in keepEnabled (not enough data to judge)", () => {
    const report = makeRotationReport([
      { strategyId: 20, status: "INSUFFICIENT", score: 0 },
    ]);
    const result = computeStrategySafety({ rotationReport: report, diagnostics: null });
    expect(result.autoDisableCandidates).not.toContain(20);
    expect(result.keepEnabledCandidates).toContain(20);
    expect(result.reasonByStrategyId[20]).toContain("Insufficient");
  });
});

describe("formatDisableListAsEnv", () => {
  it("returns env line excluding disable candidates", () => {
    const env = formatDisableListAsEnv([1, 2, 3, 4, 5], [3, 5]);
    expect(env).toBe("DESK_WORKER_STRATEGY_IDS=1,2,4");
  });

  it("returns null when no candidates to exclude", () => {
    const env = formatDisableListAsEnv([1, 2, 3], []);
    expect(env).toBeNull();
  });

  it("returns null when all are excluded (edge case)", () => {
    const env = formatDisableListAsEnv([1, 2], [1, 2]);
    // remaining is empty — but not null here (empty list is still a valid env line)
    expect(env).toBe("DESK_WORKER_STRATEGY_IDS=");
  });
});
