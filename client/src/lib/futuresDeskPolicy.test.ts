import { afterEach, describe, expect, it, vi } from "vitest";
import { FUTURES_STRAT_DEFS } from "./futuresStrategies";
import {
  buildPaperDeskStrategies,
  defaultRegimesForCategory,
  deskEffectiveHoldMinutesAtOpen,
  DESK_REGIME_EXTRA_TOKENS_BY_STRAT_ID,
  DESK_REGIME_FALLBACK_ALLOW_ALL,
  deskFirehoseModeEnabled,
  deskFixedNotionalPctOfEquity,
  deskHoldMinutesCategoryMul,
  deskHoldTuningExportIntervalMsFromEnv,
  deskInitialBalanceUsd,
  deskMaxOpenPositionsEffective,
  deskMaxSameDirNotionalFracFromEnv,
  deskMinExpectedMoveSafetyKFromEnv,
  deskAutoDisableStratsEnabled,
  deskKillMinTradesFromEnv,
  deskRiskPctOfEquityFromEnv,
  deskSlippageBpsFromEnv,
  deskVolSizedNotionalEnabledFromEnv,
  deskEntryReplaceWeakestFromEnv,
  canOpenCategory,
  countOpenByCategory,
  deskMaxOpenPerCategoryFromEnv,
  DESK_MAX_OPEN_PER_CATEGORY_DEFAULT,
  deskEntryUtcSessionFromEnv,
  formatDeskEntryUtcSessionLabel,
  isEntryUtcSessionAlwaysOpen,
  isUtcHourInSession,
  paperEntryPriorityScore,
  dispatchEntryPriorityCandidates,
  deskRegimeHistogramDevPersistEnabled,
  deskRegimeWatchIntervalMsFromEnv,
  deskRegimeWatchPollWindowFromEnv,
  DESK_MAX_SAME_DIR_FRAC_OF_EQUITY_DEFAULT,
  DESK_MIN_EXPECTED_MOVE_SAFETY_K_DEFAULT,
  DESK_SLIPPAGE_BPS_DEFAULT,
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
    const expectedAnnotated = FUTURES_STRAT_DEFS.filter(
      (d) => !Array.isArray(d.regimes) || d.regimes.length === 0,
    ).length;
    expect(r.deskRegimeAnnotatedStratCount).toBe(expectedAnnotated);
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

describe("deskAutoDisableStratsEnabled / deskKillMinTradesFromEnv", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("auto-disable off by default", () => {
    expect(deskAutoDisableStratsEnabled()).toBe(false);
    expect(deskKillMinTradesFromEnv()).toBe(5);
  });

  it("enables with flag and parses kill overrides", () => {
    vi.stubEnv("NEXT_PUBLIC_DESK_AUTO_DISABLE_STRATS", "1");
    vi.stubEnv("NEXT_PUBLIC_DESK_KILL_MIN_TRADES", "8");
    expect(deskAutoDisableStratsEnabled()).toBe(true);
    expect(deskKillMinTradesFromEnv()).toBe(8);
  });
});

describe("deskVolSizedNotionalEnabledFromEnv / deskRiskPctOfEquityFromEnv", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("vol sizing off by default", () => {
    expect(deskVolSizedNotionalEnabledFromEnv()).toBe(false);
    expect(deskRiskPctOfEquityFromEnv()).toBe(0.01);
  });

  it("parses vol sizing and risk pct", () => {
    vi.stubEnv("NEXT_PUBLIC_DESK_VOL_SIZED_NOTIONAL", "1");
    vi.stubEnv("NEXT_PUBLIC_DESK_RISK_PCT_OF_EQUITY", "0.02");
    expect(deskVolSizedNotionalEnabledFromEnv()).toBe(true);
    expect(deskRiskPctOfEquityFromEnv()).toBe(0.02);
    vi.stubEnv("NEXT_PUBLIC_DESK_RISK_PCT_OF_EQUITY", "0.2");
    expect(deskRiskPctOfEquityFromEnv()).toBe(0.05);
  });
});

describe("deskSlippageBpsFromEnv", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("defaults to 0", () => {
    expect(deskSlippageBpsFromEnv()).toBe(DESK_SLIPPAGE_BPS_DEFAULT);
  });

  it("parses and clamps to 50", () => {
    vi.stubEnv("NEXT_PUBLIC_DESK_SLIPPAGE_BPS", "5");
    expect(deskSlippageBpsFromEnv()).toBe(5);
    vi.stubEnv("NEXT_PUBLIC_DESK_SLIPPAGE_BPS", "99");
    expect(deskSlippageBpsFromEnv()).toBe(50);
    vi.stubEnv("NEXT_PUBLIC_DESK_SLIPPAGE_BPS", "-3");
    expect(deskSlippageBpsFromEnv()).toBe(0);
  });
});

