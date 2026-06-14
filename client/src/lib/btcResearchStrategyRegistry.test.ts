import { describe, expect, it } from "vitest";
import {
  ALL_BTC_RESEARCH_FAMILIES,
  BTC_RESEARCH_FAMILY_LABELS,
  BTC_RESEARCH_STRATEGIES,
} from "@/lib/trading/btcResearchStrategyRegistry";

describe("BTC_RESEARCH_STRATEGIES", () => {
  it("has no registered research strategies", () => {
    expect(BTC_RESEARCH_STRATEGIES).toEqual([]);
  });

  it("keeps family labels for historical UI filters", () => {
    for (const family of ALL_BTC_RESEARCH_FAMILIES) {
      expect(BTC_RESEARCH_FAMILY_LABELS[family]).toBeTruthy();
    }
  });
});
