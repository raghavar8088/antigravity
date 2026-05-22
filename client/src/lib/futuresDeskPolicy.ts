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
 * Regime watch (same dev gate): `NEXT_PUBLIC_DESK_REGIME_WATCH_MS` + optional
 * `NEXT_PUBLIC_DESK_REGIME_WATCH_POLL_WINDOW` — rolling `classifyRegimeTag` histogram on
 * `PRIMARY_QUOTE_SYMBOL` polls; auto-log + `__deskHoldTuningDump()` includes `primaryRegimePollWatch`.
 * Crash-resilient (dev only): `NEXT_PUBLIC_DESK_REGIME_HISTOGRAM_LS_PERSIST=1` in **development** persists a sliding
 * 24h `{t,tag}[]` for the primary symbol in `localStorage` (throttled writes); dump may include `primaryRegimeHistogram24h`.
 * `NEXT_PUBLIC_DESK_MAX_SAME_DIR_FRAC_OF_EQUITY` — max fraction of equity for sum of notionals per side (default 0.35).
 * `NEXT_PUBLIC_DESK_MIN_EXPECTED_MOVE_SAFETY_K` — ATR$ vs fee hurdle multiplier (default 1).
 * `NEXT_PUBLIC_DESK_SLIPPAGE_BPS` — adverse entry/exit slippage in bps (default 0, clamp 0–50).
 * `NEXT_PUBLIC_DESK_AUTO_DISABLE_STRATS=1` — auto-disable losing strats from Supabase rolling stats.
 * `NEXT_PUBLIC_DESK_KILL_MIN_TRADES`, `NEXT_PUBLIC_DESK_KILL_MAX_EXPECTANCY_USD`, `NEXT_PUBLIC_DESK_KILL_MAX_SUM_NET_USD`, `NEXT_PUBLIC_DESK_KILL_WINDOW_DAYS`.
 * `NEXT_PUBLIC_DESK_VOL_SIZED_NOTIONAL=1` — size opens from SL risk % of equity (`NEXT_PUBLIC_DESK_RISK_PCT_OF_EQUITY`, default 0.01).
 * `NEXT_PUBLIC_DESK_ENTRY_REPLACE_WEAKEST=1` — at max positions, close lowest `entryPriorityScore` open slot (mark `TIME`) when a strictly higher-priority candidate arrives; default is queue-only (skip).
 * `NEXT_PUBLIC_DESK_MAX_LAST_MARK_SPREAD_PCT` — max |last−mark|/mark % before entry skip (default 0.05, clamp 0–1).
 * `NEXT_PUBLIC_DESK_MAX_OPEN_PER_CATEGORY` — max concurrent opens per strategy `category` (default 3, clamp 1–12).
 * `NEXT_PUBLIC_DESK_ENTRY_UTC_START` / `NEXT_PUBLIC_DESK_ENTRY_UTC_END` — UTC entry window (0–23 / 0–24); default 0 and 24 = always open; supports wrap (e.g. 22→6).
 * BTC Future Trading module-only:
 * `NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD` — base signal threshold for that route only (default 26, clamp 22–28).
 * `NEXT_PUBLIC_BTC_FT_RELAX_CONFIRM=1` — dev-only relaxed confirmation for BTC FT route diagnostics.
 * `NEXT_PUBLIC_DESK_FORCE_PROBE_OPEN=1` — dev-only one tiny paper open after first ready BTC FT poll.
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
// CORE 20 winners include IDs 91, 92, 95, 96 which fall in the old 79–110 fake-diversity range.
// Since all non-CORE strategies have been removed, there are no fake-diversity strategies left.
export const FAKE_DIVERSITY_STRAT_IDS: readonly number[] = [];

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
 * Dev-only: persist primary-symbol regime samples for a sliding 24h window in `localStorage`.
 * Requires `next dev` so production never writes these keys.
 */
export function deskRegimeHistogramDevPersistEnabled(): boolean {
  return (
    process.env.NODE_ENV === "development" && process.env.NEXT_PUBLIC_DESK_REGIME_HISTOGRAM_LS_PERSIST === "1"
  );
}

/** Sliding window for persisted regime events (ms). */
export const DESK_REGIME_HISTOGRAM_LS_WINDOW_MS = 86_400_000;

/** Safety cap on stored `{t,tag}` rows (high-frequency polls). */
export const DESK_REGIME_HISTOGRAM_LS_MAX_EVENTS = 30_000;

export type DeskRegimePersistEvent = { t: number; tag: RegimeTag };

export function pruneDeskRegimePersistEvents(
  events: readonly DeskRegimePersistEvent[],
  nowMs: number,
  windowMs: number = DESK_REGIME_HISTOGRAM_LS_WINDOW_MS,
): DeskRegimePersistEvent[] {
  const cutoff = nowMs - windowMs;
  return events.filter((e) => Number.isFinite(e.t) && e.t >= cutoff);
}

/** Append one sample, prune by `windowMs`, then tail-cap by `maxEvents`. */
export function appendPrunedDeskRegimePersistEvent(
  events: readonly DeskRegimePersistEvent[],
  e: DeskRegimePersistEvent,
  nowMs: number,
  windowMs: number = DESK_REGIME_HISTOGRAM_LS_WINDOW_MS,
  maxEvents: number = DESK_REGIME_HISTOGRAM_LS_MAX_EVENTS,
): DeskRegimePersistEvent[] {
  const pruned = pruneDeskRegimePersistEvents(events, nowMs, windowMs);
  const next = [...pruned, e];
  if (next.length <= maxEvents) return next;
  return next.slice(-maxEvents);
}

