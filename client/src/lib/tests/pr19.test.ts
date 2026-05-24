import { describe, it, expect } from "vitest";
import {
  appendSoakSnapshot,
  soakTrendSummary,
  type SoakDaySnapshot,
} from "../futuresSoakTracker";
import { computeUnifiedReadiness } from "../futuresUnifiedReadiness";
import { computeReplaySignFlipRate } from "../futuresReplayCompare";
import type { DeskRollingPnLScorecard } from "../futuresDeskPnLTracker";
import type { GoLiveGateReport } from "../futuresGoLiveGates";
import { computeDeskRollingPnLScorecard } from "../futuresDeskPnLTracker";

function mockScorecard(
  hint: DeskRollingPnLScorecard["paperReadyHint"],
  overrides: Partial<DeskRollingPnLScorecard> = {},
): DeskRollingPnLScorecard {
  const base: DeskRollingPnLScorecard = {
    computedAt: Date.now(),
    window48hMs: 48 * 3_600_000,
    closes48h: 20,
    last20: {
      tradeCount: 20,
      expectancy: 2,
      winRate: 0.5,
      profitFactor: 1.5,
      feePctOfAbsGross: 30,
      sumNet: 40,
    },
    last50: {
      tradeCount: 50,
      expectancy: 2,
      winRate: 0.5,
      profitFactor: 1.5,
      feePctOfAbsGross: 30,
      sumNet: 100,
    },
    targets: {
      feePctMax: 50,
      expectancyMin: 0,
      winRateMin: 0.35,
      profitFactorMin: 1.0,
    },
    passesFeeTarget50: true,
    passesExpectancyTarget50: true,
    passesWinRateOrPf50: true,
    paperReadyHint: hint,
    ...overrides,
  };
  return base;
}

function mockGoLive(allBlockersPass: boolean): GoLiveGateReport {
  return {
    gates: [],
    blockers: allBlockersPass
      ? [{ id: "SAMPLE", label: "Sample", pass: true, value: "60", required: "50", severity: "BLOCKER", category: "SAMPLE" }]
      : [{ id: "SAMPLE", label: "Sample", pass: false, value: "10", required: "50", severity: "BLOCKER", category: "SAMPLE" }],
    warnings: [],
    allBlockersPass,
    score: allBlockersPass ? 1 : 0.5,
    totalProduction: 60,
    daysOfData: 10,
    computedAt: Date.now(),
    recommendation: allBlockersPass ? "PAPER_READY" : "NOT_READY",
  };
}

describe("appendSoakSnapshot", () => {
  const now = Date.now();
  const scorecard = computeDeskRollingPnLScorecard(
    Array.from({ length: 25 }, (_, i) => ({
      closedAt: new Date(now - i * 3_600_000).toISOString(),
      netPnl: 5,
      grossPnl: 7,
      fees: 2,
      strategyName: "MTF_Trend",
    })),
    now,
  );

  it("dedupes same UTC day (updates, not duplicates)", () => {
    const day = new Date(now).toISOString().slice(0, 10);
    const first = appendSoakSnapshot([], scorecard, 3);
    expect(first).toHaveLength(1);
    expect(first[0].profitModeSkipCount).toBe(3);

    const second = appendSoakSnapshot(first, scorecard, 9);
    expect(second).toHaveLength(1);
    expect(second[0].dateUtc).toBe(day);
    expect(second[0].profitModeSkipCount).toBe(9);
  });
});

describe("soakTrendSummary", () => {
  it("improving=true when last 3 days avg E better than prior 3", () => {
    const snaps: SoakDaySnapshot[] = [];
    for (let i = 0; i < 7; i++) {
      snaps.push({
        dateUtc: `2026-05-${String(10 + i).padStart(2, "0")}`,
        closes: 10,
        expectancy: i < 4 ? 0.5 : 2.5,
        feePctOfAbsGross: 30,
        winRate: 0.5,
        profitModeSkipCount: 0,
        grade: "GREEN",
      });
    }
    const summary = soakTrendSummary(snaps);
    expect(summary.improving).toBe(true);
  });
});

describe("computeUnifiedReadiness", () => {
  const emptySoak = soakTrendSummary([]);

  it("PAPER_READY when scorecard ON_TRACK and goLive blockers pass", () => {
    const result = computeUnifiedReadiness({
      scorecard: mockScorecard("ON_TRACK"),
      goLive: mockGoLive(true),
      soak: emptySoak,
      replaySignFlipRate: null,
      profitModeEnabled: true,
    });
    expect(result.state).toBe("PAPER_READY");
  });

  it("NOT_READY when profit mode off and scorecard failing edge", () => {
    const result = computeUnifiedReadiness({
      scorecard: mockScorecard("REVIEW", {
        passesFeeTarget50: false,
        passesExpectancyTarget50: false,
        last50: {
          tradeCount: 50,
          expectancy: -2,
          winRate: 0.3,
          profitFactor: 0.5,
          feePctOfAbsGross: 80,
          sumNet: -100,
        },
      }),
      goLive: mockGoLive(false),
      soak: emptySoak,
      replaySignFlipRate: null,
      profitModeEnabled: false,
    });
    expect(result.state).toBe("NOT_READY");
  });
});

describe("computeReplaySignFlipRate", () => {
  it("returns 0 when signs agree on matched trades", () => {
    const live = [
      { strategyId: 1, openedAt: "2026-05-20T10:00:00.000Z", netPnl: 5 },
      { strategyId: 2, openedAt: "2026-05-20T11:00:00.000Z", netPnl: -3 },
    ];
    const replay = [
      { strategyId: 1, openedAt: "2026-05-20T10:02:00.000Z", netPnl: 4 },
      { strategyId: 2, openedAt: "2026-05-20T11:01:00.000Z", netPnl: -1 },
    ];
    expect(computeReplaySignFlipRate(live, replay)).toBe(0);
  });

  it("returns 1 when all matched pairs flip sign", () => {
    const live = [{ strategyId: 1, openedAt: "2026-05-20T10:00:00.000Z", netPnl: 5 }];
    const replay = [{ strategyId: 1, openedAt: "2026-05-20T10:01:00.000Z", netPnl: -2 }];
    expect(computeReplaySignFlipRate(live, replay)).toBe(1);
  });
});
