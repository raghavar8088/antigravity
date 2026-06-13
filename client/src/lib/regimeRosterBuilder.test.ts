/**
 * Tests for regimeRosterBuilder — pure function, no I/O.
 */
import { describe, it, expect } from "vitest";
import { buildRegimeRosters } from "./analytics/regimeRosterBuilder";
import type { ResearchEdgeScore } from "./ai/researchEdgeScore";

// ── Helpers ───────────────────────────────────────────────────────────────────

function makeScore(
  strategyId: number,
  status: ResearchEdgeScore["status"],
  opts: {
    expectancy?: number;
    regimes?: Record<string, { trades: number; expectancy: number; winRate: number }>;
  } = {},
): ResearchEdgeScore {
  return {
    strategyId,
    strategyName: `strat-${strategyId}`,
    trades: 20,
    netPnl: opts.expectancy != null ? opts.expectancy * 20 : 10,
    expectancy: opts.expectancy ?? 0.5,
    winRate: 0.6,
    profitFactor: 1.5,
    feePctOfAbsGross: 20,
    maxDrawdownEstimate: 5,
    tpHits: 12,
    slHits: 8,
    regimeBreakdown: opts.regimes ?? {},
    confidence: 0.6,
    status,
    reason: "test",
  };
}

// ── DISABLE exclusion ─────────────────────────────────────────────────────────

describe("buildRegimeRosters — DISABLE exclusion", () => {
  it("DISABLE strategies never appear in any roster", () => {
    const scores = [
      makeScore(91, "PROMOTE"),
      makeScore(92, "DISABLE"),
    ];
    const rosters = buildRegimeRosters(scores);
    expect(rosters.chopRoster).not.toContain(92);
    expect(rosters.trendRoster).not.toContain(92);
    expect(rosters.highVolRoster).not.toContain(92);
  });

  it("DISABLE strategies appear in disabledIds", () => {
    const scores = [makeScore(92, "DISABLE"), makeScore(93, "DISABLE")];
    const rosters = buildRegimeRosters(scores);
    expect(rosters.disabledIds).toContain(92);
    expect(rosters.disabledIds).toContain(93);
  });

  it("INSUFFICIENT strategies are not in any roster or disabledIds", () => {
    const scores = [makeScore(95, "INSUFFICIENT")];
    const rosters = buildRegimeRosters(scores);
    expect(rosters.chopRoster).not.toContain(95);
    expect(rosters.trendRoster).not.toContain(95);
    expect(rosters.disabledIds).not.toContain(95);
  });
});

// ── Regime-based roster building ──────────────────────────────────────────────

describe("buildRegimeRosters — regime breakdown", () => {
  it("strategy positive in chop regime lands in chopRoster", () => {
    const scores = [
      makeScore(91, "PROMOTE", {
        regimes: {
          chop: { trades: 8, expectancy: 1.5, winRate: 0.7 },
          trendLow: { trades: 4, expectancy: -0.5, winRate: 0.3 },
        },
      }),
    ];
    const rosters = buildRegimeRosters(scores);
    expect(rosters.chopRoster).toContain(91);
    expect(rosters.trendRoster).not.toContain(91);
  });

  it("strategy positive in trendLow and trendHigh lands in trendRoster", () => {
    const scores = [
      makeScore(92, "PROMOTE", {
        regimes: {
          trendLow: { trades: 5, expectancy: 2.0, winRate: 0.75 },
          trendHigh: { trades: 5, expectancy: 1.5, winRate: 0.65 },
        },
      }),
    ];
    const rosters = buildRegimeRosters(scores);
    expect(rosters.trendRoster).toContain(92);
  });

  it("strategy positive in both chop and trend lands in highVolRoster (all-weather)", () => {
    const scores = [
      makeScore(91, "PROMOTE", {
        regimes: {
          chop: { trades: 6, expectancy: 1.0, winRate: 0.6 },
          trendLow: { trades: 6, expectancy: 1.2, winRate: 0.65 },
        },
      }),
    ];
    const rosters = buildRegimeRosters(scores);
    expect(rosters.chopRoster).toContain(91);
    expect(rosters.trendRoster).toContain(91);
    expect(rosters.highVolRoster).toContain(91);
  });

  it("strategy with < 3 regime trades does not count that regime", () => {
    const scores = [
      makeScore(91, "PROMOTE", {
        regimes: {
          chop: { trades: 2, expectancy: 5.0, winRate: 1.0 }, // < 3 → ignored
        },
      }),
    ];
    const rosters = buildRegimeRosters(scores);
    // Falls through to fallback: no regime data → uses overall expectancy + category
    // strategy 91 is not in FUTURES_STRAT_DEFS so category is "" → neither chop nor trend style
    expect(rosters.highVolRoster).not.toContain(91);
  });
});

// ── Env lines ─────────────────────────────────────────────────────────────────

describe("buildRegimeRosters — envLines", () => {
  it("env line contains correct key and sorted IDs", () => {
    const scores = [
      makeScore(95, "PROMOTE", {
        regimes: { chop: { trades: 5, expectancy: 1, winRate: 0.6 } },
      }),
      makeScore(91, "PROMOTE", {
        regimes: { chop: { trades: 5, expectancy: 1, winRate: 0.6 } },
      }),
    ];
    const rosters = buildRegimeRosters(scores);
    expect(rosters.envLines.chop).toContain("NEXT_PUBLIC_BTC_FT_STRATEGY_IDS=");
    // IDs should be sorted ascending in the env line
    const match = rosters.envLines.chop.match(/=(.+)$/);
    if (match) {
      const ids = match[1].split(",").map(Number);
      expect(ids).toEqual([...ids].sort((a, b) => a - b));
    }
  });

  it("empty roster produces env line with no IDs", () => {
    const rosters = buildRegimeRosters([]);
    expect(rosters.envLines.chop).toBe("NEXT_PUBLIC_BTC_FT_STRATEGY_IDS=");
    expect(rosters.disabledIds).toEqual([]);
  });
});
