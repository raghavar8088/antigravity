/**
 * Mock Research Strategy Pack — 500 BTC intraday strategy variants.
 *
 * ISOLATION: This file is mock-only. It is never imported by production trading
 * code (broker, OMS, paper desk worker, or real signal pipeline). All exports
 * are consumed exclusively by useMockResearchRunner and its tests.
 *
 * ID allocation: 1000–1499 (no collision with CORE 91–152, premium 500–527,
 * or category research pool 600–759).
 *
 * Strategy count per family:
 *   TrendFollowing      1000-1029  30  (15 variants × 2 sides)
 *   EmaSmaCrossover     1030-1059  30
 *   VwapStrategies      1060-1089  30
 *   RsiMeanReversion    1090-1119  30
 *   BollingerBand       1120-1149  30
 *   Breakout            1150-1179  30
 *   LiquiditySweep      1180-1209  30
 *   MomentumContinuation 1210-1239 30
 *   Scalping            1240-1269  30
 *   VolumeSpike         1270-1289  20  (10 variants × 2)
 *   VolatilityExpansion 1290-1309  20
 *   AtrBased            1310-1339  30
 *   MacdStrategies      1340-1369  30
 *   StochasticStrategies 1370-1389 20
 *   CandlePattern       1390-1419  30
 *   MultiTimeframe      1420-1449  30
 *   SupportResistance   1450-1469  20
 *   RangeChop           1470-1499  30
 *
 *   Total: 14 × 30 + 4 × 20 = 500 ✓
 */

import {
  calcAdx,
  calcAtr,
  calcBollinger,
  calcDonchian,
  calcEma,
  calcMacd,
  calcMomentum,
  calcRsi,
  calcSma,
  calcStochastic,
  calcVolumeRatio,
  calcVwap,
  calcWma,
  clampConf,
  isBearishEngulfing,
  isBearishPinBar,
  isBullishEngulfing,
  isBullishPinBar,
  isDoji,
  isEveningStar,
  isHammer,
  isMorningStar,
  isShootingStar,
  isThreeBlackCrows,
  isThreeWhiteSoldiers,
  NO_SIGNAL,
  type OHLCVCandle,
  type ResearchSignal,
} from "@/lib/ai/mockResearchIndicators";

// ── Types ────────────────────────────────────────────────────────────────────
export type ResearchFamily =
  | "TrendFollowing"
  | "EmaSmaCrossover"
  | "VwapStrategies"
  | "RsiMeanReversion"
  | "BollingerBand"
  | "Breakout"
  | "LiquiditySweep"
  | "MomentumContinuation"
  | "Scalping"
  | "VolumeSpike"
  | "VolatilityExpansion"
  | "AtrBased"
  | "MacdStrategies"
  | "StochasticStrategies"
  | "CandlePattern"
  | "MultiTimeframe"
  | "SupportResistance"
  | "RangeChop";

export const RESEARCH_FAMILY_LABELS: Record<ResearchFamily, string> = {
  TrendFollowing: "Trend Following",
  EmaSmaCrossover: "EMA/SMA Crossover",
  VwapStrategies: "VWAP",
  RsiMeanReversion: "RSI Mean Reversion",
  BollingerBand: "Bollinger Band",
  Breakout: "Breakout",
  LiquiditySweep: "Liquidity Sweep",
  MomentumContinuation: "Momentum Continuation",
  Scalping: "Scalping",
  VolumeSpike: "Volume Spike",
  VolatilityExpansion: "Volatility Expansion",
  AtrBased: "ATR-Based",
  MacdStrategies: "MACD",
  StochasticStrategies: "Stochastic",
  CandlePattern: "Candle Pattern",
  MultiTimeframe: "Multi-Timeframe",
  SupportResistance: "Support/Resistance",
  RangeChop: "Range/Chop",
};

export const ALL_RESEARCH_FAMILIES = Object.keys(RESEARCH_FAMILY_LABELS) as ResearchFamily[];

export interface ResearchStrategy {
  /** Unique ID in range 1000–1499. */
  id: number;
  name: string;
  family: ResearchFamily;
  /** Included in active research evaluations by default. */
  enabled: true;
  /** Short human-readable description of the signal logic. */
  description: string;
  /** Parameters used by the signal function, for display only. */
  params: Record<string, number | string>;
  /** Primary evaluation timeframe — informational. */
  timeframe: "1m" | "5m" | "15m";
  /** Minimum closed candles required before generating signals. */
  minCandles: number;
  /** Pure signal function. May return NO_SIGNAL if conditions not met. */
  signal: (candles: OHLCVCandle[]) => ResearchSignal;
}

// ── Signal factory helpers ────────────────────────────────────────────────────

function closes(candles: OHLCVCandle[]): number[] {
  return candles.map((c) => c.close);
}

// ── Family 1: TrendFollowing (IDs 1000–1029) ─────────────────────────────────
// EMA crossover filtered by ADX. Long when fast > slow + ADX strong.
// 15 variants × 2 sides = 30.

const TRF_VARIANTS: Array<{ fast: number; slow: number; adxMin: number; label: string }> = [
  { fast: 5,  slow: 20, adxMin: 25, label: "5/20" },
  { fast: 8,  slow: 21, adxMin: 20, label: "8/21" },
  { fast: 9,  slow: 26, adxMin: 22, label: "9/26" },
  { fast: 10, slow: 30, adxMin: 25, label: "10/30" },
  { fast: 12, slow: 26, adxMin: 20, label: "12/26" },
  { fast: 13, slow: 34, adxMin: 18, label: "13/34" },
  { fast: 14, slow: 50, adxMin: 22, label: "14/50" },
  { fast: 15, slow: 50, adxMin: 25, label: "15/50" },
  { fast: 20, slow: 50, adxMin: 20, label: "20/50" },
  { fast: 21, slow: 55, adxMin: 18, label: "21/55" },
  { fast: 8,  slow: 13, adxMin: 30, label: "8/13" },
  { fast: 5,  slow: 13, adxMin: 28, label: "5/13" },
  { fast: 3,  slow: 10, adxMin: 20, label: "3/10" },
  { fast: 3,  slow: 15, adxMin: 22, label: "3/15" },
  { fast: 3,  slow: 20, adxMin: 25, label: "3/20" },
];

function makeTrendSignal(fast: number, slow: number, adxMin: number, side: "BUY" | "SELL") {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < slow + adxMin) return NO_SIGNAL;
    const cl = closes(candles);
    const f = calcEma(cl, fast);
    const s = calcEma(cl, slow);
    const adx = calcAdx(candles, 14);
    if (adx < adxMin) return NO_SIGNAL;
    const diff = Math.abs(f - s) / s;
    if (side === "BUY" && f > s) return { side: "BUY", confidence: clampConf(50 + diff * 3000, 45) };
    if (side === "SELL" && f < s) return { side: "SELL", confidence: clampConf(50 + diff * 3000, 45) };
    return NO_SIGNAL;
  };
}

// ── Family 2: EmaSmaCrossover (IDs 1030–1059) ────────────────────────────────
// EMA crosses above/below SMA — entry on crossover.
// 15 variants × 2 sides = 30.

const EMX_VARIANTS: Array<{ ema: number; sma: number; label: string }> = [
  { ema: 5,  sma: 10,  label: "5E/10S" },
  { ema: 8,  sma: 20,  label: "8E/20S" },
  { ema: 9,  sma: 18,  label: "9E/18S" },
  { ema: 10, sma: 20,  label: "10E/20S" },
  { ema: 12, sma: 24,  label: "12E/24S" },
  { ema: 13, sma: 30,  label: "13E/30S" },
  { ema: 20, sma: 50,  label: "20E/50S" },
  { ema: 21, sma: 55,  label: "21E/55S" },
  { ema: 5,  sma: 15,  label: "5E/15S" },
  { ema: 7,  sma: 21,  label: "7E/21S" },
  { ema: 8,  sma: 34,  label: "8E/34S" },
  { ema: 15, sma: 40,  label: "15E/40S" },
  { ema: 3,  sma: 8,   label: "3E/8S" },
  { ema: 4,  sma: 12,  label: "4E/12S" },
  { ema: 6,  sma: 18,  label: "6E/18S" },
];

function makeEmaCrossSignal(emaPeriod: number, smaPeriod: number, side: "BUY" | "SELL") {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < smaPeriod + 2) return NO_SIGNAL;
    const cl = closes(candles);
    const ema = calcEma(cl, emaPeriod);
    const sma = calcSma(cl, smaPeriod);
    const prevEma = calcEma(cl.slice(0, -1), emaPeriod);
    const prevSma = calcSma(cl.slice(0, -1), smaPeriod);
    const crossedUp = prevEma <= prevSma && ema > sma;
    const crossedDn = prevEma >= prevSma && ema < sma;
    const dist = Math.abs(ema - sma) / sma;
    if (side === "BUY" && crossedUp) return { side: "BUY", confidence: clampConf(55 + dist * 2000) };
    if (side === "SELL" && crossedDn) return { side: "SELL", confidence: clampConf(55 + dist * 2000) };
    return NO_SIGNAL;
  };
}

