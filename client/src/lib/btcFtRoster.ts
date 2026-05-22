/**
 * BTC Future Trading — roster resolution.
 * Only the CORE 20 winners basket is active. Extended / generated pools removed.
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

export { BTC_FT_PREMIUM_STRATEGY_IDS };
export function isGeneratedPoolEnabled(): boolean { return false; }

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

export function resolveBtcFtActiveStrategyIds(
  opts?: { storageNamespace?: string; winnerIds?: ReadonlySet<number> },
): BtcFtRosterResolution {
  const envList = process.env.NEXT_PUBLIC_BTC_FT_STRATEGY_IDS;
  if (envList && envList.trim() !== "") {
    const valid = new Set(CORE_BTC_FT_STRATEGY_IDS);
    const parsed = envList
      .split(",")
      .map((s) => Number(s.trim()))
      .filter((id) => Number.isFinite(id) && valid.has(id));
    const ids = [...new Set(parsed)].slice(0, 24);
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
