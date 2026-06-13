import { describe, expect, it } from "vitest";
import {
  ALLOCATION_MAX_MULTIPLIER,
  ALLOCATION_MIN_MULTIPLIER,
  ALLOCATION_MIN_TRADES,
  atrPctFromAtr,
  computeAdaptiveTpPct,
  computeAllocationMultiplier,
  computeStrategyEdgeStats,
  type StrategyTradeSample,
} from "./trading/strategyAllocation";

function trade(netPnl: number, notional = 100, holdMinutes = 10): StrategyTradeSample {
  return { netPnl, notional, holdMinutes };
}

// ---------------------------------------------------------------------------
// computeStrategyEdgeStats
// ---------------------------------------------------------------------------
describe("computeStrategyEdgeStats", () => {
  it("returns all-zero stats for empty input (no fake amplification)", () => {
    const s = computeStrategyEdgeStats([]);
    expect(s.tradeCount).toBe(0);
    expect(s.winRate).toBe(0);
    expect(s.expectancyPct).toBe(0);
    expect(s.sharpe).toBe(0);
    expect(s.defensiveKelly).toBe(0);
  });

  it("rejects invalid samples (non-finite net or zero notional)", () => {
    const s = computeStrategyEdgeStats([
      trade(1),
      { netPnl: NaN, notional: 100, holdMinutes: 10 },
      { netPnl: 1, notional: 0, holdMinutes: 10 },
    ]);
    expect(s.tradeCount).toBe(1);
  });

  it("computes positive expectancy + non-zero Kelly for a winning strategy", () => {
    // 60% win rate, avg win 1%, avg loss 0.5%
    const trades = [
      trade(1), trade(1), trade(1), trade(1), trade(1), trade(1), // 6 wins of $1
      trade(-0.5), trade(-0.5), trade(-0.5), trade(-0.5), // 4 losses of $0.50
    ];
    const s = computeStrategyEdgeStats(trades);
    expect(s.tradeCount).toBe(10);
    expect(s.winRate).toBeCloseTo(0.6, 2);
    expect(s.avgWinPct).toBeCloseTo(1.0, 1);
    expect(s.avgLossPct).toBeCloseTo(0.5, 1);
    expect(s.expectancyPct).toBeCloseTo(0.4, 1);
    expect(s.rawKelly).toBeGreaterThan(0);
    expect(s.defensiveKelly).toBeLessThanOrEqual(0.25); // hard cap
  });

  it("Kelly is zero or negative for a losing strategy", () => {
    const trades = [
      trade(-1), trade(-1), trade(-1), trade(-1), trade(-1), trade(-1), // 6 losses
      trade(0.5), trade(0.5), trade(0.5), trade(0.5), // 4 wins
    ];
    const s = computeStrategyEdgeStats(trades);
    expect(s.expectancyPct).toBeLessThan(0);
    expect(s.defensiveKelly).toBe(0); // clamped to 0 (no allocation for losers)
  });

  it("Sharpe returns 0 when stddev is 0 (all identical trades)", () => {
    const s = computeStrategyEdgeStats([trade(1), trade(1), trade(1)]);
    expect(s.sharpe).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// computeAllocationMultiplier
// ---------------------------------------------------------------------------
describe("computeAllocationMultiplier", () => {
  it("returns 1.0 when sample is below ALLOCATION_MIN_TRADES (no premature allocation)", () => {
    const stats = computeStrategyEdgeStats(
      Array.from({ length: ALLOCATION_MIN_TRADES - 1 }, () => trade(1)),
    );
    const mul = computeAllocationMultiplier({ stats, cohortSharpes: [1, 1, 1] });
    expect(mul).toBe(1.0);
  });

  it("clamps the multiplier within [MIN, MAX]", () => {
    const stats = computeStrategyEdgeStats(
      // 20 huge wins + Kelly will be very high, but multiplier must clamp.
      Array.from({ length: 20 }, () => trade(20)),
    );
    const mul = computeAllocationMultiplier({ stats, cohortSharpes: [0.1, 0.1, 0.1] });
    expect(mul).toBeGreaterThanOrEqual(ALLOCATION_MIN_MULTIPLIER);
    expect(mul).toBeLessThanOrEqual(ALLOCATION_MAX_MULTIPLIER);
  });

  it("scales down (to MIN) for break-even / losing strategies (Kelly clamps to 0)", () => {
    // Exactly 50% win rate, symmetric → Kelly = 0 → kellyFactor = MIN clamp
    const trades = [
      ...Array.from({ length: 10 }, () => trade(0.5)),
      ...Array.from({ length: 10 }, () => trade(-0.5)),
    ];
    const stats = computeStrategyEdgeStats(trades);
    expect(stats.defensiveKelly).toBe(0);
    const mul = computeAllocationMultiplier({ stats, cohortSharpes: [1, 1, 1] });
    expect(mul).toBe(ALLOCATION_MIN_MULTIPLIER); // clamped to floor (no allocation for losers)
  });

  it("under-performing peer Sharpe → scales down by 0.5x", () => {
    // 20 wins, big edge → high Kelly and high Sharpe locally
    const stats = computeStrategyEdgeStats(
      Array.from({ length: 20 }, (_, i) => trade(i % 2 === 0 ? 1.5 : -0.5)),
    );
    // But cohort median Sharpe is much higher — should knock us down
    const mul = computeAllocationMultiplier({
      stats,
      cohortSharpes: [stats.sharpe * 10, stats.sharpe * 10, stats.sharpe * 10],
    });
    expect(mul).toBeLessThanOrEqual(1.0);
  });
});

// ---------------------------------------------------------------------------
// computeAdaptiveTpPct
// ---------------------------------------------------------------------------
describe("computeAdaptiveTpPct", () => {
  it("returns base TP when atrPct is zero or invalid", () => {
    expect(computeAdaptiveTpPct(0.6, 0)).toBe(0.6);
    expect(computeAdaptiveTpPct(0.6, -1)).toBe(0.6);
    expect(computeAdaptiveTpPct(0.6, NaN)).toBe(0.6);
  });

  it("tightens TP for low volatility (ATR 0.10%)", () => {
    const adj = computeAdaptiveTpPct(0.6, 0.10);
    expect(adj).toBeCloseTo(0.6 * 0.8, 2);
  });

  it("leaves TP unchanged at the normal-vol pivot (ATR 0.25%)", () => {
    const adj = computeAdaptiveTpPct(0.6, 0.25);
    expect(adj).toBeCloseTo(0.6, 2);
  });

  it("widens TP for high volatility (ATR 0.50%)", () => {
    const adj = computeAdaptiveTpPct(0.6, 0.50);
    expect(adj).toBeCloseTo(0.6 * 1.2, 2);
  });

  it("caps the widen factor at 1.4x for extreme volatility (ATR 1.0%+)", () => {
    const adj = computeAdaptiveTpPct(0.6, 1.0);
    expect(adj).toBeCloseTo(0.6 * 1.4, 2);
  });

  it("never returns NaN even for absurd inputs", () => {
    expect(computeAdaptiveTpPct(0.6, 999)).toBeCloseTo(0.6 * 1.4, 2);
    expect(computeAdaptiveTpPct(-1, 0.5)).toBe(-1); // negative base → returned unchanged
  });
});

// ---------------------------------------------------------------------------
// atrPctFromAtr
// ---------------------------------------------------------------------------
describe("atrPctFromAtr", () => {
  it("computes ATR as % of price", () => {
    expect(atrPctFromAtr(300, 100_000)).toBeCloseTo(0.3, 4);
  });

  it("returns 0 for invalid inputs (no division by zero)", () => {
    expect(atrPctFromAtr(0, 100_000)).toBe(0);
    expect(atrPctFromAtr(300, 0)).toBe(0);
    expect(atrPctFromAtr(NaN, 100_000)).toBe(0);
  });
});
