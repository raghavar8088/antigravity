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
} from "../trading/futuresProfitMode";
import { computeMTFConfluence } from "../trading/futuresMTFConfluence";

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
  function makeReport(
    overrides: Partial<{
      active: { strategyId: number }[];
      promoted: { strategyId: number }[];
      probation: { strategyId: number }[];
      suspended: { strategyId: number }[];
      insufficient: { strategyId: number }[];
      scores: { strategyId: number; status: string }[];
    }>,
  ) {
    const active = overrides.active ?? [];
    const promoted = overrides.promoted ?? [];
    const probation = overrides.probation ?? [];
    const suspended = overrides.suspended ?? [];
    const insufficient = overrides.insufficient ?? [];
    const scores = overrides.scores ?? [
      ...active.map((s) => ({ ...s, status: "ACTIVE" })),
      ...promoted.map((s) => ({ ...s, status: "PROMOTED" })),
      ...probation.map((s) => ({ ...s, status: "PROBATION" })),
      ...suspended.map((s) => ({ ...s, status: "SUSPENDED" })),
      ...insufficient.map((s) => ({ ...s, status: "INSUFFICIENT" })),
    ];
    return { active, promoted, probation, suspended, insufficient, scores } as never;
  }

  it("allows ACTIVE strategy", () => {
    const report = makeReport({ active: [{ strategyId: 1 }], promoted: [{ strategyId: 2 }] });
    expect(profitModeAllowsRotationStrategy(1, report, cfg())).toBe(true);
    expect(profitModeAllowsRotationStrategy(2, report, cfg())).toBe(true);
  });

  it("allows INSUFFICIENT strategy (< 5 trades, not enough data to block)", () => {
    const report = makeReport({
      active: [{ strategyId: 99 }],
      promoted: [],
      insufficient: [{ strategyId: 42 }],
    });
    expect(profitModeAllowsRotationStrategy(42, report, cfg())).toBe(true);
  });

  it("allows strategy not present in report at all (no data)", () => {
    const report = makeReport({
      active: [{ strategyId: 99 }],
      promoted: [],
    });
    // strategy 77 is not in the report at all
    expect(profitModeAllowsRotationStrategy(77, report, cfg())).toBe(true);
  });

  it("allows PROBATION strategy in strict mode while evidence accumulates", () => {
    const report = makeReport({
      active: [{ strategyId: 99 }],
      promoted: [],
      probation: [{ strategyId: 3 }],
    });
    expect(profitModeAllowsRotationStrategy(3, report, cfg())).toBe(true);
  });

  it("allows PROBATION when onlyPromotedOrActive=false (STRICT=0)", () => {
    const report = makeReport({
      active: [{ strategyId: 99 }],
      promoted: [],
      probation: [{ strategyId: 3 }],
    });
    expect(profitModeAllowsRotationStrategy(3, report, cfg({ onlyPromotedOrActive: false }))).toBe(true);
  });

  it("allows PROBATION but blocks SUSPENDED in strict mode", () => {
    const report = makeReport({
      active: [{ strategyId: 1 }],
      promoted: [{ strategyId: 2 }],
      probation: [{ strategyId: 3 }],
      suspended: [{ strategyId: 4 }],
    });
    expect(profitModeAllowsRotationStrategy(3, report, cfg())).toBe(true);
    expect(profitModeAllowsRotationStrategy(4, report, cfg())).toBe(false);
  });

  it("falls back to allow all when active+promoted is empty (zero-roster guard)", () => {
    // All strategies are INSUFFICIENT — desk must not be zeroed out
    const report = makeReport({
      active: [],
      promoted: [],
      insufficient: [{ strategyId: 10 }, { strategyId: 11 }],
    });
    expect(profitModeAllowsRotationStrategy(10, report, cfg())).toBe(true);
    expect(profitModeAllowsRotationStrategy(11, report, cfg())).toBe(true);
    // A completely new strategy also passes the fallback
    expect(profitModeAllowsRotationStrategy(99, report, cfg())).toBe(true);
  });

  it("no-op when profit mode disabled", () => {
    const report = makeReport({ active: [], promoted: [], suspended: [{ strategyId: 5 }] });
    expect(profitModeAllowsRotationStrategy(5, report, cfg({ enabled: false }))).toBe(true);
  });
});
