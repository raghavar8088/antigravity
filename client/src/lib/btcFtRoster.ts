/**
 * BTC Future Trading — roster resolution.
 *
 * Active tiers:
 *   CORE 20 (91–152): always active production winners.
 *   Premium (500–503): active when in promotedStrategyIds.
 *   Research pool (600–759): researchOnly — active only in research mode or after promotion.
 *
 * Default desk (no env overrides): CORE 20 + promoted premium only.
 * Research mode (NEXT_PUBLIC_BTC_FT_RESEARCH_MODE=1): full pool per category.
 * Category filter (NEXT_PUBLIC_BTC_FT_CATEGORY=scalping|...): narrows research pool.
 */

import { applyWinnersOnlyGate } from "@/lib/futuresDeskPolicy";
import { buildPaperDeskStrategies } from "@/lib/futuresDeskPolicy";
import { FUTURES_STRAT_DEFS } from "@/lib/futuresStrategies";
import { BTC_FT_PREMIUM_STRATEGY_IDS } from "@/lib/btcFtPremiumStrategies";
import {
  SCALPING_STRATEGIES,
  DAY_TRADING_STRATEGIES,
  SWING_TRADING_STRATEGIES,
  POSITION_TRADING_STRATEGIES,
  TREND_TRADING_STRATEGIES,
  RANGE_TRADING_STRATEGIES,
  BREAKOUT_TRADING_STRATEGIES,
  MOMENTUM_TRADING_STRATEGIES,
  LEGACY_CORE_CATEGORY_MAP,
} from "@/lib/futuresCategoryStrategies";
import type { TradingCategoryId, FuturesStratDef } from "@/lib/futuresStratTypes";

export const CORE_BTC_FT_STRATEGY_IDS: readonly number[] = [
  91, 92, 95, 96, 111, 112, 117, 118, 123, 124, 125, 126, 131, 132, 133, 134, 139, 140, 151, 152,
  ...BTC_FT_PREMIUM_STRATEGY_IDS,
];

// Per-category ID arrays for the research pool
export const CATEGORY_STRATEGY_IDS: Readonly<Record<TradingCategoryId, readonly number[]>> = {
  scalping:          SCALPING_STRATEGIES.map((d) => d.id),
  day_trading:       DAY_TRADING_STRATEGIES.map((d) => d.id),
  swing_trading:     SWING_TRADING_STRATEGIES.map((d) => d.id),
  position_trading:  POSITION_TRADING_STRATEGIES.map((d) => d.id),
  trend_trading:     TREND_TRADING_STRATEGIES.map((d) => d.id),
  range_trading:     RANGE_TRADING_STRATEGIES.map((d) => d.id),
  breakout_trading:  BREAKOUT_TRADING_STRATEGIES.map((d) => d.id),
  momentum_trading:  MOMENTUM_TRADING_STRATEGIES.map((d) => d.id),
};

/** All 160 research-pool IDs (may include 0 from unimplemented category stubs). */
export const BTC_FT_RESEARCH_CATEGORY_IDS: readonly number[] = (
  Object.values(CATEGORY_STRATEGY_IDS) as number[][]
).flat();

/** CORE winners + all research category IDs. Used in research sessions only. */
export const BTC_FT_RESEARCH_FULL_POOL: readonly number[] = [
  ...CORE_BTC_FT_STRATEGY_IDS,
  ...BTC_FT_RESEARCH_CATEGORY_IDS,
];

/** Extended IDs — empty (legacy field; replaced by CATEGORY_STRATEGY_IDS). */
export const BTC_FT_EXTENDED_STRATEGY_IDS: readonly number[] = [];
/** Generated IDs — empty (no generated pool). */
export const BTC_FT_GENERATED_STRATEGY_IDS: readonly number[] = [];

export { BTC_FT_PREMIUM_STRATEGY_IDS, LEGACY_CORE_CATEGORY_MAP };
export function isGeneratedPoolEnabled(): boolean { return false; }

export const BTC_FUTURE_TRADING_STRATEGY_IDS: number[] = [...CORE_BTC_FT_STRATEGY_IDS];

export type BtcFtRosterSource = "env" | "core+ranked" | "core" | "winners" | "research" | "full";

export type BtcFtRosterResolution = {
  ids: number[];
  source: BtcFtRosterSource;
  isLargeRoster: boolean;
};

export type BtcFtStrategyRankingRow = {
  id: number;
  expectancy: number;
  trades: number;
  winRate?: number;
};

export function loadExtendedIdsFromRankings(_topN = 10, _minTrades = 10): number[] { return []; }

export function btcFtUseRankedEnabled(): boolean {
  return process.env.NEXT_PUBLIC_BTC_FT_USE_RANKED === "1";
}

export function btcFtRankedMinExpectancyFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_BTC_FT_RANKED_MIN_EXPECTANCY;
  if (raw === undefined || raw === "") return 0;
  const n = Number(raw);
  return Number.isFinite(n) ? n : 0;
}

export function btcFtRankedMinTradesFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_BTC_FT_RANKED_MIN_TRADES;
  if (raw === undefined || raw === "") return 5;
  const n = Number(raw);
  return Number.isFinite(n) && n > 0 ? Math.floor(n) : 5;
}

export function winnerIdsFromRankings(
  rankings: ReadonlyArray<BtcFtStrategyRankingRow>,
  minExpectancy = btcFtRankedMinExpectancyFromEnv(),
  minTrades = btcFtRankedMinTradesFromEnv(),
): ReadonlySet<number> {
  const winners = rankings.filter(
    (r) => r.trades >= minTrades && r.expectancy >= minExpectancy,
  );
  return new Set(winners.map((r) => r.id));
}

/** Active category filter from env (default "all"). */
export function btcFtActiveCategoryFromEnv(): TradingCategoryId | "all" {
  const raw = process.env.NEXT_PUBLIC_BTC_FT_CATEGORY;
  if (!raw || raw === "" || raw === "all") return "all";
  const valid: TradingCategoryId[] = [
    "scalping", "day_trading", "swing_trading", "position_trading",
    "trend_trading", "range_trading", "breakout_trading", "momentum_trading",
  ];
  return valid.includes(raw as TradingCategoryId) ? (raw as TradingCategoryId) : "all";
}

/** Research mode gate: true when NEXT_PUBLIC_BTC_FT_RESEARCH_MODE=1. */
export function btcFtResearchModeEnabled(): boolean {
  return process.env.NEXT_PUBLIC_BTC_FT_RESEARCH_MODE === "1";
}

/**
 * Build the active strategy def list for a given category in research mode.
 * Applies RR widen, fake-diversity filter (off), capped at 8 per session.
 */
export function buildCategoryRoster(
  categoryId: TradingCategoryId,
  opts?: { winnerIds?: ReadonlySet<number>; researchMode?: boolean },
): FuturesStratDef[] {
  const researchMode = opts?.researchMode ?? btcFtResearchModeEnabled();
  const categoryIds = new Set(CATEGORY_STRATEGY_IDS[categoryId]);
  const defs = FUTURES_STRAT_DEFS.filter((d) => categoryIds.has(d.id));

  const winnerIds = opts?.winnerIds;
  const gated =
    !researchMode && winnerIds && winnerIds.size > 0
      ? applyWinnersOnlyGate(defs, winnerIds)
      : defs;

  const built = buildPaperDeskStrategies(gated, {
    strategyIdAllowlist: null,
    minTpSlRatio: 2.0,
    allowFakeDiversity: false,
  });

  // Cap at 8 strategies per category session (burst guard friendly)
  return built.strategies.slice(0, 8);
}

export function resolveBtcFtActiveStrategyIds(
  opts?: { storageNamespace?: string; winnerIds?: ReadonlySet<number> },
): BtcFtRosterResolution {
  // Research mode: expose category pool (capped by category env filter)
  if (btcFtResearchModeEnabled()) {
    const cat = btcFtActiveCategoryFromEnv();
    const poolIds =
      cat === "all"
        ? [...BTC_FT_RESEARCH_FULL_POOL]
        : [...CORE_BTC_FT_STRATEGY_IDS, ...CATEGORY_STRATEGY_IDS[cat]];
    return { ids: poolIds, source: "research", isLargeRoster: poolIds.length > 24 };
  }

  const envList = process.env.NEXT_PUBLIC_BTC_FT_STRATEGY_IDS;
  if (envList && envList.trim() !== "") {
    const valid = new Set(CORE_BTC_FT_STRATEGY_IDS);
    const parsed = envList
      .split(",")
      .map((s) => Number(s.trim()))
      .filter((id) => Number.isFinite(id) && valid.has(id));
    const ids = [...new Set(parsed)];
    if (ids.length > 0) {
      return { ids, source: "env", isLargeRoster: false };
    }
  }

  const winnerIds = opts?.winnerIds;
  if (winnerIds && winnerIds.size > 0) {
    const coreDefs = FUTURES_STRAT_DEFS.filter((d) => CORE_BTC_FT_STRATEGY_IDS.includes(d.id));
    const gated = applyWinnersOnlyGate(coreDefs, winnerIds);
    if (gated.length > 0) {
      return { ids: gated.map((d) => d.id), source: "core+ranked", isLargeRoster: false };
    }
  }

  return { ids: [...CORE_BTC_FT_STRATEGY_IDS], source: "core", isLargeRoster: false };
}