// ── Family 3: VwapStrategies (IDs 1060–1089) ─────────────────────────────────
// Price relative to VWAP with RSI confirmation.
// 15 variants × 2 sides = 30.

const VWAP_VARIANTS: Array<{ rsiPeriod: number; rsiLong: number; rsiShort: number; devPct: number; label: string }> = [
  { rsiPeriod: 14, rsiLong: 35, rsiShort: 65, devPct: 0.10, label: "rsi14/d0.1" },
  { rsiPeriod: 14, rsiLong: 30, rsiShort: 70, devPct: 0.15, label: "rsi14/d0.15" },
  { rsiPeriod: 7,  rsiLong: 35, rsiShort: 65, devPct: 0.10, label: "rsi7/d0.1" },
  { rsiPeriod: 7,  rsiLong: 30, rsiShort: 70, devPct: 0.05, label: "rsi7/d0.05" },
  { rsiPeriod: 21, rsiLong: 38, rsiShort: 62, devPct: 0.20, label: "rsi21/d0.2" },
  { rsiPeriod: 14, rsiLong: 40, rsiShort: 60, devPct: 0.08, label: "rsi14/d0.08" },
  { rsiPeriod: 9,  rsiLong: 35, rsiShort: 65, devPct: 0.12, label: "rsi9/d0.12" },
  { rsiPeriod: 14, rsiLong: 25, rsiShort: 75, devPct: 0.20, label: "rsi14/d0.2" },
  { rsiPeriod: 7,  rsiLong: 25, rsiShort: 75, devPct: 0.08, label: "rsi7/d0.08" },
  { rsiPeriod: 9,  rsiLong: 30, rsiShort: 70, devPct: 0.15, label: "rsi9/d0.15" },
  { rsiPeriod: 14, rsiLong: 42, rsiShort: 58, devPct: 0.05, label: "rsi14/d0.05" },
  { rsiPeriod: 21, rsiLong: 30, rsiShort: 70, devPct: 0.12, label: "rsi21/d0.12" },
  { rsiPeriod: 7,  rsiLong: 40, rsiShort: 60, devPct: 0.05, label: "rsi7/d0.05b" },
  { rsiPeriod: 9,  rsiLong: 40, rsiShort: 60, devPct: 0.10, label: "rsi9/d0.1" },
  { rsiPeriod: 14, rsiLong: 33, rsiShort: 67, devPct: 0.18, label: "rsi14/d0.18" },
];

function makeVwapSignal(
  rsiPeriod: number, rsiLong: number, rsiShort: number, devPct: number, side: "BUY" | "SELL"
) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < 20) return NO_SIGNAL;
    const cl = closes(candles);
    const vwap = calcVwap(candles);
    const rsi = calcRsi(cl, rsiPeriod);
    const price = cl[cl.length - 1];
    const deviation = (price - vwap) / vwap;
    if (side === "BUY" && deviation < -devPct / 100 && rsi < rsiLong) {
      return { side: "BUY", confidence: clampConf(60 - rsi * 0.5 + Math.abs(deviation) * 500) };
    }
    if (side === "SELL" && deviation > devPct / 100 && rsi > rsiShort) {
      return { side: "SELL", confidence: clampConf(60 + (rsi - 50) * 0.5 + deviation * 500) };
    }
    return NO_SIGNAL;
  };
}

// ── Family 4: RsiMeanReversion (IDs 1090–1119) ───────────────────────────────
// RSI oversold → BUY, overbought → SELL.
// 15 variants × 2 sides = 30.

const RSI_VARIANTS: Array<{ period: number; oversold: number; overbought: number; label: string }> = [
  { period: 14, oversold: 30, overbought: 70, label: "14/30/70" },
  { period: 14, oversold: 25, overbought: 75, label: "14/25/75" },
  { period: 14, oversold: 20, overbought: 80, label: "14/20/80" },
  { period: 7,  oversold: 30, overbought: 70, label: "7/30/70" },
  { period: 7,  oversold: 25, overbought: 75, label: "7/25/75" },
  { period: 7,  oversold: 20, overbought: 80, label: "7/20/80" },
  { period: 9,  oversold: 30, overbought: 70, label: "9/30/70" },
  { period: 9,  oversold: 25, overbought: 75, label: "9/25/75" },
  { period: 21, oversold: 30, overbought: 70, label: "21/30/70" },
  { period: 21, oversold: 35, overbought: 65, label: "21/35/65" },
  { period: 5,  oversold: 20, overbought: 80, label: "5/20/80" },
  { period: 5,  oversold: 25, overbought: 75, label: "5/25/75" },
  { period: 14, oversold: 35, overbought: 65, label: "14/35/65" },
  { period: 28, oversold: 30, overbought: 70, label: "28/30/70" },
  { period: 28, oversold: 25, overbought: 75, label: "28/25/75" },
];

function makeRsiSignal(period: number, oversold: number, overbought: number, side: "BUY" | "SELL") {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < period + 2) return NO_SIGNAL;
    const rsi = calcRsi(closes(candles), period);
    if (side === "BUY" && rsi < oversold) {
      return { side: "BUY", confidence: clampConf(80 - rsi * 1.2) };
    }
    if (side === "SELL" && rsi > overbought) {
      return { side: "SELL", confidence: clampConf(30 + (rsi - 50) * 1.2) };
    }
    return NO_SIGNAL;
  };
}

// ── Family 5: BollingerBand (IDs 1120–1149) ───────────────────────────────────
// Price at/below lower band → BUY; at/above upper band → SELL.
// 15 variants × 2 sides = 30.

const BB_VARIANTS: Array<{ period: number; mult: number; rsiConfirm: number; label: string }> = [
  { period: 20, mult: 2.0, rsiConfirm: 40, label: "20/2.0" },
  { period: 20, mult: 2.5, rsiConfirm: 35, label: "20/2.5" },
  { period: 20, mult: 1.5, rsiConfirm: 45, label: "20/1.5" },
  { period: 14, mult: 2.0, rsiConfirm: 40, label: "14/2.0" },
  { period: 14, mult: 2.5, rsiConfirm: 35, label: "14/2.5" },
  { period: 30, mult: 2.0, rsiConfirm: 40, label: "30/2.0" },
  { period: 50, mult: 2.0, rsiConfirm: 38, label: "50/2.0" },
  { period: 20, mult: 3.0, rsiConfirm: 30, label: "20/3.0" },
  { period: 10, mult: 2.0, rsiConfirm: 40, label: "10/2.0" },
  { period: 10, mult: 1.5, rsiConfirm: 45, label: "10/1.5" },
  { period: 20, mult: 1.8, rsiConfirm: 42, label: "20/1.8" },
  { period: 25, mult: 2.0, rsiConfirm: 40, label: "25/2.0" },
  { period: 14, mult: 1.5, rsiConfirm: 45, label: "14/1.5" },
  { period: 20, mult: 2.2, rsiConfirm: 38, label: "20/2.2" },
  { period: 30, mult: 1.5, rsiConfirm: 45, label: "30/1.5" },
];

function makeBbSignal(period: number, mult: number, rsiConfirm: number, side: "BUY" | "SELL") {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < period + 2) return NO_SIGNAL;
    const cl = closes(candles);
    const bb = calcBollinger(cl, period, mult);
    const rsi = calcRsi(cl, 14);
    const price = cl[cl.length - 1];
    if (side === "BUY" && price <= bb.lower && rsi < rsiConfirm) {
      return { side: "BUY", confidence: clampConf(90 - bb.percentB * 40) };
    }
    if (side === "SELL" && price >= bb.upper && rsi > (100 - rsiConfirm)) {
      return { side: "SELL", confidence: clampConf(50 + bb.percentB * 40) };
    }
    return NO_SIGNAL;
  };
}

// ── Family 6: Breakout (IDs 1150–1179) ───────────────────────────────────────
// Donchian channel breakout with volume confirmation.
// 15 variants × 2 sides = 30.

