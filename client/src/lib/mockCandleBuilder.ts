/**
 * Mock Candle Builder — assembles 1-minute OHLCV candles from live BTC price
 * ticks (Binance trade stream). Used exclusively by useMockCandleBuilder and
 * the mock research runner. Never connected to production order flow.
 *
 * Pure functions — no React, no I/O.
 */

import type { OHLCVCandle } from "@/lib/ai/mockResearchIndicators";

export type { OHLCVCandle };

/** Maximum number of closed candles to keep in the sliding window. */
export const CANDLE_WINDOW_SIZE = 250; // ~4 hours of 1-min bars, enough for EMA200 research strategies

/** State for the in-progress candle bar. */
export interface OpenCandle {
  /** Minute-epoch ms — floored to the start of the current minute. */
  minuteTs: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
  /** Number of ticks accumulated in this bar. */
  tickCount: number;
}

/** Full candle builder state — immutable; always produce new objects. */
export interface CandleBuilderState {
  /** Fully closed 1-min OHLCV candles, oldest → newest. */
  closedCandles: OHLCVCandle[];
  /** Current in-progress bar (null until first tick). */
  openCandle: OpenCandle | null;
}

export const INITIAL_CANDLE_STATE: CandleBuilderState = {
  closedCandles: [],
  openCandle: null,
};

/** Floor a timestamp to the start of its 1-minute bar. */
export function floorToMinute(ts: number): number {
  return ts - (ts % 60_000);
}

/**
 * Apply a new price tick to the candle builder state.
 *
 * When the tick falls in the same minute as the open candle, we update OHLCV.
 * When it falls in a NEW minute, we close the open candle (appending to
 * closedCandles) and start a fresh bar. If multiple minutes were skipped (e.g.
 * no ticks received for 2+ minutes), we do NOT synthesise the missing bars —
 * the gap is natural and strategies that require a minimum number of candles
 * will simply wait.
 *
 * @param state  Previous builder state.
 * @param price  Latest trade price from Binance stream.
 * @param volume Volume of the individual trade tick (default 0 when unknown).
 * @param now    Current epoch ms (default Date.now()).
 */
export function applyTick(
  state: CandleBuilderState,
  price: number,
  volume = 0,
  now = Date.now(),
): CandleBuilderState {
  if (!Number.isFinite(price) || price <= 0) return state;

  const minuteTs = floorToMinute(now);
  const { openCandle, closedCandles } = state;

  if (openCandle === null || minuteTs > openCandle.minuteTs) {
    // Close the previous open candle (if any) and start a new one.
    let nextClosed = closedCandles;
    if (openCandle !== null) {
      const closed: OHLCVCandle = {
        time: openCandle.minuteTs,
        open: openCandle.open,
        high: openCandle.high,
        low: openCandle.low,
        close: openCandle.close,
        volume: openCandle.volume,
      };
      nextClosed = [...closedCandles, closed];
      if (nextClosed.length > CANDLE_WINDOW_SIZE) {
        nextClosed = nextClosed.slice(nextClosed.length - CANDLE_WINDOW_SIZE);
      }
    }
    return {
      closedCandles: nextClosed,
      openCandle: {
        minuteTs,
        open: price,
        high: price,
        low: price,
        close: price,
        volume,
        tickCount: 1,
      },
    };
  }

  // Same minute — update the open candle.
  return {
    closedCandles,
    openCandle: {
      ...openCandle,
      high: Math.max(openCandle.high, price),
      low: Math.min(openCandle.low, price),
      close: price,
      volume: openCandle.volume + volume,
      tickCount: openCandle.tickCount + 1,
    },
  };
}

/**
 * Build a snapshot of the current candle window for strategy evaluation.
 *
 * Returns the closed candles followed by a synthetic "partial" candle built
 * from the in-progress bar so strategies can react to intra-bar price action.
 * The partial candle has the same structure as a closed candle so strategies
 * don't need special-case code for it.
 */
export function getCandleSnapshot(state: CandleBuilderState): OHLCVCandle[] {
  const { closedCandles, openCandle } = state;
  if (openCandle === null) return closedCandles;
  const partial: OHLCVCandle = {
    time: openCandle.minuteTs,
    open: openCandle.open,
    high: openCandle.high,
    low: openCandle.low,
    close: openCandle.close,
    volume: openCandle.volume,
  };
  return [...closedCandles, partial];
}

/**
 * How many fully closed candles are in the builder state.
 * Strategies use this to decide whether enough history exists.
 */
export function closedCandleCount(state: CandleBuilderState): number {
  return state.closedCandles.length;
}
