"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  applyTick,
  getCandleSnapshot,
  INITIAL_CANDLE_STATE,
  type CandleBuilderState,
} from "@/lib/mockCandleBuilder";
import type { OHLCVCandle } from "@/lib/mockResearchIndicators";

export interface UseMockCandleBuilderResult {
  /** All closed 1-min candles + in-progress partial bar (oldest → newest). */
  snapshot: OHLCVCandle[];
  /** Number of fully closed candles available. */
  closedCount: number;
  /** Epoch ms of the most recent closed candle's start (null = none yet). */
  lastClosedMinute: number | null;
  /**
   * Whether a new closed candle was produced on the last tick.
   * Flips true for one render cycle then resets. Consumers can use this as
   * a signal to run strategy evaluation on the latest completed bar.
   */
  newCandleReady: boolean;
}

/**
 * Builds 1-minute OHLCV candles from the live BTC price tick.
 *
 * @param price  Live BTC USD price from useLiveBTCPrice (0 = not connected).
 */
export function useMockCandleBuilder(price: number): UseMockCandleBuilderResult {
  const stateRef = useRef<CandleBuilderState>(INITIAL_CANDLE_STATE);
  const [snapshot, setSnapshot] = useState<OHLCVCandle[]>([]);
  const [closedCount, setClosedCount] = useState(0);
  const [lastClosedMinute, setLastClosedMinute] = useState<number | null>(null);
  const [newCandleReady, setNewCandleReady] = useState(false);

  const prevClosedCountRef = useRef(0);

  const processTick = useCallback((px: number) => {
    if (!Number.isFinite(px) || px <= 0) return;

    const prev = stateRef.current;
    const prevClosed = prev.closedCandles.length;
    const next = applyTick(prev, px);
    stateRef.current = next;

    const currClosed = next.closedCandles.length;
    const gotNewCandle = currClosed > prevClosed;

    setSnapshot(getCandleSnapshot(next));
    setClosedCount(currClosed);

    if (gotNewCandle) {
      const newest = next.closedCandles[currClosed - 1];
      setLastClosedMinute(newest.time);
      prevClosedCountRef.current = currClosed;
      setNewCandleReady(true);
    }
  }, []);

  // Apply each price tick to the candle builder.
  useEffect(() => {
    processTick(price);
  }, [price, processTick]);

  // Reset newCandleReady after one render so consumers see a single "pulse".
  useEffect(() => {
    if (newCandleReady) {
      const id = setTimeout(() => setNewCandleReady(false), 0);
      return () => clearTimeout(id);
    }
  }, [newCandleReady]);

  return { snapshot, closedCount, lastClosedMinute, newCandleReady };
}
