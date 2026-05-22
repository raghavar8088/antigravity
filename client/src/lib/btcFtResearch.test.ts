import { afterEach, describe, expect, it, vi } from "vitest";
import {
  computeVerdict,
  resolveResearchActiveIds,
  resolveResearchPool,
  poolGeneratedCount,
  isResearchModeEnabled,
  resolveBtcFtActiveStrategyIds,
} from "./btcFtResearch";
import { BTC_FUTURE_TRADING_STRATEGY_IDS } from "./btcFtRoster";
import { aggregateResearchStratStats, type ResearchDbRow } from "./paperTradesAnalytics";

afterEach(() => {
  vi.unstubAllEnvs();
});

// ---------------------------------------------------------------------------
// Verdict logic tests
// ---------------------------------------------------------------------------
describe("computeVerdict", () => {
  it("INSUFFICIENT_DATA when tradeCount < 10", () => {
    expect(computeVerdict({ tradeCount: 0, sumNet: 10, expectancy: 1 })).toBe("INSUFFICIENT_DATA");
    expect(computeVerdict({ tradeCount: 9, sumNet: 10, expectancy: 1 })).toBe("INSUFFICIENT_DATA");
  });

  it("LOSER when tradeCount >= 15 and sumNet < -2", () => {
    expect(computeVerdict({ tradeCount: 15, sumNet: -3, expectancy: -0.2 })).toBe("LOSER");
    expect(computeVerdict({ tradeCount: 20, sumNet: -5, expectancy: -0.25 })).toBe("LOSER");
  });

  it("LOSER when tradeCount >= 15 and expectancy < -0.10", () => {
    expect(computeVerdict({ tradeCount: 15, sumNet: -1, expectancy: -0.11 })).toBe("LOSER");
  });

  it("WINNER when tradeCount >= 20, expectancy > 0, sumNet > 0", () => {
    expect(computeVerdict({ tradeCount: 20, sumNet: 5, expectancy: 0.25 })).toBe("WINNER");
    expect(computeVerdict({ tradeCount: 50, sumNet: 100, expectancy: 2 })).toBe("WINNER");
  });

  it("CANDIDATE in intermediate zone (10-19 trades, not clearly loser)", () => {
    expect(computeVerdict({ tradeCount: 10, sumNet: -0.5, expectancy: -0.05 })).toBe("CANDIDATE");
    expect(computeVerdict({ tradeCount: 15, sumNet: 1, expectancy: 0.067 })).toBe("CANDIDATE");
  });

  it("CANDIDATE at 20+ trades with expectancy = 0 (not strictly positive)", () => {
    expect(computeVerdict({ tradeCount: 20, sumNet: 0, expectancy: 0 })).toBe("CANDIDATE");
  });

  it("LOSER takes priority over WINNER threshold if sumNet < -2 at 20 trades", () => {
    // trades >= 15 and sumNet < -2 → LOSER (checked before WINNER)
    expect(computeVerdict({ tradeCount: 20, sumNet: -3, expectancy: -0.15 })).toBe("LOSER");
  });

  // ---- Tighter v2 thresholds (12 trades, sumNet < -$1 OR expectancy < -$0.05) ----
  it("LOSER fires at 12 trades when sumNet < -1 (was 15 trades / -$2)", () => {
    expect(computeVerdict({ tradeCount: 12, sumNet: -1.2, expectancy: -0.1 })).toBe("LOSER");
  });

  it("LOSER fires at 12 trades when expectancy < -0.05 (was -0.10)", () => {
    expect(computeVerdict({ tradeCount: 13, sumNet: -0.5, expectancy: -0.06 })).toBe("LOSER");
  });

  it("11 trades is still INSUFFICIENT_DATA territory (no premature retire)", () => {
    expect(computeVerdict({ tradeCount: 11, sumNet: -5, expectancy: -1 })).toBe("CANDIDATE");
  });

  it("WINNER blocked when feePctOfGross >= 80 — fee-dominated edge is rejected", () => {
    expect(
      computeVerdict({ tradeCount: 25, sumNet: 1.5, expectancy: 0.06, feePctOfGross: 85 }),
    ).toBe("CANDIDATE");
  });

  it("WINNER granted when feePctOfGross < 80 and other gates pass", () => {
    expect(
      computeVerdict({ tradeCount: 25, sumNet: 1.5, expectancy: 0.06, feePctOfGross: 60 }),
    ).toBe("WINNER");
  });

  it("WINNER granted when feePctOfGross is null (legacy stats without fee data)", () => {
    expect(
      computeVerdict({ tradeCount: 25, sumNet: 1.5, expectancy: 0.06, feePctOfGross: null }),
    ).toBe("WINNER");
  });
});

