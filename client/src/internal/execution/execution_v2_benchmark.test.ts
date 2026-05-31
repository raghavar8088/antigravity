import { describe, expect, it } from "vitest";
import { SignalAggregatorV2 } from "@/internal/trading/aggregator_v2";
import type { Signal } from "@/internal/strategy/evaluator";
import type { SignalQualityScore } from "@/internal/trading/signal_scoring";

function signal(i: number): Signal {
  return {
    Symbol: "BTCUSD",
    Direction: "BUY",
    Confidence: 70 + (i % 20),
    Entry: 100 + (i % 10) * 0.01,
    StopLoss: 98,
    TakeProfit: 104,
    Timestamp: i,
    StrategyID: 90 + (i % 100),
    rawScore: 70 + (i % 20),
    reason: "benchmark",
  };
}

function scored(i: number): SignalQualityScore {
  return {
    SignalScore: 60 + (i % 40),
    signal: signal(i),
    components: [],
    explanation: "benchmark",
  };
}

describe("Execution V2 performance envelope", () => {
  it("aggregates 1000 scored signals without throttling", () => {
    const agg = new SignalAggregatorV2({ maxPerSymbol: 5 });
    const signals = Array.from({ length: 1000 }, (_, i) => scored(i));
    const started = performance.now();

    const result = agg.aggregate(signals);
    const latencyMs = performance.now() - started;

    expect(result.candidates.length).toBeGreaterThan(0);
    expect(latencyMs).toBeLessThan(50);
  });
});
