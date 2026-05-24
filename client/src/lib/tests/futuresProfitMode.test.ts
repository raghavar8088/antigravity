import { describe, it, expect, beforeEach, afterEach } from "vitest";
import {
  applyProfitModeThreshold,
  profitModeAllowsStrategyInChop,
  profitModeCounterTrendSkip,
  profitModeFromEnv,
  profitModeMaxOpenPositions,
  profitModeMaxSameSide,
  profitModePassesQuality,
  profitModeAllowsRotationStrategy,
  type ProfitModeConfig,
} from "../futuresProfitMode";
import { computeMTFConfluence } from "../futuresMTFConfluence";

const cfg = (overrides: Partial<ProfitModeConfig> = {}): ProfitModeConfig => ({
  enabled: true,
  minQualityScore: 70,
  minMtfConfluence: 65,
  chopThresholdBoost: 10,
  minExpectedMoveK: 3,
  maxSameSideChop: 1,
  maxSameSideTrend: 2,
  maxOpenPositions: 6,
  dailyStratCap: 4,
  requireHighQualityInChop: true,
  blockCounterTrend: true,
  onlyPromotedOrActive: true,
  ...overrides,
});

describe("applyProfitModeThreshold", () => {
  it("adds chop boost when enabled", () => {
    expect(applyProfitModeThreshold(30, "chop", cfg())).toBe(40);
  });

  it("leaves trend threshold unchanged", () => {
    expect(applyProfitModeThreshold(30, "trendHigh", cfg())).toBe(30);
  });

  it("no-op when disabled", () => {
    expect(applyProfitModeThreshold(30, "chop", cfg({ enabled: false }))).toBe(30);
  });
});

describe("profitModePassesQuality", () => {
  it("rejects below min quality score", () => {
    const r = profitModePassesQuality(60, false, "trendHigh", cfg());
    expect(r.pass).toBe(false);
  });

  it("requires high quality in chop", () => {
    const r = profitModePassesQuality(72, false, "chop", cfg());
    expect(r.pass).toBe(false);
    expect(r.reason).toBe("PROFIT_MODE_CHOP_NOT_HQ");
  });
});

describe("profitModeCounterTrendSkip", () => {
  it("blocks short in bull MTF bias", () => {
    const mtf = computeMTFConfluence(
      [
        { tf: "1h", close: 100, ema20: 102, ema50: 100, rsi: 62, atr: 1, volumeRatio: 1.2, isAvailable: true },
        { tf: "4h", close: 100, ema20: 102, ema50: 100, rsi: 62, atr: 1, volumeRatio: 1.2, isAvailable: true },
        { tf: "1d", close: 100, ema20: 102, ema50: 100, rsi: 62, atr: 1, volumeRatio: 1.2, isAvailable: true },
      ],
      "SHORT",
    );
    expect(profitModeCounterTrendSkip("SHORT", mtf, cfg())).toMatch(/COUNTER_TREND/);
  });
});

describe("profitModeMaxOpenPositions", () => {
  it("caps desk max open", () => {
    expect(profitModeMaxOpenPositions(12, cfg())).toBe(6);
  });
});

describe("profitModeMaxSameSide", () => {
  it("uses 1 in chop and 2 in trend", () => {
    expect(profitModeMaxSameSide("chop", cfg())).toBe(1);
    expect(profitModeMaxSameSide("trendHigh", cfg())).toBe(2);
  });
});

describe("profitModeFromEnv", () => {
  const prev = process.env.NEXT_PUBLIC_DESK_PROFIT_MODE;

  afterEach(() => {
    if (prev === undefined) delete process.env.NEXT_PUBLIC_DESK_PROFIT_MODE;
    else process.env.NEXT_PUBLIC_DESK_PROFIT_MODE = prev;
  });

  it("disabled by default", () => {
    delete process.env.NEXT_PUBLIC_DESK_PROFIT_MODE;
    expect(profitModeFromEnv().enabled).toBe(false);
  });

  it("enabled when env is 1", () => {
    process.env.NEXT_PUBLIC_DESK_PROFIT_MODE = "1";
    expect(profitModeFromEnv().enabled).toBe(true);
    expect(profitModeFromEnv().minQualityScore).toBe(70);
  });
});

describe("profitModeAllowsStrategyInChop", () => {
  it("blocks breakout in chop when not chop-only regimes", () => {
    expect(
      profitModeAllowsStrategyInChop("BREAKOUT", "Breakout_Long", ["trendHigh"], "chop", cfg()),
    ).toBe(false);
  });
});

describe("profitModeAllowsRotationStrategy", () => {
  it("blocks ids not in active or promoted", () => {
    const report = {
      active: [{ strategyId: 1 }],
      promoted: [{ strategyId: 2 }],
      probation: [{ strategyId: 3 }],
      suspended: [],
    } as never;
    expect(profitModeAllowsRotationStrategy(1, report, cfg())).toBe(true);
    expect(profitModeAllowsRotationStrategy(3, report, cfg())).toBe(false);
  });
});
