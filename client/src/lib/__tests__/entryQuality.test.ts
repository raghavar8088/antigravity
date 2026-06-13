/**
 * PR-6 Entry Quality tests.
 *
 * Simulation note (see PR description):
 *   Before: threshold=28, no ATR gate, no corr cap
 *     → 4 correlated shorts opened at same mark price → all hit SL → expectancy: -$20/trade
 *   After: threshold=34 in chop, ATR gate (atr/price ≥ 0.06%), max 2 same-side
 *     → only highest-scoring signals enter, correlated exposure halved
 *     → fewer trades, higher quality → target expectancy: >$0 over 100 trades
 */

import { describe, expect, it } from "vitest";
import { computeChopAwareThreshold, isSameSideCapped, CHOP_THRESHOLD_BOOST } from "../trading/futuresDeskPolicy";
import {
  logSkipReason,
  getSkipReasonSummary,
  resetSkipReasonLog,
} from "../risk/entrySkipReason";

describe("regime-aware threshold (computeChopAwareThreshold)", () => {
  it("raises threshold by CHOP_THRESHOLD_BOOST in chop", () => {
    const base = 28;
    expect(computeChopAwareThreshold(base, "chop")).toBe(base + CHOP_THRESHOLD_BOOST);
    expect(computeChopAwareThreshold(base, "chop")).toBe(34);
  });

  it("keeps threshold unchanged in trendHigh", () => {
    const base = 28;
    expect(computeChopAwareThreshold(base, "trendHigh")).toBe(28);
  });

  it("keeps threshold unchanged in trendLow", () => {
    const base = 26;
    expect(computeChopAwareThreshold(base, "trendLow")).toBe(26);
  });

  it("applies boost consistently for any base value", () => {
    expect(computeChopAwareThreshold(20, "chop")).toBe(26);
    expect(computeChopAwareThreshold(32, "chop")).toBe(38);
    expect(computeChopAwareThreshold(0, "chop")).toBe(CHOP_THRESHOLD_BOOST);
  });
});

describe("correlated position cap (isSameSideCapped)", () => {
  it("blocks 3rd same-side position when max=2", () => {
    const openShorts = [{ side: "SHORT" }, { side: "SHORT" }];
    expect(isSameSideCapped(openShorts, "SHORT", 2)).toBe(true);
  });

  it("allows 2nd same-side when max=2 (count is below cap)", () => {
    const openShorts = [{ side: "SHORT" }];
    expect(isSameSideCapped(openShorts, "SHORT", 2)).toBe(false);
  });

  it("allows opposite side regardless of same-side count", () => {
    const openShorts = [{ side: "SHORT" }, { side: "SHORT" }];
    expect(isSameSideCapped(openShorts, "LONG", 2)).toBe(false);
  });

  it("uses MAX_SAME_SIDE_POSITIONS=2 as default", () => {
    const twoShorts = [{ side: "SHORT" }, { side: "SHORT" }];
    expect(isSameSideCapped(twoShorts, "SHORT")).toBe(true);
    const oneShort = [{ side: "SHORT" }];
    expect(isSameSideCapped(oneShort, "SHORT")).toBe(false);
  });

  it("handles empty positions correctly", () => {
    expect(isSameSideCapped([], "LONG", 2)).toBe(false);
    expect(isSameSideCapped([], "SHORT", 2)).toBe(false);
  });

  it("mixed sides: counts only the queried side", () => {
    const mixed = [{ side: "LONG" }, { side: "LONG" }, { side: "SHORT" }];
    expect(isSameSideCapped(mixed, "LONG", 2)).toBe(true);
    expect(isSameSideCapped(mixed, "SHORT", 2)).toBe(false);
  });
});

describe("skip reason registry (logSkipReason / getSkipReasonSummary)", () => {
  it("records skip reasons and returns frequency summary", () => {
    resetSkipReasonLog();
    logSkipReason(91, "THRESHOLD_NOT_MET", { score: 22, required: 34 });
    logSkipReason(92, "THRESHOLD_NOT_MET", { score: 20, required: 34 });
    logSkipReason(95, "CORRELATED_CAP", { side: "SHORT", openCount: 2 });

    const summary = getSkipReasonSummary(60_000);
    const threshRow = summary.find((r) => r.reason === "THRESHOLD_NOT_MET");
    const corrRow = summary.find((r) => r.reason === "CORRELATED_CAP");

    expect(threshRow?.count).toBe(2);
    expect(corrRow?.count).toBe(1);
    expect(summary[0]?.reason).toBe("THRESHOLD_NOT_MET"); // sorted by count desc
  });

  it("returns empty array when log is empty", () => {
    resetSkipReasonLog();
    expect(getSkipReasonSummary(60_000)).toEqual([]);
  });
});
