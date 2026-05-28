/**
 * Tests for perpMarketFeatures — pure function, no I/O.
 */
import { describe, it, expect } from "vitest";
import { extractPerpMarketFeatures } from "./perpMarketFeatures";

describe("extractPerpMarketFeatures — funding annualized", () => {
  it("annualises funding correctly (3/day × 365 × rate × 100)", () => {
    const result = extractPerpMarketFeatures({
      fundingRate: 0.0001,
      markPrice: 60000,
      regime: "chop",
    });
    // 0.0001 × 1095 × 100 = 10.95%
    expect(result.fundingAnnualized).toBeCloseTo(10.95);
  });

  it("negative funding produces negative annualized", () => {
    const result = extractPerpMarketFeatures({
      fundingRate: -0.0003,
      markPrice: 60000,
      regime: "trendLow",
    });
    expect(result.fundingAnnualized).toBeLessThan(0);
  });
});

describe("extractPerpMarketFeatures — fundingZScore", () => {
  it("returns null with < 3 history points", () => {
    const result = extractPerpMarketFeatures({
      fundingRate: 0.0001,
      markPrice: 60000,
      regime: "chop",
      recentFundingHistory: [0.0001, 0.0002],
    });
    expect(result.fundingZScore).toBeNull();
  });

  it("returns a number with ≥ 3 history points", () => {
    const result = extractPerpMarketFeatures({
      fundingRate: 0.0005,
      markPrice: 60000,
      regime: "chop",
      recentFundingHistory: [0.0001, 0.0002, 0.0001, 0.0001],
    });
    expect(result.fundingZScore).not.toBeNull();
    expect(typeof result.fundingZScore).toBe("number");
  });

  it("z-score of mean value is ~0", () => {
    const history = [0.0001, 0.0002, 0.0003];
    const mean = history.reduce((s, v) => s + v, 0) / history.length;
    const result = extractPerpMarketFeatures({
      fundingRate: mean,
      markPrice: 60000,
      regime: "chop",
      recentFundingHistory: history,
    });
    expect(Math.abs(result.fundingZScore ?? 1)).toBeLessThan(0.01);
  });
});

describe("extractPerpMarketFeatures — volumeZScore", () => {
  it("returns null when volume is not provided", () => {
    const result = extractPerpMarketFeatures({
      fundingRate: 0,
      markPrice: 60000,
      regime: "chop",
    });
    expect(result.volumeZScore).toBeNull();
  });

  it("returns null when volume history is too short", () => {
    const result = extractPerpMarketFeatures({
      fundingRate: 0,
      markPrice: 60000,
      regime: "chop",
      volume: 1000,
      recentVolumeHistory: [900, 1100],
    });
    expect(result.volumeZScore).toBeNull();
  });
});

describe("extractPerpMarketFeatures — fee hurdle", () => {
  it("default fee hurdle is 0.2% (0.1% in + 0.1% out)", () => {
    const result = extractPerpMarketFeatures({
      fundingRate: 0,
      markPrice: 60000,
      regime: "chop",
    });
    expect(result.feeHurdlePct).toBeCloseTo(0.2);
  });

  it("custom fees produce correct hurdle", () => {
    const result = extractPerpMarketFeatures({
      fundingRate: 0,
      markPrice: 60000,
      regime: "chop",
      takerFeePct: 0.0005,
      makerFeePct: 0.0002,
    });
    // taker + max(taker, maker) = 0.05% + 0.05% = 0.10%
    expect(result.feeHurdlePct).toBeCloseTo(0.1);
  });
});

describe("extractPerpMarketFeatures — spread and ATR", () => {
  it("markLastSpreadPct is 0 when lastPrice is absent", () => {
    const result = extractPerpMarketFeatures({
      fundingRate: 0,
      markPrice: 60000,
      regime: "chop",
    });
    expect(result.markLastSpreadPct).toBe(0);
  });

  it("markLastSpreadPct computes correctly", () => {
    const result = extractPerpMarketFeatures({
      fundingRate: 0,
      markPrice: 60060,
      lastPrice: 60000,
      regime: "chop",
    });
    expect(result.markLastSpreadPct).toBeCloseTo(0.1);
  });

  it("volatilityAtrPct is 0 when ATR is absent", () => {
    const result = extractPerpMarketFeatures({
      fundingRate: 0,
      markPrice: 60000,
      regime: "chop",
    });
    expect(result.volatilityAtrPct).toBe(0);
  });

  it("volatilityAtrPct is ATR/price*100", () => {
    const result = extractPerpMarketFeatures({
      fundingRate: 0,
      markPrice: 50000,
      atr: 500,
      regime: "trendHigh",
    });
    expect(result.volatilityAtrPct).toBeCloseTo(1.0);
  });
});
