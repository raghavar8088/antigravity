/**
 * BTC Future Trading — roster resolution (Part 1).
 *
 * Problem: running 120 strategies × 1 symbol × threshold 26 × regime/min-move/session gates
 * → on choppy 1m bars evalPairs is high but candidatesBuilt ≈ 0.
 *
 * Fix: DEFAULT to CORE only (~20 IDs). Extended roster requires explicit env opt-in.
 *
 * Env controls:
 *   NEXT_PUBLIC_BTC_FT_STRATEGY_IDS   comma-separated id list (cap 120)
 *   NEXT_PUBLIC_BTC_FT_USE_RANKED=1   CORE + top-10 from btc_ft_strategy_rankings.json (if present)
 *
 * See: README.md#btc-ft-no-trades
 */

import { BTC_FT_EXTENDED_DEFS_FULL } from "@/lib/btcFtStrategyTemplates";

// ---------------------------------------------------------------------------
// CORE basket — legacy curated IDs with full evalMinuteSignal wiring.
// Trend, breakout, MTF, flow, session, etc. — verified strats only.
// ---------------------------------------------------------------------------
export const CORE_BTC_FT_STRATEGY_IDS: readonly number[] = [
  91, 92, 95, 96, 111, 112, 117, 118, 123, 124, 125, 126, 131, 132, 133, 134, 139, 140, 151, 152,
] as const;

/** Extended IDs built from BTC FT templates (200–299). */
export const BTC_FT_EXTENDED_STRATEGY_IDS: readonly number[] = BTC_FT_EXTENDED_DEFS_FULL.map((d) => d.id);

/** Legacy full roster (CORE + extended). Kept for backward compat. */
export const BTC_FUTURE_TRADING_STRATEGY_IDS: number[] = [
  ...CORE_BTC_FT_STRATEGY_IDS,
  ...BTC_FT_EXTENDED_STRATEGY_IDS,
];

// ---------------------------------------------------------------------------
// Rankings loader
// ---------------------------------------------------------------------------

export type BtcFtStrategyRankingRow = {
  id: number;
  expectancy: number;
  trades: number;
  winRate?: number;
};

/**
 * Load top-N ranked IDs from `client/fixtures/replay/btc_ft_strategy_rankings.json` (if present).
 * Node/browser safe: returns [] when file is absent (fixture generation optional).
 * Minimum trade count guard: rows with < minTrades are excluded.
 */
export function loadExtendedIdsFromRankings(topN = 10, minTrades = 10): number[] {
  // Browser: rankings are loaded at build time via a JSON import shim. In SSR/test environments
  // they might not exist — always handle gracefully.
  let rows: BtcFtStrategyRankingRow[] = [];
  try {
    // Dynamic require for Node (scripts/tests). In browser this is dead-code eliminated.
    if (typeof require !== "undefined") {
      // eslint-disable-next-line @typescript-eslint/no-require-imports
      const raw = require("../../fixtures/replay/btc_ft_strategy_rankings.json") as unknown;
      if (Array.isArray(raw)) {
        rows = (raw as BtcFtStrategyRankingRow[]).filter(
          (r) => typeof r.id === "number" && typeof r.expectancy === "number" && typeof r.trades === "number",
        );
      }
    }
  } catch {
    // file absent — silent
  }
  return rows
    .filter((r) => r.trades >= minTrades)
    .sort((a, b) => b.expectancy - a.expectancy)
    .slice(0, topN)
    .map((r) => r.id);
}

// ---------------------------------------------------------------------------
// Roster resolution
// ---------------------------------------------------------------------------

export type BtcFtRosterSource = "env" | "core+ranked" | "core";

export type BtcFtRosterResolution = {
  ids: number[];
  source: BtcFtRosterSource;
  isLargeRoster: boolean;
};

/**
 * Resolve the active strategy IDs for BTC Future Trading module.
 *
 * Priority:
 *  1. `NEXT_PUBLIC_BTC_FT_STRATEGY_IDS` (comma list, cap 120)
 *  2. `NEXT_PUBLIC_BTC_FT_USE_RANKED=1` → CORE + top-10 from rankings
 *  3. Default → CORE only (≤ 30 IDs)
 *
 * Hard cap: 120 IDs regardless of env.
 */
export function resolveBtcFtActiveStrategyIds(): BtcFtRosterResolution {
  const envList = process.env.NEXT_PUBLIC_BTC_FT_STRATEGY_IDS;
  if (envList && envList.trim() !== "") {
    const parsed = envList
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean)
      .map(Number)
      .filter((n) => Number.isFinite(n) && n > 0);
    const ids = [...new Set(parsed)].slice(0, 120);
    return { ids, source: "env", isLargeRoster: ids.length > 30 };
  }

  const useRanked = process.env.NEXT_PUBLIC_BTC_FT_USE_RANKED === "1";
  if (useRanked) {
    const ranked = loadExtendedIdsFromRankings(10, 10);
    // Merge: CORE first, then ranked (dedup)
    const merged = [...new Set([...CORE_BTC_FT_STRATEGY_IDS, ...ranked])];
    return { ids: merged, source: "core+ranked", isLargeRoster: false };
  }

  // Default: CORE only
  return { ids: [...CORE_BTC_FT_STRATEGY_IDS], source: "core", isLargeRoster: false };
}
