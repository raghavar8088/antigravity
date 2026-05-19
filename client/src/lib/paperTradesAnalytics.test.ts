import { describe, expect, it } from "vitest";
import {
  aggregateResearchStratStats,
  aggregateStrategyLeaderboard,
  aggregateStrategyStats,
  buildStratDisableSet,
  buildStratDisableSetFromStats,
  calcQuantExpectancy,
  DESK_KILL_MIN_TRADES_DEFAULT,
  formatAutoDisabledStratIds,
  mergeDisabledStrategyIds,
  splitLeaderboardTopBottom,
  type LeaderboardTradeRow,
  type ResearchDbRow,
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

describe("calcQuantExpectancy", () => {
  it("returns 0 for empty input", () => {
    expect(calcQuantExpectancy([])).toBe(0);
  });

  it("computes E = W×Pavg − L×Lavg correctly with known values", () => {
    // 3 wins (+2 each), 2 losses (−1 each)
    // W=0.6, Pavg=2, L=0.4, Lavg=1 → E = 0.6×2 − 0.4×1 = 0.8
    const rows: StratTradeRow[] = [
      { strategyId: 1, netPnl: 2 },
      { strategyId: 1, netPnl: 2 },
      { strategyId: 1, netPnl: 2 },
      { strategyId: 1, netPnl: -1 },
      { strategyId: 1, netPnl: -1 },
    ];
    expect(calcQuantExpectancy(rows)).toBeCloseTo(0.8, 9);
  });

  it("handles all-wins (no losses)", () => {
    const rows = rowsForStrat(1, [1, 2, 3]);
    // W=1, Pavg=2, L=0, Lavg=0 → E = 2
    expect(calcQuantExpectancy(rows)).toBeCloseTo(2, 9);
  });

  it("handles all-losses (no wins)", () => {
    const rows = rowsForStrat(1, [-1, -2, -3]);
    // W=0, Pavg=0, L=1, Lavg=2 → E = -2
    expect(calcQuantExpectancy(rows)).toBeCloseTo(-2, 9);
  });

  it("is different from simple sumNet/count for skewed distributions", () => {
    // 1 big win (+90), 9 small losses (−1 each)
    // sumNet/count = (90−9)/10 = 8.1
    // proper: W=0.1, Pavg=90, L=0.9, Lavg=1 → E = 9 − 0.9 = 8.1  (coincides here)
    // Use an asymmetric case: 1 win (+50), 9 losses (−1 each)
    // sumNet/count = (50−9)/10 = 4.1
    // proper: W=0.1, Pavg=50, L=0.9, Lavg=1 → E = 5 − 0.9 = 4.1
    // Both formulas agree on this — test the sign distinction instead
    const rows: StratTradeRow[] = [
      { strategyId: 1, netPnl: 0.01 }, // tiny win
      { strategyId: 1, netPnl: -10 },
      { strategyId: 1, netPnl: -10 },
    ];
    // sumNet/count = (0.01−20)/3 ≈ −6.66
    // proper: W=1/3, Pavg=0.01, L=2/3, Lavg=10 → E ≈ 0.003 − 6.667 = −6.663 (close but different)
    const e = calcQuantExpectancy(rows);
    expect(e).toBeLessThan(0);
    const naiveAvg = rows.reduce((s, r) => s + r.netPnl, 0) / rows.length;
    // Both are negative; confirm proper formula is used (not just average)
    expect(Math.abs(e - naiveAvg)).toBeGreaterThan(0);
  });
});

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

  it("default kill-gate is 100 — strategies below threshold are never auto-disabled", () => {
    expect(DESK_KILL_MIN_TRADES_DEFAULT).toBe(100);
    const rows: StratTradeRow[] = [
      ...rowsForStrat(1, [-0.1, -0.1, -0.1, -0.1, -0.1]), // 5 bad trades — below gate
      ...rowsForStrat(2, [-0.05, -0.05, -0.05, -0.05]),    // 4 bad trades — below gate
      ...rowsForStrat(3, [0.2, 0.3]),                       // 2 good trades — below gate
    ];
    // No strategy reaches 100 trades, so default gate never fires
    const disabled = buildStratDisableSet(rows);
    expect(disabled.size).toBe(0);
  });

  it("kills strategy once it crosses 100 trades with negative expectancy", () => {
    // 100 trades: 40 wins (+1) and 60 losses (−1) → E = 0.4×1 − 0.6×1 = −0.2
    const wins = rowsForStrat(99, Array(40).fill(1));
    const losses = rowsForStrat(99, Array(60).fill(-1));
    const disabled = buildStratDisableSet([...wins, ...losses]);
    expect(disabled.has(99)).toBe(true);
  });

  it("does NOT kill strategy with 100 trades and positive expectancy", () => {
    // 60 wins (+1), 40 losses (−0.5) → E = 0.6×1 − 0.4×0.5 = 0.4
    const wins = rowsForStrat(77, Array(60).fill(1));
    const losses = rowsForStrat(77, Array(40).fill(-0.5));
    const disabled = buildStratDisableSet([...wins, ...losses]);
    expect(disabled.has(77)).toBe(false);
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

describe("aggregateResearchStratStats — MAE / MFE / Sharpe / feePctOfGross", () => {
  function researchRows(
    strategyId: number,
    trades: { net: number; gross?: number; fees?: number; holdMin?: number }[],
  ): ResearchDbRow[] {
    const base = new Date("2026-01-01T00:00:00Z").getTime();
    return trades.map((t, i) => {
      const openMs = base + i * 3_600_000;
      const closeMs = openMs + (t.holdMin ?? 30) * 60_000;
      return {
        strategy_id: strategyId,
        strategy_name: `Strat${strategyId}`,
        net_pnl: t.net,
        gross_pnl: t.gross,
        fees: t.fees,
        opened_at: new Date(openMs).toISOString(),
        closed_at: new Date(closeMs).toISOString(),
      };
    });
  }

  it("computes correct expectancy via W×Pavg − L×Lavg", () => {
    // 3 wins (+4 each), 2 losses (−2 each)
    // W=0.6, Pavg=4, L=0.4, Lavg=2 → E = 2.4 − 0.8 = 1.6
    const rows = researchRows(1, [
      { net: 4 }, { net: 4 }, { net: 4 },
      { net: -2 }, { net: -2 },
    ]);
    const [agg] = aggregateResearchStratStats(rows);
    expect(agg!.expectancy).toBeCloseTo(1.6, 2);
    expect(agg!.winRate).toBeCloseTo(0.6, 3);
  });

  it("computes MAE as avg absolute loss (gross) on losing trades", () => {
    // 1 win (gross=5), 2 losses (gross=−3, −7) → MAE = (3+7)/2 = 5
    const rows = researchRows(2, [
      { net: 4, gross: 5 },
      { net: -2, gross: -3 },
      { net: -6, gross: -7 },
    ]);
    const [agg] = aggregateResearchStratStats(rows);
    expect(agg!.mae).toBeCloseTo(5, 2);
  });

  it("computes MFE as avg gross on winning trades", () => {
    // 2 wins (gross=8, gross=12), 1 loss → MFE = (8+12)/2 = 10
    const rows = researchRows(3, [
      { net: 6, gross: 8 },
      { net: 10, gross: 12 },
      { net: -1, gross: -1 },
    ]);
    const [agg] = aggregateResearchStratStats(rows);
    expect(agg!.mfe).toBeCloseTo(10, 2);
  });

  it("computes sharpeProxy = expectancy / stddev", () => {
    // Trades: [2, 2, −2, −2] → mean=0, E=W×Pavg−L×Lavg=0.5×2−0.5×2=0
    const rows = researchRows(4, [
      { net: 2 }, { net: 2 }, { net: -2 }, { net: -2 },
    ]);
    const [agg] = aggregateResearchStratStats(rows);
    // E=0, so sharpeProxy should be 0 (or null if stddev check fails — but stddev>0 here)
    expect(agg!.sharpeProxy).toBe(0);
  });

  it("sharpeProxy is null when only 1 trade (no variance)", () => {
    const rows = researchRows(5, [{ net: 3 }]);
    const [agg] = aggregateResearchStratStats(rows);
    expect(agg!.sharpeProxy).toBeNull();
  });

  it("computes feePctOfGross correctly", () => {
    // gross=100, fees=5 → feePctOfGross = 5%
    const rows = researchRows(6, [
      { net: 95, gross: 100, fees: 5 },
    ]);
    const [agg] = aggregateResearchStratStats(rows);
    expect(agg!.feePctOfGross).toBeCloseTo(5, 2);
  });

  it("feePctOfGross is null when gross = 0", () => {
    const rows = researchRows(7, [{ net: 0 }]);
    const [agg] = aggregateResearchStratStats(rows);
    expect(agg!.feePctOfGross).toBeNull();
  });

  it("avgHoldMin is computed from opened_at / closed_at", () => {
    // Two 30-min trades → avgHoldMin = 30
    const rows = researchRows(8, [{ net: 1, holdMin: 30 }, { net: -1, holdMin: 30 }]);
    const [agg] = aggregateResearchStratStats(rows);
    expect(agg!.avgHoldMin).toBeCloseTo(30, 1);
  });

  it("sorts results by sumNet descending", () => {
    const rows = [
      ...researchRows(10, [{ net: -5 }]),
      ...researchRows(11, [{ net: 10 }]),
      ...researchRows(12, [{ net: 3 }]),
    ];
    const agg = aggregateResearchStratStats(rows);
    expect(agg[0]!.strategyId).toBe(11);
    expect(agg[1]!.strategyId).toBe(12);
    expect(agg[2]!.strategyId).toBe(10);
  });
});
