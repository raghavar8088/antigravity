import { afterEach, describe, expect, it, vi } from "vitest";
import { FUTURES_STRAT_DEFS } from "./futuresStrategies";
import {
  buildPaperDeskStrategies,
  defaultRegimesForCategory,
  deskEffectiveHoldMinutesAtOpen,
  DESK_REGIME_EXTRA_TOKENS_BY_STRAT_ID,
  DESK_REGIME_FALLBACK_ALLOW_ALL,
  deskHoldMinutesCategoryMul,
  deskHoldTuningExportIntervalMsFromEnv,
  deskMaxSameDirNotionalFracFromEnv,
  deskMinExpectedMoveSafetyKFromEnv,
  deskRegimeHistogramDevPersistEnabled,
  deskRegimeWatchIntervalMsFromEnv,
  deskRegimeWatchPollWindowFromEnv,
  DESK_MAX_SAME_DIR_FRAC_OF_EQUITY_DEFAULT,
  DESK_MIN_EXPECTED_MOVE_SAFETY_K_DEFAULT,
  DESK_REGIME_HISTOGRAM_LS_MAX_EVENTS,
  DESK_REGIME_HISTOGRAM_LS_WINDOW_MS,
  FAKE_DIVERSITY_STRAT_IDS,
  histogramRegimePolls,
  HOLD_MUL_AFTER_TP_WIDEN,
  mergeDeskRegimeExtras,
  appendPrunedDeskRegimePersistEvent,
  parseDeskRegimePersistLsPayload,
  pruneDeskRegimePersistEvents,
  regimeHistogramShares,
  serializeDeskRegimePersistLsPayload,
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

  it("attaches default regimes by category when defs omit regimes, plus desk extra tokens when mapped", () => {
    const raw = FUTURES_STRAT_DEFS.find((s) => s.id === 3)!;
    expect(raw.regimes).toBeUndefined();
    expect(DESK_REGIME_EXTRA_TOKENS_BY_STRAT_ID[3]?.length).toBeGreaterThan(0);
    const r = buildPaperDeskStrategies([raw], {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: true,
    });
    expect(r.strategies[0]!.regimes).toEqual(["chop", "trendLow", "trendHigh"]);
    expect(r.deskRegimeAnnotatedStratCount).toBe(1);
  });

  it("does not merge desk regime extras when strat id is not in override map", () => {
    const raw = FUTURES_STRAT_DEFS.find((s) => s.id === 5)!;
    expect(DESK_REGIME_EXTRA_TOKENS_BY_STRAT_ID[5]).toBeUndefined();
    const r = buildPaperDeskStrategies([raw], {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: true,
    });
    expect(r.strategies[0]!.regimes).toEqual(["trendLow", "trendHigh"]);
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

  it("does not merge desk regime extras onto explicit regimes (even when id is in override map)", () => {
    const raw = FUTURES_STRAT_DEFS.find((s) => s.id === 3)!;
    expect(DESK_REGIME_EXTRA_TOKENS_BY_STRAT_ID[3]).toBeDefined();
    const r = buildPaperDeskStrategies([{ ...raw, regimes: ["chop"] }], {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: true,
    });
    expect(r.strategies[0]!.regimes).toEqual(["chop"]);
    expect(r.deskRegimeAnnotatedStratCount).toBe(0);
  });

  it("empty regimes array on def is treated as missing → defaults + desk extras when mapped", () => {
    const raw = FUTURES_STRAT_DEFS.find((s) => s.id === 3)!;
    const r = buildPaperDeskStrategies([{ ...raw, regimes: [] }], {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: true,
    });
    expect(r.strategies[0]!.regimes).toEqual(["chop", "trendLow", "trendHigh"]);
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

describe("mergeDeskRegimeExtras / histogramRegimePolls", () => {
  it("merges missing tokens in canonical order", () => {
    expect(mergeDeskRegimeExtras(["chop", "trendLow"], ["trendHigh"])).toEqual(["chop", "trendLow", "trendHigh"]);
    expect(mergeDeskRegimeExtras(["trendLow", "trendHigh"], ["chop"])).toEqual(["chop", "trendLow", "trendHigh"]);
  });

  it("histogramRegimePolls + regimeHistogramShares", () => {
    const h = histogramRegimePolls(["trendHigh", "trendHigh", "chop"]);
    expect(h).toEqual({ chop: 1, trendLow: 0, trendHigh: 2 });
    const s = regimeHistogramShares(h);
    expect(s.chop).toBeCloseTo(1 / 3, 6);
    expect(s.trendHigh).toBeCloseTo(2 / 3, 6);
  });
});

describe("deskRegimeWatch env helpers", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("deskRegimeWatchIntervalMsFromEnv is 0 without analysis mode", () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("NEXT_PUBLIC_DESK_HOLD_TUNING_ANALYSIS_MODE", "");
    vi.stubEnv("NEXT_PUBLIC_DESK_REGIME_WATCH_MS", "5000");
    expect(deskRegimeWatchIntervalMsFromEnv()).toBe(0);
  });

  it("deskRegimeWatchIntervalMsFromEnv parses when analysis mode on", () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("NEXT_PUBLIC_DESK_HOLD_TUNING_ANALYSIS_MODE", "1");
    vi.stubEnv("NEXT_PUBLIC_DESK_REGIME_WATCH_MS", "12000.4");
    expect(deskRegimeWatchIntervalMsFromEnv()).toBe(12000);
  });

  it("deskRegimeWatchPollWindowFromEnv clamps when analysis on", () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("NEXT_PUBLIC_DESK_HOLD_TUNING_ANALYSIS_MODE", "1");
    vi.stubEnv("NEXT_PUBLIC_DESK_REGIME_WATCH_POLL_WINDOW", "5");
    expect(deskRegimeWatchPollWindowFromEnv()).toBe(20);
    vi.stubEnv("NEXT_PUBLIC_DESK_REGIME_WATCH_POLL_WINDOW", "9000");
    expect(deskRegimeWatchPollWindowFromEnv()).toBe(2000);
  });
});

describe("deskRegimeHistogramDevPersistEnabled", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("is false outside development", () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("NEXT_PUBLIC_DESK_REGIME_HISTOGRAM_LS_PERSIST", "1");
    expect(deskRegimeHistogramDevPersistEnabled()).toBe(false);
  });

  it("is true in development when flag is 1", () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("NEXT_PUBLIC_DESK_REGIME_HISTOGRAM_LS_PERSIST", "1");
    expect(deskRegimeHistogramDevPersistEnabled()).toBe(true);
  });
});

