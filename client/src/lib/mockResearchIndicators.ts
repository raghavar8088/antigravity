/**
 * Mock Research — pure TA indicator functions for client-side strategy evaluation.
 *
 * All functions are pure (no side effects, no I/O). Used exclusively by the
 * mock research runner — never imported by production trading code paths.
 */

// ── OHLCV candle ─────────────────────────────────────────────────────────────
export interface OHLCVCandle {
  /** Epoch ms — start of the 1-minute bar. */
  time: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
}

// ── Signal output ─────────────────────────────────────────────────────────────
export interface ResearchSignal {
  side: "BUY" | "SELL" | "NO_SIGNAL";
  /** 0–100. Higher = stronger condition alignment. */
  confidence: number;
}

export const NO_SIGNAL: ResearchSignal = { side: "NO_SIGNAL", confidence: 0 };

// ── Moving averages ───────────────────────────────────────────────────────────
export function calcEma(values: number[], period: number): number {
  if (values.length === 0) return 0;
  if (values.length < period) return values[values.length - 1];
  const k = 2 / (period + 1);
  let e = values.slice(0, period).reduce((s, v) => s + v, 0) / period;
  for (let i = period; i < values.length; i++) e = values[i] * k + e * (1 - k);
  return e;
}

export function calcSma(values: number[], period: number): number {
  if (values.length === 0) return 0;
  const slice = values.slice(-Math.min(period, values.length));
  return slice.reduce((s, v) => s + v, 0) / slice.length;
}

export function calcWma(values: number[], period: number): number {
  const slice = values.slice(-Math.min(period, values.length));
  let wSum = 0;
  let dSum = 0;
  for (let i = 0; i < slice.length; i++) {
    const w = i + 1;
    wSum += slice[i] * w;
    dSum += w;
  }
  return dSum > 0 ? wSum / dSum : slice[slice.length - 1] ?? 0;
}

// ── RSI ───────────────────────────────────────────────────────────────────────
export function calcRsi(values: number[], period = 14): number {
  if (values.length < period + 1) return 50;
  let avgGain = 0;
  let avgLoss = 0;
  const start = values.length - period;
  for (let i = start; i < values.length; i++) {
    const d = values[i] - values[i - 1];
    if (d > 0) avgGain += d;
    else avgLoss -= d;
  }
  avgGain /= period;
  avgLoss /= period;
  if (avgLoss === 0) return 100;
  return 100 - 100 / (1 + avgGain / avgLoss);
}

// ── MACD ──────────────────────────────────────────────────────────────────────
export function calcMacd(
  values: number[],
  fastPeriod = 12,
  slowPeriod = 26,
  signalPeriod = 9,
): { line: number; signal: number; histogram: number } {
  if (values.length < slowPeriod + signalPeriod) {
    return { line: 0, signal: 0, histogram: 0 };
  }
  const kf = 2 / (fastPeriod + 1);
  const ks = 2 / (slowPeriod + 1);
  let ef = values[0];
  let es = values[0];
  const macdSeries: number[] = [];
  for (const v of values) {
    ef = v * kf + ef * (1 - kf);
    es = v * ks + es * (1 - ks);
    macdSeries.push(ef - es);
  }
  const line = macdSeries[macdSeries.length - 1];
  const signal = calcEma(macdSeries, signalPeriod);
  return { line, signal, histogram: line - signal };
}

// ── Bollinger Bands ───────────────────────────────────────────────────────────
export function calcBollinger(
  values: number[],
  period = 20,
  mult = 2,
): { upper: number; middle: number; lower: number; bandwidth: number; percentB: number } {
  const n = Math.min(period, values.length);
  const slice = values.slice(-n);
  const middle = slice.reduce((s, v) => s + v, 0) / n;
  const variance = slice.reduce((s, v) => s + (v - middle) ** 2, 0) / n;
  const std = Math.sqrt(variance);
  const upper = middle + mult * std;
  const lower = middle - mult * std;
  const bandwidth = upper - lower;
  const last = values[values.length - 1] ?? middle;
  const percentB = bandwidth > 0 ? (last - lower) / bandwidth : 0.5;
  return { upper, middle, lower, bandwidth, percentB };
}