const DESK_REGIME_LS_PAYLOAD_VERSION = 1 as const;

export function parseDeskRegimePersistLsPayload(raw: unknown, nowMs: number): DeskRegimePersistEvent[] {
  if (!raw || typeof raw !== "object") return [];
  const o = raw as { v?: unknown; events?: unknown };
  if (o.v !== DESK_REGIME_LS_PAYLOAD_VERSION || !Array.isArray(o.events)) return [];
  const out: DeskRegimePersistEvent[] = [];
  for (const row of o.events) {
    if (!row || typeof row !== "object") continue;
    const t = (row as { t?: unknown }).t;
    const tag = (row as { tag?: unknown }).tag;
    if (typeof t !== "number" || !Number.isFinite(t)) continue;
    if (tag === "chop" || tag === "trendLow" || tag === "trendHigh") out.push({ t, tag });
  }
  return pruneDeskRegimePersistEvents(out, nowMs);
}

export function serializeDeskRegimePersistLsPayload(events: readonly DeskRegimePersistEvent[]): string {
  return JSON.stringify({ v: DESK_REGIME_LS_PAYLOAD_VERSION, events: [...events] });
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

/**
 * Parses `NEXT_PUBLIC_DESK_REGIME_WATCH_MS`. Returns **0** unless hold-tuning analysis mode is on
 * (dev + `NEXT_PUBLIC_DESK_HOLD_TUNING_ANALYSIS_MODE=1`).
 */
export function deskRegimeWatchIntervalMsFromEnv(): number {
  if (!deskHoldTuningAnalysisModeEnabled()) return 0;
  const raw = process.env.NEXT_PUBLIC_DESK_REGIME_WATCH_MS;
  if (raw === undefined || raw === "") return 0;
  const n = Number(raw);
  if (!Number.isFinite(n) || n <= 0) return 0;
  return Math.floor(n);
}

const DESK_REGIME_WATCH_POLL_WINDOW_DEFAULT = 200;
const DESK_REGIME_WATCH_POLL_WINDOW_MIN = 20;
const DESK_REGIME_WATCH_POLL_WINDOW_MAX = 2000;

/** Max primary-symbol regime samples kept for rolling histogram (analysis mode only). */
export function deskRegimeWatchPollWindowFromEnv(): number {
  if (!deskHoldTuningAnalysisModeEnabled()) return DESK_REGIME_WATCH_POLL_WINDOW_DEFAULT;
  const raw = process.env.NEXT_PUBLIC_DESK_REGIME_WATCH_POLL_WINDOW;
  if (raw === undefined || raw === "") return DESK_REGIME_WATCH_POLL_WINDOW_DEFAULT;
  const n = Number(raw);
  if (!Number.isFinite(n)) return DESK_REGIME_WATCH_POLL_WINDOW_DEFAULT;
  return Math.min(
    DESK_REGIME_WATCH_POLL_WINDOW_MAX,
    Math.max(DESK_REGIME_WATCH_POLL_WINDOW_MIN, Math.floor(n)),
  );
}

export const REGIME_TAGS_ORDER: readonly RegimeTag[] = ["chop", "trendLow", "trendHigh"];

export function emptyRegimeHistogram(): Record<RegimeTag, number> {
  return { chop: 0, trendLow: 0, trendHigh: 0 };
}

export function histogramRegimePolls(polls: readonly RegimeTag[]): Record<RegimeTag, number> {
  const h = emptyRegimeHistogram();
  for (const r of polls) {
    if (r === "chop" || r === "trendLow" || r === "trendHigh") h[r] += 1;
  }
  return h;
}

export function regimeHistogramShares(h: Record<RegimeTag, number>): Record<RegimeTag, number> {
  const t = h.chop + h.trendLow + h.trendHigh;
  if (t <= 0) return { chop: 0, trendLow: 0, trendHigh: 0 };
  return {
    chop: h.chop / t,
    trendLow: h.trendLow / t,
    trendHigh: h.trendHigh / t,
  };
}

/**
 * Union of `base` and `extras`, stable order `REGIME_TAGS_ORDER` (extras only add missing tokens).
 */
export function mergeDeskRegimeExtras(
  base: readonly RegimeTag[],
  extras: readonly RegimeTag[],
): RegimeTag[] {
  const set = new Set<RegimeTag>([...base, ...extras]);
  return REGIME_TAGS_ORDER.filter((r) => set.has(r));
}

/**
 * Per-strategy **extra** regime tokens merged onto category defaults when defs omit `regimes`.
 * Narrow tuning: add missing buckets (e.g. `trendHigh`) for MR-style desks that starve vs live classifier mix.
 * Replace keys after `__deskHoldTuningDump` + session metrics; explicit `strat.regimes` on defs is never merged.
 */
export const DESK_REGIME_EXTRA_TOKENS_BY_STRAT_ID: Readonly<Partial<Record<number, readonly RegimeTag[]>>> = {
  3: ["trendHigh"],
  4: ["trendHigh"],
  7: ["trendHigh"],
  8: ["trendHigh"],
  9: ["trendHigh"],
  10: ["trendHigh"],
  19: ["trendHigh"],
  20: ["trendHigh"],
  27: ["trendHigh"],
  28: ["trendHigh"],
  31: ["trendHigh"],
  32: ["trendHigh"],
  47: ["trendHigh"],
  48: ["trendHigh"],
};

/** Default cap on same-direction **$ notional** vs equity (`equity * frac`). */
export const DESK_MAX_SAME_DIR_FRAC_OF_EQUITY_DEFAULT = 0.35;

export function deskMaxSameDirNotionalFracFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_MAX_SAME_DIR_FRAC_OF_EQUITY;
  if (raw === undefined || raw === "") return DESK_MAX_SAME_DIR_FRAC_OF_EQUITY_DEFAULT;
  const n = Number(raw);
  return Number.isFinite(n) && n > 0 && n <= 1 ? n : DESK_MAX_SAME_DIR_FRAC_OF_EQUITY_DEFAULT;
}

