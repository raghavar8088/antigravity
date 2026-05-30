import {
  calcAdx,
  calcAtr,
  calcBollinger,
  calcEmaSlope,
  type OHLCVCandle,
} from "@/lib/mockResearchIndicators";

export type MarketRegime =
  | "TRENDING"
  | "RANGING"
  | "HIGH_VOLATILITY_BREAKOUT"
  | "LOW_VOLATILITY_CHOP";

export interface RegimeSnapshot {
  regime: MarketRegime;
  /** Classifier confidence in [0, 100]. */
  confidence: number;
  adx: number;
  /** ATR(14) / price * 100. */
  atrPct: number;
  /** Bollinger bandwidth / middle band * 100. */
  bbWidthPct: number;
  /** Rolling Bollinger width percentile in [0, 1]. */
  bbWidthPercentile: number;
  /** Fractional EMA50 change over the last 10 bars. */
  emaSlope: number;
  /** Annualized realized volatility from recent 1-minute log returns. */
  realizedVol: number;
  timestamp: number;
}

function clamp(value: number, min = 0, max = 100): number {
  if (!Number.isFinite(value)) return min;
  return Math.min(max, Math.max(min, value));
}

function safePct(numerator: number, denominator: number): number {
  if (!Number.isFinite(numerator) || !Number.isFinite(denominator) || denominator === 0) return 0;
  return (numerator / denominator) * 100;
}

function realizedVolatility(closes: readonly number[], sampleBars = 20): number {
  const slice = closes.slice(-Math.max(2, sampleBars + 1));
  if (slice.length < 2) return 0;

  const logReturns: number[] = [];
  for (let i = 1; i < slice.length; i++) {
    const prev = slice[i - 1];
    const curr = slice[i];
    if (prev > 0 && curr > 0) logReturns.push(Math.log(curr / prev));
  }
  if (logReturns.length === 0) return 0;

  const mean = logReturns.reduce((sum, value) => sum + value, 0) / logReturns.length;
  const variance =
    logReturns.reduce((sum, value) => sum + (value - mean) ** 2, 0) / logReturns.length;
  return Math.sqrt(variance) * Math.sqrt(525_600) * 100;
}

function bollingerWidthPercentile(closes: readonly number[], lookback: number): number {
  if (closes.length < 20) return 0.5;

  const widths: number[] = [];
  const start = Math.max(20, closes.length - lookback);
  for (let end = start; end <= closes.length; end++) {
    const bb = calcBollinger(closes.slice(0, end), 20, 2);
    const width = bb.middle > 0 ? (bb.upper - bb.lower) / bb.middle : 0;
    if (Number.isFinite(width)) widths.push(width);
  }
  if (widths.length === 0) return 0.5;

  const current = widths[widths.length - 1];
  const below = widths.filter((width) => width <= current).length;
  return clamp(below / widths.length, 0, 1);
}

export function classifyMarketRegime(
  candles: readonly OHLCVCandle[],
  lookback = 100,
): RegimeSnapshot {
  const closes = candles.map((candle) => candle.close);
  const latest = candles[candles.length - 1];
  const price = latest?.close ?? 0;

  if (candles.length < 20 || price <= 0) {
    return {
      regime: "RANGING",
      confidence: 20,
      adx: 0,
      atrPct: 0,
      bbWidthPct: 0,
      bbWidthPercentile: 0.5,
      emaSlope: 0,
      realizedVol: 0,
      timestamp: latest?.time ?? Date.now(),
    };
  }

  const adx = calcAdx([...candles], 14);
  const atr = calcAtr([...candles], 14);
  const atrPct = safePct(atr, price);
  const bb = calcBollinger(closes, 20, 2);
  const bbWidthPct = safePct(bb.upper - bb.lower, bb.middle);
  const bbWidthPercentile = bollingerWidthPercentile(closes, lookback);
  const emaSlope = calcEmaSlope(closes, 50, 10);
  const realizedVol = realizedVolatility(closes);

  let regime: MarketRegime;
  let confidence: number;

  if (adx > 25 && Math.abs(emaSlope) > 0.001) {
    regime = "TRENDING";
    confidence = clamp(adx * 2);
  } else if (bbWidthPercentile < 0.2 && atrPct < 0.15) {
    regime = "LOW_VOLATILITY_CHOP";
    confidence = clamp((1 - bbWidthPercentile) * 100);
  } else if (bbWidthPercentile > 0.8 && atrPct > 0.3) {
    regime = "HIGH_VOLATILITY_BREAKOUT";
    confidence = clamp(bbWidthPercentile * 100);
  } else {
    regime = "RANGING";
    confidence = clamp(Math.max(20, 100 - adx * 2));
  }

  return {
    regime,
    confidence,
    adx,
    atrPct,
    bbWidthPct,
    bbWidthPercentile,
    emaSlope,
    realizedVol,
    timestamp: latest.time,
  };
}
