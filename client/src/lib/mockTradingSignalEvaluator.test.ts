import { describe, expect, it } from "vitest";
import {
  evaluateMockTradingSignals,
  MOCK_TRADING_MIN_BARS,
  resolveMockTradingStrategies,
} from "@/lib/trading/mockTradingSignalEvaluator";
import type { MockTradingBar } from "@/lib/trading/mockTradingMarketData";

function bars(count = MOCK_TRADING_MIN_BARS): MockTradingBar[] {
  return Array.from({ length: count }, (_, index) => ({
    time: 1_700_000_000 + index * 60,
    open: 100_000,
    high: 100_100,
    low: 99_900,
    close: 100_000,
    volume: 1_000,
  }));
}

describe("mockTradingSignalEvaluator", () => {
  it("resolves no strategies", () => {
    expect(resolveMockTradingStrategies()).toEqual([]);
  });

  it("evaluates zero strategies without emitting rows", () => {
    const inputBars = bars();
    const result = evaluateMockTradingSignals({
      bars: inputBars,
      markPrice: inputBars[inputBars.length - 1]!.close,
      symbol: "BTCUSD",
      strategies: resolveMockTradingStrategies(),
    });

    expect(result.error).toBeNull();
    expect(result.evaluatedStrategies).toBe(0);
    expect(result.rows).toEqual([]);
  });
});
