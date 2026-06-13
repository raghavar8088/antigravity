/**
 * Institutional-Grade BTC Futures Strategy Modules
 *
 * 10 high-priority research strategies for the Mock Trading Research Engine.
 * IDs 2100–2119 (LONG = even, SHORT = odd per pair).
 *
 * ISOLATION: Mock/research only. No broker, OMS, or exchange calls.
 * All signals route exclusively to the mock trading engine.
 *
 * Tiers:
 *   Tier 1 (highest expected edge): VWAP Trend Pullback Pro, Liquidity Sweep Reversal,
 *           BB-Keltner Squeeze, EMA Pullback Rider
 *   Tier 2: Volume Spike Momentum, ATR Compression Breakout, RSI-VWAP Rubber Band
 *   Tier 3: Bollinger Rejection Fade, MACD Momentum Deceleration, Liquidation Cascade
 */

import {
  NO_SIGNAL,
  calcAdx,
  calcAtr,
  calcBollinger,
  calcEma,
  calcEmaSlope,
  calcKeltner,
  calcMacd,
  calcRsi,
  calcSma,
  calcVolumeRatio,
  calcVwap,
  clampConf,
  isBullishPinBar,
  isBearishPinBar,
  type OHLCVCandle,
  type ResearchSignal,
} from "@/lib/ai/mockResearchIndicators";
import type { MarketRegime } from "@/lib/ai/marketRegimeClassifier";
import type { BtcRequiredData } from "@/lib/trading/btcResearchStrategyRegistry";

// ── Fee model ─────────────────────────────────────────────────────────────────
/** 0.05% taker + 5 bps slippage per side = ~0.20% round-trip cost. */
const ROUND_TRIP_COST_PCT = 0.0020;

// ── Institutional family labels ───────────────────────────────────────────────
export type InstitutionalFamily =
  | "VwapTrendPullback"
  | "LiquiditySweepReversal"
  | "BBKeltnerSqueeze"
  | "EmaPullbackRider"
  | "VolumeSpikeM"
  | "AtrCompressionBreakout"
  | "RsiVwapRubberBand"
  | "BollingerRejectionFade"
  | "MacdMomentumDecel"
  | "LiquidationCascadeSnap";

export const INSTITUTIONAL_FAMILY_LABELS: Record<InstitutionalFamily, string> = {
  VwapTrendPullback:       "VWAP Trend Pullback Pro",
  LiquiditySweepReversal:  "Liquidity Sweep Reversal",
  BBKeltnerSqueeze:        "BB-Keltner Squeeze Expansion",
  EmaPullbackRider:        "EMA Pullback Rider",
  VolumeSpikeM:            "Volume Spike Momentum",
  AtrCompressionBreakout:  "ATR Compression Breakout",
  RsiVwapRubberBand:       "RSI-VWAP Rubber Band",
  BollingerRejectionFade:  "Bollinger Rejection Fade",
  MacdMomentumDecel:       "MACD Momentum Deceleration",
  LiquidationCascadeSnap:  "Liquidation Cascade Snap-Back",
};

export const ALL_INSTITUTIONAL_FAMILIES = Object.keys(
  INSTITUTIONAL_FAMILY_LABELS,
) as InstitutionalFamily[];

// ── Extended strategy interface ───────────────────────────────────────────────

export interface InstitutionalStrategyMeta {
  /** Tier 1–3: priority ranking. */
  tier: 1 | 2 | 3;
  /** Intended evaluation timeframe. */
  timeframes: string[];
  /** TP as pct of notional (e.g. 1.5 = 1.5%). */
  tpPct: number;
  /** SL as pct of notional (e.g. 0.75 = 0.75%). */
  slPct: number;
  /** Trailing stop distance in pct from entry. */
  trailingStopPct: number;
  /** Human-readable trailing stop logic. */
  trailingStopLogic: string;
  /** Static research confidence 0–100. */
  confidenceScore: number;
  /** Typical signal quality score 0–100. */
  signalScore: number;
  /** Risk score 0–100 (lower = safer). */
  riskScore: number;
  /** Estimated historical win rate 0–1. */
  estimatedWinRate: number;
  /** Expected profit per trade after fees (as fraction of notional, e.g. 0.013 = 1.3%). */
  expectedProfitAfterFees: number;
  /** Expected loss per trade after fees (negative fraction, e.g. -0.0095 = -0.95%). */
  expectedLossAfterFees: number;
  /** Estimated net expectancy = winRate × profit + (1-winRate) × loss. */
  netExpectancyEstimate: number;
  /** Target regimes where the strategy performs best. */
  bestRegimes: MarketRegime[];
  /** Regimes to avoid. */
  worstRegimes: MarketRegime[];
  /** Returns current indicator snapshot given candle history. */
  indicatorSnapshot: (candles: OHLCVCandle[]) => Record<string, number>;
}

/** Full institutional strategy definition — compatible with BtcResearchStrategy duck typing. */
export interface InstitutionalStrategy {
  id: number;
  name: string;
  family: InstitutionalFamily;
  enabled: true;
  description: string;
  params: Record<string, number | string>;
  timeframe: "1m" | "5m" | "15m";
  minCandles: number;
  signal: (candles: OHLCVCandle[]) => ResearchSignal;
  entryRules: string[];
  exitRules: string[];
  stopLossRules: string[];
  takeProfitRules: string[];
  requiredIndicators: string[];
  requiredData: BtcRequiredData[];
  bestRegime: MarketRegime[];
  worstRegime: MarketRegime[];
  researchConfidenceScore: number;
  sourceDocument: string;
  dataFeedRequired: false;
  side: "LONG" | "SHORT" | "BOTH";
  /** Extended metadata not present in the base registry type. */
  meta: InstitutionalStrategyMeta;
}

// ── Internal helpers ──────────────────────────────────────────────────────────

function closes(candles: readonly OHLCVCandle[]): number[] {
  return candles.map((c) => c.close);
}

function latest(candles: readonly OHLCVCandle[]): OHLCVCandle {
  return candles[candles.length - 1]!;
}

function swingHigh(candles: readonly OHLCVCandle[], period: number): number {
  const slice = candles.slice(-period - 1, -1);
  return slice.length > 0 ? Math.max(...slice.map((c) => c.high)) : 0;
}

function swingLow(candles: readonly OHLCVCandle[], period: number): number {
  const slice = candles.slice(-period - 1, -1);
  return slice.length > 0 ? Math.min(...slice.map((c) => c.low)) : Infinity;
}

function computeExpectancy(
  tpPct: number,
  slPct: number,
  winRate: number,
): Pick<InstitutionalStrategyMeta, "expectedProfitAfterFees" | "expectedLossAfterFees" | "netExpectancyEstimate"> {
  const expectedProfitAfterFees = tpPct / 100 - ROUND_TRIP_COST_PCT;
  const expectedLossAfterFees = -(slPct / 100 + ROUND_TRIP_COST_PCT);
  const netExpectancyEstimate =
    expectedProfitAfterFees * winRate + expectedLossAfterFees * (1 - winRate);
  return { expectedProfitAfterFees, expectedLossAfterFees, netExpectancyEstimate };
}

