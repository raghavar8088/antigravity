import { describe, expect, it } from "vitest";
import {
  BREAKOUT_TRADING_STRATEGIES,
  CATEGORY_POOL_160,
  DAY_TRADING_STRATEGIES,
  LEGACY_CORE_CATEGORY_MAP,
  MOMENTUM_TRADING_STRATEGIES,
  POSITION_TRADING_STRATEGIES,
  RANGE_TRADING_STRATEGIES,
  SCALPING_STRATEGIES,
  SWING_TRADING_STRATEGIES,
  TREND_TRADING_STRATEGIES,
} from "../trading/futuresCategoryStrategies";
import { CATEGORY_STRATEGY_IDS, buildCategoryRoster } from "../trading/btcFtRoster";
import { TRADING_CATEGORY_IDS } from "../trading/futuresCategoryRegistry";

describe("futures category strategy pools", () => {
  it("has no category strategy definitions", () => {
    expect(SCALPING_STRATEGIES).toEqual([]);
    expect(DAY_TRADING_STRATEGIES).toEqual([]);
    expect(SWING_TRADING_STRATEGIES).toEqual([]);
    expect(POSITION_TRADING_STRATEGIES).toEqual([]);
    expect(TREND_TRADING_STRATEGIES).toEqual([]);
    expect(RANGE_TRADING_STRATEGIES).toEqual([]);
    expect(BREAKOUT_TRADING_STRATEGIES).toEqual([]);
    expect(MOMENTUM_TRADING_STRATEGIES).toEqual([]);
    expect(CATEGORY_POOL_160).toEqual([]);
    expect(LEGACY_CORE_CATEGORY_MAP.size).toBe(0);
  });

  it("resolves empty per-category ID lists and rosters", () => {
    for (const categoryId of TRADING_CATEGORY_IDS) {
      expect(CATEGORY_STRATEGY_IDS[categoryId]).toEqual([]);
      expect(buildCategoryRoster(categoryId, { researchMode: true })).toEqual([]);
    }
  });
});