const BRK_VARIANTS: Array<{ period: number; volMin: number; atrMult: number; label: string }> = [
  { period: 20, volMin: 1.3, atrMult: 1.0, label: "20/vol1.3" },
  { period: 20, volMin: 1.5, atrMult: 1.2, label: "20/vol1.5" },
  { period: 30, volMin: 1.3, atrMult: 1.0, label: "30/vol1.3" },
  { period: 10, volMin: 1.5, atrMult: 0.8, label: "10/vol1.5" },
  { period: 50, volMin: 1.2, atrMult: 1.5, label: "50/vol1.2" },
  { period: 14, volMin: 1.4, atrMult: 1.0, label: "14/vol1.4" },
  { period: 20, volMin: 2.0, atrMult: 1.0, label: "20/vol2.0" },
  { period: 40, volMin: 1.3, atrMult: 1.2, label: "40/vol1.3" },
  { period: 15, volMin: 1.5, atrMult: 1.0, label: "15/vol1.5" },
  { period: 25, volMin: 1.2, atrMult: 0.8, label: "25/vol1.2" },
  { period: 20, volMin: 1.8, atrMult: 0.5, label: "20/vol1.8" },
  { period: 10, volMin: 1.3, atrMult: 1.0, label: "10/vol1.3" },
  { period: 30, volMin: 1.5, atrMult: 1.5, label: "30/vol1.5" },
  { period: 60, volMin: 1.2, atrMult: 1.0, label: "60/vol1.2" },
  { period: 20, volMin: 1.0, atrMult: 1.5, label: "20/atr1.5" },
];

function makeBreakoutSignal(period: number, volMin: number, atrMult: number, side: "BUY" | "SELL") {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < period + 2) return NO_SIGNAL;
    const dc = calcDonchian(candles, period);
    const atr = calcAtr(candles, 14);
    const vr = calcVolumeRatio(candles, 20);
    const price = candles[candles.length - 1].close;
    const prevPrice = candles[candles.length - 2].close;
    if (side === "BUY" && prevPrice < dc.upper && price >= dc.upper && vr >= volMin) {
      return { side: "BUY", confidence: clampConf(55 + vr * 12 + atrMult * 5) };
    }
    if (side === "SELL" && prevPrice > dc.lower && price <= dc.lower && vr >= volMin) {
      return { side: "SELL", confidence: clampConf(55 + vr * 12 + atrMult * 5) };
    }
    return NO_SIGNAL;
  };
}

// ── Family 7: LiquiditySweep (IDs 1180–1209) ────────────────────────────────
// Price sweeps N-bar high/low then closes back inside → reversal.
// 15 variants × 2 sides = 30.

const LIQ_VARIANTS: Array<{ lookback: number; retracePct: number; label: string }> = [
  { lookback: 10, retracePct: 0.1, label: "10/0.1" },
  { lookback: 15, retracePct: 0.1, label: "15/0.1" },
  { lookback: 20, retracePct: 0.1, label: "20/0.1" },
  { lookback: 10, retracePct: 0.2, label: "10/0.2" },
  { lookback: 20, retracePct: 0.2, label: "20/0.2" },
  { lookback: 5,  retracePct: 0.1, label: "5/0.1" },
  { lookback: 30, retracePct: 0.1, label: "30/0.1" },
  { lookback: 10, retracePct: 0.05, label: "10/0.05" },
  { lookback: 20, retracePct: 0.05, label: "20/0.05" },
  { lookback: 15, retracePct: 0.2, label: "15/0.2" },
  { lookback: 8,  retracePct: 0.1, label: "8/0.1" },
  { lookback: 12, retracePct: 0.15, label: "12/0.15" },
  { lookback: 25, retracePct: 0.15, label: "25/0.15" },
  { lookback: 7,  retracePct: 0.08, label: "7/0.08" },
  { lookback: 18, retracePct: 0.12, label: "18/0.12" },
];

function makeLiquiditySweepSignal(lookback: number, retracePct: number, side: "BUY" | "SELL") {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < lookback + 2) return NO_SIGNAL;
    const prev = candles.slice(-lookback - 1, -1);
    const curr = candles[candles.length - 1];
    const prevHigh = Math.max(...prev.map((c) => c.high));
    const prevLow = Math.min(...prev.map((c) => c.low));
    const swept = prevHigh - prevLow;
    if (swept <= 0) return NO_SIGNAL;
    if (side === "BUY") {
      // Candle swept below prevLow then closed above it
      const swept_low = curr.low < prevLow && curr.close > prevLow;
      const retraceEnough = (prevLow - curr.low) / swept >= retracePct / 100;
      if (swept_low && retraceEnough) {
        return { side: "BUY", confidence: clampConf(65 + (prevLow - curr.low) / curr.close * 5000) };
      }
    } else {
      const swept_high = curr.high > prevHigh && curr.close < prevHigh;
      const retraceEnough = (curr.high - prevHigh) / swept >= retracePct / 100;
      if (swept_high && retraceEnough) {
        return { side: "SELL", confidence: clampConf(65 + (curr.high - prevHigh) / curr.close * 5000) };
      }
    }
    return NO_SIGNAL;
  };
}

// ── Family 8: MomentumContinuation (IDs 1210–1239) ───────────────────────────
// Rate-of-change momentum with EMA trend confirmation.
// 15 variants × 2 sides = 30.

const MOM_VARIANTS: Array<{ roc: number; ema: number; rocMin: number; label: string }> = [
  { roc: 5,  ema: 20, rocMin: 0.5, label: "roc5/ema20" },
  { roc: 10, ema: 20, rocMin: 0.8, label: "roc10/ema20" },
  { roc: 5,  ema: 50, rocMin: 0.5, label: "roc5/ema50" },
  { roc: 10, ema: 50, rocMin: 1.0, label: "roc10/ema50" },
  { roc: 3,  ema: 10, rocMin: 0.3, label: "roc3/ema10" },
  { roc: 7,  ema: 21, rocMin: 0.6, label: "roc7/ema21" },
  { roc: 14, ema: 30, rocMin: 1.2, label: "roc14/ema30" },
  { roc: 5,  ema: 13, rocMin: 0.4, label: "roc5/ema13" },
  { roc: 20, ema: 50, rocMin: 2.0, label: "roc20/ema50" },
  { roc: 3,  ema: 8,  rocMin: 0.3, label: "roc3/ema8" },
  { roc: 5,  ema: 34, rocMin: 0.6, label: "roc5/ema34" },
  { roc: 10, ema: 34, rocMin: 1.0, label: "roc10/ema34" },
  { roc: 8,  ema: 26, rocMin: 0.7, label: "roc8/ema26" },
  { roc: 12, ema: 40, rocMin: 1.0, label: "roc12/ema40" },
  { roc: 4,  ema: 15, rocMin: 0.4, label: "roc4/ema15" },
];

function makeMomentumSignal(rocPeriod: number, emaPeriod: number, rocMin: number, side: "BUY" | "SELL") {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < emaPeriod + rocPeriod + 2) return NO_SIGNAL;
    const cl = closes(candles);
    const roc = calcMomentum(cl, rocPeriod);
    const ema = calcEma(cl, emaPeriod);
    const price = cl[cl.length - 1];
    if (side === "BUY" && roc > rocMin && price > ema) {
      return { side: "BUY", confidence: clampConf(50 + roc * 8) };
    }
    if (side === "SELL" && roc < -rocMin && price < ema) {
      return { side: "SELL", confidence: clampConf(50 + Math.abs(roc) * 8) };
    }
    return NO_SIGNAL;
  };
}

// ── Family 9: Scalping (IDs 1240–1269) ───────────────────────────────────────
// Tight RSI + Bollinger Band squeeze for fast entries with volume spike.
// 15 variants × 2 sides = 30.

const SCP_VARIANTS: Array<{ rsi: number; rsiBuyMax: number; rsiSellMin: number; bbPeriod: number; label: string }> = [
  { rsi: 7,  rsiBuyMax: 35, rsiSellMin: 65, bbPeriod: 10, label: "rsi7/bb10" },
  { rsi: 7,  rsiBuyMax: 30, rsiSellMin: 70, bbPeriod: 14, label: "rsi7/bb14" },
  { rsi: 5,  rsiBuyMax: 30, rsiSellMin: 70, bbPeriod: 10, label: "rsi5/bb10" },
  { rsi: 9,  rsiBuyMax: 35, rsiSellMin: 65, bbPeriod: 20, label: "rsi9/bb20" },
  { rsi: 7,  rsiBuyMax: 40, rsiSellMin: 60, bbPeriod: 10, label: "rsi7/bb10b" },
  { rsi: 14, rsiBuyMax: 30, rsiSellMin: 70, bbPeriod: 20, label: "rsi14/bb20" },
  { rsi: 7,  rsiBuyMax: 25, rsiSellMin: 75, bbPeriod: 10, label: "rsi7/bb10c" },
  { rsi: 5,  rsiBuyMax: 20, rsiSellMin: 80, bbPeriod: 8,  label: "rsi5/bb8" },
  { rsi: 9,  rsiBuyMax: 30, rsiSellMin: 70, bbPeriod: 14, label: "rsi9/bb14" },
  { rsi: 3,  rsiBuyMax: 25, rsiSellMin: 75, bbPeriod: 8,  label: "rsi3/bb8" },
  { rsi: 7,  rsiBuyMax: 35, rsiSellMin: 65, bbPeriod: 20, label: "rsi7/bb20" },
  { rsi: 14, rsiBuyMax: 35, rsiSellMin: 65, bbPeriod: 14, label: "rsi14/bb14" },
  { rsi: 9,  rsiBuyMax: 25, rsiSellMin: 75, bbPeriod: 10, label: "rsi9/bb10" },
  { rsi: 5,  rsiBuyMax: 30, rsiSellMin: 70, bbPeriod: 14, label: "rsi5/bb14" },
  { rsi: 7,  rsiBuyMax: 30, rsiSellMin: 70, bbPeriod: 8,  label: "rsi7/bb8" },
];

