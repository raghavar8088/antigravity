import {
  calcAdx,
  calcAtr,
  calcBollinger,
  calcEmaSlope,
  type OHLCVCandle,
} from "@/lib/ai/mockResearchIndicators";

/**
 * Seven-regime taxonomy for institutional-grade regime detection.
 *
 * New values (v2):
 *   STRONG_TREND        — ADX > 30, clear directional momentum
 *   WEAK_TREND          — ADX 20–30, moderate directional bias
 *   RANGE               — ADX < 20, price oscillating in a defined band
 *   HIGH_VOLATILITY     — ATR well above 30-bar average, no clear direction
 *   LOW_VOLATILITY      — ATR compressed, Bollinger band squeeze
 *   BREAKOUT            — Expansion from a volatility squeeze with momentum confirmation
 *   REVERSAL            — Sharp directional change after extended trend (divergence detected)
 *
 * Legacy values kept in the union for backward compatibility with stored data:
 *   TRENDING              → reclassified as STRONG_TREND or WEAK_TREND
 *   RANGING               → reclassified as RANGE
 *   HIGH_VOLATILITY_BREAKOUT → reclassified as BREAKOUT or HIGH_VOLATILITY
 *   LOW_VOLATILITY_CHOP   → reclassified as LOW_VOLATILITY
 */
export type MarketRegime =
  // v2 — fine-grained (returned by updated classifier)
  | "STRONG_TREND"
  | "WEAK_TREND"
  | "RANGE"
  | "HIGH_VOLATILITY"
  | "LOW_VOLATILITY"
  | "BREAKOUT"
  | "REVERSAL"
  // v1 — legacy aliases kept for backward compat with stored trade records
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
  /** ATR relative to its 30-bar moving average (1.0 = neutral). */
  volRatio: number;
  /** Price rate-of-change over last 5 bars (for reversal detection). */
  roc5: number;
  timestamp: number;
}

/** Human-readable label for each regime. */
export const REGIME_LABELS: Record<MarketRegime, string> = {
  STRONG_TREND: "Strong Trend",
  WEAK_TREND: "Weak Trend",
  RANGE: "Range",
  HIGH_VOLATILITY: "High Volatility",
  LOW_VOLATILITY: "Low Volatility",
  BREAKOUT: "Breakout",
  REVERSAL: "Reversal",
  // legacy
  TRENDING: "Trending (legacy)",
  RANGING: "Ranging (legacy)",
  HIGH_VOLATILITY_BREAKOUT: "HV Breakout (legacy)",
  LOW_VOLATILITY_CHOP: "LV Chop (legacy)",
};

/** Regimes where trend-following strategies are expected to outperform. */
export const TREND_FAVOURABLE_REGIMES: ReadonlySet<MarketRegime> = new Set([
  "STRONG_TREND", "WEAK_TREND", "BREAKOUT", "TRENDING",
]);

/** Regimes where mean-reversion / range strategies are expected to outperform. */
export const RANGE_FAVOURABLE_REGIMES: ReadonlySet<MarketRegime> = new Set([
  "RANGE", "LOW_VOLATILITY", "RANGING", "LOW_VOLATILITY_CHOP",
]);

