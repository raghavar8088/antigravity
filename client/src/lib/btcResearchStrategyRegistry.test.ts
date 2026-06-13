import { describe, expect, it } from "vitest";
import { BTC_RESEARCH_STRATEGIES } from "@/lib/trading/btcResearchStrategyRegistry";
import type { OHLCVCandle } from "@/lib/ai/mockResearchIndicators";

const T0 = 1_700_000_000_000;

function rangeCandles(count = 30): OHLCVCandle[] {
  return Array.from({ length: count }, (_, index) => ({
    time: T0 + index * 60_000,
    open: 60_000,
    high: 60_050,
    low: 59_950,
    close: 60_000 + Math.sin(index) * 10,
    volume: 1_000,
  }));
}

describe("BTC_RESEARCH_STRATEGIES", () => {
  it("loads research-backed and data-pending strategies with unique IDs", () => {
    expect(BTC_RESEARCH_STRATEGIES.length).toBeGreaterThanOrEqual(50);
    expect(BTC_RESEARCH_STRATEGIES.filter((strategy) => !strategy.dataFeedRequired)).toHaveLength(60);
    const ids = BTC_RESEARCH_STRATEGIES.map((strategy) => strategy.id);
    expect(new Set(ids).size).toBe(ids.length);
    expect(ids.every((id) => id >= 2000 && id <= 2099)).toBe(true);
  });

  it("keeps external-feed strategies as no-signal stubs", () => {
    const stubs = BTC_RESEARCH_STRATEGIES.filter((strategy) => strategy.dataFeedRequired);
    expect(stubs.length).toBeGreaterThan(0);
    for (const strategy of stubs) {
      expect(strategy.signal(rangeCandles()).side).toBe("NO_SIGNAL");
    }
  });

  it("generates BUY and SELL liquidity sweep signals from OHLCV structure", () => {
    const longStrategy = BTC_RESEARCH_STRATEGIES.find(
      (strategy) => strategy.family === "StopHuntSfp" && strategy.side === "LONG",
    );
    const shortStrategy = BTC_RESEARCH_STRATEGIES.find(
      (strategy) => strategy.family === "StopHuntSfp" && strategy.side === "SHORT",
    );

    const base = rangeCandles(25);
    const longCandles = [
      ...base,
      { time: T0 + 26 * 60_000, open: 60_000, high: 60_020, low: 59_800, close: 59_980, volume: 1_500 },
    ];
    const shortCandles = [
      ...base,
      { time: T0 + 26 * 60_000, open: 60_000, high: 60_200, low: 59_980, close: 60_020, volume: 1_500 },
    ];

    expect(longStrategy?.signal(longCandles).side).toBe("BUY");
    expect(shortStrategy?.signal(shortCandles).side).toBe("SELL");
  });
});