/**
 * Default \(K\) in `paperMinExpectedMoveVsFees` (ATR$ move ≥ K × round-trip fees).
 * Tightened 2026-05-21 from 1.0 → 1.1 to require a 10% safety margin over fees;
 * stops entries where a single-ATR move barely covers the round-trip cost.
 */
export const DESK_MIN_EXPECTED_MOVE_SAFETY_K_DEFAULT = 1.1;

export function deskMinExpectedMoveSafetyKFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_MIN_EXPECTED_MOVE_SAFETY_K;
  if (raw === undefined || raw === "") return DESK_MIN_EXPECTED_MOVE_SAFETY_K_DEFAULT;
  const n = Number(raw);
  return Number.isFinite(n) && n > 0 ? n : DESK_MIN_EXPECTED_MOVE_SAFETY_K_DEFAULT;
}

/**
 * Maker-first paper fee model.
 *
 * When ON (default 2026-05-21): paper math uses `paperRoundTripBlendedFees`
 * (~0.088% RT with Delta's 0.02% maker + 0.10% taker rates and 70% maker fill
 * probability), modeling a post-only entry path with taker fallback.
 *
 * When OFF: paper math falls back to `paperRoundTripTakerFees` (all-taker,
 * 0.20% RT) — useful for honest worst-case comparison.
 *
 * IMPORTANT: this is the *model* only. The live trading boundary must actually
 * implement post-only orders with a taker-fallback timeout for the math to
 * hold. The model intentionally stays conservative against pure-maker assumptions.
 *
 * Env to disable: `NEXT_PUBLIC_DESK_PAPER_MAKER_FILL_MODEL=0`.
 */
export function deskPaperMakerFillModelEnabled(): boolean {
  return process.env.NEXT_PUBLIC_DESK_PAPER_MAKER_FILL_MODEL !== "0";
}

/** Per-leg maker fee assumed by the maker-first paper model. Override via env. */
export function deskPaperMakerFeePctFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_PAPER_MAKER_FEE_PCT;
  if (raw === undefined || raw.trim() === "") return 0.0005;
  const n = Number(raw);
  if (!Number.isFinite(n) || n < 0) return 0.0005;
  return Math.min(0.005, n);
}

/** Probability a limit entry fills as maker before timeout (0.7 conservative). */
export function deskPaperMakerFillProbabilityFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_PAPER_MAKER_FILL_PROBABILITY;
  if (raw === undefined || raw.trim() === "") return 0.7;
  const n = Number(raw);
  if (!Number.isFinite(n)) return 0.7;
  return Math.min(1, Math.max(0, n));
}

/**
 * Default adverse slippage (bps) on paper entry/exit fills.
 * Tightened 2026-05-21 from 0 → 5 for honest paper math: a 0-bps assumption
 * makes paper results look better than live would. 5 bps × 2 legs = 10 bps RT slip
 * on top of the 20 bps RT taker fee floor. Override via env if simulating maker fills.
 */
export const DESK_SLIPPAGE_BPS_DEFAULT = 5;

/** Parses `NEXT_PUBLIC_DESK_SLIPPAGE_BPS`; clamps to [0, 50]. */
export function deskSlippageBpsFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_SLIPPAGE_BPS;
  if (raw === undefined || raw === "") return DESK_SLIPPAGE_BPS_DEFAULT;
  const n = Number(raw);
  if (!Number.isFinite(n) || n <= 0) return DESK_SLIPPAGE_BPS_DEFAULT;
  return Math.min(50, Math.max(0, n));
}

/**
 * Min projected USD net required to fire PROFIT_LOCK. Defaults to $0.05.
 * Stops the desk from booking a "lock" that nets a loss after fees + slippage.
 * Env: `NEXT_PUBLIC_DESK_PROFIT_LOCK_MIN_NET_USD`. Clamped to [0, 5].
 */
export function deskProfitLockMinNetUsdFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_PROFIT_LOCK_MIN_NET_USD;
  if (raw === undefined || raw.trim() === "") return 0.05;
  const n = Number(raw);
  if (!Number.isFinite(n) || n < 0) return 0.05;
  return Math.min(5, n);
}

/**
 * Minimum progress-toward-TP before PROFIT_LOCK may fire. Default 0.55.
 * Env: `NEXT_PUBLIC_DESK_PROFIT_LOCK_MIN_PROGRESS`. Clamped to [0.3, 0.9].
 */
export function deskProfitLockMinProgressFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_PROFIT_LOCK_MIN_PROGRESS;
  if (raw === undefined || raw.trim() === "") return 0.55;
  const n = Number(raw);
  if (!Number.isFinite(n) || n <= 0) return 0.55;
  return Math.min(0.9, Math.max(0.3, n));
}

/**
 * Feature flag: scale per-strategy notional by observed edge
 * (Kelly fraction + Sharpe vs cohort). Default OFF — opt-in once you have
 * a few weeks of trade history. With insufficient sample the multiplier
 * is 1.0 anyway, so enabling early is safe but useless.
 *
 * Env: `NEXT_PUBLIC_DESK_ALLOCATION_BY_EDGE` (=1 to enable).
 */
export function deskAllocationByEdgeEnabled(): boolean {
  return process.env.NEXT_PUBLIC_DESK_ALLOCATION_BY_EDGE === "1";
}

/**
 * Feature flag: scale TP% by ATR volatility (high vol → wider TP, low → tighter).
 * Default ON — safe and immediately useful since ATR is computed at every bar.
 * Env: `NEXT_PUBLIC_DESK_ADAPTIVE_TP` (=0 to disable explicitly).
 */
export function deskAdaptiveTpEnabled(): boolean {
  const raw = process.env.NEXT_PUBLIC_DESK_ADAPTIVE_TP;
  if (raw === undefined || raw.trim() === "") return true;
  return raw === "1";
}

/**
 * Feature flag: per-strategy time-of-day gating. A strategy is blocked from
 * opening at UTC hour H when historical stats show win rate < 35% at that hour.
 * Default OFF — opt-in once you have ≥50 trades per strategy.
 * Env: `NEXT_PUBLIC_DESK_SESSION_GATE` (=1 to enable).
 */
export function deskSessionGateEnabled(): boolean {
  return process.env.NEXT_PUBLIC_DESK_SESSION_GATE === "1";
}

/**
 * Max concurrent open positions per side (LONG/SHORT). Default 3 — concentrated
 * book. Tightened 2026-05-21 from 6 → 3 for profitability hardening: fewer trades,
 * higher conviction per trade, lower aggregate fee drag. Env to revert: =6.
 * Env: `NEXT_PUBLIC_DESK_MAX_OPEN_PER_SIDE`. Clamped to [1, 12].
 */
export function deskMaxOpenPerSideFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_MAX_OPEN_PER_SIDE;
  if (raw === undefined || raw.trim() === "") return 3;
  const n = Math.floor(Number(raw));
  if (!Number.isFinite(n) || n <= 0) return 3;
  return Math.min(12, Math.max(1, n));
}

/**
 * Max concurrent open positions per template family (e.g. BTCFT_VWAP_V0_LONG).
 * Default 1 — strict dedup. Tightened 2026-05-21 from 2 → 1: same template
 * firing 2× on the same signal pays 2× round-trip fees with correlated risk.
 * Env to revert: =2. Env: `NEXT_PUBLIC_DESK_MAX_OPEN_PER_TEMPLATE`. Clamped to [1, 6].
 */
export function deskMaxOpenPerTemplateFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_MAX_OPEN_PER_TEMPLATE;
  if (raw === undefined || raw.trim() === "") return 1;
  const n = Math.floor(Number(raw));
  if (!Number.isFinite(n) || n <= 0) return 1;
  return Math.min(6, Math.max(1, n));
}

/**
 * Initial paper balance in USD. Default $1,000 (legacy). Bump to $1,000,000 for
 * research / firehose modes so 1% sizing = $10K notional per trade.
 * Env: `NEXT_PUBLIC_DESK_INITIAL_BALANCE_USD`. Clamped to [100, 100_000_000].
 */
export function deskInitialBalanceUsd(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_INITIAL_BALANCE_USD;
  if (raw === undefined || raw.trim() === "") return 1_000;
  const n = Number(raw);
  if (!Number.isFinite(n) || n <= 0) return 1_000;
  return Math.min(100_000_000, Math.max(100, n));
}

/**
 * Fixed % of equity to use as notional per trade. When set (> 0) this OVERRIDES
 * the vol-based sizing path so every trade uses the same notional regardless of
 * volatility — desirable for research / cohort comparison.
 *
 * Default 0 (disabled → fall back to vol-based or 10% sizing).
 * Env: `NEXT_PUBLIC_DESK_FIXED_NOTIONAL_PCT_OF_EQUITY`. Clamped to [0, 5].
 * 1.0 means 1% of equity (e.g. $10K notional on $1M paper balance).
 */
export function deskFixedNotionalPctOfEquity(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_FIXED_NOTIONAL_PCT_OF_EQUITY;
  if (raw === undefined || raw.trim() === "") return 0;
  const n = Number(raw);
  if (!Number.isFinite(n) || n < 0) return 0;
  return Math.min(5, n);
}

/**
 * Firehose mode — maximizes trade frequency for fastest per-strategy verdict
 * discovery. NOT for production live trading. When enabled:
 *   - MAX_OPEN_POSITIONS effective cap raised (see deskMaxOpenPositionsEffective)
 *   - Per-side / per-template caps loosened
 *   - Daily strat cap effectively unlimited
 *   - Cooldown / min-move gates relaxed
 *   - UI shows a prominent warning banner
 *
 * Honest invariants kept: fees, slippage, liq cross, funding accrual — all on.
 * Env: `NEXT_PUBLIC_BTC_FT_FIREHOSE` (=1 to enable).
 */