function makeScalpingSignal(
  rsiPeriod: number, rsiBuyMax: number, rsiSellMin: number, bbPeriod: number, side: "BUY" | "SELL"
) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < bbPeriod + rsiPeriod + 2) return NO_SIGNAL;
    const cl = closes(candles);
    const rsi = calcRsi(cl, rsiPeriod);
    const bb = calcBollinger(cl, bbPeriod, 2);
    const vr = calcVolumeRatio(candles, 10);
    const price = cl[cl.length - 1];
    if (side === "BUY" && rsi < rsiBuyMax && price <= bb.lower && vr > 1.1) {
      return { side: "BUY", confidence: clampConf(85 - rsi - bb.percentB * 20) };
    }
    if (side === "SELL" && rsi > rsiSellMin && price >= bb.upper && vr > 1.1) {
      return { side: "SELL", confidence: clampConf(35 + (rsi - 50) + bb.percentB * 20) };
    }
    return NO_SIGNAL;
  };
}

// ── Family 10: VolumeSpike (IDs 1270–1289) ────────────────────────────────────
// Abnormal volume spike with price direction confirmation.
// 10 variants × 2 sides = 20.

const VOL_VARIANTS: Array<{ period: number; multiple: number; label: string }> = [
  { period: 20, multiple: 2.0, label: "20/2x" },
  { period: 20, multiple: 2.5, label: "20/2.5x" },
  { period: 20, multiple: 3.0, label: "20/3x" },
  { period: 14, multiple: 2.0, label: "14/2x" },
  { period: 14, multiple: 2.5, label: "14/2.5x" },
  { period: 10, multiple: 2.0, label: "10/2x" },
  { period: 30, multiple: 2.0, label: "30/2x" },
  { period: 20, multiple: 1.5, label: "20/1.5x" },
  { period: 50, multiple: 2.0, label: "50/2x" },
  { period: 20, multiple: 4.0, label: "20/4x" },
];

function makeVolumeSignal(period: number, multiple: number, side: "BUY" | "SELL") {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < period + 1) return NO_SIGNAL;
    const vr = calcVolumeRatio(candles, period);
    if (vr < multiple) return NO_SIGNAL;
    const curr = candles[candles.length - 1];
    const bullish = curr.close > curr.open;
    if (side === "BUY" && bullish) {
      return { side: "BUY", confidence: clampConf(50 + vr * 8) };
    }
    if (side === "SELL" && !bullish) {
      return { side: "SELL", confidence: clampConf(50 + vr * 8) };
    }
    return NO_SIGNAL;
  };
}

// ── Family 11: VolatilityExpansion (IDs 1290–1309) ───────────────────────────
// Candle range > ATR × multiplier (expansion) in trend direction.
// 10 variants × 2 sides = 20.

const VOLEX_VARIANTS: Array<{ atrPeriod: number; rangeMult: number; label: string }> = [
  { atrPeriod: 14, rangeMult: 1.5, label: "atr14/1.5x" },
  { atrPeriod: 14, rangeMult: 2.0, label: "atr14/2x" },
  { atrPeriod: 7,  rangeMult: 1.5, label: "atr7/1.5x" },
  { atrPeriod: 7,  rangeMult: 2.0, label: "atr7/2x" },
  { atrPeriod: 20, rangeMult: 1.5, label: "atr20/1.5x" },
  { atrPeriod: 14, rangeMult: 1.2, label: "atr14/1.2x" },
  { atrPeriod: 14, rangeMult: 2.5, label: "atr14/2.5x" },
  { atrPeriod: 10, rangeMult: 1.5, label: "atr10/1.5x" },
  { atrPeriod: 21, rangeMult: 2.0, label: "atr21/2x" },
  { atrPeriod: 14, rangeMult: 1.8, label: "atr14/1.8x" },
];

function makeVolatilitySignal(atrPeriod: number, rangeMult: number, side: "BUY" | "SELL") {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < atrPeriod + 2) return NO_SIGNAL;
    const atr = calcAtr(candles, atrPeriod);
    const curr = candles[candles.length - 1];
    const range = curr.high - curr.low;
    const bullish = curr.close > curr.open;
    if (range < atr * rangeMult) return NO_SIGNAL;
    if (side === "BUY" && bullish) {
      return { side: "BUY", confidence: clampConf(55 + (range / atr) * 8) };
    }
    if (side === "SELL" && !bullish) {
      return { side: "SELL", confidence: clampConf(55 + (range / atr) * 8) };
    }
    return NO_SIGNAL;
  };
}

// ── Family 12: AtrBased (IDs 1310–1339) ───────────────────────────────────────
// ATR channel trailing — price breaks above/below ATR-based channel.
// 15 variants × 2 sides = 30.

const ATR_VARIANTS: Array<{ atrPeriod: number; atrMult: number; emaPeriod: number; label: string }> = [
  { atrPeriod: 14, atrMult: 1.5, emaPeriod: 20, label: "atr14/1.5/ema20" },
  { atrPeriod: 14, atrMult: 2.0, emaPeriod: 20, label: "atr14/2.0/ema20" },
  { atrPeriod: 14, atrMult: 2.5, emaPeriod: 20, label: "atr14/2.5/ema20" },
  { atrPeriod: 7,  atrMult: 1.5, emaPeriod: 10, label: "atr7/1.5/ema10" },
  { atrPeriod: 7,  atrMult: 2.0, emaPeriod: 10, label: "atr7/2.0/ema10" },
  { atrPeriod: 21, atrMult: 1.5, emaPeriod: 30, label: "atr21/1.5/ema30" },
  { atrPeriod: 14, atrMult: 1.0, emaPeriod: 20, label: "atr14/1.0/ema20" },
  { atrPeriod: 14, atrMult: 3.0, emaPeriod: 50, label: "atr14/3.0/ema50" },
  { atrPeriod: 10, atrMult: 1.5, emaPeriod: 15, label: "atr10/1.5/ema15" },
  { atrPeriod: 10, atrMult: 2.0, emaPeriod: 20, label: "atr10/2.0/ema20" },
  { atrPeriod: 20, atrMult: 2.0, emaPeriod: 30, label: "atr20/2.0/ema30" },
  { atrPeriod: 14, atrMult: 1.8, emaPeriod: 25, label: "atr14/1.8/ema25" },
  { atrPeriod: 5,  atrMult: 1.5, emaPeriod: 8,  label: "atr5/1.5/ema8" },
  { atrPeriod: 14, atrMult: 2.0, emaPeriod: 50, label: "atr14/2.0/ema50" },
  { atrPeriod: 28, atrMult: 2.0, emaPeriod: 40, label: "atr28/2.0/ema40" },
];

function makeAtrSignal(atrPeriod: number, atrMult: number, emaPeriod: number, side: "BUY" | "SELL") {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < Math.max(atrPeriod, emaPeriod) + 2) return NO_SIGNAL;
    const cl = closes(candles);
    const atr = calcAtr(candles, atrPeriod);
    const ema = calcEma(cl, emaPeriod);
    const price = cl[cl.length - 1];
    const upper = ema + atr * atrMult;
    const lower = ema - atr * atrMult;
    if (side === "BUY" && price > ema && price < upper) {
      const distPct = (price - ema) / (upper - ema);
      return { side: "BUY", confidence: clampConf(50 + distPct * 30) };
    }
    if (side === "SELL" && price < ema && price > lower) {
      const distPct = (ema - price) / (ema - lower);
      return { side: "SELL", confidence: clampConf(50 + distPct * 30) };
    }
    return NO_SIGNAL;
  };
}

// ── Family 13: MacdStrategies (IDs 1340–1369) ─────────────────────────────────
// MACD line crossover signal/zero with different parameter sets.
// 15 variants × 2 sides = 30.

