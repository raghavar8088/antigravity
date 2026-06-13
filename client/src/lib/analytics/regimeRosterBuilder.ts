/**
 * Regime-specific roster builder.
 *
 * Pure function — takes ResearchEdgeScore[] and returns three recommended
 * rosters (chop, trend, all-weather) plus env-line copy strings.
 *
 * Hard invariants:
 *   - DISABLE strategies never appear in any roster.
 *   - Rosters are recommendation-only; nothing is auto-applied.
 *   - Env copy lines never embed secrets.
 *   - No threshold lowering, no gate bypass, no guaranteed profit language.
 */

import type { ResearchEdgeScore } from "@/lib/ai/researchEdgeScore";
import { FUTURES_STRAT_DEFS } from "@/lib/trading/futuresStrategies";

// ─── Public types ─────────────────────────────────────────────────────────────

export interface RegimeRosterOutput {
  chopRoster: number[];
  trendRoster: number[];
  highVolRoster: number[];
  disabledIds: number[];
  envLines: {
    chop: string;
    trend: string;
    allWeather: string;
  };
}

// ─── Category classification ──────────────────────────────────────────────────

const TREND_CATEGORIES = new Set([
  "trend",
  "breakout",
  "momentum",
  "day",
  "day_trading",
  "swing",
  "swing_trading",
  "position",
  "position_trading",
]);

const CHOP_CATEGORIES = new Set([
  "range",
  "range_trading",
  "scalp",
  "scalping",
  "mean_reversion",
]);

// ─── Helpers ──────────────────────────────────────────────────────────────────

/** True if the strategy has positive expectancy in the named regime (≥3 trades). */
function positiveInRegime(score: ResearchEdgeScore, regimeKey: string): boolean {
  const stats = score.regimeBreakdown[regimeKey];
  if (stats && stats.trades >= 3 && stats.expectancy > 0) return true;
  return false;
}

/**
 * When regime breakdown contains only "unknown" entries (no regime tag stored),
 * fall back to overall expectancy for classification.
 */
function hasRealRegimeData(score: ResearchEdgeScore): boolean {
  return Object.keys(score.regimeBreakdown).some((k) => k !== "unknown");
}

function isPositiveInChop(score: ResearchEdgeScore, isChopStyle: boolean): boolean {
  if (positiveInRegime(score, "chop")) return true;
  if (positiveInRegime(score, "range")) return true;
  if (!hasRealRegimeData(score) && isChopStyle && score.expectancy > 0) return true;
  return false;
}

function isPositiveInTrend(score: ResearchEdgeScore, isTrendStyle: boolean): boolean {
  if (positiveInRegime(score, "trendLow")) return true;
  if (positiveInRegime(score, "trendHigh")) return true;
  if (positiveInRegime(score, "trend")) return true;
  if (!hasRealRegimeData(score) && isTrendStyle && score.expectancy > 0) return true;
  return false;
}

function toEnvLine(ids: number[]): string {
  return `NEXT_PUBLIC_BTC_FT_STRATEGY_IDS=${[...ids].sort((a, b) => a - b).join(",")}`;
}

// ─── Main export ──────────────────────────────────────────────────────────────

/**
 * Build three recommended rosters from scored strategies.
 * Only PROMOTE and KEEP strategies are considered.
 * DISABLE strategies are listed separately and excluded from all rosters.
 */
export function buildRegimeRosters(scores: ResearchEdgeScore[]): RegimeRosterOutput {
  // Build strategy-category lookup once
  const categoryMap = new Map<number, string>();
  for (const def of FUTURES_STRAT_DEFS) {
    categoryMap.set(def.id, (def.category ?? "").toLowerCase());
  }

  const eligible = scores.filter(
    (s) => s.status === "PROMOTE" || s.status === "KEEP",
  );

  const disabledIds = scores
    .filter((s) => s.status === "DISABLE")
    .map((s) => s.strategyId)
    .sort((a, b) => a - b);

  const chopRoster: number[] = [];
  const trendRoster: number[] = [];
  const allWeatherSet = new Set<number>();

  for (const score of eligible) {
    const category = categoryMap.get(score.strategyId) ?? "";
    const isChopStyle = CHOP_CATEGORIES.has(category);
    const isTrendStyle = TREND_CATEGORIES.has(category);

    const goodInChop = isPositiveInChop(score, isChopStyle);
    const goodInTrend = isPositiveInTrend(score, isTrendStyle);

    if (goodInChop) chopRoster.push(score.strategyId);
    if (goodInTrend) trendRoster.push(score.strategyId);
    if (goodInChop && goodInTrend) allWeatherSet.add(score.strategyId);
  }

  const highVolRoster = [...allWeatherSet].sort((a, b) => a - b);
  chopRoster.sort((a, b) => a - b);
  trendRoster.sort((a, b) => a - b);

  return {
    chopRoster,
    trendRoster,
    highVolRoster,
    disabledIds,
    envLines: {
      chop: toEnvLine(chopRoster),
      trend: toEnvLine(trendRoster),
      allWeather: toEnvLine(highVolRoster),
    },
  };
}