export function deskFirehoseModeEnabled(): boolean {
  return process.env.NEXT_PUBLIC_BTC_FT_FIREHOSE === "1";
}

/**
 * Effective MAX_OPEN_POSITIONS — 6 by default; 60 in firehose mode.
 * Tightened 2026-05-21 from 12 → 6 for profitability hardening (concentration
 * over shotgun). Firehose mode unchanged at 60 since research mode wants volume
 * for verdict discovery, not edge optimization. Env to revert: =12.
 * Env override: `NEXT_PUBLIC_DESK_MAX_OPEN_POSITIONS` (clamp [1, 100]).
 */
export function deskMaxOpenPositionsEffective(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_MAX_OPEN_POSITIONS;
  if (raw && raw.trim() !== "") {
    const n = Math.floor(Number(raw));
    if (Number.isFinite(n) && n > 0) return Math.min(100, Math.max(1, n));
  }
  return deskFirehoseModeEnabled() ? 60 : 6;
}

/** Default max last vs mark spread (%) for new entries. */
export const DESK_MAX_LAST_MARK_SPREAD_PCT_DEFAULT = 0.05;

/** Parses `NEXT_PUBLIC_DESK_MAX_LAST_MARK_SPREAD_PCT`; clamps to [0, 1] (%). */
export function deskMaxLastMarkSpreadPctFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_MAX_LAST_MARK_SPREAD_PCT;
  if (raw === undefined || raw === "") return DESK_MAX_LAST_MARK_SPREAD_PCT_DEFAULT;
  const n = Number(raw);
  if (!Number.isFinite(n) || n < 0) return DESK_MAX_LAST_MARK_SPREAD_PCT_DEFAULT;
  return Math.min(1, Math.max(0, n));
}

/**
 * Default max concurrent positions per strategy category.
 * Tightened 2026-05-21 from 3 → 2 for profitability hardening: avoids paying
 * for 3 correlated entries on the same category-level signal.
 */
export const DESK_MAX_OPEN_PER_CATEGORY_DEFAULT = 2;

/** Parses `NEXT_PUBLIC_DESK_MAX_OPEN_PER_CATEGORY`; clamps to [1, 12]. */
export function deskMaxOpenPerCategoryFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_MAX_OPEN_PER_CATEGORY;
  if (raw === undefined || raw === "") return DESK_MAX_OPEN_PER_CATEGORY_DEFAULT;
  const n = Number(raw);
  if (!Number.isFinite(n) || n < 1) return DESK_MAX_OPEN_PER_CATEGORY_DEFAULT;
  return Math.min(12, Math.max(1, Math.floor(n)));
}

export type PositionForCategoryCount = {
  strategyId: number;
  category?: string;
};

/** Open position count by `category` (resolves via `category` on row or `categoryByStrategyId`). */
export function countOpenByCategory(
  positions: ReadonlyArray<PositionForCategoryCount>,
  categoryByStrategyId?: ReadonlyMap<number, string>,
): Map<string, number> {
  const counts = new Map<string, number>();
  for (const p of positions) {
    const cat =
      (p.category?.trim() || categoryByStrategyId?.get(p.strategyId)?.trim() || "unknown");
    counts.set(cat, (counts.get(cat) ?? 0) + 1);
  }
  return counts;
}

/** True when another open in `category` is allowed (`categoryOpenCount` is current opens in that category). */
export function canOpenCategory(category: string, categoryOpenCount: number, max: number): boolean {
  if (!Number.isFinite(max) || max <= 0) return true;
  void category;
  return categoryOpenCount < max;
}

export type DeskEntryUtcSession = {
  startHour: number;
  endHour: number;
};

function parseDeskUtcHourEnv(raw: string | undefined, fallback: number): number {
  if (raw === undefined || raw === "") return fallback;
  const n = Number(raw);
  if (!Number.isFinite(n)) return fallback;
  return Math.floor(n);
}

/** UTC entry window from env; default `{ startHour: 0, endHour: 24 }` = always open. */
export function deskEntryUtcSessionFromEnv(): DeskEntryUtcSession {
  const startRaw = parseDeskUtcHourEnv(process.env.NEXT_PUBLIC_DESK_ENTRY_UTC_START, 0);
  const endRaw = parseDeskUtcHourEnv(process.env.NEXT_PUBLIC_DESK_ENTRY_UTC_END, 24);
  const startHour = Math.min(23, Math.max(0, startRaw));
  const endHour = Math.min(24, Math.max(0, endRaw));
  return { startHour, endHour };
}

export function isEntryUtcSessionAlwaysOpen(session: DeskEntryUtcSession): boolean {
  return session.startHour === 0 && session.endHour >= 24;
}

/**
 * Half-open UTC hour window `[startHour, endHour)`; wrap when `startHour > endHour` (e.g. 22→6 → 22–23, 0–5).
 */
export function isUtcHourInSession(hour: number, startHour: number, endHour: number): boolean {
  if (startHour === 0 && endHour >= 24) return true;
  const h = ((Math.floor(hour) % 24) + 24) % 24;
  const start = Math.min(23, Math.max(0, startHour));
  const end = Math.min(24, Math.max(0, endHour));
  if (start === end) return false;
  if (start < end) return h >= start && h < end;
  return h >= start || h < end;
}

