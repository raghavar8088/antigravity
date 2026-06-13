/**
 * futuresProfitMode.ts
 * Paper-only profitability hardening — stricter entries, fewer correlated opens.
 * Read-only policy helpers; never disables fees or fakes PnL.
 */

import type { MTFConfluenceResult } from "./futuresMTFConfluence";
import type { RotationReport } from "./futuresStrategyRotation";
import {
  deskAllocationByEdgeEnabled,
  deskProfitLockMinNetUsdFromEnv,
  deskProfitLockMinProgressFromEnv,
  deskSessionGateEnabled,
} from "./futuresDeskPolicy";

export interface ProfitModeConfig {
  enabled: boolean;
  /** Minimum signal-quality total (default 70; baseline gate uses 50). */
  minQualityScore: number;
  /** Minimum MTF confluence score (default 65; baseline gate uses 55). */
  minMtfConfluence: number;
  /** Added to adaptive signal threshold in chop (default +10). */
  chopThresholdBoost: number;
  /** ATR-vs-fees safety K in chop (default 3.0). */
  minExpectedMoveK: number;
  /** Max concurrent same-side positions in chop (default 1). */
  maxSameSideChop: number;
  /** Max concurrent same-side positions in trend regimes (default 2). */
  maxSameSideTrend: number;
  /** Cap total open positions when profit mode is on (default 6). */
  maxOpenPositions: number;
  /** Max closes per strategy per UTC day (default 4). */
  dailyStratCap: number;
  /** Require quality.isHighQuality when regime is chop. */
  requireHighQualityInChop: boolean;
  /** Block LONG when MTF bias is bearish / SHORT when bullish. */
  blockCounterTrend: boolean;
  /** Strict rotation mode; only SUSPENDED blocks entries, insufficient/missing/probation stay allowed. */
  onlyPromotedOrActive: boolean;
}

/** Exit overrides when profit mode is on (paper desk fee-bleed fix). */
export interface ProfitModeExitConfig {
  profitLockMinProgress: number;
  profitLockMinNetUsd: number;
  trailActivationProgress: number;
  breakevenTriggerProgress: number;
  /** Block PROFIT_LOCK when gross < this × round-trip fees. */
  minGrossMultipleOfFees: number;
  /** Omit paper quick-TP path (profitLockMinProgress 0.18 scalp exits). */
  disablePaperQuickTp: boolean;
}

export const PROFIT_MODE_ENV = "NEXT_PUBLIC_DESK_PROFIT_MODE";

const EXIT_DEFAULTS: ProfitModeExitConfig = {
  profitLockMinProgress: 0.55,
  profitLockMinNetUsd: 0.15,
  trailActivationProgress: 1.01,
  breakevenTriggerProgress: 0.98,
  minGrossMultipleOfFees: 2.0,
  disablePaperQuickTp: true,
};

const DEFAULTS: Omit<ProfitModeConfig, "enabled"> = {
  minQualityScore: 70,
  minMtfConfluence: 65,
  chopThresholdBoost: 10,
  minExpectedMoveK: 3.0,
  maxSameSideChop: 1,
  maxSameSideTrend: 2,
  maxOpenPositions: 6,
  dailyStratCap: 4,
  requireHighQualityInChop: true,
  blockCounterTrend: true,
  onlyPromotedOrActive: true,
};

function envEnabled(name: string): boolean {
  const v = (process.env[name] ?? "").trim().toLowerCase();
  return v === "1" || v === "true" || v === "yes";
}

function envNum(name: string, fallback: number, min: number, max: number): number {
  const raw = process.env[name];
  if (raw === undefined || raw.trim() === "") return fallback;
  const n = Number(raw);
  if (!Number.isFinite(n)) return fallback;
  return Math.min(max, Math.max(min, n));
}

export function profitModeFromEnv(): ProfitModeConfig {
  const enabled = envEnabled(PROFIT_MODE_ENV);
  return {
    enabled,
    minQualityScore: envNum("NEXT_PUBLIC_DESK_PROFIT_MIN_QUALITY", DEFAULTS.minQualityScore, 50, 95),
    minMtfConfluence: envNum("NEXT_PUBLIC_DESK_PROFIT_MIN_MTF", DEFAULTS.minMtfConfluence, 45, 90),
    chopThresholdBoost: envNum(
      "NEXT_PUBLIC_DESK_PROFIT_CHOP_THRESHOLD_BOOST",
      DEFAULTS.chopThresholdBoost,
      0,
      20,
    ),
    minExpectedMoveK: envNum(
      "NEXT_PUBLIC_DESK_PROFIT_MIN_MOVE_K",
      DEFAULTS.minExpectedMoveK,
      1,
      6,
    ),
    maxSameSideChop: envNum("NEXT_PUBLIC_DESK_PROFIT_MAX_SAME_SIDE_CHOP", DEFAULTS.maxSameSideChop, 1, 4),
    maxSameSideTrend: envNum("NEXT_PUBLIC_DESK_PROFIT_MAX_SAME_SIDE_TREND", DEFAULTS.maxSameSideTrend, 1, 6),
    maxOpenPositions: envNum("NEXT_PUBLIC_DESK_PROFIT_MAX_OPEN", DEFAULTS.maxOpenPositions, 1, 20),
    dailyStratCap: envNum("NEXT_PUBLIC_DESK_PROFIT_DAILY_STRAT_CAP", DEFAULTS.dailyStratCap, 1, 50),
    requireHighQualityInChop:
      process.env.NEXT_PUBLIC_DESK_PROFIT_REQUIRE_HQ_CHOP !== "0",
    blockCounterTrend: process.env.NEXT_PUBLIC_DESK_PROFIT_BLOCK_COUNTER_TREND !== "0",
    onlyPromotedOrActive: process.env.NEXT_PUBLIC_DESK_PROFIT_ROTATION_STRICT !== "0",
  };
}