// ---------------------------------------------------------------------------
// Batch rotation tests
// ---------------------------------------------------------------------------
describe("resolveResearchActiveIds", () => {
  const pool = Array.from({ length: 60 }, (_, i) => i + 1); // ids 1-60

  it("returns batch of size batchSize from pool", () => {
    const result = resolveResearchActiveIds({ pool, batchSize: 30, rotateEveryHours: 24, nowMs: 0 });
    expect(result.activeIds.length).toBe(30);
    expect(result.totalBatches).toBe(2);
    expect(result.batchIndex).toBe(0);
  });

  it("advances to next batch after rotateEveryHours", () => {
    const r0 = resolveResearchActiveIds({ pool, batchSize: 30, rotateEveryHours: 24, nowMs: 0 });
    const r1 = resolveResearchActiveIds({ pool, batchSize: 30, rotateEveryHours: 24, nowMs: 24 * 3600_000 });
    expect(r0.batchIndex).toBe(0);
    expect(r1.batchIndex).toBe(1);
    expect(r0.activeIds[0]).not.toBe(r1.activeIds[0]);
  });

  it("wraps around after all batches exhausted", () => {
    const r2 = resolveResearchActiveIds({ pool, batchSize: 30, rotateEveryHours: 24, nowMs: 48 * 3600_000 });
    expect(r2.batchIndex).toBe(0); // 2 batches → slot 2 mod 2 = 0
  });

  it("excludes retired IDs from pool", () => {
    const retired = new Set([1, 2, 3]);
    const result = resolveResearchActiveIds({ pool, batchSize: 30, retiredIds: retired, nowMs: 0 });
    expect(result.activeIds.every((id) => !retired.has(id))).toBe(true);
    expect(result.poolSize).toBe(pool.length - retired.size);
  });

  it("falls back to CORE when pool is empty after retirement", () => {
    const retired = new Set(pool);
    const result = resolveResearchActiveIds({ pool, retiredIds: retired, batchSize: 30 });
    expect(result.activeIds.length).toBeGreaterThan(0);
    expect(result.poolSize).toBe(0);
  });

  it("handles pool smaller than batchSize gracefully", () => {
    const smallPool = [1, 2, 3];
    const result = resolveResearchActiveIds({ pool: smallPool, batchSize: 30, nowMs: 0 });
    expect(result.activeIds.length).toBe(3);
    expect(result.totalBatches).toBe(1);
  });
});