// ── Strategy 1: VWAP Trend Pullback Pro ──────────────────────────────────────
// Tier 1 — Trending markets.
// Entry: price above VWAP, HH structure intact, pullback touches VWAP, bullish
//        rejection candle, volume above average.

function vwapTrendPullbackSignal(side: "BUY" | "SELL") {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < 30) return NO_SIGNAL;
    const cl = closes(candles);
    const curr = latest(candles);
    const vwap = calcVwap(candles);
    const adx = calcAdx(candles, 14);
    const emaSlope = calcEmaSlope(cl, 50, 10);
    const vr = calcVolumeRatio(candles, 20);

    // Higher-high structure: last 3 closes trending in direction
    const c1 = cl[cl.length - 3];
    const c2 = cl[cl.length - 2];
    const c3 = cl[cl.length - 1];

    if (side === "BUY") {
      const trend = adx > 20 && emaSlope > 0;
      const higherHighs = c2 > c1 && (c3 > c2 * 0.998 || c3 < c2 * 1.002); // structure not broken
      const touchedVwap = curr.low <= vwap * 1.0015 && curr.high >= vwap * 0.9985;
      const rejectionCandle = curr.close > curr.open && curr.close > vwap;
      const volOk = vr >= 1.2;
      if (trend && higherHighs && touchedVwap && rejectionCandle && volOk) {
        const conf = clampConf(58 + adx * 0.6 + vr * 5);
        return { side: "BUY", confidence: conf };
      }
    } else {
      const trend = adx > 20 && emaSlope < 0;
      const lowerLows = c2 < c1 && (c3 < c2 * 1.002 || c3 > c2 * 0.998);
      const touchedVwap = curr.high >= vwap * 0.9985 && curr.low <= vwap * 1.0015;
      const rejectionCandle = curr.close < curr.open && curr.close < vwap;
      const volOk = vr >= 1.2;
      if (trend && lowerLows && touchedVwap && rejectionCandle && volOk) {
        const conf = clampConf(58 + adx * 0.6 + vr * 5);
        return { side: "SELL", confidence: conf };
      }
    }
    return NO_SIGNAL;
  };
}

// ── Strategy 2: Liquidity Sweep Reversal ─────────────────────────────────────
// Tier 1 — Range/stop-hunt environment.
// Entry: sweep of recent swing H/L, wick rejection, close back inside, vol spike.

function liquiditySweepSignal(side: "BUY" | "SELL") {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < 15) return NO_SIGNAL;
    const curr = latest(candles);
    const vr = calcVolumeRatio(candles, 20);
    const atr = calcAtr(candles, 14);

    if (side === "BUY") {
      const recentLow = swingLow(candles, 10);
      // Swept below then closed above — wick rejection
      const swept = curr.low < recentLow && curr.close > recentLow;
      const wickSize = recentLow - curr.low;
      const bigWick = wickSize > atr * 0.3; // meaningful sweep
      const volSpike = vr >= 1.5;
      if (swept && bigWick && volSpike) {
        const conf = clampConf(62 + vr * 6 + (wickSize / atr) * 8);
        return { side: "BUY", confidence: conf };
      }
    } else {
      const recentHigh = swingHigh(candles, 10);
      const swept = curr.high > recentHigh && curr.close < recentHigh;
      const wickSize = curr.high - recentHigh;
      const bigWick = wickSize > atr * 0.3;
      const volSpike = vr >= 1.5;
      if (swept && bigWick && volSpike) {
        const conf = clampConf(62 + vr * 6 + (wickSize / atr) * 8);
        return { side: "SELL", confidence: conf };
      }
    }
    return NO_SIGNAL;
  };
}

// ── Strategy 3: BB-Keltner Squeeze Expansion ─────────────────────────────────
// Tier 1 — Volatility expansion.
// Entry: BB inside Keltner (squeeze), breakout close outside Keltner + volume.

function bbKeltnerSqueezeSignal(side: "BUY" | "SELL") {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < 25) return NO_SIGNAL;
    const cl = closes(candles);
    const curr = latest(candles);
    const bb = calcBollinger(cl, 20, 2);
    const kelt = calcKeltner(candles, 20, 2);
    const vr = calcVolumeRatio(candles, 20);

    // Detect squeeze: BB entirely inside Keltner
    const inSqueeze = bb.upper < kelt.upper && bb.lower > kelt.lower;

    if (!inSqueeze) return NO_SIGNAL;

    if (side === "BUY") {
      const breakout = curr.close > kelt.upper;
      const volOk = vr >= 1.2;
      if (breakout && volOk) {
        const excess = (curr.close - kelt.upper) / Math.max(0.01, kelt.upper - kelt.lower);
        return { side: "BUY", confidence: clampConf(60 + excess * 200 + vr * 8) };
      }
    } else {
      const breakout = curr.close < kelt.lower;
      const volOk = vr >= 1.2;
      if (breakout && volOk) {
        const excess = (kelt.lower - curr.close) / Math.max(0.01, kelt.upper - kelt.lower);
        return { side: "SELL", confidence: clampConf(60 + excess * 200 + vr * 8) };
      }
    }
    return NO_SIGNAL;
  };
}

// ── Strategy 4: EMA Pullback Rider ───────────────────────────────────────────
// Tier 1 — Strong trend (1h filter, 5m execution).
// Entry: ADX > 25, pullback to EMA20 or EMA50, rejection candle.

function emaPullbackSignal(side: "BUY" | "SELL") {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < 55) return NO_SIGNAL;
    const cl = closes(candles);
    const curr = latest(candles);
    const adx = calcAdx(candles, 14);
    const ema20 = calcEma(cl, 20);
    const ema50 = calcEma(cl, 50);
    const emaSlope = calcEmaSlope(cl, 50, 10);
    const atr = calcAtr(candles, 14);

    if (adx < 25) return NO_SIGNAL;

    const tolerance = atr * 0.3;

    if (side === "BUY") {
      if (emaSlope <= 0) return NO_SIGNAL;
      // Price pulled back near EMA20 or EMA50
      const nearEma20 = Math.abs(curr.low - ema20) <= tolerance;
      const nearEma50 = Math.abs(curr.low - ema50) <= tolerance;
      if (!(nearEma20 || nearEma50)) return NO_SIGNAL;
      // Rejection: bullish pin bar or close above open
      const rejection =
        isBullishPinBar(curr) ||
        (curr.close > curr.open && curr.close > ema20);
      if (rejection) {
        const emaDist = nearEma50 ? 1.2 : 1.0; // EMA50 pullback is stronger
        return { side: "BUY", confidence: clampConf(60 + adx * 0.5 * emaDist) };
      }
    } else {
      if (emaSlope >= 0) return NO_SIGNAL;
      const nearEma20 = Math.abs(curr.high - ema20) <= tolerance;
      const nearEma50 = Math.abs(curr.high - ema50) <= tolerance;
      if (!(nearEma20 || nearEma50)) return NO_SIGNAL;
      const rejection =
        isBearishPinBar(curr) ||
        (curr.close < curr.open && curr.close < ema20);
      if (rejection) {
        const emaDist = nearEma50 ? 1.2 : 1.0;
        return { side: "SELL", confidence: clampConf(60 + adx * 0.5 * emaDist) };
      }
    }
    return NO_SIGNAL;
  };
}

