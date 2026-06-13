import { describe, expect, it, vi, afterEach } from "vitest";
import {
  CORE_BTC_FT_STRATEGY_IDS,
  BTC_FUTURE_TRADING_STRATEGY_IDS,
  resolveBtcFtActiveStrategyIds,
} from "./trading/btcFtRoster";

afterEach(() => {
  vi.unstubAllEnvs();
});

describe("btcFtRoster — resolveBtcFtActiveStrategyIds", () => {
  it("defaults to CORE basket when no env is set", () => {
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_STRATEGY_IDS", "");
    const result = resolveBtcFtActiveStrategyIds();
    expect(result.source).toBe("core");
    expect(result.ids).toEqual([...CORE_BTC_FT_STRATEGY_IDS]);
    expect(result.isLargeRoster).toBe(false);
  });

  it("env comma list filters to CORE IDs (unknown IDs excluded)", () => {
    const ids = [...CORE_BTC_FT_STRATEGY_IDS, 999, 1000].join(",");
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_STRATEGY_IDS", ids);
    const result = resolveBtcFtActiveStrategyIds();
    expect(result.source).toBe("env");
    expect(result.ids.length).toBe(CORE_BTC_FT_STRATEGY_IDS.length);
    expect(result.ids).not.toContain(999);
  });

  it("env list parses correctly", () => {
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_STRATEGY_IDS", "91,92,95,96");
    const result = resolveBtcFtActiveStrategyIds();
    expect(result.source).toBe("env");
    expect(result.ids).toEqual([91, 92, 95, 96]);
    expect(result.isLargeRoster).toBe(false);
  });

  it("CORE_BTC_FT_STRATEGY_IDS includes the 20 winners", () => {
    const expected = [91, 92, 95, 96, 111, 112, 117, 118, 123, 124, 125, 126, 131, 132, 133, 134, 139, 140, 151, 152];
    for (const id of expected) {
      expect(CORE_BTC_FT_STRATEGY_IDS).toContain(id);
    }
  });

  it("BTC_FUTURE_TRADING_STRATEGY_IDS equals CORE (+ premium)", () => {
    expect(BTC_FUTURE_TRADING_STRATEGY_IDS).toEqual([...CORE_BTC_FT_STRATEGY_IDS]);
  });
});
