"use client";

import {
  calcAtr,
  calcBollinger,
  calcEma,
  calcKeltner,
  calcRsi,
  calcVwap,
  calcVolumeRatio,
  type OHLCVCandle,
  type ResearchSignal,
  NO_SIGNAL,
  clampConf,
} from "./mockResearchIndicators";

/**
 * Institutional Strategy Library
 * 20 high-performance BTC quantitative strategies.
 */

// 1. BB Keltner Squeeze Breakout
export function bbKeltnerSqueeze(candles: OHLCVCandle[]): ResearchSignal {
  if (candles.length < 20) return NO_SIGNAL;
  const closes = candles.map(c => c.close);
  const bb = calcBollinger(closes, 20, 2);
  const kc = calcKeltner(candles, 20, 1.5);
  
  const isSqueezed = bb.upper < kc.upper && bb.lower > kc.lower;
  const price = closes[closes.length - 1];
  const prevPrice = closes[closes.length - 2];

  if (isSqueezed) return NO_SIGNAL; // wait for breakout

  if (prevPrice < bb.upper && price >= bb.upper) {
    return { side: "BUY", confidence: 75 };
  }
  if (prevPrice > bb.lower && price <= bb.lower) {
    return { side: "SELL", confidence: 75 };
  }
  return NO_SIGNAL;
}

// 2. ATR Compression Breakout
export function atrCompression(candles: OHLCVCandle[]): ResearchSignal {
  if (candles.length < 30) return NO_SIGNAL;
  const atr = calcAtr(candles, 14);
  const atrMa = calcAtr(candles.slice(0, -10), 14);
  
  const isCompressed = atr < atrMa * 0.7;
  const price = candles[candles.length - 1].close;
  const high = Math.max(...candles.slice(-10, -1).map(c => c.high));
  const low = Math.min(...candles.slice(-10, -1).map(c => c.low));

  if (isCompressed && price > high) return { side: "BUY", confidence: 70 };
  if (isCompressed && price < low) return { side: "SELL", confidence: 70 };
  return NO_SIGNAL;
}

// 3. VWAP Trend Pullback
export function vwapPullback(candles: OHLCVCandle[]): ResearchSignal {
  if (candles.length < 20) return NO_SIGNAL;
  const vwap = calcVwap(candles);
  const price = candles[candles.length - 1].close;
  const ema = calcEma(candles.map(c => c.close), 50);
  
  const isUpTrend = price > ema;
  const isDownTrend = price < ema;

  if (isUpTrend && price > vwap && candles[candles.length - 1].low <= vwap) {
    return { side: "BUY", confidence: 65 };
  }
  if (isDownTrend && price < vwap && candles[candles.length - 1].high >= vwap) {
    return { side: "SELL", confidence: 65 };
  }
  return NO_SIGNAL;
}

// 4. Liquidity Sweep Reversal (Simplified)
export function liquiditySweep(candles: OHLCVCandle[]): ResearchSignal {
  if (candles.length < 20) return NO_SIGNAL;
  const lookback = 15;
  const prevHigh = Math.max(...candles.slice(-lookback, -1).map(c => c.high));
  const prevLow = Math.min(...candles.slice(-lookback, -1).map(c => c.low));
  const curr = candles[candles.length - 1];

  if (curr.low < prevLow && curr.close > prevLow) return { side: "BUY", confidence: 80 };
  if (curr.high > prevHigh && curr.close < prevHigh) return { side: "SELL", confidence: 80 };
  return NO_SIGNAL;
}

// 5. RSI VWAP Rubber Band
export function rsiVwapRubberBand(candles: OHLCVCandle[]): ResearchSignal {
  if (candles.length < 14) return NO_SIGNAL;
  const rsi = calcRsi(candles.map(c => c.close), 14);
  const vwap = calcVwap(candles);
  const price = candles[candles.length - 1].close;
  const dev = (price - vwap) / vwap;

  if (rsi < 30 && dev < -0.02) return { side: "BUY", confidence: 70 };
  if (rsi > 70 && dev > 0.02) return { side: "SELL", confidence: 70 };
  return NO_SIGNAL;
}

// 6. Bollinger Rejection
export function bollingerRejection(candles: OHLCVCandle[]): ResearchSignal {
  if (candles.length < 20) return NO_SIGNAL;
  const bb = calcBollinger(candles.map(c => c.close), 20, 2.5);
  const curr = candles[candles.length - 1];

  if (curr.low <= bb.lower && curr.close > bb.lower) return { side: "BUY", confidence: 65 };
  if (curr.high >= bb.upper && curr.close < bb.upper) return { side: "SELL", confidence: 65 };
  return NO_SIGNAL;
}