const MACD_VARIANTS: Array<{ fast: number; slow: number; signal: number; mode: "cross" | "zero"; label: string }> = [
  { fast: 12, slow: 26, signal: 9,  mode: "cross", label: "12/26/9-cross" },
  { fast: 12, slow: 26, signal: 9,  mode: "zero",  label: "12/26/9-zero" },
  { fast: 8,  slow: 17, signal: 9,  mode: "cross", label: "8/17/9-cross" },
  { fast: 5,  slow: 13, signal: 5,  mode: "cross", label: "5/13/5-cross" },
  { fast: 9,  slow: 21, signal: 7,  mode: "cross", label: "9/21/7-cross" },
  { fast: 12, slow: 26, signal: 5,  mode: "cross", label: "12/26/5-cross" },
  { fast: 3,  slow: 10, signal: 16, mode: "cross", label: "3/10/16-cross" },
  { fast: 14, slow: 28, signal: 9,  mode: "zero",  label: "14/28/9-zero" },
  { fast: 6,  slow: 19, signal: 9,  mode: "cross", label: "6/19/9-cross" },
  { fast: 12, slow: 30, signal: 9,  mode: "cross", label: "12/30/9-cross" },
  { fast: 8,  slow: 21, signal: 9,  mode: "zero",  label: "8/21/9-zero" },
  { fast: 10, slow: 22, signal: 7,  mode: "cross", label: "10/22/7-cross" },
  { fast: 5,  slow: 35, signal: 5,  mode: "zero",  label: "5/35/5-zero" },
  { fast: 12, slow: 26, signal: 12, mode: "cross", label: "12/26/12-cross" },
  { fast: 7,  slow: 14, signal: 5,  mode: "cross", label: "7/14/5-cross" },
];

function makeMacdSignal(
  fast: number, slow: number, signal: number, mode: "cross" | "zero", side: "BUY" | "SELL"
) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < slow + signal + 2) return NO_SIGNAL;
    const cl = closes(candles);
    const curr = calcMacd(cl, fast, slow, signal);
    const prev = calcMacd(cl.slice(0, -1), fast, slow, signal);
    if (mode === "cross") {
      const crossedUp = prev.histogram < 0 && curr.histogram > 0;
      const crossedDn = prev.histogram > 0 && curr.histogram < 0;
      if (side === "BUY" && crossedUp) return { side: "BUY", confidence: clampConf(60 + Math.abs(curr.histogram) * 5) };
      if (side === "SELL" && crossedDn) return { side: "SELL", confidence: clampConf(60 + Math.abs(curr.histogram) * 5) };
    } else {
      const crossedUpZero = prev.line < 0 && curr.line > 0;
      const crossedDnZero = prev.line > 0 && curr.line < 0;
      if (side === "BUY" && crossedUpZero) return { side: "BUY", confidence: clampConf(65 + Math.abs(curr.line) * 3) };
      if (side === "SELL" && crossedDnZero) return { side: "SELL", confidence: clampConf(65 + Math.abs(curr.line) * 3) };
    }
    return NO_SIGNAL;
  };
}

// ── Family 14: StochasticStrategies (IDs 1370–1389) ──────────────────────────
// %K/%D crossover in OB/OS zones.
// 10 variants × 2 sides = 20.

const STOCH_VARIANTS: Array<{ kPeriod: number; dPeriod: number; obLevel: number; osLevel: number; label: string }> = [
  { kPeriod: 14, dPeriod: 3, obLevel: 80, osLevel: 20, label: "14/3/80/20" },
  { kPeriod: 14, dPeriod: 3, obLevel: 75, osLevel: 25, label: "14/3/75/25" },
  { kPeriod: 14, dPeriod: 5, obLevel: 80, osLevel: 20, label: "14/5/80/20" },
  { kPeriod: 9,  dPeriod: 3, obLevel: 80, osLevel: 20, label: "9/3/80/20" },
  { kPeriod: 5,  dPeriod: 3, obLevel: 80, osLevel: 20, label: "5/3/80/20" },
  { kPeriod: 21, dPeriod: 7, obLevel: 80, osLevel: 20, label: "21/7/80/20" },
  { kPeriod: 14, dPeriod: 3, obLevel: 85, osLevel: 15, label: "14/3/85/15" },
  { kPeriod: 7,  dPeriod: 3, obLevel: 80, osLevel: 20, label: "7/3/80/20" },
  { kPeriod: 14, dPeriod: 3, obLevel: 70, osLevel: 30, label: "14/3/70/30" },
  { kPeriod: 5,  dPeriod: 3, obLevel: 75, osLevel: 25, label: "5/3/75/25" },
];

function makeStochasticSignal(
  kPeriod: number, dPeriod: number, obLevel: number, osLevel: number, side: "BUY" | "SELL"
) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < kPeriod + dPeriod + 2) return NO_SIGNAL;
    const curr = calcStochastic(candles, kPeriod, dPeriod);
    const prev = calcStochastic(candles.slice(0, -1), kPeriod, dPeriod);
    if (side === "BUY" && curr.k < osLevel && prev.k < curr.k && curr.k > curr.d) {
      return { side: "BUY", confidence: clampConf(80 - curr.k) };
    }
    if (side === "SELL" && curr.k > obLevel && prev.k > curr.k && curr.k < curr.d) {
      return { side: "SELL", confidence: clampConf(30 + curr.k * 0.5) };
    }
    return NO_SIGNAL;
  };
}

// ── Family 15: CandlePattern (IDs 1390–1419) ──────────────────────────────────
// Candlestick reversal / continuation patterns.
// 15 variants × 2 sides = 30.

type CandlePatternSpec = {
  pattern: "engulfing" | "hammer" | "shootingStar" | "pinBar" | "threeCandle" | "morningEvening" | "doji";
  rsiFilter: boolean;
  rsiPeriod: number;
  rsiLevel: number;
  volFilter: boolean;
  volMult: number;
  label: string;
};

const CP_VARIANTS: CandlePatternSpec[] = [
  { pattern: "engulfing",      rsiFilter: false, rsiPeriod: 14, rsiLevel: 50, volFilter: false, volMult: 1, label: "engulfing" },
  { pattern: "engulfing",      rsiFilter: true,  rsiPeriod: 14, rsiLevel: 40, volFilter: false, volMult: 1, label: "engulfing-rsi" },
  { pattern: "hammer",         rsiFilter: false, rsiPeriod: 14, rsiLevel: 50, volFilter: false, volMult: 1, label: "hammer" },
  { pattern: "hammer",         rsiFilter: true,  rsiPeriod: 7,  rsiLevel: 40, volFilter: false, volMult: 1, label: "hammer-rsi7" },
  { pattern: "shootingStar",   rsiFilter: false, rsiPeriod: 14, rsiLevel: 50, volFilter: false, volMult: 1, label: "shootingStar" },
  { pattern: "shootingStar",   rsiFilter: true,  rsiPeriod: 14, rsiLevel: 60, volFilter: false, volMult: 1, label: "shootingStar-rsi" },
  { pattern: "pinBar",         rsiFilter: false, rsiPeriod: 14, rsiLevel: 50, volFilter: false, volMult: 1, label: "pinBar" },
  { pattern: "pinBar",         rsiFilter: true,  rsiPeriod: 14, rsiLevel: 45, volFilter: true,  volMult: 1.2, label: "pinBar-vol" },
  { pattern: "threeCandle",    rsiFilter: false, rsiPeriod: 14, rsiLevel: 50, volFilter: false, volMult: 1, label: "threeSoldiers" },
  { pattern: "morningEvening", rsiFilter: false, rsiPeriod: 14, rsiLevel: 50, volFilter: false, volMult: 1, label: "mornEvening" },
  { pattern: "doji",           rsiFilter: true,  rsiPeriod: 14, rsiLevel: 40, volFilter: false, volMult: 1, label: "doji-rsi" },
  { pattern: "engulfing",      rsiFilter: false, rsiPeriod: 14, rsiLevel: 50, volFilter: true,  volMult: 1.5, label: "engulfing-vol" },
  { pattern: "hammer",         rsiFilter: true,  rsiPeriod: 14, rsiLevel: 30, volFilter: true,  volMult: 1.3, label: "hammer-rsi-vol" },
  { pattern: "shootingStar",   rsiFilter: true,  rsiPeriod: 7,  rsiLevel: 65, volFilter: true,  volMult: 1.2, label: "star-rsi7-vol" },
  { pattern: "pinBar",         rsiFilter: true,  rsiPeriod: 7,  rsiLevel: 35, volFilter: false, volMult: 1, label: "pinBar-rsi7" },
];

