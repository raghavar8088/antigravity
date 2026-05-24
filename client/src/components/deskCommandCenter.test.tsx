import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderToString } from "react-dom/server";
import { DeskCommandCenter } from "./DeskCommandCenter";
import type { BTCFuturesEngineStats } from "@/hooks/useBTCFuturesScalperEngine";
import { profitModeFromEnv } from "@/lib/futuresProfitMode";
import { scorecardShowsEnvFixCopy } from "./deskCommandCenterHelpers";
import { DEFAULT_COMMAND_CENTER_TAB } from "./deskCommandCenterTabs";

vi.mock("@/hooks/useDeskPerformanceMonitor", () => ({
  useDeskPerformanceMonitor: () => ({
    diagnostics: null,
    healthCheck: null,
    recommendations: [],
    tuneRecommendation: null,
    timeExitCount: 0,
    gradeHistory: [],
    rotationReport: null,
    goLiveGates: null,
    replaySignFlipRate: null,
    tradesAll: [],
    lastFetchAt: null,
    fetchError: null,
    isFetching: false,
  }),
}));

function mockStats(overrides: Partial<BTCFuturesEngineStats> = {}): BTCFuturesEngineStats {
  return {
    unifiedReadiness: "PAPER_READY",
    unifiedReadinessBlockers: ["Replay compare not run"],
    unifiedReadinessNextStep: "Run replay compare on recent UTC days.",
    soakHistory: [],
    soakSummary: { daysTracked: 0, greenDays: 2, avgExpectancy7d: 1.2, improving: false },
    replaySignFlipRate: null,
    deskPnLScorecard: null,
    scorecardAction: null,
    deskLastRegimeTag: "trendHigh",
    effectiveSignalThreshold: 28,
    profitModeSkipCount: 3,
    gateEvaluationCount: 100,
    skipReasonSummary: [{ reason: "SIGNAL_QUALITY_FAIL", count: 5 }],
    ...overrides,
  } as BTCFuturesEngineStats;
}

describe("scorecardShowsEnvFixCopy", () => {
  it("true when ACT with suggestedEnv", () => {
    expect(
      scorecardShowsEnvFixCopy({
        severity: "ACT",
        action: "RAISE_THRESHOLD",
        rationale: "fee bleed",
        suggestedEnv: { NEXT_PUBLIC_DESK_SIGNAL_THRESHOLD: "32" },
      } as never),
    ).toBe(true);
  });

  it("false when OK", () => {
    expect(
      scorecardShowsEnvFixCopy({
        severity: "OK",
        action: "NO_CHANGE",
        rationale: "on track",
      } as never),
    ).toBe(false);
  });
});

describe("DeskCommandCenter — SSR smoke", () => {
  beforeEach(() => {
    process.env.NEXT_PUBLIC_DESK_PROFIT_MODE = "1";
  });

  it("renders hero readiness label from props", () => {
    const html = renderToString(
      <DeskCommandCenter
        stats={mockStats()}
        cloudAccountKey="test-key"
        profitModeCfg={profitModeFromEnv()}
        deskShadowIntentsEnabled={false}
        deskTestnetOpsEnabled={false}
      />,
    );
    expect(html).toContain("PAPER READY");
  });

  it("shows Today tab by default", () => {
    expect(DEFAULT_COMMAND_CENTER_TAB).toBe("today");
    const html = renderToString(
      <DeskCommandCenter
        stats={mockStats()}
        cloudAccountKey="test-key"
        profitModeCfg={profitModeFromEnv()}
        deskShadowIntentsEnabled={false}
        deskTestnetOpsEnabled={false}
      />,
    );
    expect(html).toContain('role="tablist"');
    expect(html).toContain("Today");
  });

  it("shows Copy env fix when action has suggestedEnv", () => {
    const html = renderToString(
      <DeskCommandCenter
        stats={mockStats({
          scorecardAction: {
            severity: "WARN",
            action: "TIGHTEN_EXITS",
            rationale: "fee high",
            suggestedEnv: { NEXT_PUBLIC_DESK_PROFIT_MODE: "1" },
          } as never,
          deskPnLScorecard: {
            paperReadyHint: "REVIEW",
            closes48h: 12,
            last20: {
              tradeCount: 20,
              expectancy: 1,
              winRate: 0.4,
              profitFactor: 1,
              feePctOfAbsGross: 40,
              sumNet: 20,
            },
            last50: {
              tradeCount: 50,
              expectancy: 1,
              winRate: 0.4,
              profitFactor: 1,
              feePctOfAbsGross: 45,
              sumNet: 50,
            },
            targets: {
              feePctMax: 50,
              expectancyMin: 0,
              winRateMin: 0.35,
              profitFactorMin: 1,
            },
            passesFeeTarget50: false,
            passesExpectancyTarget50: true,
            passesWinRateOrPf50: true,
            computedAt: Date.now(),
            window48hMs: 48 * 3600000,
          } as never,
        })}
        cloudAccountKey="test-key"
        profitModeCfg={profitModeFromEnv()}
        deskShadowIntentsEnabled={false}
        deskTestnetOpsEnabled={false}
      />,
    );
    expect(html).toContain("Copy env fix");
  });

  it("does not duplicate Go-live panel in compact layout", () => {
    const html = renderToString(
      <DeskCommandCenter
        stats={mockStats({
          goLiveGates: {
            recommendation: "PAPER_READY",
            allBlockersPass: true,
            blockers: [],
            warnings: [],
            gates: [],
            score: 1,
            totalProduction: 60,
            daysOfData: 10,
            computedAt: Date.now(),
          } as never,
        })}
        cloudAccountKey="test-key"
        profitModeCfg={profitModeFromEnv()}
        deskShadowIntentsEnabled={false}
        deskTestnetOpsEnabled={false}
        advancedTabGated
      />,
    );
    const matches = html.match(/Go-live validation/g) ?? [];
    expect(matches.length).toBeLessThanOrEqual(1);
  });
});
