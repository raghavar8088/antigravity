"use client";

import { BTCFuturesScalper } from "@/components/BTCFuturesScalper";
import type { BTCFuturesEngineOptions } from "@/hooks/useBTCFuturesScalperEngine";
import { FUTURES_WATCHLIST } from "@/lib/futuresMarketData";

/**
 * Curated BTC-perp strategy basket inspired by globally popular futures archetypes:
 * trend-following, breakout, order-flow/smart-money, and multi-timeframe alignment.
 * Note: this is still a paper module; "highly profitable" is not guaranteed.
 */
const BTC_FUTURE_TRADING_STRATEGY_IDS: number[] = [
  91, 92,   // trend continuation
  95, 96,   // breakout
  111, 112, // mtf trend align
  117, 118, // mtf macd align
  123, 124, // mtf adx power
  125, 126, // mtf breakout
  131, 132, // smart money
  133, 134, // order flow
  139, 140, // wyckoff
  151, 152, // session breakout
];
const BTC_ONLY_SYMBOLS = ["BTCUSD"] as const;
const BTC_ONLY_WATCHLIST = FUTURES_WATCHLIST.filter((item) => item.symbol === "BTCUSD");

export function BTCFutureTradingScalper({
  strategyProfile,
}: {
  /**
   * Optional A/B profile: `"scalp_aggro_v1"` | `"fee_aware_v1"` (default baseline).
   * Pair with a distinct `storageNamespace` in a forked route for clean localStorage comparison.
   */
  strategyProfile?: BTCFuturesEngineOptions["strategyProfile"];
} = {}) {
  return (
    <BTCFuturesScalper
      title="BTC Future Trading"
      moduleTagline="BTC PERPETUAL FUTURES · 25x · 20 GLOBALLY USED STRATEGY ARCHETYPES"
      strategyIds={BTC_FUTURE_TRADING_STRATEGY_IDS}
      symbols={BTC_ONLY_SYMBOLS}
      signalThreshold={26}
      strategyProfile={strategyProfile}
      watchlist={BTC_ONLY_WATCHLIST}
      storageNamespace="btc_future_trading_20"
      baseBalance={1000}
    />
  );
}