// ── ATR ───────────────────────────────────────────────────────────────────────
export function calcAtr(candles: OHLCVCandle[], period = 14): number {
  if (candles.length < 2) return 0;
  const trs: number[] = [];
  for (let i = 1; i < candles.length; i++) {
    const { high, low } = candles[i];
    const pc = candles[i - 1].close;
    trs.push(Math.max(high - low, Math.abs(high - pc), Math.abs(low - pc)));
  }
  return calcSma(trs, period);
}

// ── VWAP (session) ────────────────────────────────────────────────────────────
export function calcVwap(candles: OHLCVCandle[]): number {
  let pv = 0;
  let vol = 0;
  for (const c of candles) {
    const tp = (c.high + c.low + c.close) / 3;
    pv += tp * c.volume;
    vol += c.volume;
  }
  return vol > 0 ? pv / vol : (candles[candles.length - 1]?.close ?? 0);
}

// ── Stochastic %K / %D ────────────────────────────────────────────────────────
export function calcStochastic(
  candles: OHLCVCandle[],
  kPeriod = 14,
  dPeriod = 3,
): { k: number; d: number } {
  if (candles.length < kPeriod) return { k: 50, d: 50 };
  const kSeries: number[] = [];
  for (let i = kPeriod - 1; i < candles.length; i++) {
    const slice = candles.slice(i - kPeriod + 1, i + 1);
    const lo = Math.min(...slice.map((c) => c.low));
    const hi = Math.max(...slice.map((c) => c.high));
    const cl = slice[slice.length - 1].close;
    kSeries.push(hi === lo ? 50 : ((cl - lo) / (hi - lo)) * 100);
  }
  const k = kSeries[kSeries.length - 1];
  const d = calcSma(kSeries, dPeriod);
  return { k, d };
}

// ── Momentum (rate of change %) ───────────────────────────────────────────────
export function calcMomentum(values: number[], period: number): number {
  if (values.length < period + 1) return 0;
  const curr = values[values.length - 1];
  const prev = values[values.length - 1 - period];
  return prev > 0 ? (curr / prev - 1) * 100 : 0;
}

// ── Simplified ADX (directional movement) ────────────────────────────────────
export function calcAdx(candles: OHLCVCandle[], period = 14): number {
  if (candles.length < period * 2) return 0;
  const dxArr: number[] = [];
  for (let i = 1; i < candles.length; i++) {
    const h = candles[i].high;
    const l = candles[i].low;
    const ph = candles[i - 1].high;
    const pl = candles[i - 1].low;
    const pc = candles[i - 1].close;
    const tr = Math.max(h - l, Math.abs(h - pc), Math.abs(l - pc));
    const dmPlus = h - ph > pl - l ? Math.max(h - ph, 0) : 0;
    const dmMinus = pl - l > h - ph ? Math.max(pl - l, 0) : 0;
    const diSum = dmPlus + dmMinus;
    dxArr.push(diSum > 0 && tr > 0 ? (Math.abs(dmPlus - dmMinus) / diSum) * 100 : 0);
  }
  return calcSma(dxArr, period);
}

// ── Volume ratio vs N-period average ─────────────────────────────────────────
export function calcVolumeRatio(candles: OHLCVCandle[], period = 20): number {
  if (candles.length < 2) return 1;
  const vols = candles.slice(-period).map((c) => c.volume);
  const avg = vols.reduce((s, v) => s + v, 0) / vols.length;
  return avg > 0 ? (candles[candles.length - 1].volume / avg) : 1;
}

// ── Donchian channel ──────────────────────────────────────────────────────────
export function calcDonchian(
  candles: OHLCVCandle[],
  period: number,
): { upper: number; lower: number; mid: number } {
  const slice = candles.slice(-Math.min(period, candles.length));
  if (slice.length === 0) {
    const v = candles[candles.length - 1]?.close ?? 0;
    return { upper: v, lower: v, mid: v };
  }
  const upper = Math.max(...slice.map((c) => c.high));
  const lower = Math.min(...slice.map((c) => c.low));
  return { upper, lower, mid: (upper + lower) / 2 };
}