export function formatDeskEntryUtcSessionLabel(session: DeskEntryUtcSession): string {
  if (isEntryUtcSessionAlwaysOpen(session)) return "Entries UTC 24h";
  if (session.startHour < session.endHour) {
    return `Entries UTC ${session.startHour}–${session.endHour}`;
  }
  return `Entries UTC ${session.startHour}–${session.endHour}`;
}

/**
 * Auto-disable losing strategies from Supabase rolling stats.
 * Flipped 2026-05-21 from opt-in to default-ON for profitability hardening:
 * fee-bleeding strategies should retire fast, not linger. Env to disable: =0.
 */
export function deskAutoDisableStratsEnabled(): boolean {
  return process.env.NEXT_PUBLIC_DESK_AUTO_DISABLE_STRATS !== "0";
}

/**
 * Minimum trades before kill-switch can fire. Tightened 2026-05-21 from 5 → 8
 * for better statistical confidence before retiring a strategy.
 */
export function deskKillMinTradesFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_KILL_MIN_TRADES;
  if (raw === undefined || raw === "") return 8;
  const n = Number(raw);
  return Number.isFinite(n) && n >= 1 ? Math.floor(n) : 8;
}

/**
 * Max per-trade expectancy USD before kill. Tightened 2026-05-21 from -$0.05 → -$0.02
 * for profitability hardening: kill earlier on persistent fee-bleeders.
 */
export function deskKillMaxExpectancyUsdFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_KILL_MAX_EXPECTANCY_USD;
  if (raw === undefined || raw === "") return -0.02;
  const n = Number(raw);
  return Number.isFinite(n) ? n : -0.02;
}

/**
 * Max cumulative net USD before kill. Tightened 2026-05-21 from -$1 → -$0.50
 * for profitability hardening: shorter rope.
 */
export function deskKillMaxSumNetUsdFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_KILL_MAX_SUM_NET_USD;
  if (raw === undefined || raw === "") return -0.5;
  const n = Number(raw);
  return Number.isFinite(n) ? n : -0.5;
}

/**
 * Rolling kill window (days). Tightened 2026-05-21 from 14 → 7 for faster
 * turnover — losing strategies retired within a week of the bleed.
 */
export function deskKillWindowDaysFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_KILL_WINDOW_DAYS;
  if (raw === undefined || raw === "") return 7;
  const n = Number(raw);
  if (!Number.isFinite(n)) return 7;
  return Math.min(90, Math.max(1, Math.floor(n)));
}

export function deskVolSizedNotionalEnabledFromEnv(): boolean {
  // Default ON (P1.3.1): set NEXT_PUBLIC_DESK_VOL_SIZED_NOTIONAL=0 to disable.
  return process.env.NEXT_PUBLIC_DESK_VOL_SIZED_NOTIONAL !== "0";
}

export const DESK_RISK_PCT_OF_EQUITY_DEFAULT = 0.01;

/** Parses `NEXT_PUBLIC_DESK_RISK_PCT_OF_EQUITY`; clamps to [0.002, 0.05]. */
export function deskRiskPctOfEquityFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_DESK_RISK_PCT_OF_EQUITY;
  if (raw === undefined || raw === "") return DESK_RISK_PCT_OF_EQUITY_DEFAULT;
  const n = Number(raw);
  if (!Number.isFinite(n) || n <= 0) return DESK_RISK_PCT_OF_EQUITY_DEFAULT;
  return Math.min(0.05, Math.max(0.002, n));
}

/** Default: queue-only at max slots (skip lower-priority candidates). */
export function deskEntryReplaceWeakestFromEnv(): boolean {
  return process.env.NEXT_PUBLIC_DESK_ENTRY_REPLACE_WEAKEST === "1";
}

/**
 * Module-only signal threshold for BTC Future Trading route.
 * Env: `NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD`, default 28, clamp [18, 32].
 * Tightened 2026-05-21 production baseline from 26 → 28 for profitability
 * hardening (higher conviction per fire, fewer marginal entries paying fees).
 * Research mode lowers via env (commit ec0e18a set 20 for research probes).
 * Lower values (e.g. 24 or 22) allow more candidates on chop — only change via env, never globally.
 */
export function btcFtSignalThresholdFromEnv(fallback = 28): number {
  const raw = process.env.NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD;
  if (raw === undefined || raw === "") return fallback;
  const n = Number(raw);
  if (!Number.isFinite(n)) return fallback;
  // Clamp 18–32: dev-friendly (18) to conservative (32); 26 is prod baseline.
  return Math.min(32, Math.max(18, Math.round(n)));
}

/**
 * Alias used by engine to read the BTC FT module-only threshold.
 * Named separately so call-sites in the hook stay explicit about which module they serve.
 */
export function deskBtcFtSignalThresholdFromEnv(): number {
  return btcFtSignalThresholdFromEnv(28);
}

/** Paper desk only — relaxes HTF/confluence confirm for BTC FT module when env is set. */
export function btcFtRelaxConfirmEnabledFromEnv(): boolean {
  return process.env.NEXT_PUBLIC_BTC_FT_RELAX_CONFIRM === "1";
}

