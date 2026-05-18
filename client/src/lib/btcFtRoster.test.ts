import { describe, expect, it, vi, afterEach } from "vitest";
import {
  CORE_BTC_FT_STRATEGY_IDS,
  BTC_FUTURE_TRADING_STRATEGY_IDS,
  resolveBtcFtActiveStrategyIds,
} from "./btcFtRoster";

afterEach(() => {
  vi.unstubAllEnvs();
});

describe("btcFtRoster — resolveBtcFtActiveStrategyIds", () => {
  it("defaults to full roster (124 IDs: 20 CORE + 4 premium + 100 extended) when no env is set", () => {
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_STRATEGY_IDS", "");
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_USE_RANKED", "");
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_USE_CORE_ONLY", "");
    const result = resolveBtcFtActiveStrategyIds();
    expect(result.source).toBe("full");
    expect(result.ids.length).toBe(124);
    expect(result.isLargeRoster).toBe(true);
  });

  it("USE_CORE_ONLY=1 returns CORE basket only", () => {
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_STRATEGY_IDS", "");
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_USE_RANKED", "");
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_USE_CORE_ONLY", "1");
    const result = resolveBtcFtActiveStrategyIds();
    expect(result.source).toBe("core");
    expect(result.ids).toEqual([...CORE_BTC_FT_STRATEGY_IDS]);
    expect(result.isLargeRoster).toBe(false);
  });

  it("env comma list parses correctly and caps at 120", () => {
    const ids = Array.from({ length: 130 }, (_, i) => i + 91).join(",");
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_STRATEGY_IDS", ids);
    const result = resolveBtcFtActiveStrategyIds();
    expect(result.source).toBe("env");
    expect(result.ids.length).toBe(120); // hard cap
  });

  it("env list > 30 ids sets isLargeRoster", () => {
    const ids = Array.from({ length: 31 }, (_, i) => i + 91).join(",");
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_STRATEGY_IDS", ids);
    const result = resolveBtcFtActiveStrategyIds();
    expect(result.isLargeRoster).toBe(true);
  });

  it("env list ≤ 30 ids does NOT set isLargeRoster", () => {
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_STRATEGY_IDS", "91,92,95,96");
    const result = resolveBtcFtActiveStrategyIds();
    expect(result.isLargeRoster).toBe(false);
  });

  it("CORE_BTC_FT_STRATEGY_IDS includes 91, 92, 95, 96 (fake-div range but wired strats)", () => {
    expect(CORE_BTC_FT_STRATEGY_IDS).toContain(91);
    expect(CORE_BTC_FT_STRATEGY_IDS).toContain(92);
    expect(CORE_BTC_FT_STRATEGY_IDS).toContain(95);
    expect(CORE_BTC_FT_STRATEGY_IDS).toContain(96);
  });

  it("full BTC_FUTURE_TRADING_STRATEGY_IDS contains 124 IDs (20 CORE + 4 premium + 100 extended)", () => {
    expect(BTC_FUTURE_TRADING_STRATEGY_IDS.length).toBe(124);
  });
});