// ── Strategy 5: Volume Spike Momentum ────────────────────────────────────────
// Tier 2 — News-driven moves.
// Entry: volume > 3× average, break of structure, momentum candle close.

function volumeSpikeSignal(side: "BUY" | "SELL") {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < 25) return NO_SIGNAL;
    const curr = latest(candles);
    const vr = calcVolumeRatio(candles, 20);
    if (vr < 3) return NO_SIGNAL;

    if (side === "BUY") {
      const structureBreak = curr.close > swingHigh(candles, 10);
      const momentumCandle = curr.close > curr.open && (curr.close - curr.open) > (curr.high - curr.low) * 0.5;
      if (structureBreak && momentumCandle) {
        return { side: "BUY", confidence: clampConf(55 + vr * 6) };
      }
    } else {
      const structureBreak = curr.close < swingLow(candles, 10);
      const momentumCandle = curr.close < curr.open && (curr.open - curr.close) > (curr.high - curr.low) * 0.5;
      if (structureBreak && momentumCandle) {
        return { side: "SELL", confidence: clampConf(55 + vr * 6) };
      }
    }
    return NO_SIGNAL;
  };
}

// ── Strategy 6: ATR Compression Breakout ─────────────────────────────────────
// Tier 2 — Pre-breakout conditions.
// Entry: ATR compression (current ATR < 0.70× rolling avg ATR), range break.

function atrCompressionSignal(side: "BUY" | "SELL") {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < 40) return NO_SIGNAL;
    const curr = latest(candles);
    const atrNow = calcAtr(candles, 14);
    // Rolling avg ATR: ATR of earlier window
    const atrAvg = calcAtr(candles.slice(0, -10), 14);
    if (atrAvg <= 0) return NO_SIGNAL;
    const compressed = atrNow < atrAvg * 0.70;
    if (!compressed) return NO_SIGNAL;

    const rangeHigh = swingHigh(candles, 20);
    const rangeLow  = swingLow(candles, 20);

    if (side === "BUY") {
      if (curr.close > rangeHigh) {
        const expansionFactor = atrNow > 0 ? (curr.close - rangeHigh) / atrNow : 0;
        return { side: "BUY", confidence: clampConf(58 + expansionFactor * 20 + (1 - atrNow / atrAvg) * 20) };
      }
    } else {
      if (curr.close < rangeLow) {
        const expansionFactor = atrNow > 0 ? (rangeLow - curr.close) / atrNow : 0;
        return { side: "SELL", confidence: clampConf(58 + expansionFactor * 20 + (1 - atrNow / atrAvg) * 20) };
      }
    }
    return NO_SIGNAL;
  };
}

// ── Strategy 7: RSI-VWAP Rubber Band ─────────────────────────────────────────
// Tier 2 — Mean reversion.
// Entry: RSI extreme + large VWAP deviation + reclaim signal.

function rsiVwapRubberBandSignal(side: "BUY" | "SELL") {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < 20) return NO_SIGNAL;
    const cl = closes(candles);
    const curr = latest(candles);
    const rsi = calcRsi(cl, 14);
    const vwap = calcVwap(candles);
    const adx = calcAdx(candles, 14);
    // Only trade in choppy/ranging regimes (adx < 20)
    if (adx > 25) return NO_SIGNAL;
    const stretch = 0.015; // 1.5%

    if (side === "BUY") {
      const extremeRsi = rsi < 20;
      const stretched = curr.close < vwap * (1 - stretch);
      // Reclaim: close above the open (starting mean-reversion move)
      const reclaim = curr.close > curr.open;
      if (extremeRsi && stretched && reclaim) {
        const deviation = (vwap - curr.close) / vwap;
        return { side: "BUY", confidence: clampConf(65 + deviation * 500 + (20 - rsi) * 0.8) };
      }
    } else {
      const extremeRsi = rsi > 80;
      const stretched = curr.close > vwap * (1 + stretch);
      const reclaim = curr.close < curr.open;
      if (extremeRsi && stretched && reclaim) {
        const deviation = (curr.close - vwap) / vwap;
        return { side: "SELL", confidence: clampConf(65 + deviation * 500 + (rsi - 80) * 0.8) };
      }
    }
    return NO_SIGNAL;
  };
}

// ── Strategy 8: Bollinger Rejection Fade ─────────────────────────────────────
// Tier 3 — Sideways markets.
// Entry: prior candle closes outside BB, current candle closes back inside.

function bollingerRejectionFadeSignal(side: "BUY" | "SELL") {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < 22) return NO_SIGNAL;
    const cl = closes(candles);
    const bb = calcBollinger(cl, 20, 2);
    const adx = calcAdx(candles, 14);
    if (adx > 30) return NO_SIGNAL; // band fades only in low-trend regimes

    const prev = candles[candles.length - 2];
    const curr = latest(candles);
    const prevBb = calcBollinger(cl.slice(0, -1), 20, 2);

    if (side === "BUY") {
      // Prior candle closed below BB lower; current closes back inside
      const prevOutside = prev.close < prevBb.lower;
      const currInside = curr.close >= bb.lower && curr.close <= bb.upper;
      if (prevOutside && currInside) {
        const penetration = (prevBb.lower - prev.close) / Math.max(0.01, prevBb.bandwidth);
        return { side: "BUY", confidence: clampConf(58 + penetration * 80 + (30 - adx) * 0.5) };
      }
    } else {
      const prevOutside = prev.close > prevBb.upper;
      const currInside = curr.close >= bb.lower && curr.close <= bb.upper;
      if (prevOutside && currInside) {
        const penetration = (prev.close - prevBb.upper) / Math.max(0.01, prevBb.bandwidth);
        return { side: "SELL", confidence: clampConf(58 + penetration * 80 + (30 - adx) * 0.5) };
      }
    }
    return NO_SIGNAL;
  };
}

// ── Strategy 9: MACD Momentum Deceleration ───────────────────────────────────
// Tier 3 — Trend exhaustion.
// Entry: MACD histogram shrinking from extreme, price at S/R confluence.

