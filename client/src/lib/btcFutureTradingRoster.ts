/**
 * BTC Future Trading — strategies shown in the paper desk leaderboard and replay tooling.
 * Core basket plus extended BTC FT template IDs (200–299); keep in sync everywhere.
 */

import { BTC_FT_EXTENDED_DEFS_FULL } from "@/lib/btcFtStrategyTemplates";

/** Legacy curated basket (trend, breakout, MTF, flow, session, etc.). */
export const BTC_FUTURE_TRADING_CORE_STRATEGY_IDS: readonly number[] = [
  91, 92, 95, 96, 111, 112, 117, 118, 123, 124, 125, 126, 131, 132, 133, 134, 139, 140, 151, 152,
];

export const BTC_FT_EXTENDED_STRATEGY_IDS: readonly number[] = BTC_FT_EXTENDED_DEFS_FULL.map((d) => d.id);

/** Full roster passed to `BTCFuturesScalper`, paper-replay API, and CLI replay helpers. */
export const BTC_FUTURE_TRADING_STRATEGY_IDS: number[] = [
  ...BTC_FUTURE_TRADING_CORE_STRATEGY_IDS,
  ...BTC_FT_EXTENDED_STRATEGY_IDS,
];