// ---------------------------------------------------------------------------
// resolveResearchPool / poolGeneratedCount — generated pool removed
// ---------------------------------------------------------------------------
describe("resolveResearchPool — generated pool removed", () => {
  it("never includes 300–399 (research pool deleted)", () => {
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_RESEARCH_MODE", "1");
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_GENERATED_POOL", "1");
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_POOL_IDS", "");
    const pool = resolveResearchPool();
    const hasGenerated = pool.some((id) => id >= 300 && id <= 399);
    expect(hasGenerated).toBe(false);
    expect(poolGeneratedCount()).toBe(0);
  });

  it("pool is CORE-only regardless of research mode", () => {
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_RESEARCH_MODE", "");
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_POOL_IDS", "");
    const pool = resolveResearchPool();
    expect(pool.length).toBeLessThanOrEqual(BTC_FUTURE_TRADING_STRATEGY_IDS.length);
    expect(poolGeneratedCount()).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// aggregateResearchStratStats tests
// ---------------------------------------------------------------------------
describe("aggregateResearchStratStats", () => {
  function makeRow(overrides: Partial<ResearchDbRow>): ResearchDbRow {
    return {
      strategy_id: 1,
      net_pnl: 1.0,
      gross_pnl: 2.0,
      fees: 0.2,
      opened_at: "2026-01-01T00:00:00Z",
      closed_at: "2026-01-01T00:20:00Z",
      ...overrides,
    };
  }

  it("computes expectancy and winRate correctly", () => {
    const rows = [
      makeRow({ net_pnl: 2, strategy_id: 1 }),
      makeRow({ net_pnl: -1, strategy_id: 1 }),
      makeRow({ net_pnl: 3, strategy_id: 1 }),
    ];
    const stats = aggregateResearchStratStats(rows);
    expect(stats[0]!.tradeCount).toBe(3);
    expect(stats[0]!.sumNet).toBeCloseTo(4);
    expect(stats[0]!.expectancy).toBeCloseTo(4 / 3);
    expect(stats[0]!.winRate).toBeCloseTo(2 / 3);
  });

  it("computes avgHoldMin from opened_at / closed_at", () => {
    // 20 min hold
    const rows = [makeRow({ opened_at: "2026-01-01T00:00:00Z", closed_at: "2026-01-01T00:20:00Z" })];
    const stats = aggregateResearchStratStats(rows);
    expect(stats[0]!.avgHoldMin).toBeCloseTo(20);
  });

  it("computes feePctOfGross", () => {
    // gross 10, fees 2 → 20%
    const rows = [makeRow({ gross_pnl: 10, fees: 2, net_pnl: 8 })];
    const stats = aggregateResearchStratStats(rows);
    expect(stats[0]!.feePctOfGross).toBeCloseTo(20);
  });

  it("handles missing optional fields gracefully", () => {
    const rows = [{ strategy_id: 5, net_pnl: 1.5 }];
    const stats = aggregateResearchStratStats(rows);
    expect(stats[0]!.avgHoldMin).toBeNull();
    expect(stats[0]!.feePctOfGross).toBeNull();
    expect(stats[0]!.lastTradeAt).toBeNull();
  });

  it("sorts by sumNet descending", () => {
    const rows = [
      makeRow({ strategy_id: 1, net_pnl: -5 }),
      makeRow({ strategy_id: 2, net_pnl: 10 }),
      makeRow({ strategy_id: 3, net_pnl: 2 }),
    ];
    const stats = aggregateResearchStratStats(rows);
    expect(stats[0]!.strategyId).toBe(2);
    expect(stats[2]!.strategyId).toBe(1);
  });
});

// ---------------------------------------------------------------------------
// isResearchModeEnabled
// ---------------------------------------------------------------------------
describe("isResearchModeEnabled", () => {
  it("returns false when env is not set", () => {
    // In vitest, NEXT_PUBLIC_BTC_FT_RESEARCH_MODE is unset by default
    expect(isResearchModeEnabled()).toBe(false);
  });
});

describe("resolveBtcFtActiveStrategyIds winners-only", () => {
  it("uses promoted winners only and caps to 20 when WINNERS_ONLY=1", () => {
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_WINNERS_ONLY", "1");
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_STRATEGY_IDS", "");
    // Use valid CORE 20 IDs (extended 200-series removed)
    const winnerIds = [91, 92, 95, 96, 111, 112, 117, 118, 123, 124, 125, 126, 131, 132, 133, 134, 139, 140, 151, 152, 500];

    const result = resolveBtcFtActiveStrategyIds({ winnerIds });

    expect(result.winnersOnly).toBe(true);
    expect(result.source).toBe("winners");
    expect(result.ids).toEqual(winnerIds.slice(0, 20));
    expect(result.isLargeRoster).toBe(false);
  });

  it("falls back to explicit BTC_FT_STRATEGY_IDS when winners-only has no promoted winners", () => {
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_WINNERS_ONLY", "1");
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_STRATEGY_IDS", "91,92,95");

    const result = resolveBtcFtActiveStrategyIds({ winnerIds: [] });

    expect(result.winnersOnly).toBe(true);
    expect(result.source).toBe("env");
    expect(result.ids).toEqual([91, 92, 95]);
  });

  it("returns empty winners roster when no promoted or explicit IDs exist", () => {
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_WINNERS_ONLY", "1");
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_STRATEGY_IDS", "");

    const result = resolveBtcFtActiveStrategyIds({ winnerIds: [] });

    expect(result.winnersOnly).toBe(true);
    expect(result.source).toBe("winners-empty");
    expect(result.ids).toEqual([]);
  });
});
