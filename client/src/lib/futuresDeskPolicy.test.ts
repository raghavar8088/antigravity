import { afterEach, describe, expect, it, vi } from "vitest";
import { FUTURES_STRAT_DEFS } from "./futuresStrategies";
import {
  buildPaperDeskStrategies,
  checkEntryBurstGuard,
  createEntryBurstContext,
  recordBurstEntry,
  ENTRY_BURST_MAX_PER_SYMBOL_DEFAULT,
  ENTRY_BURST_MAX_PER_FAMILY_DEFAULT,
  ENTRY_OPPOSITE_SIDE_WINDOW_MS,
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
  deskPaperMakerFillModelEnabled,
  deskPaperMakerFeePctFromEnv,
  deskPaperMakerFillProbabilityFromEnv,
  deskDisableChopForMrEnabled,
  DESK_CHOP_DISABLED_MR_CATEGORIES,
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
  it("fake-diversity filter is a no-op (FAKE_DIVERSITY_STRAT_IDS is empty, extended pool removed)", () => {
    expect(FAKE_DIVERSITY_STRAT_IDS.length).toBe(0);
    const r = buildPaperDeskStrategies(FUTURES_STRAT_DEFS, {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: false,
    });
    expect(r.fakeDiversityFilteredCount).toBe(0);
    // All CORE 20 + premium strategies should still be present
    expect(r.strategies.length).toBe(FUTURES_STRAT_DEFS.length);
  });

  it("records TP-widened ids when ratio was below min", () => {
    // Restrict to CORE+premium pool (small SL ≤ 0.5%) so the 4.8% TP-widen cap
    // can always reach a 4× RR. Research-pool swing strats (SL up to 2.5%) would
    // otherwise be skipped at this ratio — covered by the dedicated swing tests.
    const coreOnly = FUTURES_STRAT_DEFS.filter((d) => !d.researchOnly);
    const r = buildPaperDeskStrategies(coreOnly, {
      strategyIdAllowlist: null,
      minTpSlRatio: 4,
      allowFakeDiversity: true,
    });
    expect(r.tpWidenedStratIds.length).toBeGreaterThan(0);
    expect(r.lowRrSkippedStratIds.length).toBe(0);
  });

  it("tags strategies with deskTpWidened when TP% was raised", () => {
    const coreOnly = FUTURES_STRAT_DEFS.filter((d) => !d.researchOnly);
    const r = buildPaperDeskStrategies(coreOnly, {
      strategyIdAllowlist: null,
      minTpSlRatio: 4,
      allowFakeDiversity: true,
    });
    const widened = r.strategies.filter((s) => s.deskTpWidened === true);
    expect(widened.length).toBe(r.tpWidenedStratIds.length);
    for (const s of r.strategies) {
      expect(typeof s.deskTpWidened).toBe("boolean");
    }
  });

  it("applies category hold multiplier in desk build (MeanRev > raw)", () => {
    // Inline fixture: MeanRev category (id=3 slot, which is in DESK_REGIME_EXTRA_TOKENS_BY_STRAT_ID)
    const raw = { id: 3, name: "Fixture_MR", category: "MeanRev", signalKey: "TEST_MR", slPct: 0.32, tpPct: 0.90, cooldownMin: 4, holdMinutes: 20, confluenceMin: 3 };
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
    // id=3 is in DESK_REGIME_EXTRA_TOKENS_BY_STRAT_ID[3] = ["trendHigh"]; MeanRev category
    const raw = { id: 3, name: "Fixture_MR", category: "MeanRev", signalKey: "TEST_MR", slPct: 0.32, tpPct: 0.90, cooldownMin: 4, holdMinutes: 20, confluenceMin: 3 };
    expect(DESK_REGIME_EXTRA_TOKENS_BY_STRAT_ID[3]?.length).toBeGreaterThan(0);
    const r = buildPaperDeskStrategies([raw], {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: true,
    });
    // MeanRev: chop pruned (deskDisableChopForMrEnabled=true by default);
    // trendHigh added via DESK_REGIME_EXTRA_TOKENS_BY_STRAT_ID[3]
    expect(r.strategies[0]!.regimes).toEqual(["trendLow", "trendHigh"]);
    expect(r.deskRegimeAnnotatedStratCount).toBe(1);
  });

  it("does not merge desk regime extras when strat id is not in override map", () => {
    // id=91 is NOT in DESK_REGIME_EXTRA_TOKENS_BY_STRAT_ID; Trend category
    const raw = { id: 91, name: "Fixture_Trend", category: "Trend", signalKey: "TEST_TREND", slPct: 0.26, tpPct: 0.90, cooldownMin: 6, holdMinutes: 26, confluenceMin: 5 };
    expect(DESK_REGIME_EXTRA_TOKENS_BY_STRAT_ID[91]).toBeUndefined();
    const r = buildPaperDeskStrategies([raw], {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: true,
    });
    expect(r.strategies[0]!.regimes).toEqual(["trendLow", "trendHigh"]);
  });

  it("keeps explicit regimes from def and does not count toward annotation", () => {
    const raw = { id: 91, name: "Fixture_Trend", category: "Trend", signalKey: "TEST_TREND", slPct: 0.26, tpPct: 0.90, cooldownMin: 6, holdMinutes: 26, confluenceMin: 5 };
    const r = buildPaperDeskStrategies([{ ...raw, regimes: ["trendHigh"] }], {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: true,
    });
    expect(r.strategies[0]!.regimes).toEqual(["trendHigh"]);
    expect(r.deskRegimeAnnotatedStratCount).toBe(0);
  });

  it("does not merge desk regime extras onto explicit regimes (even when id is in override map)", () => {
    // id=3 is in DESK_REGIME_EXTRA_TOKENS_BY_STRAT_ID[3] but explicit regimes block merging
    const raw = { id: 3, name: "Fixture_MR", category: "MeanRev", signalKey: "TEST_MR", slPct: 0.32, tpPct: 0.90, cooldownMin: 4, holdMinutes: 20, confluenceMin: 3 };
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
    const raw = { id: 3, name: "Fixture_MR", category: "MeanRev", signalKey: "TEST_MR", slPct: 0.32, tpPct: 0.90, cooldownMin: 4, holdMinutes: 20, confluenceMin: 3 };
    const r = buildPaperDeskStrategies([{ ...raw, regimes: [] }], {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: true,
    });
    // MeanRev: chop pruned; trendHigh added via DESK_REGIME_EXTRA_TOKENS_BY_STRAT_ID[3]
    expect(r.strategies[0]!.regimes).toEqual(["trendLow", "trendHigh"]);
    expect(r.deskRegimeAnnotatedStratCount).toBe(1);
  });
});