/**
 * Compute a per-strategy adaptive threshold (P1.1.2).
 *
 * Adjusts the global base threshold ±4 based on the strategy's rolling win-rate:
 * - win-rate ≥ 55%  → lower threshold by 4 (let this edge run more freely)
 * - win-rate 45–55% → keep base (neutral)
 * - win-rate 35–45% → raise by 2 (higher conviction required)
 * - win-rate < 35%  → raise by 4 (near-dormant; only fire on very clear signals)
 *
 * Returns `null` when `minTrades` is not yet met (caller should use global threshold).
 */
export function computeAdaptiveThreshold(
  baseThreshold: number,
  winRate: number | null,
  trades: number,
  minTrades = 8,
): number | null {
  if (winRate === null || trades < minTrades) return null;
  if (winRate >= 0.55) return Math.max(18, baseThreshold - 4);
  if (winRate >= 0.45) return baseThreshold;
  if (winRate >= 0.35) return Math.min(32, baseThreshold + 2);
  return Math.min(32, baseThreshold + 4);
}

/** Show entry funnel panel on BTC Future Trading without global DESK_ENTRY_DEBUG. */
export function btcFtEntryDebugEnabledFromEnv(): boolean {
  return (
    process.env.NEXT_PUBLIC_BTC_FT_ENTRY_DEBUG === "1" ||
    process.env.NEXT_PUBLIC_DESK_ENTRY_DEBUG === "1"
  );
}

/** After ~45m, lower threshold for strats with few trades (paper discovery on chop). */
export function btcFtPaperEnsureTradesFromEnv(): boolean {
  return process.env.NEXT_PUBLIC_BTC_FT_PAPER_ENSURE_TRADES !== "0";
}

/** Module-only min-move K multiplier (default 1). Use 0.85–0.9 on chop with large rosters. */
export function btcFtMinMoveKMulFromEnv(fallback = 1): number {
  const raw = process.env.NEXT_PUBLIC_BTC_FT_MIN_MOVE_K_MUL;
  if (raw === undefined || raw === "") return fallback;
  const n = Number(raw);
  if (!Number.isFinite(n) || n <= 0) return fallback;
  return Math.min(2, Math.max(0.5, n));
}

export function deskForceProbeOpenEnabledFromEnv(): boolean {
  return process.env.NODE_ENV === "development" && process.env.NEXT_PUBLIC_DESK_FORCE_PROBE_OPEN === "1";
}

// ========== Entry priority (P1-F) ==========

/**
 * Heuristic entry rank when more signals pass than `MAX_OPEN_POSITIONS` slots.
 * Higher = fill slot first. Not ML — uses live signal score + desk policy bonuses.
 *
 * Components: `signalScore` (dominant), +3 regime match, +2 desk-widened TP, +0.5×min(tp/sl,4) for RR.
 */
export type PaperEntryPriorityInput = {
  signalScore: number;
  stratId: number;
  category: string;
  regimeMatch: boolean;
  deskTpWidened?: boolean;
  slPct: number;
  tpPct: number;
};

export function paperEntryPriorityScore(input: PaperEntryPriorityInput): number {
  const sl = Number.isFinite(input.slPct) && input.slPct > 0 ? input.slPct : 0;
  const tp = Number.isFinite(input.tpPct) && input.tpPct > 0 ? input.tpPct : 0;
  const tpSlRatio = sl > 0 ? tp / sl : 0;
  let score = Number.isFinite(input.signalScore) ? input.signalScore : 0;
  if (input.regimeMatch) score += 3;
  if (input.deskTpWidened === true) score += 2;
  score += Math.min(4, tpSlRatio * 0.5);
  return score;
}

export type PaperEntryPriorityCandidate<T> = {
  priority: number;
  payload: T;
};

export type EntryPriorityDispatchResult<T> = {
  toOpen: T[];
  skippedLowPriority: number;
  replaceWeakestCount: number;
};

/**
 * Slot policy per poll tick:
 * - **Queue-only** (`replaceWeakest=false`): open top-N by priority while slots remain; skip the rest.
 * - **Replace-weakest** (`replaceWeakest=true`): when full, allow at most one swap if best candidate strictly beats weakest incumbent priority.
 */
export function dispatchEntryPriorityCandidates<T>(
  candidates: PaperEntryPriorityCandidate<T>[],
  openSlots: number,
  options: { replaceWeakest: boolean; weakestIncumbentPriority: number | null },
): EntryPriorityDispatchResult<T> {
  const sorted = [...candidates].sort((a, b) => b.priority - a.priority);
  const toOpen: T[] = [];
  let slots = Math.max(0, openSlots);
  let skippedLowPriority = 0;
  let replaceWeakestCount = 0;
  let weakest = options.weakestIncumbentPriority;

  for (const c of sorted) {
    if (slots > 0) {
      toOpen.push(c.payload);
      slots -= 1;
      continue;
    }
    if (
      options.replaceWeakest &&
      weakest !== null &&
      Number.isFinite(weakest) &&
      c.priority > weakest
    ) {
      toOpen.push(c.payload);
      replaceWeakestCount += 1;
      weakest = c.priority;
      continue;
    }
    skippedLowPriority += 1;
  }

  return { toOpen, skippedLowPriority, replaceWeakestCount };
}

