import { describe, expect, it } from "vitest";
import {
  ALL_RESEARCH_FAMILIES,
  RESEARCH_FAMILIES_WITH_STRATEGIES,
  RESEARCH_FAMILY_LABELS,
  RESEARCH_STRATEGIES,
  RESEARCH_STRATEGY_BY_ID,
} from "@/lib/ai/mockResearchStrategies";

describe("RESEARCH_STRATEGIES registry", () => {
  it("has no mock research strategies", () => {
    expect(RESEARCH_STRATEGIES).toEqual([]);
    expect(RESEARCH_STRATEGY_BY_ID.size).toBe(0);
    expect(RESEARCH_FAMILIES_WITH_STRATEGIES).toEqual([]);
  });

  it("has no active research family filters", () => {
    expect(ALL_RESEARCH_FAMILIES).toEqual([]);
    expect(RESEARCH_FAMILY_LABELS).toEqual({});
  });
});
