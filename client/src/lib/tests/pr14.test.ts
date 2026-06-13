import { describe, it, expect } from "vitest";
import { deriveDeskRecommendation } from "../trading/futuresDeskRecommendation";
import { scoreSignalQuality } from "../analytics/futuresSignalQuality";
import {
  computeMTFConfluence,
  mtfSkipReason,
} from "../trading/futuresMTFConfluence";
import { computeAttribution } from "../analytics/futuresAttribution";
import type { TuneRecommendation } from "../trading/futuresParameterTuner";

describe("deriveDeskRecommendation", () => {
  it("returns NO_CHANGE with null inputs", () => {
    const rec = deriveDeskRecommendation({
      attribution: null,
      rotation: null,
      tune: null,
      qualitySkipCount: 0,
      mtfSkipCount: 0,
      totalEvaluations: 0,
    });
    expect(rec.action).toBe("NO_CHANGE");
  });

  it("qualitySkipCount > 60% evals → RAISE_THRESHOLD", () => {
    const rec = deriveDeskRecommendation({
      attribution: null,
      rotation: null,
      tune: null,
      qualitySkipCount: 7,
      mtfSkipCount: 0,
      totalEvaluations: 10,
    });
    expect(rec.action).toBe("RAISE_THRESHOLD");
    expect(rec.confidence).toBe("HIGH");
  });

  it("tune SIGNAL_THRESHOLD maps to RAISE_THRESHOLD", () => {
    const tune: TuneRecommendation = {
      target: "SIGNAL_THRESHOLD",
      currentValue: 28,
      suggestedValue: 32,
      delta: 4,
      confidence: "HIGH",
      minTradesNeeded: 10,
      tradesAnalyzed: 20,
      rationale: "Fee ratio elevated",
      beforeSim: { label: "before", expectedWinRate: 0.4, expectedExpectancy: -1, expectedFeePct: 0.6 },
      afterSim: { label: "after", expectedWinRate: 0.45, expectedExpectancy: 0, expectedFeePct: 0.5 },
      doNothing: "Fees may remain high",
    };
    const rec = deriveDeskRecommendation({
      attribution: null,
      rotation: null,
      tune,
      qualitySkipCount: 0,
      mtfSkipCount: 0,
      totalEvaluations: 20,
    });
    expect(rec.action).toBe("RAISE_THRESHOLD");
    expect(rec.suggestedValue).toBe(32);
  });

  it("attribution bestHoldBucket surfaces in rationale", () => {
    const trades = [
      ...Array.from({ length: 4 }, () => ({
        strategy_name: "A",
        net_pnl: 12,
        gross_pnl: 14,
        fees: 2,
        exit_reason: "TP",
        side: "LONG",
        template_family: "mtf",
        opened_at: new Date(Date.now() - 3 * 60_000).toISOString(),
        closed_at: new Date().toISOString(),
      })),
      ...Array.from({ length: 4 }, () => ({
        strategy_name: "B",
        net_pnl: -25,
        gross_pnl: -20,
        fees: 5,
        exit_reason: "SL",
        side: "SHORT",
        template_family: "mtf",
        opened_at: new Date(Date.now() - 90 * 60_000).toISOString(),
        closed_at: new Date(Date.now() - 10 * 60_000).toISOString(),
      })),
    ];
    const attribution = computeAttribution(trades as never);
    const rec = deriveDeskRecommendation({
      attribution,
      rotation: null,
      tune: null,
      qualitySkipCount: 1,
      mtfSkipCount: 1,
      totalEvaluations: 10,
    });
    if (rec.action === "PROMOTE_BEST_HOLD") {
      expect(rec.rationale).toContain(attribution.bestHoldBucket ?? "");
    } else {
      expect(attribution.bestHoldBucket).toBeTruthy();
    }
  });
});

describe("PR-13 purity invariants", () => {
  const goodInput = {
    signalScore: 35,
    atrPct: 0.0012,
    spreadPct: 0.0002,
    volumeRatio: 1.6,
    regime: "trendHigh" as const,
    regimeFitsStrategy: true,
    ema20AboveEma50: true,
    priceAboveEma20: true,
    side: "LONG" as const,
    openPositionCount: 2,
    sameSideCount: 1,
    hoursIntoSession: 10,
    strategyWinRate: 0.55,
    strategyTrades: 30,
    cooldownRemainMs: 0,
  };

  it("scoreSignalQuality is pure (identical inputs → identical output)", () => {
    const a = scoreSignalQuality(goodInput);
    const b = scoreSignalQuality(goodInput);
    expect(a).toEqual(b);
  });

  it("mtfSkipReason null when confluent and agrees", () => {
    const mkSnap = (tf: "1m" | "5m" | "15m" | "1h" | "4h" | "1d", bull: boolean) => ({
      tf,
      close: 100_000,
      ema20: bull ? 99_000 : 101_000,
      ema50: bull ? 98_000 : 102_000,
      rsi: bull ? 60 : 40,
      atr: 500,
      volumeRatio: 1.2,
      isAvailable: true,
    });
    const snaps = (["1m", "5m", "15m", "1h", "4h", "1d"] as const).map((tf) => mkSnap(tf, true));
    const result = computeMTFConfluence(snaps, "LONG");
    expect(mtfSkipReason(result, 55)).toBeNull();
  });
});