export function isChopRegime(regime: string): boolean {
  return regime === "chop";
}

/**
 * Raises the entry bar in chop when profit mode is enabled.
 * Never returns below `base`.
 */
export function applyProfitModeThreshold(
  base: number,
  regime: string,
  config: ProfitModeConfig,
): number {
  if (!config.enabled) return base;
  if (!isChopRegime(regime)) return base;
  return base + config.chopThresholdBoost;
}

export function profitModeMaxSameSide(regime: string, config: ProfitModeConfig): number {
  if (!config.enabled) return config.maxSameSideTrend;
  return isChopRegime(regime) ? config.maxSameSideChop : config.maxSameSideTrend;
}

export function profitModeMaxOpenPositions(
  deskCap: number,
  config: ProfitModeConfig,
): number {
  if (!config.enabled) return deskCap;
  return Math.min(deskCap, config.maxOpenPositions);
}

export function profitModeDailyStratCap(config: ProfitModeConfig): number {
  return config.enabled ? config.dailyStratCap : 0;
}

/**
 * Effective min-move safety K for the ATR-vs-fees gate in `openPosition`.
 */
export function profitModeMinExpectedMoveK(
  regime: string,
  config: ProfitModeConfig,
  baseDeskK: number,
): number {
  if (!config.enabled) return baseDeskK;
  if (isChopRegime(regime)) return Math.max(baseDeskK, config.minExpectedMoveK);
  return baseDeskK;
}

export function profitModePassesQuality(
  qualityTotal: number,
  isHighQuality: boolean,
  regime: string,
  config: ProfitModeConfig,
): { pass: boolean; reason: string | null } {
  if (!config.enabled) return { pass: true, reason: null };
  if (qualityTotal < config.minQualityScore) {
    return {
      pass: false,
      reason: `PROFIT_MODE_QUALITY:${qualityTotal}<${config.minQualityScore}`,
    };
  }
  if (config.requireHighQualityInChop && isChopRegime(regime) && !isHighQuality) {
    return { pass: false, reason: "PROFIT_MODE_CHOP_NOT_HQ" };
  }
  return { pass: true, reason: null };
}

export function profitModeMtfMinScore(config: ProfitModeConfig, baseline = 55): number {
  return config.enabled ? config.minMtfConfluence : baseline;
}

export function profitModeCounterTrendSkip(
  side: "LONG" | "SHORT",
  mtf: MTFConfluenceResult,
  config: ProfitModeConfig,
): string | null {
  if (!config.enabled || !config.blockCounterTrend) return null;
  const bias = mtf.overallBias;
  if (bias === "BULLISH" && side === "SHORT") return "PROFIT_MODE_COUNTER_TREND:SHORT_IN_BULL";
  if (bias === "BEARISH" && side === "LONG") return "PROFIT_MODE_COUNTER_TREND:LONG_IN_BEAR";
  return null;
}

export function profitModeAllowsRotationStrategy(
  strategyId: number,
  report: RotationReport | null,
  config: ProfitModeConfig,
): boolean {
  // Disabled or non-strict → never block on rotation status
  if (!config.enabled || !config.onlyPromotedOrActive) return true;
  // No report yet → allow (diagnostics poll hasn't fired)
  if (!report) return true;
  // Fallback: if active+promoted is empty the rotation engine has no qualified strats.
  // Rather than zero out the desk, allow all non-suspended roster IDs so entries
  // can continue while the monitor collects enough data to form an opinion.
  if (report.active.length === 0 && report.promoted.length === 0) return true;
  // Find this strategy's entry across all scored buckets
  const entry = report.scores.find((s) => s.strategyId === strategyId);
  // Not in report at all → allow (insufficient data, can't judge)
  if (!entry) return true;
  // INSUFFICIENT → always allow (< 5 trades, rotation has no signal yet)
  if (entry.status === "INSUFFICIENT") return true;
  // Only SUSPENDED blocks execution. PROBATION remains tradable while the desk
  // collects more paper evidence; otherwise rotation can zero the whole desk.
  return entry.status !== "SUSPENDED";
}

