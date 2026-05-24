import { describe, it, expect, beforeEach } from "vitest";
import {
  blockStrategyRuntime,
  clearRuntimeBlocklist,
  computeAdaptiveThreshold,
  getRuntimeBlocklist,
  isStrategyRuntimeBlocked,
  unblockStrategyRuntime,
} from "../futuresDeskPolicy";

describe("computeAdaptiveThreshold", () => {
  it("chop adds 6", () => {
    expect(computeAdaptiveThreshold(28, "chop")).toBe(34);
  });

  it("trendLow adds 2", () => {
    expect(computeAdaptiveThreshold(28, "trendLow")).toBe(30);
  });

  it("trendHigh adds 0", () => {
    expect(computeAdaptiveThreshold(28, "trendHigh")).toBe(28);
  });

  it("grade F adds 4 more", () => {
    expect(computeAdaptiveThreshold(28, "chop", "F")).toBe(38);
  });

  it("grade C adds 2 more", () => {
    expect(computeAdaptiveThreshold(28, "chop", "C")).toBe(36);
  });

  it("grade A adds 0 more", () => {
    expect(computeAdaptiveThreshold(28, "chop", "A")).toBe(34);
  });

  it("high fee adds 3 more", () => {
    expect(computeAdaptiveThreshold(28, "chop", "F", 0.8)).toBe(40);
  });

  it("caps total boost at 12", () => {
    // chop(6) + F(4) + highFee(3) = 13 → capped at 12 → 28 + 12 = 40
    expect(computeAdaptiveThreshold(28, "chop", "F", 0.8)).toBe(40);
  });

  it("never goes below base threshold", () => {
    expect(computeAdaptiveThreshold(28, "trendHigh", "A", 0.1)).toBe(28);
  });

  it("unknown regime treated as trendHigh (0 boost)", () => {
    expect(computeAdaptiveThreshold(28, "sideways")).toBe(28);
  });
});

describe("runtime blocklist", () => {
  beforeEach(() => {
    clearRuntimeBlocklist();
  });

  it("blocks a strategy", () => {
    blockStrategyRuntime(999);
    expect(isStrategyRuntimeBlocked(999)).toBe(true);
  });

  it("unblocks a strategy", () => {
    blockStrategyRuntime(998);
    unblockStrategyRuntime(998);
    expect(isStrategyRuntimeBlocked(998)).toBe(false);
  });

  it("getRuntimeBlocklist returns all blocked ids", () => {
    blockStrategyRuntime(997);
    expect(getRuntimeBlocklist()).toContain(997);
  });
});
