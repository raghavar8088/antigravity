/**
 * BTC Future Trading — roster resolution.
 * Only the CORE 20 winners basket is active. Extended / generated pools removed.
 */

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

/**
 * Resolve the active strategy IDs for BTC Future Trading module.
 * Priority: env list → core only.
 */
export function resolveBtcFtActiveStrategyIds(
  _opts?: { storageNamespace?: string; winnerIds?: number[] },
): BtcFtRosterResolution {
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
  return { ids: [...CORE_BTC_FT_STRATEGY_IDS], source: "core", isLargeRoster: false };
}
