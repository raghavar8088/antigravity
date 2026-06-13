/**
 * PR-6 Entry Quality tests.
 */

import { describe, expect, it } from "vitest";
import {
  blockStrategyRuntime,
  clearRuntimeBlocklist,
  computeAdaptiveThreshold,
  computeChopAwareThreshold,
  getRuntimeBlocklist,
  isSameSideCapped,
  isStrategyRuntimeBlocked,
  unblockStrategyRuntime,
  CHOP_THRESHOLD_BOOST,
} from "../trading/futuresDeskPolicy";
import {
  logSkipReason,
  getSkipReasonSummary,
  resetSkipReasonLog,
} from "../risk/entrySkipReason";

describe("runtime strategy blocklist", () => {
  it("blocks and unblocks strategy ids in memory", () => {
    clearRuntimeBlocklist();
    expect(isStrategyRuntimeBlocked(91)).toBe(false);
    blockStrategyRuntime(91);
    expect(isStrategyRuntimeBlocked(91)).toBe(true);
    expect(getRuntimeBlocklist()).toEqual([91]);
    unblockStrategyRuntime(91);
    expect(isStrategyRuntimeBlocked(91)).toBe(false);
  });

  it("clearRuntimeBlocklist resets all blocks", () => {
    blockStrategyRuntime(92);
    blockStrategyRuntime(93);
    clearRuntimeBlocklist();
    expect(getRuntimeBlocklist()).toEqual([]);
  });
});

describe("computeAdaptiveThreshold", () => {
  it("caps total boost at 12", () => {
    expect(
      computeAdaptiveThreshold(28, "chop", "F", 0.6),
    ).toBe(40);
  });

  it("never returns below base", () => {
    expect(computeAdaptiveThreshold(30, "trendHigh", "A", 0.1)).toBe(30);
  });

  it("adds trendLow +2 and grade C +2", () => {
    expect(computeAdaptiveThreshold(28, "trendLow", "C", null)).toBe(32);
  });
});

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
    expect(summary[0]?.reason).toBe("THRESHOLD_NOT_MET");
  });

  it("returns empty array when log is empty", () => {
    resetSkipReasonLog();
    expect(getSkipReasonSummary(60_000)).toEqual([]);
  });
});
