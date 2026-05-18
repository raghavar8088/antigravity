import { describe, expect, it } from "vitest";
import {
  computeStrategyHourlyStats,
  isStrategyInProvenSession,
  SESSION_MIN_HOUR_TRADES,
  SESSION_MIN_TOTAL_TRADES,
  type SessionTradeSample,
} from "./strategySessionStats";

function sample(strategyId: number, netPnl: number, isoTime: string): SessionTradeSample {
  return { strategyId, netPnl, closedAtMs: Date.parse(isoTime) };
}

describe("computeStrategyHourlyStats", () => {
  it("buckets trades by UTC closed-at hour and skips other strategies", () => {
    const stats = computeStrategyHourlyStats(91, [
      sample(91, 1.0, "2026-05-18T08:30:00Z"),  // hour 8 win
      sample(91, -0.5, "2026-05-18T08:45:00Z"), // hour 8 loss
      sample(91, 2.0, "2026-05-18T14:15:00Z"),  // hour 14 win
      sample(92, 99, "2026-05-18T08:00:00Z"),   // wrong strat — ignored
    ]);
    expect(stats.totalTrades).toBe(3);
    expect(stats.byHour[8]!.trades).toBe(2);
    expect(stats.byHour[8]!.wins).toBe(1);
    expect(stats.byHour[8]!.winRate).toBeCloseTo(0.5, 2);
    expect(stats.byHour[8]!.expectancy).toBeCloseTo(0.25, 2);
    expect(stats.byHour[14]!.trades).toBe(1);
    expect(stats.byHour[14]!.wins).toBe(1);
    // Untouched hours stay zero
    expect(stats.byHour[3]!.trades).toBe(0);
    expect(stats.byHour[3]!.winRate).toBe(0);
  });

  it("ignores non-finite / out-of-range timestamps", () => {
    const stats = computeStrategyHourlyStats(91, [
      { strategyId: 91, netPnl: 1, closedAtMs: NaN },
      { strategyId: 91, netPnl: Infinity, closedAtMs: Date.now() },
    ]);
    expect(stats.totalTrades).toBe(0);
  });
});

describe("isStrategyInProvenSession", () => {
  function buildStats(opts: {
    totalAcrossOtherHours: number;
    hour: number;
    /** Net PnL value for each trade in the target hour. Positive = win, negative = loss. */
    hourTradeNets: number[];
  }) {
    const trades: SessionTradeSample[] = [];
    // Fill other hours with neutral trades
    const otherHour = (opts.hour + 6) % 24;
    for (let i = 0; i < opts.totalAcrossOtherHours; i++) {
      trades.push({
        strategyId: 91,
        netPnl: i % 2 === 0 ? 0.5 : -0.4,
        closedAtMs: Date.UTC(2026, 4, 18, otherHour, i % 60),
      });
    }
    // Add target-hour trades with explicit net values
    opts.hourTradeNets.forEach((net, i) => {
      trades.push({
        strategyId: 91,
        netPnl: net,
        closedAtMs: Date.UTC(2026, 4, 18, opts.hour, i % 60),
      });
    });
    return computeStrategyHourlyStats(91, trades);
  }

  it("allows entry when total sample is below threshold (no premature filtering)", () => {
    const stats = buildStats({
      totalAcrossOtherHours: 10,
      hour: 14,
      hourTradeNets: Array.from({ length: 10 }, () => -1), // all losses
    });
    expect(stats.totalTrades).toBeLessThan(SESSION_MIN_TOTAL_TRADES);
    expect(isStrategyInProvenSession(stats, 14)).toBe(true);
  });

  it("allows entry when the queried hour has fewer than min-hour trades", () => {
    const stats = buildStats({
      totalAcrossOtherHours: SESSION_MIN_TOTAL_TRADES + 10,
      hour: 14,
      hourTradeNets: Array.from({ length: SESSION_MIN_HOUR_TRADES - 1 }, () => -1),
    });
    expect(isStrategyInProvenSession(stats, 14)).toBe(true);
  });

  it("blocks entry when both winRate < 35% AND expectancy < 0 at this hour", () => {
    // 4 wins of +0.5, 16 losses of -1 → winRate 0.2, expectancy = (2 - 16) / 20 = -0.7
    const wins = Array.from({ length: 4 }, () => 0.5);
    const losses = Array.from({ length: 16 }, () => -1);
    const stats = buildStats({
      totalAcrossOtherHours: SESSION_MIN_TOTAL_TRADES + 10,
      hour: 14,
      hourTradeNets: [...wins, ...losses],
    });
    expect(stats.byHour[14]!.winRate).toBeCloseTo(0.2, 2);
    expect(stats.byHour[14]!.expectancy).toBeLessThan(0);
    expect(isStrategyInProvenSession(stats, 14)).toBe(false);
  });

  it("allows entry when winRate is poor but expectancy is positive (asymmetric win/loss)", () => {
    // 5 wins of +10, 15 losses of -1 → winRate 0.25 (below 0.35) but expectancy = (50-15)/20 = +1.75
    const wins = Array.from({ length: 5 }, () => 10);
    const losses = Array.from({ length: 15 }, () => -1);
    const stats = buildStats({
      totalAcrossOtherHours: SESSION_MIN_TOTAL_TRADES + 10,
      hour: 14,
      hourTradeNets: [...wins, ...losses],
    });
    expect(stats.byHour[14]!.winRate).toBeCloseTo(0.25, 2);
    expect(stats.byHour[14]!.expectancy).toBeGreaterThan(0);
    expect(isStrategyInProvenSession(stats, 14)).toBe(true);
  });

  it("returns true for hours outside 0..23 (defensive)", () => {
    const stats = buildStats({
      totalAcrossOtherHours: SESSION_MIN_TOTAL_TRADES + 10,
      hour: 14,
      hourTradeNets: [],
    });
    expect(isStrategyInProvenSession(stats, -1)).toBe(true);
    expect(isStrategyInProvenSession(stats, 25)).toBe(true);
  });
});
