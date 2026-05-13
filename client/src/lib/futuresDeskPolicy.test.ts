import { afterEach, describe, expect, it, vi } from "vitest";
import { FUTURES_STRAT_DEFS } from "./futuresStrategies";
import {
  buildPaperDeskStrategies,
  defaultRegimesForCategory,
  deskEffectiveHoldMinutesAtOpen,
  DESK_REGIME_FALLBACK_ALLOW_ALL,
  deskHoldMinutesCategoryMul,
  deskHoldTuningExportIntervalMsFromEnv,
  deskMaxSameDirNotionalFracFromEnv,
  deskMinExpectedMoveSafetyKFromEnv,
  DESK_MAX_SAME_DIR_FRAC_OF_EQUITY_DEFAULT,
  DESK_MIN_EXPECTED_MOVE_SAFETY_K_DEFAULT,
  FAKE_DIVERSITY_STRAT_IDS,
  HOLD_MUL_AFTER_TP_WIDEN,
} from "./futuresDeskPolicy";

describe("deskEffectiveHoldMinutesAtOpen", () => {
  it("bumps base hold only for scalp_aggro_v1 when desk widened TP", () => {
    expect(deskEffectiveHoldMinutesAtOpen(20, "baseline", true)).toEqual({ holdMinutes: 20, profileAdjusted: false });
    expect(deskEffectiveHoldMinutesAtOpen(20, "scalp_aggro_v1", false)).toEqual({ holdMinutes: 20, profileAdjusted: false });
    expect(deskEffectiveHoldMinutesAtOpen(20, "scalp_aggro_v1", undefined)).toEqual({ holdMinutes: 20, profileAdjusted: false });
    const r = deskEffectiveHoldMinutesAtOpen(20, "scalp_aggro_v1", true);
    expect(r.profileAdjusted).toBe(true);
    expect(r.holdMinutes).toBeCloseTo(20 * HOLD_MUL_AFTER_TP_WIDEN, 8);
  });
});

describe("buildPaperDeskStrategies", () => {
  it("filters fake-diversity IDs 79–110 when allowFakeDiversity is false", () => {
    const r = buildPaperDeskStrategies(FUTURES_STRAT_DEFS, {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: false,
    });
    const ids = new Set(r.strategies.map((s) => s.id));
    for (const id of FAKE_DIVERSITY_STRAT_IDS) {
      expect(ids.has(id)).toBe(false);
    }
    expect(r.fakeDiversityFilteredCount).toBe(FAKE_DIVERSITY_STRAT_IDS.length);
  });

  it("records TP-widened ids when ratio was below min", () => {
    const r = buildPaperDeskStrategies(FUTURES_STRAT_DEFS, {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: true,
    });
    expect(r.tpWidenedStratIds.length).toBeGreaterThan(0);
    expect(r.lowRrSkippedStratIds.length).toBe(0);
  });

  it("tags strategies with deskTpWidened when TP% was raised", () => {
    const r = buildPaperDeskStrategies(FUTURES_STRAT_DEFS, {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: true,
    });
    const widened = r.strategies.filter((s) => s.deskTpWidened === true);
    expect(widened.length).toBe(r.tpWidenedStratIds.length);
    for (const s of r.strategies) {
      expect(typeof s.deskTpWidened).toBe("boolean");
    }
  });

  it("applies category hold multiplier in desk build (MeanRev > raw)", () => {
    const raw = FUTURES_STRAT_DEFS.find((s) => s.id === 3)!;
    expect(deskHoldMinutesCategoryMul(raw.category)).toBeGreaterThan(1);
    const r = buildPaperDeskStrategies([raw], {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: true,
    });
    const built = r.strategies.find((s) => s.id === 3);
    expect(built!.holdMinutes).toBeCloseTo(raw.holdMinutes * deskHoldMinutesCategoryMul(raw.category), 4);
  });

  it("attaches default regimes by category when defs omit regimes", () => {
    const raw = FUTURES_STRAT_DEFS.find((s) => s.id === 3)!;
    expect(raw.regimes).toBeUndefined();
    const r = buildPaperDeskStrategies([raw], {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: true,
    });
    expect(r.strategies[0]!.regimes).toEqual(["chop", "trendLow"]);
    expect(r.deskRegimeAnnotatedStratCount).toBe(1);
  });

  it("keeps explicit regimes from def and does not count toward annotation", () => {
    const raw = FUTURES_STRAT_DEFS.find((s) => s.id === 5)!;
    const r = buildPaperDeskStrategies([{ ...raw, regimes: ["trendHigh"] }], {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: true,
    });
    expect(r.strategies[0]!.regimes).toEqual(["trendHigh"]);
    expect(r.deskRegimeAnnotatedStratCount).toBe(0);
  });

  it("empty regimes array on def is treated as missing → defaults apply", () => {
    const raw = FUTURES_STRAT_DEFS.find((s) => s.id === 3)!;
    const r = buildPaperDeskStrategies([{ ...raw, regimes: [] }], {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: true,
    });
    expect(r.strategies[0]!.regimes).toEqual(["chop", "trendLow"]);
    expect(r.deskRegimeAnnotatedStratCount).toBe(1);
  });
});

