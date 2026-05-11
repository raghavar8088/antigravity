"use client";

import { BTCFuturesScalper } from "@/components/BTCFuturesScalper";

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

export function BTCFutureTradingScalper() {
  return (
    <BTCFuturesScalper
      title="BTC Future Trading"
      moduleTagline="BTC PERPETUAL FUTURES · 25x · 20 GLOBALLY USED STRATEGY ARCHETYPES"
      strategyIds={BTC_FUTURE_TRADING_STRATEGY_IDS}
      storageNamespace="btc_future_trading_20"
      baseBalance={1000}
    />
  );
}