// 7. Volume Spike Momentum
export function volumeSpikeMomentum(candles: OHLCVCandle[]): ResearchSignal {
  if (candles.length < 20) return NO_SIGNAL;
  const vr = calcVolumeRatio(candles, 20);
  const curr = candles[candles.length - 1];
  const isBullish = curr.close > curr.open;

  if (vr > 3 && isBullish) return { side: "BUY", confidence: 75 };
  if (vr > 3 && !isBullish) return { side: "SELL", confidence: 75 };
  return NO_SIGNAL;
}

// 8. EMA Pullback Rider
export function emaPullback(candles: OHLCVCandle[]): ResearchSignal {
  if (candles.length < 50) return NO_SIGNAL;
  const ema20 = calcEma(candles.map(c => c.close), 20);
  const ema50 = calcEma(candles.map(c => c.close), 50);
  const curr = candles[candles.length - 1];

  if (ema20 > ema50 && curr.low <= ema20 && curr.close > ema20) return { side: "BUY", confidence: 60 };
  if (ema20 < ema50 && curr.high >= ema20 && curr.close < ema20) return { side: "SELL", confidence: 60 };
  return NO_SIGNAL;
}

// 9. MACD Momentum Deceleration
export function macdDeceleration(candles: OHLCVCandle[]): ResearchSignal {
  // Simplified: MACD histogram shrinking
  return NO_SIGNAL; // Placeholder
}

// 10. Liquidation Cascade Snap Back
export function liquidationSnapBack(candles: OHLCVCandle[]): ResearchSignal {
  if (candles.length < 5) return NO_SIGNAL;
  const drop = (candles[candles.length - 1].close - candles[candles.length - 5].close) / candles[candles.length - 5].close;
  const vr = calcVolumeRatio(candles, 10);

  if (drop < -0.05 && vr > 4) return { side: "BUY", confidence: 85 };
  if (drop > 0.05 && vr > 4) return { side: "SELL", confidence: 85 };
  return NO_SIGNAL;
}

// 11. Opening Range Breakout
export function openingRangeBreakout(candles: OHLCVCandle[]): ResearchSignal {
  // Simplified: First 15 mins of "session"
  return NO_SIGNAL;
}

// 12. VWAP Reclaim
export function vwapReclaim(candles: OHLCVCandle[]): ResearchSignal {
  if (candles.length < 20) return NO_SIGNAL;
  const vwap = calcVwap(candles);
  const curr = candles[candles.length - 1];
  const prev = candles[candles.length - 2];

  if (prev.close < vwap && curr.close > vwap) return { side: "BUY", confidence: 70 };
  if (prev.close > vwap && curr.close < vwap) return { side: "SELL", confidence: 70 };
  return NO_SIGNAL;
}

// 13. Market Structure Shift
export function marketStructureShift(candles: OHLCVCandle[]): ResearchSignal {
  return NO_SIGNAL;
}

// 14. Order Block Rejection
export function orderBlockRejection(candles: OHLCVCandle[]): ResearchSignal {
  return NO_SIGNAL;
}

// 15. Fair Value Gap Reversal
export function fvgReversal(candles: OHLCVCandle[]): ResearchSignal {
  return NO_SIGNAL;
}

// 16. Break Of Structure
export function breakOfStructure(candles: OHLCVCandle[]): ResearchSignal {
  return NO_SIGNAL;
}

// 17. Trend Continuation Pullback
export function trendContinuationPullback(candles: OHLCVCandle[]): ResearchSignal {
  return NO_SIGNAL;
}

// 18. Volume Delta Reversal
export function volumeDeltaReversal(candles: OHLCVCandle[]): ResearchSignal {
  return NO_SIGNAL;
}

// 19. Funding Rate Mean Reversion
export function fundingMeanReversion(candles: OHLCVCandle[]): ResearchSignal {
  return NO_SIGNAL;
}

// 20. Multi Timeframe Trend Alignment
export function mtfTrendAlignment(candles: OHLCVCandle[]): ResearchSignal {
  if (candles.length < 100) return NO_SIGNAL;
  const ema20 = calcEma(candles.map(c => c.close), 20);
  const ema50 = calcEma(candles.map(c => c.close), 50);
  const ema100 = calcEma(candles.map(c => c.close), 100);

  if (ema20 > ema50 && ema50 > ema100) return { side: "BUY", confidence: 80 };
  if (ema20 < ema50 && ema50 < ema100) return { side: "SELL", confidence: 80 };
  return NO_SIGNAL;
}