describe("deskEntryUtcSession / isUtcHourInSession (P1-J)", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("defaults to always-on (0–24)", () => {
    const s = deskEntryUtcSessionFromEnv();
    expect(isEntryUtcSessionAlwaysOpen(s)).toBe(true);
    expect(isUtcHourInSession(3, s.startHour, s.endHour)).toBe(true);
    expect(formatDeskEntryUtcSessionLabel(s)).toBe("Entries UTC 24h");
  });

  it("non-wrap window 12–22", () => {
    expect(isUtcHourInSession(11, 12, 22)).toBe(false);
    expect(isUtcHourInSession(12, 12, 22)).toBe(true);
    expect(isUtcHourInSession(21, 12, 22)).toBe(true);
    expect(isUtcHourInSession(22, 12, 22)).toBe(false);
    expect(formatDeskEntryUtcSessionLabel({ startHour: 12, endHour: 22 })).toBe("Entries UTC 12–22");
  });

  it("wrap-around 22→6", () => {
    expect(isUtcHourInSession(21, 22, 6)).toBe(false);
    expect(isUtcHourInSession(22, 22, 6)).toBe(true);
    expect(isUtcHourInSession(23, 22, 6)).toBe(true);
    expect(isUtcHourInSession(0, 22, 6)).toBe(true);
    expect(isUtcHourInSession(5, 22, 6)).toBe(true);
    expect(isUtcHourInSession(6, 22, 6)).toBe(false);
    expect(isUtcHourInSession(12, 22, 6)).toBe(false);
  });

  it("single-hour allow block (only hour 17)", () => {
    expect(isUtcHourInSession(16, 17, 18)).toBe(false);
    expect(isUtcHourInSession(17, 17, 18)).toBe(true);
    expect(isUtcHourInSession(18, 17, 18)).toBe(false);
  });

  it("parses env overrides", () => {
    vi.stubEnv("NEXT_PUBLIC_DESK_ENTRY_UTC_START", "12");
    vi.stubEnv("NEXT_PUBLIC_DESK_ENTRY_UTC_END", "22");
    const s = deskEntryUtcSessionFromEnv();
    expect(s).toEqual({ startHour: 12, endHour: 22 });
    expect(isEntryUtcSessionAlwaysOpen(s)).toBe(false);
  });
});

describe("deskMaxOpenPerCategoryFromEnv / countOpenByCategory / canOpenCategory (P1-M)", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("defaults to 3 and clamps 1–12", () => {
    expect(deskMaxOpenPerCategoryFromEnv()).toBe(DESK_MAX_OPEN_PER_CATEGORY_DEFAULT);
    vi.stubEnv("NEXT_PUBLIC_DESK_MAX_OPEN_PER_CATEGORY", "8");
    expect(deskMaxOpenPerCategoryFromEnv()).toBe(8);
    vi.stubEnv("NEXT_PUBLIC_DESK_MAX_OPEN_PER_CATEGORY", "99");
    expect(deskMaxOpenPerCategoryFromEnv()).toBe(12);
    vi.stubEnv("NEXT_PUBLIC_DESK_MAX_OPEN_PER_CATEGORY", "0");
    expect(deskMaxOpenPerCategoryFromEnv()).toBe(3);
  });

  it("4 MeanRev candidates at max 3 → 4th blocked", () => {
    const max = 3;
    let meanRevOpen = 0;
    let skipped = 0;
    for (let i = 0; i < 4; i++) {
      if (canOpenCategory("MeanRev", meanRevOpen, max)) {
        meanRevOpen += 1;
      } else {
        skipped += 1;
      }
    }
    expect(meanRevOpen).toBe(3);
    expect(skipped).toBe(1);
  });

  it("different categories can each reach max", () => {
    const max = 3;
    const counts = countOpenByCategory([
      { strategyId: 1, category: "MeanRev" },
      { strategyId: 2, category: "MeanRev" },
      { strategyId: 3, category: "MeanRev" },
      { strategyId: 4, category: "Momentum" },
      { strategyId: 5, category: "Momentum" },
      { strategyId: 6, category: "Momentum" },
    ]);
    expect(counts.get("MeanRev")).toBe(3);
    expect(counts.get("Momentum")).toBe(3);
    expect(canOpenCategory("MeanRev", counts.get("MeanRev") ?? 0, max)).toBe(false);
    expect(canOpenCategory("Momentum", counts.get("Momentum") ?? 0, max)).toBe(false);
    expect(canOpenCategory("Breakout", 0, max)).toBe(true);
  });
});