describe("defaultRegimesForCategory", () => {
  it("maps MeanRev / Confluence / unknown per v1 table", () => {
    expect(defaultRegimesForCategory("MeanRev")).toEqual(["chop", "trendLow"]);
    expect(defaultRegimesForCategory("Confluence")).toEqual([...DESK_REGIME_FALLBACK_ALLOW_ALL]);
    expect(defaultRegimesForCategory("TotallyUnknownCategory")).toEqual([...DESK_REGIME_FALLBACK_ALLOW_ALL]);
  });

  it("full desk build annotates every included strat when defs omit regimes", () => {
    const r = buildPaperDeskStrategies(FUTURES_STRAT_DEFS, {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: true,
    });
    expect(r.deskRegimeAnnotatedStratCount).toBe(r.strategies.length);
    for (const s of r.strategies) {
      expect(s.regimes?.length).toBeGreaterThan(0);
    }
  });
});

describe("deskHoldTuningExportIntervalMsFromEnv", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("returns 0 when unset, empty, non-positive, or non-finite", () => {
    vi.stubEnv("NEXT_PUBLIC_DESK_HOLD_TUNING_EXPORT_MS", "");
    expect(deskHoldTuningExportIntervalMsFromEnv()).toBe(0);
    vi.stubEnv("NEXT_PUBLIC_DESK_HOLD_TUNING_EXPORT_MS", "0");
    expect(deskHoldTuningExportIntervalMsFromEnv()).toBe(0);
    vi.stubEnv("NEXT_PUBLIC_DESK_HOLD_TUNING_EXPORT_MS", "-10");
    expect(deskHoldTuningExportIntervalMsFromEnv()).toBe(0);
    vi.stubEnv("NEXT_PUBLIC_DESK_HOLD_TUNING_EXPORT_MS", "nan");
    expect(deskHoldTuningExportIntervalMsFromEnv()).toBe(0);
  });

  it("returns floored positive milliseconds", () => {
    vi.stubEnv("NEXT_PUBLIC_DESK_HOLD_TUNING_EXPORT_MS", "45000.9");
    expect(deskHoldTuningExportIntervalMsFromEnv()).toBe(45000);
  });
});

describe("deskMaxSameDirNotionalFracFromEnv / deskMinExpectedMoveSafetyKFromEnv", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("defaults for same-dir frac and safety K", () => {
    expect(deskMaxSameDirNotionalFracFromEnv()).toBe(DESK_MAX_SAME_DIR_FRAC_OF_EQUITY_DEFAULT);
    expect(deskMinExpectedMoveSafetyKFromEnv()).toBe(DESK_MIN_EXPECTED_MOVE_SAFETY_K_DEFAULT);
  });

  it("parses valid overrides", () => {
    vi.stubEnv("NEXT_PUBLIC_DESK_MAX_SAME_DIR_FRAC_OF_EQUITY", "0.5");
    vi.stubEnv("NEXT_PUBLIC_DESK_MIN_EXPECTED_MOVE_SAFETY_K", "1.5");
    expect(deskMaxSameDirNotionalFracFromEnv()).toBe(0.5);
    expect(deskMinExpectedMoveSafetyKFromEnv()).toBe(1.5);
  });
});