function makeCandlePatternSignal(spec: CandlePatternSpec, side: "BUY" | "SELL") {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < 5) return NO_SIGNAL;
    const cl = closes(candles);
    const rsi = spec.rsiFilter ? calcRsi(cl, spec.rsiPeriod) : 50;
    const vr = spec.volFilter ? calcVolumeRatio(candles, 20) : 2;
    if (spec.volFilter && vr < spec.volMult) return NO_SIGNAL;

    let matched = false;
    if (side === "BUY") {
      if (spec.rsiFilter && rsi > spec.rsiLevel) return NO_SIGNAL;
      switch (spec.pattern) {
        case "engulfing":      matched = isBullishEngulfing(candles); break;
        case "hammer":         matched = isHammer(candles[candles.length - 1]); break;
        case "shootingStar":   break; // shooting star is bearish — not for BUY
        case "pinBar":         matched = isBullishPinBar(candles[candles.length - 1]); break;
        case "threeCandle":    matched = isThreeWhiteSoldiers(candles); break;
        case "morningEvening": matched = isMorningStar(candles); break;
        case "doji":           matched = isDoji(candles[candles.length - 1]); break;
      }
    } else {
      if (spec.rsiFilter && rsi < (100 - spec.rsiLevel)) return NO_SIGNAL;
      switch (spec.pattern) {
        case "engulfing":      matched = isBearishEngulfing(candles); break;
        case "hammer":         break; // hammer is bullish
        case "shootingStar":   matched = isShootingStar(candles[candles.length - 1]); break;
        case "pinBar":         matched = isBearishPinBar(candles[candles.length - 1]); break;
        case "threeCandle":    matched = isThreeBlackCrows(candles); break;
        case "morningEvening": matched = isEveningStar(candles); break;
        case "doji":           matched = isDoji(candles[candles.length - 1]); break;
      }
    }
    if (!matched) return NO_SIGNAL;
    return { side, confidence: clampConf(65 + (spec.volFilter ? vr * 5 : 0)) };
  };
}

// ── Family 16: MultiTimeframe (IDs 1420–1449) ─────────────────────────────────
// Simulate higher-timeframe alignment using multiple EMA periods + RSI.
// 15 variants × 2 sides = 30.

const MTF_VARIANTS: Array<{
  emaFast: number; emaMid: number; emaSlow: number; rsiPeriod: number; rsiLimit: number; label: string;
}> = [
  { emaFast: 5,  emaMid: 20, emaSlow: 50,  rsiPeriod: 14, rsiLimit: 30, label: "5/20/50" },
  { emaFast: 8,  emaMid: 21, emaSlow: 55,  rsiPeriod: 14, rsiLimit: 30, label: "8/21/55" },
  { emaFast: 9,  emaMid: 26, emaSlow: 100, rsiPeriod: 14, rsiLimit: 30, label: "9/26/100" },
  { emaFast: 13, emaMid: 34, emaSlow: 89,  rsiPeriod: 14, rsiLimit: 35, label: "13/34/89" },
  { emaFast: 5,  emaMid: 13, emaSlow: 34,  rsiPeriod: 7,  rsiLimit: 30, label: "5/13/34" },
  { emaFast: 10, emaMid: 30, emaSlow: 60,  rsiPeriod: 14, rsiLimit: 35, label: "10/30/60" },
  { emaFast: 20, emaMid: 50, emaSlow: 100, rsiPeriod: 21, rsiLimit: 35, label: "20/50/100" },
  { emaFast: 3,  emaMid: 8,  emaSlow: 21,  rsiPeriod: 7,  rsiLimit: 25, label: "3/8/21" },
  { emaFast: 12, emaMid: 26, emaSlow: 50,  rsiPeriod: 14, rsiLimit: 30, label: "12/26/50" },
  { emaFast: 5,  emaMid: 20, emaSlow: 80,  rsiPeriod: 14, rsiLimit: 35, label: "5/20/80" },
  { emaFast: 7,  emaMid: 21, emaSlow: 63,  rsiPeriod: 14, rsiLimit: 35, label: "7/21/63" },
  { emaFast: 4,  emaMid: 13, emaSlow: 40,  rsiPeriod: 9,  rsiLimit: 30, label: "4/13/40" },
  { emaFast: 6,  emaMid: 18, emaSlow: 54,  rsiPeriod: 14, rsiLimit: 35, label: "6/18/54" },
  { emaFast: 15, emaMid: 40, emaSlow: 120, rsiPeriod: 21, rsiLimit: 35, label: "15/40/120" },
  { emaFast: 8,  emaMid: 30, emaSlow: 90,  rsiPeriod: 14, rsiLimit: 30, label: "8/30/90" },
];

function makeMtfSignal(
  emaFast: number, emaMid: number, emaSlow: number, rsiPeriod: number, rsiLimit: number, side: "BUY" | "SELL"
) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < emaSlow + 2) return NO_SIGNAL;
    const cl = closes(candles);
    const fast = calcEma(cl, emaFast);
    const mid = calcEma(cl, emaMid);
    const slow = calcEma(cl, emaSlow);
    const rsi = calcRsi(cl, rsiPeriod);
    const allBullish = fast > mid && mid > slow;
    const allBearish = fast < mid && mid < slow;
    if (side === "BUY" && allBullish && rsi < (100 - rsiLimit)) {
      return { side: "BUY", confidence: clampConf(60 + (fast / slow - 1) * 2000) };
    }
    if (side === "SELL" && allBearish && rsi > rsiLimit) {
      return { side: "SELL", confidence: clampConf(60 + (1 - fast / slow) * 2000) };
    }
    return NO_SIGNAL;
  };
}

// ── Family 17: SupportResistance (IDs 1450–1469) ──────────────────────────────
// Bounce from N-bar support or rejection from N-bar resistance.
// 10 variants × 2 sides = 20.

const SR_VARIANTS: Array<{ lookback: number; proximityPct: number; rsiFilter: boolean; label: string }> = [
  { lookback: 20, proximityPct: 0.3, rsiFilter: false, label: "20/0.3" },
  { lookback: 20, proximityPct: 0.5, rsiFilter: false, label: "20/0.5" },
  { lookback: 10, proximityPct: 0.3, rsiFilter: false, label: "10/0.3" },
  { lookback: 30, proximityPct: 0.3, rsiFilter: false, label: "30/0.3" },
  { lookback: 50, proximityPct: 0.3, rsiFilter: false, label: "50/0.3" },
  { lookback: 20, proximityPct: 0.2, rsiFilter: true,  label: "20/0.2-rsi" },
  { lookback: 15, proximityPct: 0.3, rsiFilter: true,  label: "15/0.3-rsi" },
  { lookback: 40, proximityPct: 0.4, rsiFilter: false, label: "40/0.4" },
  { lookback: 20, proximityPct: 0.1, rsiFilter: false, label: "20/0.1" },
  { lookback: 25, proximityPct: 0.5, rsiFilter: true,  label: "25/0.5-rsi" },
];

function makeSupportResistanceSignal(
  lookback: number, proximityPct: number, rsiFilter: boolean, side: "BUY" | "SELL"
) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < lookback + 2) return NO_SIGNAL;
    const cl = closes(candles);
    const hist = candles.slice(-lookback - 1, -1);
    const support = Math.min(...hist.map((c) => c.low));
    const resistance = Math.max(...hist.map((c) => c.high));
    const price = cl[cl.length - 1];
    const range = resistance - support;
    if (range <= 0) return NO_SIGNAL;
    const rsi = rsiFilter ? calcRsi(cl, 14) : 50;
    if (side === "BUY") {
      const nearSupport = price <= support * (1 + proximityPct / 100);
      if (nearSupport && (!rsiFilter || rsi < 40)) {
        const prox = Math.max(0, 1 - (price - support) / (range * proximityPct / 100));
        return { side: "BUY", confidence: clampConf(55 + prox * 35) };
      }
    } else {
      const nearResistance = price >= resistance * (1 - proximityPct / 100);
      if (nearResistance && (!rsiFilter || rsi > 60)) {
        const prox = Math.max(0, 1 - (resistance - price) / (range * proximityPct / 100));
        return { side: "SELL", confidence: clampConf(55 + prox * 35) };
      }
    }
    return NO_SIGNAL;
  };
}

// ── Family 18: RangeChop (IDs 1470–1499) ──────────────────────────────────────
// Mean reversion in ranging market: low ADX + price at range extremes.
// 15 variants × 2 sides = 30.

