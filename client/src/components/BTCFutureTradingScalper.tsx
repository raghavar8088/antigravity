"use client";

import { BTCFuturesScalper } from "@/components/BTCFuturesScalper";
import type { BTCFuturesEngineOptions } from "@/hooks/useBTCFuturesScalperEngine";
import { btcFtRelaxConfirmEnabledFromEnv, btcFtSignalThresholdFromEnv } from "@/lib/futuresDeskPolicy";
import { FUTURES_WATCHLIST } from "@/lib/futuresMarketData";
import { resolveBtcFtActiveStrategyIds } from "@/lib/btcFtRoster";
import { DeskBanner } from "@/components/desk/ui";

const BTC_ONLY_SYMBOLS = ["BTCUSD"] as const;
const BTC_ONLY_WATCHLIST = FUTURES_WATCHLIST.filter((item) => item.symbol === "BTCUSD");

// Resolve once at module load (server / SSR) — same env for entire session.
const ACTIVE_ROSTER = resolveBtcFtActiveStrategyIds();

export function BTCFutureTradingScalper({
  strategyProfile,
}: {
  /**
   * Optional A/B profile: `"scalp_aggro_v1"` | `"fee_aware_v1"` (default baseline).
   * Pair with a distinct `storageNamespace` in a forked route for clean localStorage comparison.
   */
  strategyProfile?: BTCFuturesEngineOptions["strategyProfile"];
} = {}) {
  const sourceLabel =
    ACTIVE_ROSTER.source === "env"
      ? "env"
      : ACTIVE_ROSTER.source === "core+ranked"
      ? "core + ranked"
      : "core";

  const threshold = btcFtSignalThresholdFromEnv(26);

  const tagline = `BTC PERPETUAL FUTURES · 25x · ${ACTIVE_ROSTER.ids.length} STRATEGIES (${sourceLabel.toUpperCase()}) · THRESHOLD ${threshold}`;

  return (
    <>
      {ACTIVE_ROSTER.isLargeRoster && (
        <DeskBanner variant="warning" title="Large roster on single symbol">
          You have {ACTIVE_ROSTER.ids.length} strategies active via NEXT_PUBLIC_BTC_FT_STRATEGY_IDS.
          On choppy 1m bars with threshold {threshold}, candidatesBuilt ≈ 0 is expected.
          See{" "}
          <a
            href="#btc-ft-no-trades"
            style={{ textDecoration: "underline", fontWeight: 600 }}
          >
            README#btc-ft-no-trades
          </a>{" "}
          for troubleshooting.
        </DeskBanner>
      )}
      <BTCFuturesScalper
        title="BTC Future Trading"
        moduleTagline={tagline}
        strategyIds={ACTIVE_ROSTER.ids}
        symbols={BTC_ONLY_SYMBOLS}
        signalThreshold={threshold}
        relaxEntryConfirmation={btcFtRelaxConfirmEnabledFromEnv()}
        strategyProfile={strategyProfile}
        watchlist={BTC_ONLY_WATCHLIST}
        storageNamespace="btc_future_trading_desk_v3"
        baseBalance={1000}
      />
    </>
  );
}
