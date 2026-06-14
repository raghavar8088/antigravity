"use client";

import { useMemo } from "react";
import {
  classifyMarketRegime,
  type MarketRegime,
  type RegimeSnapshot,
} from "@/lib/ai/marketRegimeClassifier";
import type { OHLCVCandle } from "@/lib/ai/mockResearchIndicators";

const MIN_CANDLES_FOR_REGIME = 20;

export interface UseMarketRegimeArgs {
  candles: readonly OHLCVCandle[];
  newCandleReady: boolean;
}

export interface UseMarketRegimeResult {
  regime: MarketRegime | null;
  snapshot: RegimeSnapshot | null;
}

/**
 * Classifies the current market regime from the live OHLCV candle history.
 * Re-runs classification whenever `newCandleReady` flips (each closed 1m bar).
 */
export function useMarketRegime({
  candles,
}: UseMarketRegimeArgs): UseMarketRegimeResult {
  const snapshot = useMemo<RegimeSnapshot | null>(() => {
    if (candles.length < MIN_CANDLES_FOR_REGIME) return null;
    try {
      return classifyMarketRegime(candles);
    } catch {
      return null;
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [candles.length]);

  return {
    regime: snapshot?.regime ?? null,
    snapshot,
  };
}