describe("deskRegime persist LS helpers", () => {
  it("pruneDeskRegimePersistEvents drops older than window", () => {
    const now = 10_000;
    const ev = [
      { t: 1000, tag: "chop" as const },
      { t: 7000, tag: "trendHigh" as const },
    ];
    expect(pruneDeskRegimePersistEvents(ev, now, 4000)).toEqual([{ t: 7000, tag: "trendHigh" }]);
  });

  it("appendPrunedDeskRegimePersistEvent tail-caps past maxEvents", () => {
    const base = Array.from({ length: DESK_REGIME_HISTOGRAM_LS_MAX_EVENTS }, (_, i) => ({
      t: i,
      tag: "chop" as const,
    }));
    const next = appendPrunedDeskRegimePersistEvent(
      base,
      { t: DESK_REGIME_HISTOGRAM_LS_MAX_EVENTS + 10, tag: "trendLow" },
      DESK_REGIME_HISTOGRAM_LS_MAX_EVENTS + 10,
      DESK_REGIME_HISTOGRAM_LS_WINDOW_MS,
      DESK_REGIME_HISTOGRAM_LS_MAX_EVENTS,
    );
    expect(next.length).toBe(DESK_REGIME_HISTOGRAM_LS_MAX_EVENTS);
    expect(next[next.length - 1]?.tag).toBe("trendLow");
  });

  it("parse + serialize round-trip", () => {
    const now = 500_000;
    const events = [
      { t: 400_000, tag: "chop" as const },
      { t: 450_000, tag: "trendHigh" as const },
    ];
    const json = serializeDeskRegimePersistLsPayload(events);
    const parsed = parseDeskRegimePersistLsPayload(JSON.parse(json) as unknown, now);
    expect(parsed).toEqual(events);
  });

  it("parseDeskRegimePersistLsPayload rejects bad payloads", () => {
    expect(parseDeskRegimePersistLsPayload(null, 0)).toEqual([]);
    expect(parseDeskRegimePersistLsPayload({ v: 2, events: [] }, 0)).toEqual([]);
    expect(
      parseDeskRegimePersistLsPayload({ v: 1, events: [{ t: "x", tag: "chop" }, { t: 1, tag: "bogus" }] }, 10_000),
    ).toEqual([]);
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
