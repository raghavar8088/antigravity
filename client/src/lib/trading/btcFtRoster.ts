/**
 * BTC Future Trading — roster resolution.
 *
 * Strategy inventories have been removed from the application. This resolver
 * now returns empty rosters while preserving the public API used by the desk,
 * diagnostics, and UI.
 */

import { applyWinnersOnlyGate, buildPaperDeskStrategies } from "@/lib/trading/futuresDeskPolicy";
import { FUTURES_STRAT_DEFS } from "@/lib/trading/futuresStrategies";
import { BTC_FT_PREMIUM_STRATEGY_IDS } from "@/lib/trading/btcFtPremiumStrategies";
import {
  BREAKOUT_TRADING_STRATEGIES,
  DAY_TRADING_STRATEGIES,
  LEGACY_CORE_CATEGORY_MAP,
  MOMENTUM_TRADING_STRATEGIES,
  POSITION_TRADING_STRATEGIES,
  RANGE_TRADING_STRATEGIES,
  SCALPING_STRATEGIES,
  SWING_TRADING_STRATEGIES,
  TREND_TRADING_STRATEGIES,
} from "@/lib/trading/futuresCategoryStrategies";
import type { FuturesStratDef, TradingCategoryId } from "@/lib/trading/futuresStratTypes";

export const CORE_BTC_FT_STRATEGY_IDS: readonly number[] = [];

export const CATEGORY_STRATEGY_IDS: Readonly<Record<TradingCategoryId, readonly number[]>> = {
  scalping: SCALPING_STRATEGIES.map((d) => d.id),
  day_trading: DAY_TRADING_STRATEGIES.map((d) => d.id),
  swing_trading: SWING_TRADING_STRATEGIES.map((d) => d.id),
  position_trading: POSITION_TRADING_STRATEGIES.map((d) => d.id),
  trend_trading: TREND_TRADING_STRATEGIES.map((d) => d.id),
  range_trading: RANGE_TRADING_STRATEGIES.map((d) => d.id),
  breakout_trading: BREAKOUT_TRADING_STRATEGIES.map((d) => d.id),
  momentum_trading: MOMENTUM_TRADING_STRATEGIES.map((d) => d.id),
};

export const BTC_FT_RESEARCH_CATEGORY_IDS: readonly number[] = (
  Object.values(CATEGORY_STRATEGY_IDS) as number[][]
).flat();

export const BTC_FT_RESEARCH_FULL_POOL: readonly number[] = [
  ...CORE_BTC_FT_STRATEGY_IDS,
  ...BTC_FT_RESEARCH_CATEGORY_IDS,
];

export const BTC_FT_EXTENDED_STRATEGY_IDS: readonly number[] = [];
export const BTC_FT_GENERATED_STRATEGY_IDS: readonly number[] = [];

export { BTC_FT_PREMIUM_STRATEGY_IDS, LEGACY_CORE_CATEGORY_MAP };

export function isGeneratedPoolEnabled(): boolean {
  return false;
}

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

export function loadExtendedIdsFromRankings(_topN = 10, _minTrades = 10): number[] {
  return [];
}

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

export function btcFtActiveCategoryFromEnv(): TradingCategoryId | "all" {
  const raw = process.env.NEXT_PUBLIC_BTC_FT_CATEGORY;
  if (!raw || raw === "" || raw === "all") return "all";
  const valid: TradingCategoryId[] = [
    "scalping",
    "day_trading",
    "swing_trading",
    "position_trading",
    "trend_trading",
    "range_trading",
    "breakout_trading",
    "momentum_trading",
  ];
  return valid.includes(raw as TradingCategoryId) ? (raw as TradingCategoryId) : "all";
}

export function btcFtResearchModeEnabled(): boolean {
  return process.env.NEXT_PUBLIC_BTC_FT_RESEARCH_MODE === "1";
}

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

  return built.strategies.slice(0, 8);
}

export function resolveBtcFtActiveStrategyIds(
  opts?: { storageNamespace?: string; winnerIds?: ReadonlySet<number> },
): BtcFtRosterResolution {
  if (btcFtResearchModeEnabled()) {
    const cat = btcFtActiveCategoryFromEnv();
    const poolIds =
      cat === "all"
        ? [...BTC_FT_RESEARCH_FULL_POOL]
        : [...CORE_BTC_FT_STRATEGY_IDS, ...CATEGORY_STRATEGY_IDS[cat]];
    return { ids: poolIds, source: "research", isLargeRoster: false };
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