function macdMomentumDecelSignal(side: "BUY" | "SELL") {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < 40) return NO_SIGNAL;
    const cl = closes(candles);
    const curr = latest(candles);
    const macdNow = calcMacd(cl, 12, 26, 9);
    const macdPrev = calcMacd(cl.slice(0, -1), 12, 26, 9);
    const macdPrev2 = calcMacd(cl.slice(0, -2), 12, 26, 9);
    const rsi = calcRsi(cl, 14);

    // Deceleration: histogram magnitude shrinking for 2 consecutive bars
    const histShrinking =
      Math.abs(macdNow.histogram) < Math.abs(macdPrev.histogram) &&
      Math.abs(macdPrev.histogram) < Math.abs(macdPrev2.histogram);

    if (side === "BUY") {
      // Negative histogram decelerating (downtrend exhausting) near support
      const negDecelerating = macdPrev2.histogram < 0 && macdPrev.histogram < 0 && histShrinking;
      const nearSupport = curr.close <= calcEma(cl, 50) * 1.005; // within 0.5% of EMA50
      const rsiOk = rsi < 45;
      if (negDecelerating && nearSupport && rsiOk) {
        const decel = 1 - Math.abs(macdNow.histogram) / Math.max(0.001, Math.abs(macdPrev2.histogram));
        return { side: "BUY", confidence: clampConf(55 + decel * 30 + (45 - rsi) * 0.4) };
      }
    } else {
      const posDecelerating = macdPrev2.histogram > 0 && macdPrev.histogram > 0 && histShrinking;
      const nearResistance = curr.close >= calcEma(cl, 50) * 0.995;
      const rsiOk = rsi > 55;
      if (posDecelerating && nearResistance && rsiOk) {
        const decel = 1 - Math.abs(macdNow.histogram) / Math.max(0.001, Math.abs(macdPrev2.histogram));
        return { side: "SELL", confidence: clampConf(55 + decel * 30 + (rsi - 55) * 0.4) };
      }
    }
    return NO_SIGNAL;
  };
}

// ── Strategy 10: Liquidation Cascade Snap-Back ───────────────────────────────
// Tier 3 — Panic markets.
// Entry: extreme liquidation wick (> 2.5× ATR), RSI extreme, volume climax, snap-back close.

function liquidationCascadeSignal(side: "BUY" | "SELL") {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < 20) return NO_SIGNAL;
    const cl = closes(candles);
    const curr = latest(candles);
    const atr = calcAtr(candles, 14);
    const rsi = calcRsi(cl, 7); // fast RSI for panic detection
    const vr = calcVolumeRatio(candles, 20);
    if (atr <= 0) return NO_SIGNAL;

    if (side === "BUY") {
      // Lower wick is the liquidation wick
      const lowerWick = Math.min(curr.open, curr.close) - curr.low;
      const extremeWick = lowerWick > atr * 2.5;
      const rsiExtreme = rsi < 20;
      const volumeClimax = vr >= 3;
      const snapBack = curr.close > curr.open; // buyers absorbed
      if (extremeWick && rsiExtreme && volumeClimax && snapBack) {
        const wickRatio = lowerWick / atr;
        return { side: "BUY", confidence: clampConf(60 + wickRatio * 5 + vr * 4 + (20 - rsi) * 0.6) };
      }
    } else {
      const upperWick = curr.high - Math.max(curr.open, curr.close);
      const extremeWick = upperWick > atr * 2.5;
      const rsiExtreme = rsi > 80;
      const volumeClimax = vr >= 3;
      const snapBack = curr.close < curr.open;
      if (extremeWick && rsiExtreme && volumeClimax && snapBack) {
        const wickRatio = upperWick / atr;
        return { side: "SELL", confidence: clampConf(60 + wickRatio * 5 + vr * 4 + (rsi - 80) * 0.6) };
      }
    }
    return NO_SIGNAL;
  };
}

// ── Indicator snapshots ───────────────────────────────────────────────────────

function vwapTrendSnapshot(candles: OHLCVCandle[]): Record<string, number> {
  const cl = closes(candles);
  return {
    vwap: calcVwap(candles),
    adx: calcAdx(candles, 14),
    ema50slope: calcEmaSlope(cl, 50, 10),
    volumeRatio: calcVolumeRatio(candles, 20),
    close: cl[cl.length - 1] ?? 0,
  };
}

function sweepSnapshot(candles: OHLCVCandle[]): Record<string, number> {
  return {
    atr: calcAtr(candles, 14),
    volumeRatio: calcVolumeRatio(candles, 20),
    swingHigh10: swingHigh(candles, 10),
    swingLow10: swingLow(candles, 10),
    close: candles[candles.length - 1]?.close ?? 0,
  };
}

function bbKeltnerSnapshot(candles: OHLCVCandle[]): Record<string, number> {
  const cl = closes(candles);
  const bb = calcBollinger(cl, 20, 2);
  const kelt = calcKeltner(candles, 20, 2);
  const squeeze = bb.upper < kelt.upper && bb.lower > kelt.lower ? 1 : 0;
  return {
    bbUpper: bb.upper, bbLower: bb.lower,
    keltUpper: kelt.upper, keltLower: kelt.lower,
    squeeze,
    volumeRatio: calcVolumeRatio(candles, 20),
    bandwidth: bb.bandwidth,
  };
}

function emaPullbackSnapshot(candles: OHLCVCandle[]): Record<string, number> {
  const cl = closes(candles);
  return {
    ema20: calcEma(cl, 20),
    ema50: calcEma(cl, 50),
    adx: calcAdx(candles, 14),
    atr: calcAtr(candles, 14),
    ema50slope: calcEmaSlope(cl, 50, 10),
    close: cl[cl.length - 1] ?? 0,
  };
}

function volSpikeSnapshot(candles: OHLCVCandle[]): Record<string, number> {
  return {
    volumeRatio: calcVolumeRatio(candles, 20),
    swingHigh10: swingHigh(candles, 10),
    swingLow10: swingLow(candles, 10),
    close: candles[candles.length - 1]?.close ?? 0,
  };
}

function atrCompressionSnapshot(candles: OHLCVCandle[]): Record<string, number> {
  const atrNow = calcAtr(candles, 14);
  const atrAvg = candles.length > 10 ? calcAtr(candles.slice(0, -10), 14) : atrNow;
  return {
    atrNow,
    atrAvg,
    atrRatio: atrAvg > 0 ? atrNow / atrAvg : 1,
    rangeHigh: swingHigh(candles, 20),
    rangeLow: swingLow(candles, 20),
    close: candles[candles.length - 1]?.close ?? 0,
  };
}

function rubberBandSnapshot(candles: OHLCVCandle[]): Record<string, number> {
  const cl = closes(candles);
  return {
    rsi14: calcRsi(cl, 14),
    vwap: calcVwap(candles),
    adx: calcAdx(candles, 14),
    close: cl[cl.length - 1] ?? 0,
  };
}

