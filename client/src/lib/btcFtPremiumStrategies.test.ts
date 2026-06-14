import { describe, expect, it } from "vitest";
import {
  BTC_FT_PREMIUM_DEFS,
  BTC_FT_PREMIUM_ID_END,
  BTC_FT_PREMIUM_ID_START,
  BTC_FT_PREMIUM_STRATEGY_IDS,
  PREMIUM_NOTIONAL_MULTIPLIER,
} from "./trading/btcFtPremiumStrategies";

describe("btcFtPremiumStrategies — empty inventory", () => {
  it("has no premium strategy definitions or IDs", () => {
    expect(BTC_FT_PREMIUM_DEFS).toEqual([]);
    expect(BTC_FT_PREMIUM_STRATEGY_IDS).toEqual([]);
  });

  it("keeps reserved premium metadata constants", () => {
    expect(BTC_FT_PREMIUM_ID_START).toBe(500);
    expect(BTC_FT_PREMIUM_ID_END).toBe(527);
    expect(PREMIUM_NOTIONAL_MULTIPLIER).toBe(2.0);
  });
});