/** Regimes requiring extra caution (wider SL, lower sizing). */
export const HIGH_RISK_REGIMES: ReadonlySet<MarketRegime> = new Set([
  "HIGH_VOLATILITY", "BREAKOUT", "REVERSAL", "FLASH_CRASH" as MarketRegime,
  "HIGH_VOLATILITY_BREAKOUT",
]);

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
      regime: "RANGE",
      confidence: 20,
      adx: 0,
      atrPct: 0,
      bbWidthPct: 0,
      bbWidthPercentile: 0.5,
      emaSlope: 0,
      realizedVol: 0,
      volRatio: 1,
      roc5: 0,
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

  // Additional indicators for fine-grained regime detection
  const volRatio = _atrVolRatio(candles, 14, 30);
  const roc5 = _rateOfChange(closes, 5);
  const prevEmaSlope = closes.length >= 15 ? calcEmaSlope(closes.slice(0, -5), 50, 10) : emaSlope;
  const slopeReversal = Math.sign(emaSlope) !== Math.sign(prevEmaSlope) && Math.abs(emaSlope) > 0.0005;

  let regime: MarketRegime;
  let confidence: number;

  // ── Seven-regime decision tree ─────────────────────────────────────────────
  //
  // Priority order:
  //   1. BREAKOUT — squeeze then expansion (BB percentile crossing up from <30 → >70)
  //   2. REVERSAL — slope sign change after extended trend
  //   3. STRONG_TREND — ADX > 30, strong momentum
  //   4. WEAK_TREND — ADX 20–30, moderate bias
  //   5. HIGH_VOLATILITY — ATR >> average, no trend
  //   6. LOW_VOLATILITY — ATR << average, BB squeeze
  //   7. RANGE — default: oscillating, no clear direction

  if (bbWidthPercentile > 0.75 && volRatio > 1.5 && adx < 25) {
    // Volatility expansion from a compressed state without a clear trend → BREAKOUT
    regime = "BREAKOUT";
    confidence = clamp(bbWidthPercentile * 80 + (volRatio - 1) * 20);
  } else if (slopeReversal && adx > 18 && Math.abs(roc5) > 0.005) {
    // EMA slope reversed + momentum → REVERSAL
    regime = "REVERSAL";
    confidence = clamp(50 + Math.abs(roc5) * 5000);
  } else if (adx > 30 && Math.abs(emaSlope) > 0.002) {
    // Very strong directional trend
    regime = "STRONG_TREND";
    confidence = clamp(adx * 2.5);
  } else if (adx > 20 && Math.abs(emaSlope) > 0.0005) {
    // Moderate directional bias
    regime = "WEAK_TREND";
    confidence = clamp(adx * 2);
  } else if (volRatio > 2.0 || (bbWidthPercentile > 0.85 && atrPct > 0.4)) {
    // Extremely elevated volatility without trend
    regime = "HIGH_VOLATILITY";
    confidence = clamp(Math.min(volRatio * 30, 90));
  } else if (bbWidthPercentile < 0.2 && volRatio < 0.7 && atrPct < 0.15) {
    // Volatility compression / squeeze
    regime = "LOW_VOLATILITY";
    confidence = clamp((1 - bbWidthPercentile) * 100);
  } else {
    // Catch-all: sideways, oscillating, no volatility extreme
    regime = "RANGE";
    confidence = clamp(Math.max(30, 100 - adx * 2.5));
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
    volRatio,
    roc5,
    timestamp: latest.time,
  };
}

// ── Additional indicator helpers ──────────────────────────────────────────────

/** ATR(period) / ATR_MA(maLength) ratio — measures volatility regime vs recent average. */
function _atrVolRatio(candles: readonly OHLCVCandle[], period: number, maLength: number): number {
  if (candles.length < period + maLength) return 1;
  const currentAtr = calcAtr([...candles], period);
  // Compute rolling ATRs for the MA
  const atrSeries: number[] = [];
  for (let i = period; i <= candles.length; i++) {
    atrSeries.push(calcAtr([...candles.slice(0, i)], period));
  }
  const recent = atrSeries.slice(-maLength);
  const avgAtr = recent.reduce((s, v) => s + v, 0) / Math.max(1, recent.length);
  return avgAtr > 0 ? currentAtr / avgAtr : 1;
}

/** Rate-of-change: (close[last] - close[last-n]) / close[last-n]. */
function _rateOfChange(closes: readonly number[], n: number): number {
  const last = closes[closes.length - 1];
  const prev = closes[closes.length - 1 - n];
  if (!Number.isFinite(last) || !Number.isFinite(prev) || prev <= 0) return 0;
  return (last - prev) / prev;
}