function bbFadeSnapshot(candles: OHLCVCandle[]): Record<string, number> {
  const cl = closes(candles);
  const bb = calcBollinger(cl, 20, 2);
  return {
    bbUpper: bb.upper, bbLower: bb.lower, bbMiddle: bb.middle,
    adx: calcAdx(candles, 14),
    percentB: bb.percentB,
    close: cl[cl.length - 1] ?? 0,
  };
}

function macdDecelSnapshot(candles: OHLCVCandle[]): Record<string, number> {
  const cl = closes(candles);
  const macd = calcMacd(cl, 12, 26, 9);
  return {
    macdLine: macd.line,
    macdSignal: macd.signal,
    macdHistogram: macd.histogram,
    rsi14: calcRsi(cl, 14),
    ema50: calcEma(cl, 50),
    close: cl[cl.length - 1] ?? 0,
  };
}

function liquidationSnapSnapshot(candles: OHLCVCandle[]): Record<string, number> {
  const cl = closes(candles);
  const curr = candles[candles.length - 1]!;
  const atr = calcAtr(candles, 14);
  const lowerWick = Math.min(curr.open, curr.close) - curr.low;
  const upperWick = curr.high - Math.max(curr.open, curr.close);
  return {
    rsi7: calcRsi(cl, 7),
    atr,
    lowerWick,
    upperWick,
    lowerWickAtrRatio: atr > 0 ? lowerWick / atr : 0,
    upperWickAtrRatio: atr > 0 ? upperWick / atr : 0,
    volumeRatio: calcVolumeRatio(candles, 20),
    close: cl[cl.length - 1] ?? 0,
  };
}

// ── Strategy registry ─────────────────────────────────────────────────────────

const SOURCE = "BTC Institutional Research Strategy Modules 2026";

function makeStrategy(
  id: number,
  name: string,
  family: InstitutionalFamily,
  side: "LONG" | "SHORT",
  tier: 1 | 2 | 3,
  timeframe: "1m" | "5m" | "15m",
  minCandles: number,
  description: string,
  params: Record<string, number | string>,
  entryRules: string[],
  exitRules: string[],
  stopLossRules: string[],
  takeProfitRules: string[],
  requiredIndicators: string[],
  bestRegimes: MarketRegime[],
  worstRegimes: MarketRegime[],
  researchConf: number,
  tpPct: number,
  slPct: number,
  trailingStopPct: number,
  trailingLogic: string,
  confidenceScore: number,
  signalScore: number,
  riskScore: number,
  estimatedWinRate: number,
  signalFn: (side: "BUY" | "SELL") => (c: OHLCVCandle[]) => ResearchSignal,
  snapshotFn: (c: OHLCVCandle[]) => Record<string, number>,
): InstitutionalStrategy {
  const bSide = side === "LONG" ? "BUY" : "SELL";
  const exp = computeExpectancy(tpPct, slPct, estimatedWinRate);
  return {
    id,
    name,
    family,
    enabled: true,
    description,
    params: { ...params, side: bSide },
    timeframe,
    minCandles,
    signal: signalFn(bSide),
    entryRules,
    exitRules,
    stopLossRules,
    takeProfitRules,
    requiredIndicators,
    requiredData: ["OHLCV", "VOLUME"],
    bestRegime: bestRegimes,
    worstRegime: worstRegimes,
    researchConfidenceScore: researchConf,
    sourceDocument: SOURCE,
    dataFeedRequired: false,
    side,
    meta: {
      tier,
      timeframes: [timeframe],
      tpPct,
      slPct,
      trailingStopPct,
      trailingStopLogic: trailingLogic,
      confidenceScore,
      signalScore,
      riskScore,
      estimatedWinRate,
      ...exp,
      bestRegimes,
      worstRegimes,
      indicatorSnapshot: snapshotFn,
    },
  };
}