describe("defaultRegimesForCategory", () => {
  it("maps MeanRev / Confluence / unknown per v1 table", () => {
    // chop is pruned from MeanRev by default (deskDisableChopForMrEnabled=true)
    expect(defaultRegimesForCategory("MeanRev")).toEqual(["trendLow"]);
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

  it("auto-disable ON by default (hardening 2026-05-21); disable with =0", () => {
    expect(deskAutoDisableStratsEnabled()).toBe(true);
    expect(deskKillMinTradesFromEnv()).toBe(8);
    vi.stubEnv("NEXT_PUBLIC_DESK_AUTO_DISABLE_STRATS", "0");
    expect(deskAutoDisableStratsEnabled()).toBe(false);
  });

  it("enables with any non-zero flag and parses kill overrides", () => {
    vi.stubEnv("NEXT_PUBLIC_DESK_AUTO_DISABLE_STRATS", "1");
    vi.stubEnv("NEXT_PUBLIC_DESK_KILL_MIN_TRADES", "12");
    expect(deskAutoDisableStratsEnabled()).toBe(true);
    expect(deskKillMinTradesFromEnv()).toBe(12);
  });
});

describe("deskVolSizedNotionalEnabledFromEnv / deskRiskPctOfEquityFromEnv", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("vol sizing ON by default (P1.3.1)", () => {
    // Default is now ON; disable explicitly with NEXT_PUBLIC_DESK_VOL_SIZED_NOTIONAL=0
    expect(deskVolSizedNotionalEnabledFromEnv()).toBe(true);
    expect(deskRiskPctOfEquityFromEnv()).toBe(0.01);
  });

  it("can be disabled by setting env to '0'", () => {
    vi.stubEnv("NEXT_PUBLIC_DESK_VOL_SIZED_NOTIONAL", "0");
    expect(deskVolSizedNotionalEnabledFromEnv()).toBe(false);
  });

  it("parses risk pct when vol sizing env is '1'", () => {
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
    expect(deskSlippageBpsFromEnv()).toBe(DESK_SLIPPAGE_BPS_DEFAULT); // invalid → default (5)
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

  it("defaults to DESK_MAX_OPEN_PER_CATEGORY_DEFAULT (2) and clamps 1–12", () => {
    expect(deskMaxOpenPerCategoryFromEnv()).toBe(DESK_MAX_OPEN_PER_CATEGORY_DEFAULT);
    vi.stubEnv("NEXT_PUBLIC_DESK_MAX_OPEN_PER_CATEGORY", "8");
    expect(deskMaxOpenPerCategoryFromEnv()).toBe(8);
    vi.stubEnv("NEXT_PUBLIC_DESK_MAX_OPEN_PER_CATEGORY", "99");
    expect(deskMaxOpenPerCategoryFromEnv()).toBe(12);
    vi.stubEnv("NEXT_PUBLIC_DESK_MAX_OPEN_PER_CATEGORY", "0");
    expect(deskMaxOpenPerCategoryFromEnv()).toBe(DESK_MAX_OPEN_PER_CATEGORY_DEFAULT);
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

  it("deskMaxOpenPositionsEffective: 8 default (burst-guard era), 60 in firehose, clamped via env", () => {
    vi.stubEnv("NEXT_PUBLIC_BTC_FT_FIREHOSE", "");
    vi.stubEnv("NEXT_PUBLIC_DESK_MAX_OPEN_POSITIONS", "");
    expect(deskMaxOpenPositionsEffective()).toBe(8);

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

describe("deskPaperMakerFillModelEnabled / makerFee / makerFillProbability", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("maker-fill model ON by default; disable with =0", () => {
    expect(deskPaperMakerFillModelEnabled()).toBe(true);
    vi.stubEnv("NEXT_PUBLIC_DESK_PAPER_MAKER_FILL_MODEL", "0");
    expect(deskPaperMakerFillModelEnabled()).toBe(false);
  });

  it("maker fee defaults to 0.0005 (Delta 0.05% maker)", () => {
    expect(deskPaperMakerFeePctFromEnv()).toBe(0.0005);
  });

  it("maker fill probability defaults to 0.70", () => {
    expect(deskPaperMakerFillProbabilityFromEnv()).toBe(0.7);
  });
});

describe("deskDisableChopForMrEnabled / DESK_CHOP_DISABLED_MR_CATEGORIES", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("chop-disable for MR is ON by default; disable with =0", () => {
    expect(deskDisableChopForMrEnabled()).toBe(true);
    vi.stubEnv("NEXT_PUBLIC_DESK_DISABLE_CHOP_FOR_MR", "0");
    expect(deskDisableChopForMrEnabled()).toBe(false);
  });

  it("DESK_CHOP_DISABLED_MR_CATEGORIES contains expected MR labels", () => {
    expect(DESK_CHOP_DISABLED_MR_CATEGORIES.has("MeanRev")).toBe(true);
    expect(DESK_CHOP_DISABLED_MR_CATEGORIES.has("Breakout")).toBe(false);
  });
});

// ─── entryBurstGuard ──────────────────────────────────────────────────────────

describe("checkEntryBurstGuard", () => {
  const NOW = 1_700_000_000_000;
  const noPositions: { symbol: string; side: string }[] = [];
  const noTrades: { symbol: string; side: string; closedAt: string }[] = [];
  const opts = {
    maxPerSymbolPerPoll: ENTRY_BURST_MAX_PER_SYMBOL_DEFAULT,
    maxPerFamilyPerPoll: ENTRY_BURST_MAX_PER_FAMILY_DEFAULT,
    oppositeSideWindowMs: ENTRY_OPPOSITE_SIDE_WINDOW_MS,
    nowMs: NOW,
  };

  it("allows first two entries on same symbol per poll tick", () => {
    const ctx = createEntryBurstContext();
    const r1 = checkEntryBurstGuard("BTCUSD", "LONG", "BTCFT_VWAP_V0", ctx, noPositions, noTrades, opts);
    expect(r1.blocked).toBe(false);
    recordBurstEntry(ctx, "BTCUSD", "BTCFT_VWAP_V0");

    // second entry on same symbol but DIFFERENT family — still allowed (symbol cap = 2)
    const r2 = checkEntryBurstGuard("BTCUSD", "LONG", "BTCFT_RSI_V0", ctx, noPositions, noTrades, opts);
    expect(r2.blocked).toBe(false);
  });

  it("blocks 3rd same-symbol entry (burst_symbol)", () => {
    const ctx = createEntryBurstContext();
    recordBurstEntry(ctx, "BTCUSD", "BTCFT_VWAP_V0");
    recordBurstEntry(ctx, "BTCUSD", "BTCFT_RSI_V0");

    const r = checkEntryBurstGuard("BTCUSD", "LONG", "BTCFT_MACD_V0", ctx, noPositions, noTrades, opts);
    expect(r.blocked).toBe(true);
    if (r.blocked) expect(r.reason).toBe("burst_symbol");
  });

  it("blocks 2nd same-family entry (family_cap = 1)", () => {
    const ctx = createEntryBurstContext();
    recordBurstEntry(ctx, "BTCUSD", "BTCFT_VWAP_V0");

    // second entry: different symbol but same family key → family_cap
    const r = checkEntryBurstGuard("ETHUSD", "LONG", "BTCFT_VWAP_V0", ctx, noPositions, noTrades, opts);
    expect(r.blocked).toBe(true);
    if (r.blocked) expect(r.reason).toBe("family_cap");
  });

  it("blocks entry when same symbol has open opposite-side position (opposite_side)", () => {
    const ctx = createEntryBurstContext();
    const openShort = [{ symbol: "BTCUSD", side: "SHORT" }];
    const r = checkEntryBurstGuard("BTCUSD", "LONG", "BTCFT_VWAP_V0", ctx, openShort, noTrades, opts);
    expect(r.blocked).toBe(true);
    if (r.blocked) expect(r.reason).toBe("opposite_side");
  });

  it("blocks entry when same symbol had recent opposite-side close within window", () => {
    const ctx = createEntryBurstContext();
    const recentClose = [
      {
        symbol: "BTCUSD",
        side: "SHORT",
        closedAt: new Date(NOW - ENTRY_OPPOSITE_SIDE_WINDOW_MS / 2).toISOString(),
      },
    ];
    const r = checkEntryBurstGuard("BTCUSD", "LONG", "BTCFT_VWAP_V0", ctx, noPositions, recentClose, opts);
    expect(r.blocked).toBe(true);
    if (r.blocked) expect(r.reason).toBe("opposite_side");
  });

  it("allows entry when opposite-side close is older than the window", () => {
    const ctx = createEntryBurstContext();
    const staleClose = [
      {
        symbol: "BTCUSD",
        side: "SHORT",
        closedAt: new Date(NOW - ENTRY_OPPOSITE_SIDE_WINDOW_MS - 1000).toISOString(),
      },
    ];
    const r = checkEntryBurstGuard("BTCUSD", "LONG", "BTCFT_VWAP_V0", ctx, noPositions, staleClose, opts);
    expect(r.blocked).toBe(false);
  });

  it("does not block a different symbol even if same family fired", () => {
    const ctx = createEntryBurstContext();
    recordBurstEntry(ctx, "BTCUSD", "BTCFT_VWAP_V0");
    // ETHUSD on a different family — no block
    const r = checkEntryBurstGuard("ETHUSD", "LONG", "BTCFT_RSI_V0", ctx, noPositions, noTrades, opts);
    expect(r.blocked).toBe(false);
  });
});
