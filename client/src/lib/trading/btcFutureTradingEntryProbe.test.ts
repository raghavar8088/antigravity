import { describe, expect, it } from "vitest";
import { BTC_FUTURE_TRADING_STRATEGY_IDS } from "./btcFutureTradingRoster";
import { CORE_BTC_FT_STRATEGY_IDS } from "./btcFtRoster";
import { FUTURES_STRAT_DEFS } from "./futuresStrategies";
import { buildPaperDeskStrategies } from "./futuresDeskPolicy";

describe("BTC Future Trading desk roster entry probe", () => {
  it("has no futures strategy definitions or active IDs", () => {
    expect(FUTURES_STRAT_DEFS).toEqual([]);
    expect(CORE_BTC_FT_STRATEGY_IDS).toEqual([]);
    expect(BTC_FUTURE_TRADING_STRATEGY_IDS).toEqual([]);
  });

  it("builds an empty paper desk strategy list", () => {
    const built = buildPaperDeskStrategies(FUTURES_STRAT_DEFS, {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: true,
    });

    expect(built.strategies).toEqual([]);
    expect(built.fakeDiversityFilteredCount).toBe(0);
    expect(built.lowRrSkippedStratIds).toEqual([]);
  });
});
