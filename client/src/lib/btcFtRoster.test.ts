import { describe, expect, it, vi, afterEach } from "vitest";
import {
  BTC_FUTURE_TRADING_STRATEGY_IDS,
  CORE_BTC_FT_STRATEGY_IDS,
  resolveBtcFtActiveStrategyIds,
} from "./trading/btcFtRoster";

afterEach(() => {
  vi.unstubAllEnvs();
});

describe("btcFtRoster — empty strategy inventory", () => {
  it("defaults to an empty core roster", () => {
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_STRATEGY_IDS", "");
    const result = resolveBtcFtActiveStrategyIds();

    expect(result.source).toBe("core");
    expect(result.ids).toEqual([]);
    expect(result.isLargeRoster).toBe(false);
    expect(CORE_BTC_FT_STRATEGY_IDS).toEqual([]);
    expect(BTC_FUTURE_TRADING_STRATEGY_IDS).toEqual([]);
  });

  it("ignores env IDs because no active strategy IDs remain valid", () => {
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_STRATEGY_IDS", "91,92,500");
    const result = resolveBtcFtActiveStrategyIds();

    expect(result.source).toBe("core");
    expect(result.ids).toEqual([]);
  });
});
