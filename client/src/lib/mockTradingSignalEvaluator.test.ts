import { describe, expect, it } from "vitest";
import { FUTURES_STRAT_DEFS } from "@/lib/trading/futuresStrategies";
import { buildPaperDeskStrategies } from "@/lib/trading/futuresDeskPolicy";
import { CORE_BTC_FT_STRATEGY_IDS } from "@/lib/trading/btcFtRoster";
import {
  evaluateMockTradingSignals,
  MOCK_TRADING_MIN_BARS,
  resolveMockTradingStrategies,
} from "@/lib/trading/mockTradingSignalEvaluator";
import type { MockTradingBar } from "@/lib/trading/mockTradingMarketData";
import { isExecutableTraceRow } from "@/lib/trading/mockTradingEngine";

function bullishBars(base = 100_000, count = 40): MockTradingBar[] {
  const bars: MockTradingBar[] = [];
  let price = base;
  for (let i = 0; i < count; i++) {
    price *= 1.002 + (i % 3) * 0.0003;
    const high = price * 1.001;
    const low = price * 0.998;
    const open = i === 0 ? low : bars[i - 1]!.close;
    bars.push({
      time: 1_700_000_000 + i * 60,
      open,
      high,
      low,
      close: price,
      volume: 5000 + i * 200,
    });
  }
  return bars;
}

describe("mockTradingSignalEvaluator", () => {
  it("resolves a non-empty CORE strategy roster", () => {
    const strategies = resolveMockTradingStrategies();
    expect(strategies.length).toBeGreaterThan(0);
    expect(strategies.every((s) => CORE_BTC_FT_STRATEGY_IDS.includes(s.id))).toBe(true);
  });

  it("returns insufficient bars error below minimum", () => {
    const bars = bullishBars(100_000, MOCK_TRADING_MIN_BARS - 1);
    const result = evaluateMockTradingSignals({
      bars,
      markPrice: bars[bars.length - 1]!.close,
      symbol: "BTCUSD",
      strategies: resolveMockTradingStrategies().slice(0, 3),
    });
    expect(result.error).toMatch(/insufficient bars/i);
    expect(result.rows).toHaveLength(0);
  });

  it("emits executable CANDIDATE rows on strong bullish synthetic bars", () => {
    const bars = bullishBars(100_000, 45);
    const markPrice = bars[bars.length - 1]!.close;
    const coreRaw = FUTURES_STRAT_DEFS.filter((s) => CORE_BTC_FT_STRATEGY_IDS.includes(s.id));
    const { strategies } = buildPaperDeskStrategies(coreRaw, {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: false,
    });

    const result = evaluateMockTradingSignals({
      bars,
      markPrice,
      symbol: "BTCUSD",
      tickAt: 1_700_000_000_000,
      strategies,
    });

    expect(result.error).toBeNull();
    expect(result.evaluatedStrategies).toBe(strategies.length);
    const candidates = result.rows.filter((row) => row.status === "CANDIDATE");
    expect(candidates.length).toBeGreaterThan(0);
    expect(candidates.some((row) => isExecutableTraceRow(row))).toBe(true);
    expect(new Set(candidates.map((row) => row.traceId)).size).toBe(candidates.length);
  });
});