/** Stricter paper exits when profit mode is on; null → use legacy paper overrides. */
export function profitModeExitConfig(config: ProfitModeConfig): ProfitModeExitConfig | null {
  if (!config.enabled) return null;
  return {
    profitLockMinProgress: envNum(
      "NEXT_PUBLIC_DESK_PROFIT_EXIT_LOCK_PROGRESS",
      EXIT_DEFAULTS.profitLockMinProgress,
      0.3,
      0.95,
    ),
    profitLockMinNetUsd: envNum(
      "NEXT_PUBLIC_DESK_PROFIT_EXIT_LOCK_MIN_NET",
      EXIT_DEFAULTS.profitLockMinNetUsd,
      0.05,
      2,
    ),
    trailActivationProgress: envNum(
      "NEXT_PUBLIC_DESK_PROFIT_EXIT_TRAIL_PROGRESS",
      EXIT_DEFAULTS.trailActivationProgress,
      0.3,
      1.05,
    ),
    breakevenTriggerProgress: envNum(
      "NEXT_PUBLIC_DESK_PROFIT_EXIT_BE_PROGRESS",
      EXIT_DEFAULTS.breakevenTriggerProgress,
      0.3,
      1.05,
    ),
    minGrossMultipleOfFees: envNum(
      "NEXT_PUBLIC_DESK_PROFIT_EXIT_MIN_GROSS_FEE_MUL",
      EXIT_DEFAULTS.minGrossMultipleOfFees,
      1,
      5,
    ),
    disablePaperQuickTp: process.env.NEXT_PUBLIC_DESK_PROFIT_DISABLE_QUICK_TP !== "0",
  };
}

/** Profit mode auto-enables session gate (v2 #4) unless explicitly disabled. */
export function profitModeSessionGateEnabled(config: ProfitModeConfig): boolean {
  if (!config.enabled) return deskSessionGateEnabled();
  return process.env.NEXT_PUBLIC_DESK_PROFIT_SESSION_GATE !== "0";
}

/** Profit mode auto-enables allocation-by-edge (v2 #1) unless explicitly disabled. */
export function profitModeAllocationByEdgeEnabled(config: ProfitModeConfig): boolean {
  if (!config.enabled) return deskAllocationByEdgeEnabled();
  return process.env.NEXT_PUBLIC_DESK_PROFIT_ALLOCATION_BY_EDGE !== "0";
}

const CHOP_ONLY_REGIME: readonly string[] = ["chop"];

const CHOP_NATIVE_KEYWORDS = [
  "mean_revert",
  "vwap",
  "rsi",
  "range",
  "squeeze",
  "bb",
  "scalp",
] as const;

const TREND_BLOCK_KEYWORDS = [
  "mtf",
  "breakout",
  "bull_flag",
  "prm_breakout",
  "trend",
  "momentum",
  "ema_cross",
] as const;

function normalizeTemplateHaystack(templateFamily: string, strategyName: string): string {
  return `${templateFamily} ${strategyName}`.toLowerCase().replace(/-/g, "_");
}

function matchesKeyword(hay: string, keywords: readonly string[]): boolean {
  return keywords.some((k) => hay.includes(k));
}

function isChopOnlyRegimes(regimes: readonly string[] | undefined): boolean {
  return (
    regimes?.length === 1 &&
    regimes[0]?.toLowerCase() === CHOP_ONLY_REGIME[0]
  );
}

/**
 * Profit-mode chop whitelist — blocks trend/breakout templates in chop unless chop-only regimes.
 * Inactive when profit mode is off or regime is not chop.
 */
export function profitModeAllowsStrategyInChop(
  templateFamily: string,
  strategyName: string,
  regimes: readonly string[] | undefined,
  regime: string,
  config: ProfitModeConfig,
): boolean {
  if (!config.enabled || regime !== "chop") return true;
  if (isChopOnlyRegimes(regimes)) return true;

  const hay = normalizeTemplateHaystack(templateFamily, strategyName);
  if (matchesKeyword(hay, CHOP_NATIVE_KEYWORDS)) return true;
  if (matchesKeyword(hay, TREND_BLOCK_KEYWORDS)) return false;
  return true;
}

/** Keywords used to block trend templates in chop (for docs / tests). */
export function profitModeChopBlockedTrendKeywords(): readonly string[] {
  return TREND_BLOCK_KEYWORDS;
}

export function profitModeChopAllowedNativeKeywords(): readonly string[] {
  return CHOP_NATIVE_KEYWORDS;
}

/** Desk default exit opts when profit mode is off (for UI labels). */
export function deskDefaultExitLabels(): {
  profitLockMinProgress: number;
  profitLockMinNetUsd: number;
} {
  return {
    profitLockMinProgress: deskProfitLockMinProgressFromEnv(),
    profitLockMinNetUsd: deskProfitLockMinNetUsdFromEnv(),
  };
}
