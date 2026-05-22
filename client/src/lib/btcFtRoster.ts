/**
 * BTC Future Trading — roster resolution.
 * Only the CORE 20 winners basket is active. Extended / generated pools removed.
 *
 * When `NEXT_PUBLIC_BTC_FT_USE_RANKED=1`, the roster is filtered by the
 * rankings JSON produced by `npm run rank:btc-ft`. Strategies below
 * `NEXT_PUBLIC_BTC_FT_RANKED_MIN_EXPECTANCY` (default 0) or with fewer than
 * `NEXT_PUBLIC_BTC_FT_RANKED_MIN_TRADES` (default 5) trades are excluded.
 * When no rankings file is available, falls back to full CORE basket.
 */

import { applyWinnersOnlyGate } from "@/lib/futuresDeskPolicy";
import { FUTURES_STRAT_DEFS } from "@/lib/futuresStrategies";
import { BTC_FT_PREMIUM_STRATEGY_IDS } from "@/lib/btcFtPremiumStrategies";

export const CORE_BTC_FT_STRATEGY_IDS: readonly number[] = [
  91, 92, 95, 96, 111, 112, 117, 118, 123, 124, 125, 126, 131, 132, 133, 134, 139, 140, 151, 152,
  ...BTC_FT_PREMIUM_STRATEGY_IDS,
];

/** Extended IDs — empty (research pool removed). */
export const BTC_FT_EXTENDED_STRATEGY_IDS: readonly number[] = [];

/** Generated IDs — empty (research pool removed). */
export const BTC_FT_GENERATED_STRATEGY_IDS: readonly number[] = [];

/** Re-export so legacy imports compile. */
export { BTC_FT_PREMIUM_STRATEGY_IDS };
export function isGeneratedPoolEnabled(): boolean { return false; }

/** Full production roster = CORE only. */
export const BTC_FUTURE_TRADING_STRATEGY_IDS: number[] = [...CORE_BTC_FT_STRATEGY_IDS];

/** Research full pool = CORE only (no extended/generated). */
export const BTC_FT_RESEARCH_FULL_POOL: number[] = [...CORE_BTC_FT_STRATEGY_IDS];

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

export function loadExtendedIdsFromRankings(_topN = 10, _minTrades = 10): number[] { return []; }

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
 * Strategies that meet minExpectancy + minTrades thresholds are promoted.
 * Returns empty set when no rows qualify (caller should fall back to CORE).
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
 *
 * Priority:
 * 1. Explicit `NEXT_PUBLIC_BTC_FT_STRATEGY_IDS` env list (operator override)
 * 2. CORE IDs filtered by winners gate when `opts.winnerIds` provided + non-empty
 * 3. Full CORE basket
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
    const ids = [...new Set(parsed)].slice(0, 120);
    return { ids, source: "env", isLargeRoster: false };
  }

  // 2. Winners gate — only when caller provides non-empty winnerIds
  const winnerIds = opts?.winnerIds;
  if (winnerIds && winnerIds.size > 0) {
    const coreDefs = FUTURES_STRAT_DEFS.filter((d) => CORE_BTC_FT_STRATEGY_IDS.includes(d.id));
    const gated = applyWinnersOnlyGate(coreDefs, winnerIds);
    // Fall back to full CORE if gate would empty the desk
    if (gated.length > 0) {
      return { ids: gated.map((d) => d.id), source: "core+ranked", isLargeRoster: false };
    }
  }

  // 3. Full CORE basket
  return { ids: [...CORE_BTC_FT_STRATEGY_IDS], source: "core", isLargeRoster: false };
}
