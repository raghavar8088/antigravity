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
  it("defaults to CORE only (≤ 30 IDs) when no env is set", () => {
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_STRATEGY_IDS", "");
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_USE_RANKED", "");
    const result = resolveBtcFtActiveStrategyIds();
    expect(result.source).toBe("core");
    expect(result.ids.length).toBeGreaterThan(0);
    expect(result.ids.length).toBeLessThanOrEqual(30);
    expect(result.isLargeRoster).toBe(false);
  });

  it("CORE IDs match CORE_BTC_FT_STRATEGY_IDS in default mode", () => {
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_STRATEGY_IDS", "");
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_USE_RANKED", "");
    const result = resolveBtcFtActiveStrategyIds();
    expect(result.ids).toEqual([...CORE_BTC_FT_STRATEGY_IDS]);
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

  it("full BTC_FUTURE_TRADING_STRATEGY_IDS still contains 120 IDs (backward compat)", () => {
    expect(BTC_FUTURE_TRADING_STRATEGY_IDS.length).toBe(120);
  });
});