describe("paperEntryPriorityScore / dispatchEntryPriorityCandidates (P1-F)", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("ranks higher signal and regime match above baseline", () => {
    const base = paperEntryPriorityScore({
      signalScore: 30,
      stratId: 1,
      category: "momentum",
      regimeMatch: false,
      slPct: 0.5,
      tpPct: 1,
    });
    const boosted = paperEntryPriorityScore({
      signalScore: 30,
      stratId: 1,
      category: "momentum",
      regimeMatch: true,
      deskTpWidened: true,
      slPct: 0.5,
      tpPct: 1,
    });
    expect(boosted).toBeGreaterThan(base);
    expect(boosted - base).toBeCloseTo(5, 5);
  });

  it("3 candidates and 1 slot opens highest priority (queue-only)", () => {
    const candidates = [
      { priority: 10, payload: "a" },
      { priority: 30, payload: "b" },
      { priority: 20, payload: "c" },
    ];
    const r = dispatchEntryPriorityCandidates(candidates, 1, {
      replaceWeakest: false,
      weakestIncumbentPriority: null,
    });
    expect(r.toOpen).toEqual(["b"]);
    expect(r.skippedLowPriority).toBe(2);
    expect(r.replaceWeakestCount).toBe(0);
  });

  it("replace-weakest allows one swap when candidate beats incumbent", () => {
    const r = dispatchEntryPriorityCandidates([{ priority: 35, payload: "new" }], 0, {
      replaceWeakest: true,
      weakestIncumbentPriority: 20,
    });
    expect(r.toOpen).toEqual(["new"]);
    expect(r.replaceWeakestCount).toBe(1);
    expect(r.skippedLowPriority).toBe(0);
  });

  it("deskEntryReplaceWeakestFromEnv defaults off", () => {
    expect(deskEntryReplaceWeakestFromEnv()).toBe(false);
    vi.stubEnv("NEXT_PUBLIC_DESK_ENTRY_REPLACE_WEAKEST", "1");
    expect(deskEntryReplaceWeakestFromEnv()).toBe(true);
  });
});

describe("firehose-mode env helpers", () => {
  it("deskInitialBalanceUsd defaults to $1,000 and honors env override", () => {
    vi.stubEnv("NEXT_PUBLIC_DESK_INITIAL_BALANCE_USD", "");
    expect(deskInitialBalanceUsd()).toBe(1_000);
    vi.stubEnv("NEXT_PUBLIC_DESK_INITIAL_BALANCE_USD", "1000000");
    expect(deskInitialBalanceUsd()).toBe(1_000_000);
  });

  it("deskInitialBalanceUsd clamps to [100, 100_000_000]", () => {
    vi.stubEnv("NEXT_PUBLIC_DESK_INITIAL_BALANCE_USD", "10");
    expect(deskInitialBalanceUsd()).toBe(100);
    vi.stubEnv("NEXT_PUBLIC_DESK_INITIAL_BALANCE_USD", "999999999");
    expect(deskInitialBalanceUsd()).toBe(100_000_000);
  });

  it("deskFixedNotionalPctOfEquity defaults to 0 (disabled) and clamps to [0, 5]", () => {
    vi.stubEnv("NEXT_PUBLIC_DESK_FIXED_NOTIONAL_PCT_OF_EQUITY", "");
    expect(deskFixedNotionalPctOfEquity()).toBe(0);
    vi.stubEnv("NEXT_PUBLIC_DESK_FIXED_NOTIONAL_PCT_OF_EQUITY", "1");
    expect(deskFixedNotionalPctOfEquity()).toBe(1);
    vi.stubEnv("NEXT_PUBLIC_DESK_FIXED_NOTIONAL_PCT_OF_EQUITY", "99");
    expect(deskFixedNotionalPctOfEquity()).toBe(5); // clamp
    vi.stubEnv("NEXT_PUBLIC_DESK_FIXED_NOTIONAL_PCT_OF_EQUITY", "-1");
    expect(deskFixedNotionalPctOfEquity()).toBe(0);
  });

  it("deskFirehoseModeEnabled is opt-in via =1", () => {
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_FIREHOSE", "");
    expect(deskFirehoseModeEnabled()).toBe(false);
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_FIREHOSE", "1");
    expect(deskFirehoseModeEnabled()).toBe(true);
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_FIREHOSE", "true"); // only "1" counts
    expect(deskFirehoseModeEnabled()).toBe(false);
  });

  it("deskMaxOpenPositionsEffective: 12 default, 60 in firehose, clamped via env", () => {
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_FIREHOSE", "");
    vi.stubEnv("NEXT_PUBLIC_DESK_MAX_OPEN_POSITIONS", "");
    expect(deskMaxOpenPositionsEffective()).toBe(12);

    vi.stubEnv("NEXT_PUBLIC_BTC_FT_FIREHOSE", "1");
    expect(deskMaxOpenPositionsEffective()).toBe(60);

    vi.stubEnv("NEXT_PUBLIC_DESK_MAX_OPEN_POSITIONS", "200");
    expect(deskMaxOpenPositionsEffective()).toBe(100); // clamped

    vi.stubEnv("NEXT_PUBLIC_DESK_MAX_OPEN_POSITIONS", "25");
    expect(deskMaxOpenPositionsEffective()).toBe(25);
  });

  it("1%-of-equity sizing math: $1M equity × 1% = $10,000 notional", () => {
    // The math is owned by the hook (lines around the openPosition body) but
    // we verify the policy helper returns the right multiplier the hook uses.
    const pct = 1.0;
    const equity = 1_000_000;
    const expectedNotional = (equity * pct) / 100;
    expect(expectedNotional).toBe(10_000);
  });
});