// ── Candle pattern helpers ────────────────────────────────────────────────────
export function isBullishEngulfing(candles: OHLCVCandle[]): boolean {
  if (candles.length < 2) return false;
  const prev = candles[candles.length - 2];
  const curr = candles[candles.length - 1];
  return (
    prev.close < prev.open &&
    curr.close > curr.open &&
    curr.open <= prev.close &&
    curr.close >= prev.open
  );
}

export function isBearishEngulfing(candles: OHLCVCandle[]): boolean {
  if (candles.length < 2) return false;
  const prev = candles[candles.length - 2];
  const curr = candles[candles.length - 1];
  return (
    prev.close > prev.open &&
    curr.close < curr.open &&
    curr.open >= prev.close &&
    curr.close <= prev.open
  );
}

export function isDoji(candle: OHLCVCandle): boolean {
  const body = Math.abs(candle.close - candle.open);
  const range = candle.high - candle.low;
  return range > 0 && body / range < 0.1;
}

export function isHammer(candle: OHLCVCandle): boolean {
  const body = Math.abs(candle.close - candle.open);
  const lowerShadow = Math.min(candle.open, candle.close) - candle.low;
  const upperShadow = candle.high - Math.max(candle.open, candle.close);
  return body > 0 && lowerShadow >= body * 2 && upperShadow <= body * 0.5;
}

export function isShootingStar(candle: OHLCVCandle): boolean {
  const body = Math.abs(candle.close - candle.open);
  const lowerShadow = Math.min(candle.open, candle.close) - candle.low;
  const upperShadow = candle.high - Math.max(candle.open, candle.close);
  return body > 0 && upperShadow >= body * 2 && lowerShadow <= body * 0.5;
}

export function isInvertedHammer(candle: OHLCVCandle): boolean {
  return isShootingStar(candle) && candle.close > candle.open;
}

export function isBullishPinBar(candle: OHLCVCandle): boolean {
  const body = Math.abs(candle.close - candle.open);
  const lowerShadow = Math.min(candle.open, candle.close) - candle.low;
  const totalRange = candle.high - candle.low;
  return totalRange > 0 && lowerShadow / totalRange > 0.6 && body / totalRange < 0.3;
}

export function isBearishPinBar(candle: OHLCVCandle): boolean {
  const body = Math.abs(candle.close - candle.open);
  const upperShadow = candle.high - Math.max(candle.open, candle.close);
  const totalRange = candle.high - candle.low;
  return totalRange > 0 && upperShadow / totalRange > 0.6 && body / totalRange < 0.3;
}

export function isThreeWhiteSoldiers(candles: OHLCVCandle[]): boolean {
  if (candles.length < 3) return false;
  const [c1, c2, c3] = candles.slice(-3);
  return (
    c1.close > c1.open &&
    c2.close > c2.open &&
    c3.close > c3.open &&
    c2.close > c1.close &&
    c3.close > c2.close
  );
}

export function isThreeBlackCrows(candles: OHLCVCandle[]): boolean {
  if (candles.length < 3) return false;
  const [c1, c2, c3] = candles.slice(-3);
  return (
    c1.close < c1.open &&
    c2.close < c2.open &&
    c3.close < c3.open &&
    c2.close < c1.close &&
    c3.close < c2.close
  );
}

export function isMorningStar(candles: OHLCVCandle[]): boolean {
  if (candles.length < 3) return false;
  const [c1, c2, c3] = candles.slice(-3);
  const c1Body = c1.open - c1.close; // bearish
  const c3Body = c3.close - c3.open; // bullish
  return (
    c1Body > 0 &&
    isDoji(c2) &&
    c3Body > 0 &&
    c3Body > c1Body * 0.5
  );
}

export function isEveningStar(candles: OHLCVCandle[]): boolean {
  if (candles.length < 3) return false;
  const [c1, c2, c3] = candles.slice(-3);
  const c1Body = c1.close - c1.open; // bullish
  const c3Body = c3.open - c3.close; // bearish
  return (
    c1Body > 0 &&
    isDoji(c2) &&
    c3Body > 0 &&
    c3Body > c1Body * 0.5
  );
}

// ── Confidence helper ─────────────────────────────────────────────────────────
/** Clamp a raw score to a confidence in [minConf, 100]. */
export function clampConf(raw: number, minConf = 40): number {
  return Math.min(100, Math.max(minConf, Math.round(raw)));
}
