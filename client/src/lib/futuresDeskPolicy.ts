/**
 * BTC futures **paper** desk policy (Phase 2–3).
 * Phase 3: IDs 79–110 are “fake diversity” placeholders without dedicated `evalMinuteSignal` wiring
 * for their branded categories — excluded by default. Set `NEXT_PUBLIC_DESK_ENABLE_FAKE_DIV_STRATS=1` to include.
 *
 * Hold tuning: `NEXT_PUBLIC_DESK_HOLD_TUNING_ANALYSIS_MODE=1` in **development** registers
 * `window.__deskHoldTuningDump()` (JSON per strat × deskTpWidened × exitReason). Use before changing
 * `DESK_HOLD_MINUTES_MUL_BY_CATEGORY` or raw defs.
 * Optional: `NEXT_PUBLIC_DESK_HOLD_TUNING_EXPORT_MS` (positive ms) throttles auto `console.info` of that
 * payload while analysis mode is on; unset = no auto-log.
 * `NEXT_PUBLIC_DESK_MAX_SAME_DIR_FRAC_OF_EQUITY` — max fraction of equity for sum of notionals per side (default 0.35).
 * `NEXT_PUBLIC_DESK_MIN_EXPECTED_MOVE_SAFETY_K` — ATR$ vs fee hurdle multiplier (default 1).
 */

import type { FuturesStratDef, RegimeTag } from "./futuresStrategies";
import type { FuturesStrategyProfile } from "./futuresSessionMetrics";
import { paperWidenTpToMinSlRatio } from "./futuresPaperMath";

/** When `scalp_aggro_v1` + desk-widened TP, multiply strat base `holdMinutes` before profile `holdTimeMul` in `paperResolveHardExit`. */
export const HOLD_MUL_AFTER_TP_WIDEN = 1.18;

export function deskEffectiveHoldMinutesAtOpen(
  baseHoldMinutes: number,
  profile: FuturesStrategyProfile,
  deskTpWidened: boolean | undefined,
): { holdMinutes: number; profileAdjusted: boolean } {
  if (
    profile === "scalp_aggro_v1" &&
    deskTpWidened === true &&
    Number.isFinite(baseHoldMinutes) &&
    baseHoldMinutes > 0
  ) {
    return { holdMinutes: baseHoldMinutes * HOLD_MUL_AFTER_TP_WIDEN, profileAdjusted: true };
  }
  return { holdMinutes: baseHoldMinutes, profileAdjusted: false };
}

/** Inclusive range 79…110 — see Phase 3 note in module header. */
export const FAKE_DIVERSITY_STRAT_IDS: readonly number[] = Array.from({ length: 32 }, (_, i) => 79 + i);

/** Hard cap on widened TP% — beyond this, strategy is excluded rather than distorting intent. */
export const DESK_WIDEN_TP_MAX_PCT = 4.8;

export function deskFakeDiversityEnabledViaEnv(): boolean {
  return process.env.NEXT_PUBLIC_DESK_ENABLE_FAKE_DIV_STRATS === "1";
}

export function deskMinTpSlRatioFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_MIN_TP_SL_RATIO;
  if (raw === undefined || raw === "") return 2;
  const n = Number(raw);
  return Number.isFinite(n) && n >= 1 ? n : 2;
}

/**
 * Dev-only: register `window.__deskHoldTuningDump()` JSON export (per strat × deskTpWidened × exitReason).
 * Requires `next dev` (NODE_ENV=development) so production bundles never expose the hook.
 */
export function deskHoldTuningAnalysisModeEnabled(): boolean {
  return process.env.NODE_ENV === "development" && process.env.NEXT_PUBLIC_DESK_HOLD_TUNING_ANALYSIS_MODE === "1";
}

/**
 * Parses `NEXT_PUBLIC_DESK_HOLD_TUNING_EXPORT_MS`. Returns **0** when unset/invalid (no auto-log).
 * Only used when `deskHoldTuningAnalysisModeEnabled()` is true (see hook).
 */
export function deskHoldTuningExportIntervalMsFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_HOLD_TUNING_EXPORT_MS;
  if (raw === undefined || raw === "") return 0;
  const n = Number(raw);
  if (!Number.isFinite(n) || n <= 0) return 0;
  return Math.floor(n);
}

/** Default cap on same-direction **$ notional** vs equity (`equity * frac`). */
export const DESK_MAX_SAME_DIR_FRAC_OF_EQUITY_DEFAULT = 0.35;

export function deskMaxSameDirNotionalFracFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_MAX_SAME_DIR_FRAC_OF_EQUITY;
  if (raw === undefined || raw === "") return DESK_MAX_SAME_DIR_FRAC_OF_EQUITY_DEFAULT;
  const n = Number(raw);
  return Number.isFinite(n) && n > 0 && n <= 1 ? n : DESK_MAX_SAME_DIR_FRAC_OF_EQUITY_DEFAULT;
}

/** Default \(K\) in `paperMinExpectedMoveVsFees` (ATR$ move ≥ K × round-trip fees). */
export const DESK_MIN_EXPECTED_MOVE_SAFETY_K_DEFAULT = 1;

export function deskMinExpectedMoveSafetyKFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_MIN_EXPECTED_MOVE_SAFETY_K;
  if (raw === undefined || raw === "") return DESK_MIN_EXPECTED_MOVE_SAFETY_K_DEFAULT;
  const n = Number(raw);
  return Number.isFinite(n) && n > 0 ? n : DESK_MIN_EXPECTED_MOVE_SAFETY_K_DEFAULT;
}

/**
 * Optional desk-side **base** `holdMinutes` multiplier by strategy `category` (applied in `buildPaperDeskStrategies`).
 * Goal: give a bit more clock where TIME exits bleed before TP; keep bumps modest (≤~15%) to limit fee churn.
 * Re-verify with `__deskHoldTuningDump()` + `rankWorstTimeOffenders` before raising further.
 */
export const DESK_HOLD_MINUTES_MUL_BY_CATEGORY: Readonly<Record<string, number>> = {
  MeanRev: 1.15,
  Stoch: 1.12,
  RSI: 1.12,
  Momentum: 1.1,
};

const HOLD_MUL_CAP = 1.25;

export function deskHoldMinutesCategoryMul(category: string): number {
  const raw = DESK_HOLD_MINUTES_MUL_BY_CATEGORY[category];
  if (typeof raw !== "number" || !Number.isFinite(raw) || raw < 1) return 1;
  return Math.min(raw, HOLD_MUL_CAP);
}

/**
 * v1 desk default: **allow all three** regimes when category is unknown (future defs / fake-diversity names).
 * Confluence uses the same — multi-timeframe stack can fire in any bucket until telemetry says otherwise.
 */
export const DESK_REGIME_FALLBACK_ALLOW_ALL: readonly RegimeTag[] = ["chop", "trendLow", "trendHigh"];

/** Mean-reversion / range / oscillator dip — prefer chop + mild trend drift, exclude pure trendHigh. */
const DESK_REGIME_MR: readonly RegimeTag[] = ["chop", "trendLow"];

/** Directional / breakout / flow — prefer trend buckets, exclude pure chop. */
const DESK_REGIME_IMPULSE: readonly RegimeTag[] = ["trendLow", "trendHigh"];

/**
 * Conservative **category → default `regimes[]`** for the paper desk when defs omit `regimes`.
 * Explicit `strat.regimes` in `FUTURES_STRAT_DEFS` always wins (see `buildPaperDeskStrategies`).
 *
 * Risk if too strict: spikes `deskSkippedByRegime` and lowers trades/hr — tune after a live session snapshot.
 */
