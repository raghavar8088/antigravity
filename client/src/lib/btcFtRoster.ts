/**
 * BTC Future Trading — roster resolution.
 *
 * Resolution order:
 *  1. Explicit `NEXT_PUBLIC_BTC_FT_STRATEGY_IDS` env list (operator override)
 *  2. CORE IDs filtered by winners gate when `opts.winnerIds` provided + non-empty
 *  3. Full CORE basket
 *
 * Extended (200–299) and generated (300–399) pools are handled by `btcFtResearch.ts`
 * when research mode is active.
 */

import { applyWinnersOnlyGate } from "@/lib/futuresDeskPolicy";
import { FUTURES_STRAT_DEFS } from "@/lib/futuresStrategies";
import { BTC_FT_PREMIUM_STRATEGY_IDS } from "@/lib/btcFtPremiumStrategies";

/** CORE 20 winners basket — only active strategies. */
export const CORE_BTC_FT_STRATEGY_IDS: readonly number[] = [
  91, 92, 95, 96, 111, 112, 117, 118, 123, 124, 125, 126, 131, 132, 133, 134, 139, 140, 151, 152,
  ...BTC_FT_PREMIUM_STRATEGY_IDS,
];

/** Extended IDs (200–299) — research pool. */
export const BTC_FT_EXTENDED_STRATEGY_IDS: readonly number[] = Array.from({ length: 100 }, (_, i) => 200 + i);

/** Generated IDs (300–399) — research pool. */
export const BTC_FT_GENERATED_STRATEGY_IDS: readonly number[] = Array.from({ length: 100 }, (_, i) => 300 + i);

/** Re-export so legacy imports compile. */
export { BTC_FT_PREMIUM_STRATEGY_IDS };

/** Full production roster = CORE 20 + premium. */
export const BTC_FUTURE_TRADING_STRATEGY_IDS: number[] = [...CORE_BTC_FT_STRATEGY_IDS];

/** Research full pool = CORE + extended + generated. */
export const BTC_FT_RESEARCH_FULL_POOL: number[] = [
  ...CORE_BTC_FT_STRATEGY_IDS,
  ...BTC_FT_EXTENDED_STRATEGY_IDS,
  ...BTC_FT_GENERATED_STRATEGY_IDS,
];

export type BtcFtRosterSource = "env" | "core+ranked" | "core" | "winners" | "full";

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

/** True when `NEXT_PUBLIC_BTC_FT_USE_RANKED=1`. */
export function btcFtUseRankedEnabled(): boolean {
  return process.env.NEXT_PUBLIC_BTC_FT_USE_RANKED === "1";
}

/** Min expectancy (USD) for a strategy to be treated as a "winner". Default $0. */
export function btcFtRankedMinExpectancyFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_BTC_FT_RANKED_MIN_EXPECTANCY;
  if (raw === undefined || raw === "") return 0;
  const n = Number(raw);
  return Number.isFinite(n) ? n : 0;
}

/** Min trades for a strategy to be eligible for ranking gate. Default 5. */
export function btcFtRankedMinTradesFromEnv(): number {
  const raw = process.env.NEXT_PUBLIC_BTC_FT_RANKED_MIN_TRADES;
  if (raw === undefined || raw === "") return 5;
  const n = Number(raw);
  return Number.isFinite(n) && n > 0 ? Math.floor(n) : 5;
}

/**
 * Derive winner IDs from a rankings array produced by `npm run rank:btc-ft`.
 */
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

/**
 * Resolve the active strategy IDs for BTC Future Trading module.
 */
export function resolveBtcFtActiveStrategyIds(
  opts?: { storageNamespace?: string; winnerIds?: ReadonlySet<number> },
): BtcFtRosterResolution {
  // 1. Explicit env list
  const envList = process.env.NEXT_PUBLIC_BTC_FT_STRATEGY_IDS;
  if (envList && envList.trim() !== "") {
    const parsed = envList
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean)
      .map(Number)
      .filter((n) => Number.isFinite(n) && n > 0);
    const ids = [...new Set(parsed)].slice(0, 200);
    return { ids, source: "env", isLargeRoster: ids.length > 50 };
  }

  // 2. Winners gate
  const winnerIds = opts?.winnerIds;
  if (winnerIds && winnerIds.size > 0) {
    const coreDefs = FUTURES_STRAT_DEFS.filter((d) => CORE_BTC_FT_STRATEGY_IDS.includes(d.id));
    const gated = applyWinnersOnlyGate(coreDefs, winnerIds);
    if (gated.length > 0) {
      return { ids: gated.map((d) => d.id), source: "core+ranked", isLargeRoster: false };
    }
  }

  // 3. Full CORE basket
  return { ids: [...CORE_BTC_FT_STRATEGY_IDS], source: "core", isLargeRoster: false };
}
