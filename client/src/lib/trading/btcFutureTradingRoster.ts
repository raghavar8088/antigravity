import { BTC_FT_PREMIUM_STRATEGY_IDS } from "@/lib/trading/btcFtPremiumStrategies";

/** CORE 20 winners basket — only active strategies. */
export const BTC_FUTURE_TRADING_CORE_STRATEGY_IDS: readonly number[] = [
  91, 92, 95, 96, 111, 112, 117, 118, 123, 124, 125, 126, 131, 132, 133, 134, 139, 140, 151, 152,
];

export const BTC_FT_EXTENDED_STRATEGY_IDS: readonly number[] = [];

/** Full production roster = CORE 20 + premium (500–503). */
export const BTC_FUTURE_TRADING_STRATEGY_IDS: number[] = [
  ...BTC_FUTURE_TRADING_CORE_STRATEGY_IDS,
  ...BTC_FT_PREMIUM_STRATEGY_IDS,
];