export const DESK_DEFAULT_REGIMES_BY_CATEGORY: Readonly<Record<string, readonly RegimeTag[]>> = {
  MeanRev: DESK_REGIME_MR,
  BB: DESK_REGIME_MR,
  RSI: DESK_REGIME_MR,
  Stoch: DESK_REGIME_MR,
  VWAP: DESK_REGIME_MR,
  "Williams MR": DESK_REGIME_MR,
  "CCI MR": DESK_REGIME_MR,
  "Keltner MR": DESK_REGIME_MR,
  "Donchian MR": DESK_REGIME_MR,
  "VWAP MR": DESK_REGIME_MR,
  "RSI Div": DESK_REGIME_MR,
  "MACD Div": DESK_REGIME_MR,
  "Stoch Div": DESK_REGIME_MR,
  Confluence: DESK_REGIME_FALLBACK_ALLOW_ALL,
  Momentum: DESK_REGIME_IMPULSE,
  Trend: DESK_REGIME_IMPULSE,
  "Donchian Trend": DESK_REGIME_IMPULSE,
  "ADX Trend": DESK_REGIME_IMPULSE,
  Vol: DESK_REGIME_IMPULSE,
  MACD: DESK_REGIME_IMPULSE,
  OBV: DESK_REGIME_IMPULSE,
  Ribbon: DESK_REGIME_IMPULSE,
  Squeeze: DESK_REGIME_IMPULSE,
  "Williams Trend": DESK_REGIME_IMPULSE,
  "CCI Trend": DESK_REGIME_IMPULSE,
  "Keltner Trend": DESK_REGIME_IMPULSE,
  "ROC Trend": DESK_REGIME_IMPULSE,
};

export function defaultRegimesForCategory(category: string): RegimeTag[] {
  const row = DESK_DEFAULT_REGIMES_BY_CATEGORY[category];
  return [...(row ?? DESK_REGIME_FALLBACK_ALLOW_ALL)];
}

export type DeskStrategyBuildResult = {
  strategies: FuturesStratDef[];
  tpWidenedStratIds: readonly number[];
  lowRrSkippedStratIds: readonly number[];
  fakeDiversityFilteredCount: number;
  /** Built strats where `regimes` was filled from `defaultRegimesForCategory` (defs had no non-empty `regimes`). */
  deskRegimeAnnotatedStratCount: number;
};

/**
 * Apply allow-list, fake-diversity filter, and TP widen vs `minTpSlRatio` (or skip if widen exceeds cap).
 */
export function buildPaperDeskStrategies(
  raw: readonly FuturesStratDef[],
  opts: {
    strategyIdAllowlist: Set<number> | null;
    minTpSlRatio: number;
    allowFakeDiversity: boolean;
  },
): DeskStrategyBuildResult {
  const tpWidenedStratIds: number[] = [];
  const lowRrSkippedStratIds: number[] = [];
  let fakeDiversityFilteredCount = 0;

  const afterAllow = raw.filter((d) => {
    if (opts.strategyIdAllowlist && !opts.strategyIdAllowlist.has(d.id)) return false;
    if (!opts.allowFakeDiversity && FAKE_DIVERSITY_STRAT_IDS.includes(d.id)) {
      fakeDiversityFilteredCount += 1;
      return false;
    }
    return true;
  });

  const strategies: FuturesStratDef[] = [];
  let deskRegimeAnnotatedStratCount = 0;
  for (const d of afterAllow) {
    const w = paperWidenTpToMinSlRatio(d.slPct, d.tpPct, opts.minTpSlRatio, DESK_WIDEN_TP_MAX_PCT);
    if (!w.included) {
      lowRrSkippedStratIds.push(d.id);
      continue;
    }
    const widened = Math.abs(w.tpPct - d.tpPct) > 1e-9;
    if (widened) tpWidenedStratIds.push(d.id);
    const holdMul = deskHoldMinutesCategoryMul(d.category);
    const holdMinutes = Math.round(d.holdMinutes * holdMul * 100) / 100;
    const hadExplicitRegimes = Array.isArray(d.regimes) && d.regimes.length > 0;
    const regimes = hadExplicitRegimes ? [...d.regimes!] : defaultRegimesForCategory(d.category);
    if (!hadExplicitRegimes) deskRegimeAnnotatedStratCount += 1;
    strategies.push({
      ...d,
      tpPct: w.tpPct,
      deskTpWidened: widened,
      holdMinutes,
      regimes,
    });
  }

  return {
    strategies,
    tpWidenedStratIds,
    lowRrSkippedStratIds,
    fakeDiversityFilteredCount,
    deskRegimeAnnotatedStratCount,
  };
}