export const INSTITUTIONAL_STRATEGIES: InstitutionalStrategy[] = [
  // ── 1. VWAP Trend Pullback Pro ───────────────────────────────────────────
  makeStrategy(
    2100, "VWAP_TrendPullback_Long", "VwapTrendPullback", "LONG",
    1, "5m", 30,
    "VWAP pullback in uptrend — HH structure, touch, rejection candle, volume spike",
    { vwap_period: "session", adx_min: 20, vol_min: 1.2 },
    [
      "Price above session VWAP",
      "Higher-high structure intact (last 3 closes)",
      "Pullback candle low touches VWAP ±0.15%",
      "Bullish rejection close above VWAP",
      "Volume ≥ 1.2× 20-bar average",
    ],
    ["Exit at 2R target", "Exit on VWAP structure failure", "Previous swing high"],
    ["SL at VWAP level (failed structure)"],
    ["TP1 at 1R (50% position)", "TP2 at 2R or swing high"],
    ["VWAP", "ADX", "EMA50 Slope", "Volume Ratio"],
    ["TRENDING"],
    ["RANGING", "LOW_VOLATILITY_CHOP"],
    78, 1.5, 0.75, 0.5,
    "Trail SL to VWAP as price moves in favour; widen by ATR×0.5 on each new HH",
    78, 72, 35, 0.58,
    vwapTrendPullbackSignal, vwapTrendSnapshot,
  ),
  makeStrategy(
    2101, "VWAP_TrendPullback_Short", "VwapTrendPullback", "SHORT",
    1, "5m", 30,
    "VWAP pullback in downtrend — LL structure, touch, rejection candle, volume spike",
    { vwap_period: "session", adx_min: 20, vol_min: 1.2 },
    [
      "Price below session VWAP",
      "Lower-low structure intact (last 3 closes)",
      "Pullback candle high touches VWAP ±0.15%",
      "Bearish rejection close below VWAP",
      "Volume ≥ 1.2× 20-bar average",
    ],
    ["Exit at 2R target", "Exit on VWAP structure failure", "Previous swing low"],
    ["SL at VWAP level (failed structure)"],
    ["TP1 at 1R", "TP2 at 2R or swing low"],
    ["VWAP", "ADX", "EMA50 Slope", "Volume Ratio"],
    ["TRENDING"],
    ["RANGING", "LOW_VOLATILITY_CHOP"],
    78, 1.5, 0.75, 0.5,
    "Trail SL to VWAP as price moves in favour; widen by ATR×0.5 on each new LL",
    75, 70, 38, 0.55,
    vwapTrendPullbackSignal, vwapTrendSnapshot,
  ),

  // ── 2. Liquidity Sweep Reversal ───────────────────────────────────────────
  makeStrategy(
    2102, "LiquiditySweep_Reversal_Long", "LiquiditySweepReversal", "LONG",
    1, "5m", 15,
    "Bullish reversal after sweeping swing low — wick rejection + volume spike",
    { lookback: 10, min_wick_atr: 0.3, vol_min: 1.5 },
    [
      "Current low breaks below 10-bar swing low",
      "Candle closes back above swept low",
      "Lower wick ≥ 0.3× ATR(14)",
      "Volume ≥ 1.5× 20-bar average",
    ],
    ["Exit at mid-range of prior consolidation", "Exit at opposite liquidity zone"],
    ["SL at 0.5% below entry (below swept wick)"],
    ["TP at range midpoint or 1.5R"],
    ["ATR", "Volume Ratio", "Swing High/Low"],
    ["RANGING"],
    ["TRENDING"],
    76, 1.2, 0.5, 0.4,
    "Move SL to breakeven once 0.5× TP is reached; no further trail",
    76, 70, 40, 0.62,
    liquiditySweepSignal, sweepSnapshot,
  ),
  makeStrategy(
    2103, "LiquiditySweep_Reversal_Short", "LiquiditySweepReversal", "SHORT",
    1, "5m", 15,
    "Bearish reversal after sweeping swing high — wick rejection + volume spike",
    { lookback: 10, min_wick_atr: 0.3, vol_min: 1.5 },
    [
      "Current high breaks above 10-bar swing high",
      "Candle closes back below swept high",
      "Upper wick ≥ 0.3× ATR(14)",
      "Volume ≥ 1.5× 20-bar average",
    ],
    ["Exit at mid-range of prior consolidation", "Exit at opposite liquidity zone"],
    ["SL at 0.5% above entry (above swept wick)"],
    ["TP at range midpoint or 1.5R"],
    ["ATR", "Volume Ratio", "Swing High/Low"],
    ["RANGING"],
    ["TRENDING"],
    76, 1.2, 0.5, 0.4,
    "Move SL to breakeven once 0.5× TP is reached; no further trail",
    73, 68, 42, 0.60,
    liquiditySweepSignal, sweepSnapshot,
  ),

  // ── 3. BB-Keltner Squeeze ─────────────────────────────────────────────────
  makeStrategy(
    2104, "BBKeltner_Squeeze_Long", "BBKeltnerSqueeze", "LONG",
    1, "5m", 25,
    "Bollinger squeeze breakout above Keltner channel upper band with volume",
    { bb_period: 20, bb_mult: 2, kelt_period: 20, kelt_mult: 2, vol_min: 1.2 },
    [
      "BB(20,2) entirely inside Keltner(20,2) — volatility compressed",
      "Breakout close above Keltner upper band",
      "Volume ≥ 1.2× 20-bar average",
    ],
    ["Exit at 2R", "Exit on ATR trailing stop triggered"],
    ["SL at ATR trailing stop from breakout candle close"],
    ["TP1 at 1× ATR, TP2 at 2× ATR from entry"],
    ["Bollinger Bands", "Keltner Channel", "ATR", "Volume Ratio"],
    ["HIGH_VOLATILITY_BREAKOUT"],
    ["LOW_VOLATILITY_CHOP", "RANGING"],
    80, 2.0, 1.0, 0.8,
    "ATR trailing stop — move SL up by ATR×1.0 every time price advances ATR×0.5",
    80, 75, 45, 0.54,
    bbKeltnerSqueezeSignal, bbKeltnerSnapshot,
  ),
  makeStrategy(
    2105, "BBKeltner_Squeeze_Short", "BBKeltnerSqueeze", "SHORT",
    1, "5m", 25,
    "Bollinger squeeze breakout below Keltner channel lower band with volume",
    { bb_period: 20, bb_mult: 2, kelt_period: 20, kelt_mult: 2, vol_min: 1.2 },
    [
      "BB(20,2) entirely inside Keltner(20,2) — volatility compressed",
      "Breakout close below Keltner lower band",
      "Volume ≥ 1.2× 20-bar average",
    ],
    ["Exit at 2R", "Exit on ATR trailing stop triggered"],
    ["SL at ATR trailing stop from breakout candle close"],
    ["TP1 at 1× ATR, TP2 at 2× ATR from entry"],
    ["Bollinger Bands", "Keltner Channel", "ATR", "Volume Ratio"],
    ["HIGH_VOLATILITY_BREAKOUT"],
    ["LOW_VOLATILITY_CHOP", "RANGING"],
    80, 2.0, 1.0, 0.8,
    "ATR trailing stop — move SL down by ATR×1.0 every time price declines ATR×0.5",
    77, 72, 48, 0.52,
    bbKeltnerSqueezeSignal, bbKeltnerSnapshot,
  ),

  // ── 4. EMA Pullback Rider ─────────────────────────────────────────────────
  makeStrategy(
    2106, "EMA_Pullback_Rider_Long", "EmaPullbackRider", "LONG",
    1, "5m", 55,
    "Pullback to EMA20/EMA50 in strong ADX uptrend with bullish rejection candle",
    { ema_fast: 20, ema_slow: 50, adx_min: 25 },
    [
      "ADX(14) > 25 — strong uptrend",
      "Candle low within ATR×0.3 of EMA20 or EMA50",
      "Bullish pin bar or close above EMA20",
      "EMA50 slope positive",
    ],
    ["Exit at swing target", "Exit on trend failure (close below EMA50)"],
    ["SL at EMA50 (trend failure level)"],
    ["TP at previous swing high; partial at 1R"],
    ["EMA20", "EMA50", "ADX", "ATR"],
    ["TRENDING"],
    ["RANGING", "HIGH_VOLATILITY_BREAKOUT"],
    82, 2.0, 1.0, 0.6,
    "Trail SL to EMA20 once 1R profit reached; close on daily candle close below EMA50",
    82, 76, 32, 0.60,
    emaPullbackSignal, emaPullbackSnapshot,
  ),
  makeStrategy(
    2107, "EMA_Pullback_Rider_Short", "EmaPullbackRider", "SHORT",
    1, "5m", 55,
    "Pullback to EMA20/EMA50 in strong ADX downtrend with bearish rejection candle",
    { ema_fast: 20, ema_slow: 50, adx_min: 25 },
    [
      "ADX(14) > 25 — strong downtrend",
      "Candle high within ATR×0.3 of EMA20 or EMA50",
      "Bearish pin bar or close below EMA20",
      "EMA50 slope negative",
    ],
    ["Exit at swing target", "Exit on trend failure (close above EMA50)"],
    ["SL at EMA50 (trend failure level)"],
    ["TP at previous swing low; partial at 1R"],
    ["EMA20", "EMA50", "ADX", "ATR"],
    ["TRENDING"],
    ["RANGING", "HIGH_VOLATILITY_BREAKOUT"],
    82, 2.0, 1.0, 0.6,
    "Trail SL to EMA20 once 1R profit reached; close on daily candle close above EMA50",
    79, 73, 35, 0.58,
    emaPullbackSignal, emaPullbackSnapshot,
  ),

  // ── 5. Volume Spike Momentum ──────────────────────────────────────────────
  makeStrategy(
    2108, "VolSpike_Momentum_Long", "VolumeSpikeM", "LONG",
    2, "1m", 25,
    "Volume > 3× average + structure break above swing high + momentum close",
    { vol_multiple: 3, lookback: 10 },
    [
      "Volume ≥ 3× 20-bar average",
      "Close breaks above 10-bar swing high",
      "Bullish momentum candle (body > 50% of range)",
    ],
    ["Exit at 1.5R", "Exit on momentum fade (close below prior candle low)"],
    ["SL at prior candle low"],
    ["TP at 1.5R"],
    ["Volume Ratio", "Swing High"],
    ["HIGH_VOLATILITY_BREAKOUT", "TRENDING"],
    ["LOW_VOLATILITY_CHOP"],
    72, 1.5, 0.75, 0.5,
    "No trail — fixed TP/SL for fast news-driven moves",
    72, 65, 55, 0.52,
    volumeSpikeSignal, volSpikeSnapshot,
  ),
  makeStrategy(
    2109, "VolSpike_Momentum_Short", "VolumeSpikeM", "SHORT",
    2, "1m", 25,
    "Volume > 3× average + structure break below swing low + momentum close",
    { vol_multiple: 3, lookback: 10 },
    [
      "Volume ≥ 3× 20-bar average",
      "Close breaks below 10-bar swing low",
      "Bearish momentum candle (body > 50% of range)",
    ],
    ["Exit at 1.5R", "Exit on momentum fade (close above prior candle high)"],
    ["SL at prior candle high"],
    ["TP at 1.5R"],
    ["Volume Ratio", "Swing Low"],
    ["HIGH_VOLATILITY_BREAKOUT", "TRENDING"],
    ["LOW_VOLATILITY_CHOP"],
    72, 1.5, 0.75, 0.5,
    "No trail — fixed TP/SL for fast news-driven moves",
    70, 63, 58, 0.50,
    volumeSpikeSignal, volSpikeSnapshot,
  ),

  // ── 6. ATR Compression Breakout ───────────────────────────────────────────
  makeStrategy(
    2110, "ATR_Compression_Breakout_Long", "AtrCompressionBreakout", "LONG",
    2, "5m", 40,
    "ATR compressed below 70% of rolling average — breakout above 20-bar range",
    { atr_period: 14, compression_threshold: 0.7, range_period: 20 },
    [
      "ATR(14) < 0.70× rolling ATR average (compression)",
      "Close breaks above 20-bar range high",
    ],
    ["Exit at 2R", "ATR trailing stop"],
    ["SL at ATR trail from entry"],
    ["TP1 at 1× ATR, TP2 at 2R"],
    ["ATR", "Donchian Channel"],
    ["HIGH_VOLATILITY_BREAKOUT"],
    ["LOW_VOLATILITY_CHOP"],
    74, 2.0, 1.0, 0.8,
    "ATR trailing stop: set to entry − 1× ATR; move up each time price reaches new high",
    74, 68, 50, 0.53,
    atrCompressionSignal, atrCompressionSnapshot,
  ),
  makeStrategy(
    2111, "ATR_Compression_Breakout_Short", "AtrCompressionBreakout", "SHORT",
    2, "5m", 40,
    "ATR compressed below 70% of rolling average — breakout below 20-bar range",
    { atr_period: 14, compression_threshold: 0.7, range_period: 20 },
    [
      "ATR(14) < 0.70× rolling ATR average (compression)",
      "Close breaks below 20-bar range low",
    ],
    ["Exit at 2R", "ATR trailing stop"],
    ["SL at ATR trail from entry"],
    ["TP1 at 1× ATR, TP2 at 2R"],
    ["ATR", "Donchian Channel"],
    ["HIGH_VOLATILITY_BREAKOUT"],
    ["LOW_VOLATILITY_CHOP"],
    74, 2.0, 1.0, 0.8,
    "ATR trailing stop: set to entry + 1× ATR; move down each time price reaches new low",
    71, 66, 52, 0.51,
    atrCompressionSignal, atrCompressionSnapshot,
  ),

  // ── 7. RSI-VWAP Rubber Band ───────────────────────────────────────────────
  makeStrategy(
    2112, "RSI_VWAP_RubberBand_Long", "RsiVwapRubberBand", "LONG",
    2, "5m", 20,
    "Mean reversion: RSI < 20 + price > 1.5% below VWAP in low-ADX environment",
    { rsi_period: 14, vwap_stretch_pct: 1.5, adx_max: 25 },
    [
      "RSI(14) < 20 (oversold extreme)",
      "Close > 1.5% below session VWAP",
      "ADX(14) < 25 (choppy/ranging)",
      "Close > Open (starting to reclaim)",
    ],
    ["Exit at VWAP touch", "Exit at +0.5× VWAP deviation"],
    ["SL at 0.75% below entry"],
    ["TP at VWAP level"],
    ["RSI", "VWAP", "ADX"],
    ["RANGING", "LOW_VOLATILITY_CHOP"],
    ["TRENDING", "HIGH_VOLATILITY_BREAKOUT"],
    70, 1.5, 0.75, 0.5,
    "Move SL to breakeven once 50% of distance to VWAP is covered",
    70, 65, 45, 0.58,
    rsiVwapRubberBandSignal, rubberBandSnapshot,
  ),
  makeStrategy(
    2113, "RSI_VWAP_RubberBand_Short", "RsiVwapRubberBand", "SHORT",
    2, "5m", 20,
    "Mean reversion: RSI > 80 + price > 1.5% above VWAP in low-ADX environment",
    { rsi_period: 14, vwap_stretch_pct: 1.5, adx_max: 25 },
    [
      "RSI(14) > 80 (overbought extreme)",
      "Close > 1.5% above session VWAP",
      "ADX(14) < 25 (choppy/ranging)",
      "Close < Open (starting to reclaim downward)",
    ],
    ["Exit at VWAP touch", "Exit at −0.5× VWAP deviation"],
    ["SL at 0.75% above entry"],
    ["TP at VWAP level"],
    ["RSI", "VWAP", "ADX"],
    ["RANGING", "LOW_VOLATILITY_CHOP"],
    ["TRENDING", "HIGH_VOLATILITY_BREAKOUT"],
    70, 1.5, 0.75, 0.5,
    "Move SL to breakeven once 50% of distance to VWAP is covered",
    67, 62, 48, 0.56,
    rsiVwapRubberBandSignal, rubberBandSnapshot,
  ),

  // ── 8. Bollinger Rejection Fade ───────────────────────────────────────────
  makeStrategy(
    2114, "BB_Rejection_Fade_Long", "BollingerRejectionFade", "LONG",
    3, "1m", 22,
    "Prior candle closed below BB lower; current closes back inside (mean reversion)",
    { bb_period: 20, bb_mult: 2, adx_max: 30 },
    [
      "Prior candle close < BB(20,2) lower band",
      "Current close returns inside BB",
      "ADX < 30 (sideways market)",
    ],
    ["Exit at BB midband", "Exit at opposite band"],
    ["SL at 0.5% below BB lower band"],
    ["TP at BB midband"],
    ["Bollinger Bands", "ADX"],
    ["RANGING", "LOW_VOLATILITY_CHOP"],
    ["TRENDING"],
    66, 1.0, 0.5, 0.4,
    "No trailing stop — fixed target at BB midband",
    66, 60, 42, 0.60,
    bollingerRejectionFadeSignal, bbFadeSnapshot,
  ),
  makeStrategy(
    2115, "BB_Rejection_Fade_Short", "BollingerRejectionFade", "SHORT",
    3, "1m", 22,
    "Prior candle closed above BB upper; current closes back inside (mean reversion)",
    { bb_period: 20, bb_mult: 2, adx_max: 30 },
    [
      "Prior candle close > BB(20,2) upper band",
      "Current close returns inside BB",
      "ADX < 30 (sideways market)",
    ],
    ["Exit at BB midband", "Exit at opposite band"],
    ["SL at 0.5% above BB upper band"],
    ["TP at BB midband"],
    ["Bollinger Bands", "ADX"],
    ["RANGING", "LOW_VOLATILITY_CHOP"],
    ["TRENDING"],
    66, 1.0, 0.5, 0.4,
    "No trailing stop — fixed target at BB midband",
    63, 58, 44, 0.58,
    bollingerRejectionFadeSignal, bbFadeSnapshot,
  ),

  // ── 9. MACD Momentum Deceleration ────────────────────────────────────────
  makeStrategy(
    2116, "MACD_Decel_Long", "MacdMomentumDecel", "LONG",
    3, "5m", 40,
    "MACD histogram decelerating from negative extreme near EMA50 support (RSI < 45)",
    { macd_fast: 12, macd_slow: 26, macd_signal: 9 },
    [
      "MACD histogram negative and shrinking for 2 consecutive bars",
      "Price within 0.5% of EMA50",
      "RSI(14) < 45",
    ],
    ["Exit on MACD histogram sign change to positive", "Exit at 1.5R"],
    ["SL at 0.75% below entry"],
    ["TP at MACD reversal (histogram crosses zero)"],
    ["MACD", "EMA50", "RSI"],
    ["TRENDING", "RANGING"],
    ["HIGH_VOLATILITY_BREAKOUT"],
    65, 1.5, 0.75, 0.6,
    "Move SL to breakeven when MACD crosses signal line",
    65, 60, 50, 0.55,
    macdMomentumDecelSignal, macdDecelSnapshot,
  ),
  makeStrategy(
    2117, "MACD_Decel_Short", "MacdMomentumDecel", "SHORT",
    3, "5m", 40,
    "MACD histogram decelerating from positive extreme near EMA50 resistance (RSI > 55)",
    { macd_fast: 12, macd_slow: 26, macd_signal: 9 },
    [
      "MACD histogram positive and shrinking for 2 consecutive bars",
      "Price within 0.5% of EMA50",
      "RSI(14) > 55",
    ],
    ["Exit on MACD histogram sign change to negative", "Exit at 1.5R"],
    ["SL at 0.75% above entry"],
    ["TP at MACD reversal (histogram crosses zero)"],
    ["MACD", "EMA50", "RSI"],
    ["TRENDING", "RANGING"],
    ["HIGH_VOLATILITY_BREAKOUT"],
    65, 1.5, 0.75, 0.6,
    "Move SL to breakeven when MACD crosses signal line",
    62, 58, 52, 0.53,
    macdMomentumDecelSignal, macdDecelSnapshot,
  ),

  // ── 10. Liquidation Cascade Snap-Back ─────────────────────────────────────
  makeStrategy(
    2118, "Liquidation_CascadeSnap_Long", "LiquidationCascadeSnap", "LONG",
    3, "1m", 20,
    "Panic sell wick > 2.5× ATR, RSI(7) < 20, volume climax — snap-back absorption",
    { wick_atr_multiple: 2.5, rsi_period: 7, rsi_max: 20, vol_min: 3 },
    [
      "Lower wick > 2.5× ATR(14) — extreme liquidation wick",
      "RSI(7) < 20 — panic oversold",
      "Volume ≥ 3× 20-bar average — climactic selling",
      "Close > Open — buyers absorbed the sell pressure",
    ],
    ["Exit at mean reversion target (±1× ATR from entry)", "Exit at VWAP"],
    ["SL at wick low"],
    ["TP at 1.5× lower wick size above entry"],
    ["ATR", "RSI(7)", "Volume Ratio"],
    ["HIGH_VOLATILITY_BREAKOUT"],
    ["LOW_VOLATILITY_CHOP", "RANGING"],
    62, 1.5, 0.75, 0.5,
    "No trailing stop — snap-back trades are fast; exit at mean reversion target or time stop",
    62, 58, 60, 0.55,
    liquidationCascadeSignal, liquidationSnapSnapshot,
  ),
  makeStrategy(
    2119, "Liquidation_CascadeSnap_Short", "LiquidationCascadeSnap", "SHORT",
    3, "1m", 20,
    "Panic buy wick > 2.5× ATR, RSI(7) > 80, volume climax — snap-back absorption",
    { wick_atr_multiple: 2.5, rsi_period: 7, rsi_min: 80, vol_min: 3 },
    [
      "Upper wick > 2.5× ATR(14) — extreme liquidation wick",
      "RSI(7) > 80 — panic overbought",
      "Volume ≥ 3× 20-bar average — climactic buying",
      "Close < Open — sellers absorbed the buy pressure",
    ],
    ["Exit at mean reversion target (±1× ATR from entry)", "Exit at VWAP"],
    ["SL at wick high"],
    ["TP at 1.5× upper wick size below entry"],
    ["ATR", "RSI(7)", "Volume Ratio"],
    ["HIGH_VOLATILITY_BREAKOUT"],
    ["LOW_VOLATILITY_CHOP", "RANGING"],
    62, 1.5, 0.75, 0.5,
    "No trailing stop — snap-back trades are fast; exit at mean reversion target or time stop",
    60, 56, 62, 0.53,
    liquidationCascadeSignal, liquidationSnapSnapshot,
  ),
];

// ── Public exports ─────────────────────────────────────────────────────────────

/** O(1) lookup by strategy ID. */
export const INSTITUTIONAL_STRATEGY_BY_ID: ReadonlyMap<number, InstitutionalStrategy> = new Map(
  INSTITUTIONAL_STRATEGIES.map((s) => [s.id, s]),
);

/** All institutional strategy IDs (2100–2119). */
export const INSTITUTIONAL_STRATEGY_IDS: readonly number[] =
  INSTITUTIONAL_STRATEGIES.map((s) => s.id);

// Verify unique IDs and correct count at module load
const _ids = INSTITUTIONAL_STRATEGIES.map((s) => s.id);
if (new Set(_ids).size !== _ids.length || _ids.length !== 20) {
  throw new Error(
    `Institutional strategy registry must have exactly 20 unique IDs, got ${_ids.length} (${new Set(_ids).size} unique)`,
  );
}