export function weakestEntryPriorityIndex(
  incumbents: ReadonlyArray<{ entryPriorityScore?: number }>,
): number {
  let idx = -1;
  let min = Infinity;
  for (let i = 0; i < incumbents.length; i++) {
    const p = incumbents[i]?.entryPriorityScore;
    const v = Number.isFinite(p) ? (p as number) : 0;
    if (v < min) {
      min = v;
      idx = i;
    }
  }
  return idx;
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

/**
 * Categories whose default-regimes have `chop` pruned when chop-disable is on.
 * MR / oscillator desks chronically underperform in chop after fees: a 0.4–0.5%
 * mean-reversion target is too tight to beat the 0.3% RT fee+slip floor.
 * Premium-tier and explicit per-strat regimes are NOT affected.
 */
export const DESK_CHOP_DISABLED_MR_CATEGORIES: ReadonlySet<string> = new Set([
  "MeanRev",
  "BB",
  "RSI",
  "Stoch",
  "VWAP",
  "Williams MR",
  "CCI MR",
  "Keltner MR",
  "Donchian MR",
  "VWAP MR",
  "RSI Div",
  "MACD Div",
  "Stoch Div",
]);

/**
 * Disable `chop` regime for MR categories listed in `DESK_CHOP_DISABLED_MR_CATEGORIES`.
 * Default ON: chop + 0.2% taker + 5 bps slip is the highest-loss-rate state for these desks.
 * Env to revert: `NEXT_PUBLIC_DESK_DISABLE_CHOP_FOR_MR=0`.
 */
export function deskDisableChopForMrEnabled(): boolean {
  return process.env.NEXT_PUBLIC_DESK_DISABLE_CHOP_FOR_MR !== "0";
}

export function defaultRegimesForCategory(category: string): RegimeTag[] {
  const row = DESK_DEFAULT_REGIMES_BY_CATEGORY[category];
  const base = [...(row ?? DESK_REGIME_FALLBACK_ALLOW_ALL)];
  if (deskDisableChopForMrEnabled() && DESK_CHOP_DISABLED_MR_CATEGORIES.has(category)) {
    return base.filter((r) => r !== "chop");
  }
  return base;
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
    let regimes = hadExplicitRegimes ? [...d.regimes!] : defaultRegimesForCategory(d.category);
    const extraTok = DESK_REGIME_EXTRA_TOKENS_BY_STRAT_ID[d.id];
    if (!hadExplicitRegimes && extraTok && extraTok.length > 0) {
      regimes = mergeDeskRegimeExtras(regimes, [...extraTok]);
    }
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

// ============================================================
// WINNERS_ONLY + RESEARCH_MODE production gating (2026 blueprint)
// ============================================================

/**
 * Research mode gate.
 *
 * When OFF (default): only strategies with a verified positive expectancy
 * (from MongoDB) are eligible for the live execution path — WINNERS_ONLY mode.
 *
 * When ON (`NEXT_PUBLIC_FUTURES_RESEARCH_MODE=1`): the full 120+ strategy pool
 * is available, including unverified hypotheses. Intended for paper-only
 * research sessions; must never be set in production-adjacent environments.
 *
 * Env: `NEXT_PUBLIC_FUTURES_RESEARCH_MODE` (=1 to unlock full pool).
 */
export function deskResearchModeEnabled(): boolean {
  return process.env.NEXT_PUBLIC_FUTURES_RESEARCH_MODE === "1";
}

/**
 * WINNERS_ONLY filter: given a set of strategy IDs with verified positive
 * expectancy (from the MongoDB analytics endpoint), filter `stratDefs` to
 * only include promoted winners.
 *
 * When `promotedIds` is empty (no data yet) or research mode is on, returns
 * `stratDefs` unchanged so the desk doesn't go dark on first boot.
 *
 * Call this BEFORE `buildPaperDeskStrategies` to apply the production gate.
 */
export function applyWinnersOnlyGate(
  stratDefs: readonly FuturesStratDef[],
  promotedIds: ReadonlySet<number>,
): FuturesStratDef[] {
  if (deskResearchModeEnabled()) return [...stratDefs];
  if (promotedIds.size === 0) return [...stratDefs];
  return stratDefs.filter((d) => promotedIds.has(d.id));
}

/**
 * Rolling expectancy block: returns `true` when the engine should be
 * prevented from opening new positions because rolling 100-trade expectancy
 * is ≤ 0 for the module.
 *
 * Requires research mode to be OFF (production-adjacent only).
 * `expectancy` should come from the `/api/paper-trades/analytics` endpoint
 * aggregated over the last 100+ closed trades for this module.
 *
 * `tradeCount` is used as a guard: below 100 trades the sample is too small
 * to block reliably, so the gate passes (returns `false`).
 */
export function deskRollingExpectancyBlocked(
  expectancy: number,
  tradeCount: number,
  minTrades = 100,
): boolean {
  if (deskResearchModeEnabled()) return false;
  if (!Number.isFinite(expectancy) || !Number.isFinite(tradeCount)) return false;
  if (tradeCount < minTrades) return false;
  return expectancy <= 0;
}

/**
 * Max open positions per templateFamily — hard cap of 1 for production mode.
 * In research mode this is relaxed to `deskMaxOpenPerTemplateFromEnv()`.
 *
 * This prevents correlated over-exposure when multiple strategy IDs share
 * the same underlying template logic.
 */
export function deskEffectiveMaxOpenPerFamily(): number {
  return deskResearchModeEnabled() ? deskMaxOpenPerTemplateFromEnv() : 1;
}