const RANGE_VARIANTS: Array<{
  bbPeriod: number; bbMult: number; adxMax: number; wmaFilter: boolean; label: string;
}> = [
  { bbPeriod: 20, bbMult: 2.0, adxMax: 25, wmaFilter: false, label: "bb20/adx25" },
  { bbPeriod: 20, bbMult: 2.0, adxMax: 20, wmaFilter: false, label: "bb20/adx20" },
  { bbPeriod: 20, bbMult: 2.5, adxMax: 25, wmaFilter: false, label: "bb20-2.5/adx25" },
  { bbPeriod: 14, bbMult: 2.0, adxMax: 25, wmaFilter: false, label: "bb14/adx25" },
  { bbPeriod: 30, bbMult: 2.0, adxMax: 25, wmaFilter: false, label: "bb30/adx25" },
  { bbPeriod: 20, bbMult: 2.0, adxMax: 30, wmaFilter: false, label: "bb20/adx30" },
  { bbPeriod: 20, bbMult: 1.5, adxMax: 20, wmaFilter: false, label: "bb20-1.5/adx20" },
  { bbPeriod: 20, bbMult: 2.0, adxMax: 25, wmaFilter: true,  label: "bb20/adx25-wma" },
  { bbPeriod: 10, bbMult: 2.0, adxMax: 25, wmaFilter: false, label: "bb10/adx25" },
  { bbPeriod: 20, bbMult: 3.0, adxMax: 25, wmaFilter: false, label: "bb20-3.0/adx25" },
  { bbPeriod: 50, bbMult: 2.0, adxMax: 25, wmaFilter: false, label: "bb50/adx25" },
  { bbPeriod: 20, bbMult: 2.0, adxMax: 15, wmaFilter: false, label: "bb20/adx15" },
  { bbPeriod: 14, bbMult: 2.5, adxMax: 20, wmaFilter: true,  label: "bb14-2.5/adx20" },
  { bbPeriod: 20, bbMult: 2.0, adxMax: 25, wmaFilter: true,  label: "bb20/adx25-wma2" },
  { bbPeriod: 25, bbMult: 2.0, adxMax: 22, wmaFilter: false, label: "bb25/adx22" },
];

function makeRangeChopSignal(
  bbPeriod: number, bbMult: number, adxMax: number, wmaFilter: boolean, side: "BUY" | "SELL"
) {
  return (candles: OHLCVCandle[]): ResearchSignal => {
    if (candles.length < bbPeriod + 14 + 2) return NO_SIGNAL;
    const cl = closes(candles);
    const adx = calcAdx(candles, 14);
    if (adx > adxMax) return NO_SIGNAL; // only trade in choppy/ranging market
    const bb = calcBollinger(cl, bbPeriod, bbMult);
    const price = cl[cl.length - 1];
    if (wmaFilter) {
      const wma = calcWma(cl, bbPeriod);
      // WMA acts as trend filter — only take fades near the WMA band side
      if (side === "BUY" && price < wma && price <= bb.lower) {
        return { side: "BUY", confidence: clampConf(65 + (adxMax - adx) * 1.5) };
      }
      if (side === "SELL" && price > wma && price >= bb.upper) {
        return { side: "SELL", confidence: clampConf(65 + (adxMax - adx) * 1.5) };
      }
      return NO_SIGNAL;
    }
    if (side === "BUY" && price <= bb.lower) {
      return { side: "BUY", confidence: clampConf(60 + (adxMax - adx) * 1.5) };
    }
    if (side === "SELL" && price >= bb.upper) {
      return { side: "SELL", confidence: clampConf(60 + (adxMax - adx) * 1.5) };
    }
    return NO_SIGNAL;
  };
}

// ── Build strategy list ───────────────────────────────────────────────────────

function buildPair<P extends object>(
  baseId: number,
  family: ResearchFamily,
  timeframe: "1m" | "5m" | "15m",
  minCandles: number,
  variants: P[],
  getLabel: (p: P) => string,
  getDesc: (p: P, side: "BUY" | "SELL") => string,
  getParams: (p: P) => Record<string, number | string>,
  makeSignalFn: (p: P, side: "BUY" | "SELL") => (c: OHLCVCandle[]) => ResearchSignal,
  namePrefix: string,
): ResearchStrategy[] {
  const out: ResearchStrategy[] = [];
  let id = baseId;
  for (const p of variants) {
    const label = getLabel(p);
    for (const side of ["BUY", "SELL"] as const) {
      const sideTag = side === "BUY" ? "Long" : "Short";
      out.push({
        id: id++,
        name: `${namePrefix}_${label}_${sideTag}`,
        family,
        enabled: true,
        description: getDesc(p, side),
        params: { ...getParams(p), side },
        timeframe,
        minCandles,
        signal: makeSignalFn(p, side),
      });
    }
  }
  return out;
}

