import { describe, expect, it } from "vitest";
import { SignalAggregatorV2 } from "@/internal/trading/aggregator_v2";
import type { Signal } from "@/internal/strategy/evaluator";
import type { SignalQualityScore } from "@/internal/trading/signal_scoring";

function signal(overrides: Partial<Signal> = {}): Signal {
  return {
    Symbol: "BTCUSD",
    Direction: "BUY",
    Confidence: 80,
    Entry: 100,
    StopLoss: 98,
    TakeProfit: 104,
    Timestamp: 1,
    StrategyID: 91,
    rawScore: 80,
    reason: "test",
    ...overrides,
    strategyName: overrides.strategyName ?? "AggregatorTestStrategy",
  };
}

function scored(s: Signal, score: number): SignalQualityScore {
  return {
    SignalScore: score,
    signal: s,
    components: [],
    explanation: `score=${score}`,
  };
}

describe("SignalAggregatorV2", () => {
  it("deduplicates identical signals and keeps the best quality row", () => {
    const agg = new SignalAggregatorV2();
    const a = scored(signal(), 70);
    const b = scored(signal(), 85);

    const result = agg.aggregate([a, b]);

    expect(result.deduplicated).toBe(1);
    expect(result.candidates).toHaveLength(1);
    expect(result.candidates[0]!.quality.SignalScore).toBe(85);
    expect(result.candidates[0]!.duplicateCount).toBe(2);
  });

  it("drops unresolved BUY/SELL conflicts", () => {
    const agg = new SignalAggregatorV2({ minConflictScoreGap: 10 });
    const result = agg.aggregate([
      scored(signal({ Direction: "BUY", StrategyID: 91 }), 80),
      scored(signal({ Direction: "SELL", StrategyID: 92 }), 75),
    ]);

    expect(result.conflictsResolved).toBe(2);
    expect(result.candidates).toHaveLength(0);
  });

  it("keeps the stronger side when conflict score gap is decisive", () => {
    const agg = new SignalAggregatorV2({ minConflictScoreGap: 5 });
    const result = agg.aggregate([
      scored(signal({ Direction: "BUY", StrategyID: 91 }), 90),
      scored(signal({ Direction: "SELL", StrategyID: 92 }), 70),
    ]);

    expect(result.candidates).toHaveLength(1);
    expect(result.candidates[0]!.signal.Direction).toBe("BUY");
  });
});
