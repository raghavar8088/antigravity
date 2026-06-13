import { describe, expect, it } from "vitest";
import { classifyMarketRegime } from "@/lib/ai/marketRegimeClassifier";
import type { OHLCVCandle } from "@/lib/ai/mockResearchIndicators";

const T0 = 1_700_000_000_000;

function candlesFromCloses(closes: number[], range = 80): OHLCVCandle[] {
  return closes.map((close, index) => ({
    time: T0 + index * 60_000,
    open: index === 0 ? close : closes[index - 1],
    high: close + range,
    low: close - range,
    close,
    volume: 1_000 + index,
  }));
}

describe("classifyMarketRegime", () => {
  it("classifies a strong directional sequence as trending", () => {
    const closes = Array.from({ length: 140 }, (_, index) => 60_000 + index * 90);
    const snapshot = classifyMarketRegime(candlesFromCloses(closes, 60));
    expect(snapshot.regime).toBe("TRENDING");
    expect(snapshot.confidence).toBeGreaterThan(50);
  });

  it("classifies tight low-volatility candles as chop", () => {
    const closes = Array.from({ length: 140 }, (_, index) => 60_000 + Math.sin(index / 4) * 3);
    const snapshot = classifyMarketRegime(candlesFromCloses(closes, 4));
    expect(["LOW_VOLATILITY_CHOP", "RANGING"]).toContain(snapshot.regime);
    expect(snapshot.atrPct).toBeLessThan(0.15);
  });

  it("returns a conservative ranging snapshot when history is insufficient", () => {
    const snapshot = classifyMarketRegime(candlesFromCloses([60_000, 60_010, 60_005]));
    expect(snapshot.regime).toBe("RANGING");
    expect(snapshot.confidence).toBe(20);
  });
});