const RESEARCH_STRATEGIES: ResearchStrategy[] = [
  // Family 1: TrendFollowing 1000-1029
  ...buildPair<(typeof TRF_VARIANTS)[0]>(
    1000, "TrendFollowing", "1m", 60,
    TRF_VARIANTS,
    (p) => p.label,
    (p, s) => `EMA ${p.fast}/${p.slow} trend with ADX>${p.adxMin} — ${s === "BUY" ? "fast above slow" : "fast below slow"}`,
    (p) => ({ ema_fast: p.fast, ema_slow: p.slow, adx_min: p.adxMin }),
    (p, s) => makeTrendSignal(p.fast, p.slow, p.adxMin, s),
    "TRF",
  ),

  // Family 2: EmaSmaCrossover 1030-1059
  ...buildPair<(typeof EMX_VARIANTS)[0]>(
    1030, "EmaSmaCrossover", "1m", 55,
    EMX_VARIANTS,
    (p) => p.label,
    (p, s) => `EMA(${p.ema}) crosses ${s === "BUY" ? "above" : "below"} SMA(${p.sma})`,
    (p) => ({ ema_period: p.ema, sma_period: p.sma }),
    (p, s) => makeEmaCrossSignal(p.ema, p.sma, s),
    "EMX",
  ),

  // Family 3: VwapStrategies 1060-1089
  ...buildPair<(typeof VWAP_VARIANTS)[0]>(
    1060, "VwapStrategies", "1m", 30,
    VWAP_VARIANTS,
    (p) => p.label,
    (p, s) => `Price ${s === "BUY" ? "below" : "above"} VWAP by ${p.devPct}% with RSI ${s === "BUY" ? `<${p.rsiLong}` : `>${p.rsiShort}`}`,
    (p) => ({ rsi_period: p.rsiPeriod, rsi_long: p.rsiLong, rsi_short: p.rsiShort, dev_pct: p.devPct }),
    (p, s) => makeVwapSignal(p.rsiPeriod, p.rsiLong, p.rsiShort, p.devPct, s),
    "VWAP",
  ),

  // Family 4: RsiMeanReversion 1090-1119
  ...buildPair<(typeof RSI_VARIANTS)[0]>(
    1090, "RsiMeanReversion", "1m", 20,
    RSI_VARIANTS,
    (p) => p.label,
    (p, s) => `RSI(${p.period}) ${s === "BUY" ? `< ${p.oversold} (oversold)` : `> ${p.overbought} (overbought)`}`,
    (p) => ({ rsi_period: p.period, oversold: p.oversold, overbought: p.overbought }),
    (p, s) => makeRsiSignal(p.period, p.oversold, p.overbought, s),
    "RSI",
  ),

  // Family 5: BollingerBand 1120-1149
  ...buildPair<(typeof BB_VARIANTS)[0]>(
    1120, "BollingerBand", "1m", 25,
    BB_VARIANTS,
    (p) => p.label,
    (p, s) => `Price ${s === "BUY" ? "at/below lower BB" : "at/above upper BB"}(${p.period},${p.mult}) with RSI confirm`,
    (p) => ({ bb_period: p.period, bb_mult: p.mult, rsi_confirm: p.rsiConfirm }),
    (p, s) => makeBbSignal(p.period, p.mult, p.rsiConfirm, s),
    "BB",
  ),

  // Family 6: Breakout 1150-1179
  ...buildPair<(typeof BRK_VARIANTS)[0]>(
    1150, "Breakout", "1m", 65,
    BRK_VARIANTS,
    (p) => p.label,
    (p, s) => `Donchian(${p.period}) ${s === "BUY" ? "upside" : "downside"} breakout with vol>${p.volMin}× average`,
    (p) => ({ dc_period: p.period, vol_min: p.volMin, atr_mult: p.atrMult }),
    (p, s) => makeBreakoutSignal(p.period, p.volMin, p.atrMult, s),
    "BRK",
  ),

  // Family 7: LiquiditySweep 1180-1209
  ...buildPair<(typeof LIQ_VARIANTS)[0]>(
    1180, "LiquiditySweep", "1m", 25,
    LIQ_VARIANTS,
    (p) => p.label,
    (p, s) => `Price sweeps ${s === "BUY" ? "below recent low" : "above recent high"} then closes back (${p.lookback}bar/${p.retracePct}%)`,
    (p) => ({ lookback: p.lookback, retrace_pct: p.retracePct }),
    (p, s) => makeLiquiditySweepSignal(p.lookback, p.retracePct, s),
    "LIQ",
  ),

  // Family 8: MomentumContinuation 1210-1239
  ...buildPair<(typeof MOM_VARIANTS)[0]>(
    1210, "MomentumContinuation", "1m", 55,
    MOM_VARIANTS,
    (p) => p.label,
    (p, s) => `ROC(${p.roc}) ${s === "BUY" ? `> ${p.rocMin}%` : `< -${p.rocMin}%`} with price ${s === "BUY" ? "above" : "below"} EMA(${p.ema})`,
    (p) => ({ roc_period: p.roc, ema_period: p.ema, roc_min: p.rocMin }),
    (p, s) => makeMomentumSignal(p.roc, p.ema, p.rocMin, s),
    "MOM",
  ),

  // Family 9: Scalping 1240-1269
  ...buildPair<(typeof SCP_VARIANTS)[0]>(
    1240, "Scalping", "1m", 25,
    SCP_VARIANTS,
    (p) => p.label,
    (p, s) => `RSI(${p.rsi}) ${s === "BUY" ? `< ${p.rsiBuyMax}` : `> ${p.rsiSellMin}`} + price at BB(${p.bbPeriod}) ${s === "BUY" ? "lower" : "upper"} band + vol spike`,
    (p) => ({ rsi_period: p.rsi, rsi_buy_max: p.rsiBuyMax, rsi_sell_min: p.rsiSellMin, bb_period: p.bbPeriod }),
    (p, s) => makeScalpingSignal(p.rsi, p.rsiBuyMax, p.rsiSellMin, p.bbPeriod, s),
    "SCP",
  ),

  // Family 10: VolumeSpike 1270-1289
  ...buildPair<(typeof VOL_VARIANTS)[0]>(
    1270, "VolumeSpike", "1m", 25,
    VOL_VARIANTS,
    (p) => p.label,
    (p, s) => `Volume > ${p.multiple}× ${p.period}-bar avg with ${s === "BUY" ? "bullish" : "bearish"} candle`,
    (p) => ({ vol_period: p.period, vol_multiple: p.multiple }),
    (p, s) => makeVolumeSignal(p.period, p.multiple, s),
    "VOL",
  ),

  // Family 11: VolatilityExpansion 1290-1309
  ...buildPair<(typeof VOLEX_VARIANTS)[0]>(
    1290, "VolatilityExpansion", "1m", 20,
    VOLEX_VARIANTS,
    (p) => p.label,
    (p, s) => `Candle range > ATR(${p.atrPeriod}) × ${p.rangeMult} — ${s === "BUY" ? "bullish" : "bearish"} expansion`,
    (p) => ({ atr_period: p.atrPeriod, range_mult: p.rangeMult }),
    (p, s) => makeVolatilitySignal(p.atrPeriod, p.rangeMult, s),
    "VOLEX",
  ),

  // Family 12: AtrBased 1310-1339
  ...buildPair<(typeof ATR_VARIANTS)[0]>(
    1310, "AtrBased", "1m", 35,
    ATR_VARIANTS,
    (p) => p.label,
    (p, s) => `Price in ATR(${p.atrPeriod})×${p.atrMult} channel around EMA(${p.emaPeriod}) — ${s === "BUY" ? "above mid" : "below mid"}`,
    (p) => ({ atr_period: p.atrPeriod, atr_mult: p.atrMult, ema_period: p.emaPeriod }),
    (p, s) => makeAtrSignal(p.atrPeriod, p.atrMult, p.emaPeriod, s),
    "ATR",
  ),

  // Family 13: MacdStrategies 1340-1369
  ...buildPair<(typeof MACD_VARIANTS)[0]>(
    1340, "MacdStrategies", "1m", 40,
    MACD_VARIANTS,
    (p) => p.label,
    (p, s) => `MACD(${p.fast}/${p.slow}/${p.signal}) ${p.mode === "cross" ? "histogram" : "line"} ${s === "BUY" ? "crosses up" : "crosses down"}`,
    (p) => ({ fast: p.fast, slow: p.slow, signal: p.signal, mode: p.mode }),
    (p, s) => makeMacdSignal(p.fast, p.slow, p.signal, p.mode, s),
    "MACD",
  ),

  // Family 14: StochasticStrategies 1370-1389
  ...buildPair<(typeof STOCH_VARIANTS)[0]>(
    1370, "StochasticStrategies", "1m", 25,
    STOCH_VARIANTS,
    (p) => p.label,
    (p, s) => `Stochastic(${p.kPeriod}/${p.dPeriod}) ${s === "BUY" ? `%K < ${p.osLevel} + K crosses D` : `%K > ${p.obLevel} + K crosses D`}`,
    (p) => ({ k_period: p.kPeriod, d_period: p.dPeriod, ob_level: p.obLevel, os_level: p.osLevel }),
    (p, s) => makeStochasticSignal(p.kPeriod, p.dPeriod, p.obLevel, p.osLevel, s),
    "STOCH",
  ),

  // Family 15: CandlePattern 1390-1419
  ...buildPair<CandlePatternSpec>(
    1390, "CandlePattern", "1m", 10,
    CP_VARIANTS,
    (p) => p.label,
    (p, s) => `${p.label} pattern${p.rsiFilter ? " + RSI filter" : ""}${p.volFilter ? " + volume filter" : ""} — ${s}`,
    (p) => ({
      pattern: p.pattern,
      rsi_filter: p.rsiFilter ? 1 : 0,
      rsi_period: p.rsiPeriod,
      vol_filter: p.volFilter ? 1 : 0,
    }),
    (p, s) => makeCandlePatternSignal(p, s),
    "CP",
  ),

  // Family 16: MultiTimeframe 1420-1449
  ...buildPair<(typeof MTF_VARIANTS)[0]>(
    1420, "MultiTimeframe", "1m", 130,
    MTF_VARIANTS,
    (p) => p.label,
    (p, s) => `EMA(${p.emaFast}) > EMA(${p.emaMid}) > EMA(${p.emaSlow}) all ${s === "BUY" ? "bullish" : "bearish"} aligned + RSI not ${s === "BUY" ? "OB" : "OS"}`,
    (p) => ({ ema_fast: p.emaFast, ema_mid: p.emaMid, ema_slow: p.emaSlow, rsi_period: p.rsiPeriod }),
    (p, s) => makeMtfSignal(p.emaFast, p.emaMid, p.emaSlow, p.rsiPeriod, p.rsiLimit, s),
    "MTF",
  ),

  // Family 17: SupportResistance 1450-1469
  ...buildPair<(typeof SR_VARIANTS)[0]>(
    1450, "SupportResistance", "1m", 55,
    SR_VARIANTS,
    (p) => p.label,
    (p, s) => `Price near ${s === "BUY" ? "support" : "resistance"} (${p.lookback}bar, ${p.proximityPct}% proximity)${p.rsiFilter ? " + RSI" : ""}`,
    (p) => ({ lookback: p.lookback, proximity_pct: p.proximityPct, rsi_filter: p.rsiFilter ? 1 : 0 }),
    (p, s) => makeSupportResistanceSignal(p.lookback, p.proximityPct, p.rsiFilter, s),
    "SR",
  ),

  // Family 18: RangeChop 1470-1499
  ...buildPair<(typeof RANGE_VARIANTS)[0]>(
    1470, "RangeChop", "1m", 30,
    RANGE_VARIANTS,
    (p) => p.label,
    (p, s) => `Price at BB(${p.bbPeriod},${p.bbMult}) ${s === "BUY" ? "lower" : "upper"} in low-ADX(<${p.adxMax}) chop${p.wmaFilter ? " + WMA filter" : ""}`,
    (p) => ({ bb_period: p.bbPeriod, bb_mult: p.bbMult, adx_max: p.adxMax, wma_filter: p.wmaFilter ? 1 : 0 }),
    (p, s) => makeRangeChopSignal(p.bbPeriod, p.bbMult, p.adxMax, p.wmaFilter, s),
    "RANGE",
  ),
];

// Freeze the registry at module load — verify exactly 500 strategies.
const _ids = RESEARCH_STRATEGIES.map((s) => s.id);
const _unique = new Set(_ids);
if (_unique.size !== 500 || _ids.length !== 500) {
  throw new Error(`Research strategy registry must have exactly 500 strategies, got ${_ids.length} (${_unique.size} unique IDs)`);
}

// ── Public exports ────────────────────────────────────────────────────────────
export { RESEARCH_STRATEGIES };

/** Fast O(1) lookup by strategy ID. */
export const RESEARCH_STRATEGY_BY_ID: ReadonlyMap<number, ResearchStrategy> = new Map(
  RESEARCH_STRATEGIES.map((s) => [s.id, s]),
);

/** Families that contain at least one strategy (for UI filters). */
export const RESEARCH_FAMILIES_WITH_STRATEGIES: ResearchFamily[] = [
  ...new Set(RESEARCH_STRATEGIES.map((s) => s.family)),
];
