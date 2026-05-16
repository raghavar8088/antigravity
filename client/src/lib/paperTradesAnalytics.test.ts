import { describe, expect, it } from "vitest";
import {
  aggregateStrategyLeaderboard,
  aggregateStrategyStats,
  buildStratDisableSet,
  buildStratDisableSetFromStats,
  formatAutoDisabledStratIds,
  mergeDisabledStrategyIds,
  splitLeaderboardTopBottom,
  type LeaderboardTradeRow,
  type StratTradeRow,
} from "./paperTradesAnalytics";
import {
  paperTradeLeaderboardQuerySchema,
  paperTradeStrategyStatsQuerySchema,
} from "./paperTradesTypes";

function rowsForStrat(strategyId: number, netPnls: number[]): StratTradeRow[] {
  return netPnls.map((netPnl, i) => ({
    strategyId,
    netPnl,
    closedAt: `2026-05-${String(10 + i).padStart(2, "0")}T12:00:00.000Z`,
  }));
}

describe("aggregateStrategyStats + buildStratDisableSet", () => {
  it("disables loser only when tradeCount >= minTrades; winner with n=2 stays enabled", () => {
    const loser = rowsForStrat(91, [-0.2, -0.15, -0.1, -0.12, -0.08]);
    const winnerFew = rowsForStrat(92, [0.5, 0.4]);
    const rows = [...loser, ...winnerFew, ...rowsForStrat(93, [-0.3, -0.25, -0.2, -0.15, -0.1])];

    const stats = aggregateStrategyStats(rows);
    expect(stats[0]!.strategyId).toBe(93);
    expect(stats.find((s) => s.strategyId === 92)?.tradeCount).toBe(2);

    const disabled = buildStratDisableSet(rows, {
      minTrades: 5,
      maxExpectancyUsd: -0.05,
      maxSumNetUsd: -1,
    });
    expect(disabled.has(91)).toBe(true);
    expect(disabled.has(93)).toBe(true);
    expect(disabled.has(92)).toBe(false);
  });

  it("synthetic book: only strategies with n>=5 and bad expectancy/sum are disabled", () => {
    const rows: StratTradeRow[] = [
      ...rowsForStrat(1, [-0.1, -0.1, -0.1, -0.1, -0.1]),
      ...rowsForStrat(2, [-0.05, -0.05, -0.05, -0.05]),
      ...rowsForStrat(3, [0.2, 0.3]),
    ];
    expect(rows.length).toBe(11);
    const disabled = buildStratDisableSet(rows);
    expect(disabled.size).toBe(1);
    expect(disabled.has(1)).toBe(true);
    expect(disabled.has(2)).toBe(false);
    expect(disabled.has(3)).toBe(false);
  });

  it("buildStratDisableSetFromStats matches row-based disable set", () => {
    const rows = rowsForStrat(10, [-0.2, -0.2, -0.2, -0.2, -0.2]);
    const stats = aggregateStrategyStats(rows);
    expect(buildStratDisableSetFromStats(stats)).toEqual(buildStratDisableSet(rows));
  });
});

describe("mergeDisabledStrategyIds", () => {
  it("dedupes, merges, and sorts ascending", () => {
    expect(mergeDisabledStrategyIds([92, 91], [91, 93, 92])).toEqual([91, 92, 93]);
    expect(mergeDisabledStrategyIds(new Set([5]), [5, 6])).toEqual([5, 6]);
  });

  it("ignores invalid ids", () => {
    expect(mergeDisabledStrategyIds([], [0, Number.NaN, 10.9])).toEqual([10]);
  });
});

describe("formatAutoDisabledStratIds", () => {
  it("sorts and truncates long lists", () => {
    const ids = [120, 5, 91, 12];
    expect(formatAutoDisabledStratIds(ids, 8)).toBe("5,12,91…");
  });
});

describe("aggregateStrategyLeaderboard + splitLeaderboardTopBottom", () => {
  const synth: LeaderboardTradeRow[] = [
    { strategyId: 1, strategyName: "Alpha", netPnl: 2, closedAt: "2026-05-01T00:00:00Z" },
    { strategyId: 1, strategyName: "Alpha", netPnl: -1, closedAt: "2026-05-02T00:00:00Z" },
    { strategyId: 2, strategyName: "Beta", netPnl: -5, closedAt: "2026-05-01T00:00:00Z" },
    { strategyId: 2, strategyName: "Beta", netPnl: -3, closedAt: "2026-05-02T00:00:00Z" },
    { strategyId: 3, strategyName: "Gamma", netPnl: 10, closedAt: "2026-05-01T00:00:00Z" },
    { strategyId: 3, strategyName: "Gamma", netPnl: 4, closedAt: "2026-05-02T00:00:00Z" },
  ];

  it("aggregates sumNet, expectancy, and winRate then splits top/bottom", () => {
    const agg = aggregateStrategyLeaderboard(synth);
    expect(agg[0]!.strategyId).toBe(3);
    expect(agg[0]!.sumNet).toBe(14);
    expect(agg[0]!.tradeCount).toBe(2);
    expect(agg[0]!.expectancy).toBe(7);
    expect(agg[0]!.winRate).toBe(1);

    const alpha = agg.find((r) => r.strategyId === 1)!;
    expect(alpha.sumNet).toBe(1);
    expect(alpha.winRate).toBeCloseTo(0.5, 9);

    const { top, bottom } = splitLeaderboardTopBottom(agg, 2);
    expect(top.map((r) => r.strategyId)).toEqual([3, 1]);
    expect(bottom.map((r) => r.strategyId)).toEqual([2, 1]);
  });
});

describe("paperTradeLeaderboardQuerySchema", () => {
  it("defaults window_days to 30 and limit to 15", () => {
    const p = paperTradeLeaderboardQuerySchema.parse({ account_key: "btc_future_trading_20" });
    expect(p.window_days).toBe(30);
    expect(p.limit).toBe(15);
  });

  it("rejects window_days outside 1–90", () => {
    expect(
      paperTradeLeaderboardQuerySchema.safeParse({ account_key: "x", window_days: 0 }).success,
    ).toBe(false);
    expect(
      paperTradeLeaderboardQuerySchema.safeParse({ account_key: "x", window_days: 91 }).success,
    ).toBe(false);
  });
});

describe("paperTradeStrategyStatsQuerySchema", () => {
  it("defaults window_days to 14 and clamps bounds", () => {
    expect(
      paperTradeStrategyStatsQuerySchema.parse({ account_key: "btc_future_trading_20" }).window_days,
    ).toBe(14);
    expect(
      paperTradeStrategyStatsQuerySchema.parse({
        account_key: "x",
        window_days: "90",
      }).window_days,
    ).toBe(90);
    const bad = paperTradeStrategyStatsQuerySchema.safeParse({
      account_key: "x",
      window_days: 0,
    });
    expect(bad.success).toBe(false);
  });
});
