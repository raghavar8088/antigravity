import { describe, expect, it } from "vitest";

import {
  BTC_FT_GENERATED_COUNT_DEFAULT,
  BTC_FT_GENERATED_DEFS,
  BTC_FT_GENERATED_STRATEGY_IDS,
  BTC_FT_GENERATED_TEMPLATE_CYCLE,
  isGeneratedPoolEnabled,
} from "@/lib/btcFtStrategyGenerator";

describe("btcFtStrategyGenerator — stubs (research pool removed)", () => {
  it("generated pool is empty", () => {
    expect(BTC_FT_GENERATED_DEFS.length).toBe(0);
    expect(BTC_FT_GENERATED_COUNT_DEFAULT).toBe(0);
    expect(BTC_FT_GENERATED_STRATEGY_IDS.length).toBe(0);
    expect(BTC_FT_GENERATED_TEMPLATE_CYCLE.length).toBe(0);
  });

  it("isGeneratedPoolEnabled always returns false", () => {
    expect(isGeneratedPoolEnabled()).toBe(false);
  });
});
