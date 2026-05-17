"use client";

import { BTCFuturesScalper } from "@/components/BTCFuturesScalper";
import type { BTCFuturesEngineOptions } from "@/hooks/useBTCFuturesScalperEngine";
import { FUTURES_WATCHLIST } from "@/lib/futuresMarketData";
import { BTC_FUTURE_TRADING_STRATEGY_IDS } from "@/lib/btcFutureTradingRoster";

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
      moduleTagline={`BTC PERPETUAL FUTURES · 25x · ${BTC_FUTURE_TRADING_STRATEGY_IDS.length} STRATEGIES (CORE + BTC FT TEMPLATES)`}
      strategyIds={BTC_FUTURE_TRADING_STRATEGY_IDS}
      symbols={BTC_ONLY_SYMBOLS}
      signalThreshold={26}
      strategyProfile={strategyProfile}
      watchlist={BTC_ONLY_WATCHLIST}
      storageNamespace="btc_future_trading_desk_v2"
      baseBalance={1000}
    />
  );
}

