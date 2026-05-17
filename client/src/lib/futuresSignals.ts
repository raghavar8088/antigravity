/**
 * Pure signal evaluation engine for the futures paper desk (zero React).
 * All indicator math, scoring, and entry confirmation logic lives here.
 * The hook imports these — no logic duplication.
 */

import type { FuturesStratDef, RegimeTag } from "./futuresStratTypes";

// ========== SIGNAL INPUTS ==========
export type FuturesSignalInputs = {
  price: number;
  markPrice: number;
  prevPrice: number;
  fast: number;
  slow: number;
  trend: number;
  prevFast: number;
  prevSlow: number;
  mean20: number;
  std20: number;
  rsi14: number;
  high20: number;
  low20: number;
  momentum3: number;
  momentum6: number;
  volRatio: number;
  bbUpper: number;
  bbLower: number;
  bbWidth: number;
  stochK: number;
  stochD: number;
  prevStochK: number;
  prevStochD: number;
  macdLine: number;
  macdSignal: number;
  prevMacdLine: number;
  prevMacdSignal: number;
  atr14: number;
  obvSlope: number;
  momentum10: number;
  rsi7: number;
  williamsR: number;
  prevWilliamsR: number;
  cci20: number;
  roc10: number;
  keltnerUpper: number;
  keltnerLower: number;
  donchianHigh: number;
  donchianLow: number;
  donchianMid: number;
  vwapDev: number;
  adxProxy: number;
  ema5: number;
  ema13: number;
  prevEma5: number;
  prevEma13: number;
  rsi21: number;
  macdHist: number;
  prevMacdHist: number;
  htf5_fast: number;
  htf5_slow: number;
  htf5_rsi: number;
  htf5_momentum: number;
  htf5_trend: number;
  htf15_fast: number;
  htf15_slow: number;
  htf15_rsi: number;
  htf15_momentum: number;
  htf15_trend: number;
  htf5_macdHist: number;
  htf15_macdHist: number;
  htf5_bbWidth: number;
  htf15_bbWidth: number;
  htf5_adx: number;
  htf15_adx: number;
  /** Closed-bar exchange time (ms); optional — SESSION_OPEN template uses UTC hour when set. */
  lastBarTimeMs?: number;
};

// ========== INDICATOR HELPERS (private) ==========

function sma(arr: number[], period: number): number[] {
  const out: number[] = [];
  for (let i = 0; i < arr.length; i++) {
    if (i + 1 < period) {
      out.push(NaN);
      continue;
    }
    let sum = 0;
    for (let j = 0; j < period; j++) sum += arr[i - j];
    out.push(sum / period);
  }
  return out;
}

function ema(arr: number[], period: number): number[] {
  const k = 2 / (period + 1);
  const out: number[] = [];
  let prev: number | null = null;
  for (let i = 0; i < arr.length; i++) {
    if (i + 1 < period) {
      out.push(NaN);
      continue;
    }
    if (prev === null) {
      let sum = 0;
      for (let j = 0; j < period; j++) sum += arr[i - j];
      prev = sum / period;
    } else {
      prev = arr[i] * k + prev * (1 - k);
    }
    out.push(prev);
  }
  return out;
}

function rsi(closes: number[], period = 14): number[] {
  const out: number[] = [];
  let gainSum = 0;
  let lossSum = 0;
  for (let i = 1; i < closes.length; i++) {
    const change = closes[i] - closes[i - 1];
    const gain = Math.max(change, 0);
    const loss = Math.max(-change, 0);
    if (i < period) {
      gainSum += gain;
      lossSum += loss;
      out.push(NaN);
    } else if (i === period) {
      gainSum += gain;
      lossSum += loss;
      const avgGain = gainSum / period;
      const avgLoss = lossSum / period;
      const rs = avgLoss === 0 ? 0 : avgGain / avgLoss;
      out.push(avgLoss === 0 ? 100 : 100 - 100 / (1 + rs));
    } else {
      const prevAvgGain = (gainSum / period) * (period - 1) + gain;
      const prevAvgLoss = (lossSum / period) * (period - 1) + loss;
      gainSum = prevAvgGain;
      lossSum = prevAvgLoss;
      const avgGain = gainSum / period;
      const avgLoss = lossSum / period;
      const rs = avgLoss === 0 ? 0 : avgGain / avgLoss;
      out.push(avgLoss === 0 ? 100 : 100 - 100 / (1 + rs));
    }
  }
  // Align with `closes` indices: index `i` uses change into closes[i]; rsi[0] is unused (NaN).
  return [NaN, ...out];
}

function stdDev(arr: number[], period: number): number[] {
  const out: number[] = [];
  for (let i = 0; i < arr.length; i++) {
    if (i + 1 < period) {
      out.push(NaN);
      continue;
    }
    let sum = 0;
    for (let j = 0; j < period; j++) sum += arr[i - j];
    const mean = sum / period;
    let sqSum = 0;
    for (let j = 0; j < period; j++) sqSum += Math.pow(arr[i - j] - mean, 2);
    out.push(Math.sqrt(sqSum / period));
  }
  return out;
}

function stochastic(high: number[], low: number[], close: number[], kPeriod = 14, dPeriod = 3): { k: number[]; d: number[] } {
  const k: number[] = [];
  for (let i = 0; i < close.length; i++) {
    if (i + 1 < kPeriod) {
      k.push(NaN);
      continue;
    }
    let highest = -Infinity;
    let lowest = Infinity;
    for (let j = 0; j < kPeriod; j++) {
      highest = Math.max(highest, high[i - j]);
      lowest = Math.min(lowest, low[i - j]);
    }
    const range = highest - lowest;
    k.push(range === 0 ? 50 : ((close[i] - lowest) / range) * 100);
  }
  const d = sma(k, dPeriod);
  return { k, d };
}

function macd(close: number[], fast = 12, slow = 26, signal = 9): { line: number[]; signal: number[]; hist: number[] } {
  const fastEma = ema(close, fast);
  const slowEma = ema(close, slow);
  const line: number[] = [];
  for (let i = 0; i < close.length; i++) line.push(fastEma[i] - slowEma[i]);
  const sig = ema(line.filter(n => !isNaN(n)), signal);
  const paddedSig: number[] = [];
  for (let i = 0; i < line.length; i++) paddedSig.push(isNaN(line[i]) ? NaN : (sig[i - (line.length - sig.length)] ?? NaN));
  const hist: number[] = [];
  for (let i = 0; i < line.length; i++) hist.push(line[i] - paddedSig[i]);
  return { line, signal: paddedSig, hist };
}

function atr(high: number[], low: number[], close: number[], period = 14): number[] {
  const tr: number[] = [];
  for (let i = 0; i < close.length; i++) {
    if (i === 0) { tr.push(high[i] - low[i]); continue; }
    const v1 = high[i] - low[i];
    const v2 = Math.abs(high[i] - close[i - 1]);
    const v3 = Math.abs(low[i] - close[i - 1]);
    tr.push(Math.max(v1, v2, v3));
  }
  return sma(tr, period);
}

function obv(close: number[], volume: number[]): number[] {
  const out: number[] = [volume[0] ?? 0];
  for (let i = 1; i < close.length; i++) {
    if (close[i] > close[i - 1]) out.push(out[i - 1] + volume[i]);
    else if (close[i] < close[i - 1]) out.push(out[i - 1] - volume[i]);
    else out.push(out[i - 1]);
  }
  return out;
}

function slope(arr: number[], period = 5): number[] {
  const out: number[] = [];
  for (let i = 0; i < arr.length; i++) {
    if (i < period) { out.push(0); continue; }
    out.push(arr[i] - arr[i - period]);
  }
  return out;
}

function williamsR(high: number[], low: number[], close: number[], period = 14): number[] {
  const out: number[] = [];
  for (let i = 0; i < close.length; i++) {
    if (i + 1 < period) { out.push(NaN); continue; }
    let highest = -Infinity;
    let lowest = Infinity;
    for (let j = 0; j < period; j++) {
      highest = Math.max(highest, high[i - j]);
      lowest = Math.min(lowest, low[i - j]);
    }
    const range = highest - lowest;
    out.push(range === 0 ? -50 : ((highest - close[i]) / range) * -100);
  }
  return out;
}

function cci(high: number[], low: number[], close: number[], period = 20): number[] {
  const out: number[] = [];
  const tp = high.map((h, i) => (h + low[i] + close[i]) / 3);
  const ma = sma(tp, period);
  const md: number[] = [];
  for (let i = 0; i < tp.length; i++) {
    if (i + 1 < period) { md.push(NaN); continue; }
    let sum = 0;
    for (let j = 0; j < period; j++) sum += Math.abs(tp[i - j] - ma[i]);
    md.push(sum / period);
  }
  for (let i = 0; i < tp.length; i++) out.push(md[i] === 0 ? 0 : (tp[i] - ma[i]) / (0.015 * md[i]));
  return out;
}

function rateOfChange(close: number[], period = 10): number[] {
  const out: number[] = [];
  for (let i = 0; i < close.length; i++) {
    if (i < period) { out.push(NaN); continue; }
    out.push(((close[i] - close[i - period]) / close[i - period]) * 100);
  }
  return out;
}

function adxProxy(high: number[], low: number[], close: number[], period = 14): number[] {
  const tr: number[] = [];
  const plusDM: number[] = [];
  const minusDM: number[] = [];
  for (let i = 0; i < close.length; i++) {
    if (i === 0) { tr.push(high[i] - low[i]); plusDM.push(0); minusDM.push(0); continue; }
    tr.push(Math.max(high[i] - low[i], Math.abs(high[i] - close[i - 1]), Math.abs(low[i] - close[i - 1])));
    plusDM.push(high[i] - high[i - 1] > low[i - 1] - low[i] ? Math.max(high[i] - high[i - 1], 0) : 0);
    minusDM.push(low[i - 1] - low[i] > high[i] - high[i - 1] ? Math.max(low[i - 1] - low[i], 0) : 0);
  }
  const atrVals = sma(tr, period);
  const plusDI: number[] = [];
  const minusDI: number[] = [];
  for (let i = 0; i < close.length; i++) {
    if (atrVals[i] === 0 || isNaN(atrVals[i])) { plusDI.push(NaN); minusDI.push(NaN); continue; }
    plusDI.push((sma(plusDM, period)[i] / atrVals[i]) * 100);
    minusDI.push((sma(minusDM, period)[i] / atrVals[i]) * 100);
  }
  const dx: number[] = [];
  for (let i = 0; i < close.length; i++) {
    const sum = plusDI[i] + minusDI[i];
    dx.push(sum === 0 || isNaN(sum) ? NaN : (Math.abs(plusDI[i] - minusDI[i]) / sum) * 100);
  }
  return sma(dx, period);
}

function keltner(high: number[], low: number[], close: number[], emaPeriod = 20, atrPeriod = 14, multiplier = 2): { upper: number[]; lower: number[] } {
  const mid = ema(close, emaPeriod);
  const atrVals = atr(high, low, close, atrPeriod);
  const upper: number[] = [];
  const lower: number[] = [];
  for (let i = 0; i < close.length; i++) {
    upper.push(mid[i] + multiplier * atrVals[i]);
    lower.push(mid[i] - multiplier * atrVals[i]);
  }
  return { upper, lower };
}

function donchian(high: number[], low: number[], period = 20): { upper: number[]; lower: number[]; mid: number[] } {
  const upper: number[] = [];
  const lower: number[] = [];
  for (let i = 0; i < high.length; i++) {
    if (i + 1 < period) { upper.push(NaN); lower.push(NaN); continue; }
    let highest = -Infinity;
    let lowest = Infinity;
    for (let j = 0; j < period; j++) {
      highest = Math.max(highest, high[i - j]);
      lowest = Math.min(lowest, low[i - j]);
    }
    upper.push(highest);
    lower.push(lowest);
  }
  const mid = upper.map((u, i) => (u + lower[i]) / 2);
  return { upper, lower, mid };
}

/**
 * Roll N consecutive 1m bars into one bar (true OHLCV): open=first open, high=max(highs),
 * low=min(lows), close=last close, volume=sum(volumes). Remainder bars &lt; N are dropped.
 */
export function aggregate1mOhlcvToPeriodMinutes(
  open1m: number[],
  high1m: number[],
  low1m: number[],
  close1m: number[],
  volume1m: number[],
  periodMinutes: number,
): { open: number[]; high: number[]; low: number[]; close: number[]; volume: number[] } {
  const n = close1m.length;
  if (
    periodMinutes < 2 ||
    open1m.length !== n ||
    high1m.length !== n ||
    low1m.length !== n ||
    volume1m.length !== n
  ) {
    return { open: [], high: [], low: [], close: [], volume: [] };
  }
  const ao: number[] = [];
  const ah: number[] = [];
  const al: number[] = [];
  const ac: number[] = [];
  const av: number[] = [];
  for (let start = 0; start + periodMinutes <= n; start += periodMinutes) {
    const end = start + periodMinutes;
    let h = -Infinity;
    let l = Infinity;
    let vol = 0;
    for (let i = start; i < end; i++) {
      h = Math.max(h, high1m[i]);
      l = Math.min(l, low1m[i]);
      vol += volume1m[i] || 0;
    }
    ao.push(open1m[start]);
    ah.push(h);
    al.push(l);
    ac.push(close1m[end - 1]);
    av.push(vol);
  }
  return { open: ao, high: ah, low: al, close: ac, volume: av };
}

/**
 * Classifies **15m** OHLC into a coarse regime using **ADX proxy** (trend strength) and **ATR14**
 * **percentile rank** within a trailing window (volatility context).
 *
 * - **chop**: weak trend (`adx < 22`) **or** compressed ATR vs recent history (`atrPct < 0.2`).
 * - **trendHigh**: strong trend and elevated vol (`adx ≥ 38` and `atrPct ≥ 0.5`).
 * - **trendLow**: intermediate conditions.
 *
 * `atrPct` = fraction of ATR values in the lookback window (up to 48 bars, requiring ATR > 0) that are
 * `≤` the current bar’s ATR (empirical CDF at the current point).
 */
export function classifyRegimeTagFrom15mBars(high: number[], low: number[], close: number[]): RegimeTag {
  if (close.length < 25) return "chop";
  const atrS = atr(high, low, close, 14);
  const adxS = adxProxy(high, low, close);
  let i = close.length - 1;
  while (i >= 0 && (!Number.isFinite(atrS[i]) || !Number.isFinite(adxS[i]))) i--;
  if (i < 1) return "chop";

  const look = Math.min(48, i);
  const from = Math.max(0, i - look);
  const atrWindow: number[] = [];
  for (let j = from; j <= i; j++) {
    if (Number.isFinite(atrS[j]) && atrS[j] > 0) atrWindow.push(atrS[j]);
  }
  const curAtr = atrS[i];
  let atrPct = 0.5;
  if (atrWindow.length > 0 && Number.isFinite(curAtr) && curAtr > 0) {
    atrPct = atrWindow.filter((v) => v <= curAtr).length / atrWindow.length;
  }
  const adx = adxS[i];
  if (!Number.isFinite(adx)) return "chop";
  if (adx < 22 || atrPct < 0.2) return "chop";
  if (adx >= 38 && atrPct >= 0.5) return "trendHigh";
  return "trendLow";
}

/** Aggregates 1m OHLCV to 15m bars, then `classifyRegimeTagFrom15mBars`. */
export function classifyRegimeTagFrom1mOhlcv(
  open1m: number[],
  high1m: number[],
  low1m: number[],
  close1m: number[],
  volume1m: number[],
): RegimeTag {
  const agg = aggregate1mOhlcvToPeriodMinutes(open1m, high1m, low1m, close1m, volume1m, 15);
  return classifyRegimeTagFrom15mBars(agg.high, agg.low, agg.close);
}

export function htfTrend(fast: number, slow: number, momentum: number): "UP" | "DOWN" | "NEUTRAL" {
  if (fast > slow && momentum > 0) return "UP";
  if (fast < slow && momentum < 0) return "DOWN";
  return "NEUTRAL";
}

// ========== BUILD HTF FIELDS ==========

function buildHtfFields(
  opens: number[],
  closes: number[],
  highs: number[],
  lows: number[],
  volumes: number[],
  period: number,
): {
  htf5_fast: number; htf5_slow: number; htf5_rsi: number; htf5_momentum: number; htf5_trend: number;
  htf5_macdHist: number; htf5_bbWidth: number; htf5_adx: number;
  htf15_fast: number; htf15_slow: number; htf15_rsi: number; htf15_momentum: number; htf15_trend: number;
  htf15_macdHist: number; htf15_bbWidth: number; htf15_adx: number;
} {
  const agg = aggregate1mOhlcvToPeriodMinutes(opens, highs, lows, closes, volumes, period);
  const aggClose = agg.close;
  const aggHigh = agg.high;
  const aggLow = agg.low;
  const aggVol = agg.volume;

  const fast = ema(aggClose, 9);
  const slow = ema(aggClose, 21);
  const rsiVals = (() => {
    const out: number[] = [];
    let gainSum = 0;
    let lossSum = 0;
    for (let i = 1; i < aggClose.length; i++) {
      const change = aggClose[i] - aggClose[i - 1];
      const gain = Math.max(change, 0);
      const loss = Math.max(-change, 0);
      if (i < 14) { gainSum += gain; lossSum += loss; out.push(NaN); }
      else if (i === 14) { gainSum += gain; lossSum += loss; const avgGain = gainSum / 14; const avgLoss = lossSum / 14; const rs = avgLoss === 0 ? 0 : avgGain / avgLoss; out.push(avgLoss === 0 ? 100 : 100 - 100 / (1 + rs)); }
      else { const prevAvgGain = (gainSum / 14) * 13 + gain; const prevAvgLoss = (lossSum / 14) * 13 + loss; gainSum = prevAvgGain; lossSum = prevAvgLoss; const avgGain = gainSum / 14; const avgLoss = lossSum / 14; const rs = avgLoss === 0 ? 0 : avgGain / avgLoss; out.push(avgLoss === 0 ? 100 : 100 - 100 / (1 + rs)); }
    }
    return out;
  })();
  const mom = aggClose.map((c, i) => i >= 3 ? c - aggClose[i - 3] : 0);
  const macdVals = macd(aggClose);
  const mean20 = sma(aggClose, 20);
  const std20 = stdDev(aggClose, 20);
  const bbWidth = mean20.map((m, i) => m > 0 ? (4 * std20[i]) / m : 0);
  const adxVals = adxProxy(aggHigh, aggLow, aggClose);

  const idx = aggClose.length - 1;

  const result = {
    htf5_fast: 0, htf5_slow: 0, htf5_rsi: 0, htf5_momentum: 0, htf5_trend: 0,
    htf5_macdHist: 0, htf5_bbWidth: 0, htf5_adx: 0,
    htf15_fast: 0, htf15_slow: 0, htf15_rsi: 0, htf15_momentum: 0, htf15_trend: 0,
    htf15_macdHist: 0, htf15_bbWidth: 0, htf15_adx: 0,
  };

  if (period === 5) {
    result.htf5_fast = fast[idx];
    result.htf5_slow = slow[idx];
    result.htf5_rsi = rsiVals[idx];
    result.htf5_momentum = mom[idx];
    result.htf5_trend = htfTrend(fast[idx], slow[idx], mom[idx]) === "UP" ? 1 : htfTrend(fast[idx], slow[idx], mom[idx]) === "DOWN" ? -1 : 0;
    result.htf5_macdHist = macdVals.hist[idx];
    result.htf5_bbWidth = bbWidth[idx];
    result.htf5_adx = adxVals[idx];
  } else {
    result.htf15_fast = fast[idx];
    result.htf15_slow = slow[idx];
    result.htf15_rsi = rsiVals[idx];
    result.htf15_momentum = mom[idx];
    result.htf15_trend = htfTrend(fast[idx], slow[idx], mom[idx]) === "UP" ? 1 : htfTrend(fast[idx], slow[idx], mom[idx]) === "DOWN" ? -1 : 0;
    result.htf15_macdHist = macdVals.hist[idx];
    result.htf15_bbWidth = bbWidth[idx];
    result.htf15_adx = adxVals[idx];
  }

  return result;
}

// ========== BUILD SIGNAL INPUTS ==========

export function buildSignalInputs(
  opens: number[],
  closes: number[],
  highs: number[],
  lows: number[],
  volumes: number[],
  markPrice: number,
  lastBarTimeMs?: number,
): FuturesSignalInputs {
  const fast = ema(closes, 9);
  const slow = ema(closes, 21);
  const mean20 = sma(closes, 20);
  const std20 = stdDev(closes, 20);
  const rsi14 = rsi(closes, 14);
  const rsi7 = rsi(closes, 7);
  const rsi21 = rsi(closes, 21);
  const stochVals = stochastic(highs, lows, closes, 14, 3);
  const macdVals = macd(closes);
  const atr14 = atr(highs, lows, closes, 14);
  const obvSlopeVals = slope(obv(closes, volumes), 5);
  const bbUpper = mean20.map((m, i) => m + 2 * std20[i]);
  const bbLower = mean20.map((m, i) => m - 2 * std20[i]);
  const bbWidth = mean20.map((m, i) => m > 0 ? (4 * std20[i]) / m : 0);
  const williamsR_vals = williamsR(highs, lows, closes);
  const cci20 = cci(highs, lows, closes, 20);
  const roc10 = rateOfChange(closes, 10);
  const keltnerVals = keltner(highs, lows, closes);
  const donchianVals = donchian(highs, lows);
  const vwap = sma(closes.map((c, i) => c * volumes[i]), 20).map((s, i) => volumes[i] > 0 ? s / volumes[i] : 0);
  const vwapDev = closes.map((c, i) => c - vwap[i]);
  const adxVals = adxProxy(highs, lows, closes);
  const ema5 = ema(closes, 5);
  const ema13 = ema(closes, 13);

  const idx = closes.length - 1;
  const htf5 = buildHtfFields(opens, closes, highs, lows, volumes, 5);
  const htf15 = buildHtfFields(opens, closes, highs, lows, volumes, 15);

  return {
    price: closes[idx],
    markPrice,
    prevPrice: closes[idx - 1] ?? closes[idx],
    fast: fast[idx],
    slow: slow[idx],
    trend: fast[idx] - slow[idx],
    prevFast: fast[idx - 1] ?? fast[idx],
    prevSlow: slow[idx - 1] ?? slow[idx],
    mean20: mean20[idx],
    std20: std20[idx],
    rsi14: rsi14[idx],
    high20: Math.max(...closes.slice(-20)),
    low20: Math.min(...closes.slice(-20)),
    momentum3: closes[idx] - (closes[idx - 3] ?? closes[idx]),
    momentum6: closes[idx] - (closes[idx - 6] ?? closes[idx]),
    momentum10: closes[idx] - (closes[idx - 10] ?? closes[idx]),
    volRatio: volumes[idx] / (sma(volumes, 20)[idx] || volumes[idx] || 1),
    bbUpper: bbUpper[idx],
    bbLower: bbLower[idx],
    bbWidth: bbWidth[idx],
    stochK: stochVals.k[idx],
    stochD: stochVals.d[idx],
    prevStochK: stochVals.k[idx - 1] ?? stochVals.k[idx],
    prevStochD: stochVals.d[idx - 1] ?? stochVals.d[idx],
    macdLine: macdVals.line[idx],
    macdSignal: macdVals.signal[idx],
    prevMacdLine: macdVals.line[idx - 1] ?? macdVals.line[idx],
    prevMacdSignal: macdVals.signal[idx - 1] ?? macdVals.signal[idx],
    macdHist: macdVals.hist[idx],
    prevMacdHist: macdVals.hist[idx - 1] ?? macdVals.hist[idx],
    atr14: atr14[idx],
    obvSlope: obvSlopeVals[idx],
    williamsR: williamsR_vals[idx],
    prevWilliamsR: williamsR_vals[idx - 1] ?? williamsR_vals[idx],
    cci20: cci20[idx],
    roc10: roc10[idx],
    keltnerUpper: keltnerVals.upper[idx],
    keltnerLower: keltnerVals.lower[idx],
    donchianHigh: donchianVals.upper[idx],
    donchianLow: donchianVals.lower[idx],
    donchianMid: donchianVals.mid[idx],
    vwapDev: vwapDev[idx],
    adxProxy: adxVals[idx],
    ema5: ema5[idx],
    ema13: ema13[idx],
    prevEma5: ema5[idx - 1] ?? ema5[idx],
    prevEma13: ema13[idx - 1] ?? ema13[idx],
    rsi7: rsi7[idx],
    rsi21: rsi21[idx],
    ...htf5,
    ...htf15,
    lastBarTimeMs,
  };
}

// ========== BTC FT EXTENDED TEMPLATES (dedicated paths; no generic confluence fallback) ==========

function utcHourFromMs(ms: number | undefined): number | null {
  if (ms === undefined || !Number.isFinite(ms)) return null;
  return new Date(ms).getUTCHours();
}

/** London/NY-style liquidity windows in UTC (approximate). */
function btcFtSessionExpansionHour(hour: number | null): boolean {
  if (hour === null) return false;
  const london = hour >= 7 && hour <= 9;
  const ny = hour >= 13 && hour <= 15;
  return london || ny;
}

export function evalBtcFtTemplateSignal(
  s: FuturesSignalInputs,
  strat: FuturesStratDef,
): { score: number; reason: string } {
  const tpl = strat.btcFtTemplate;
  if (!tpl) return { score: 0, reason: "" };

  let score = 0;
  const reasons: string[] = [];
  const add = (pts: number, desc: string) => {
    score += pts;
    if (pts > 0) reasons.push(desc);
  };
  const short = strat.signalKey.includes("SHORT");
  const v = strat.btcFtVariant ?? 0;
  const px = s.price || 1;
  const vwapPct = s.vwapDev / px;

  if (tpl === "MTF_TREND") {
    if (short) {
      if (s.htf5_trend < 0 && s.htf15_trend < 0) add(18, "HTF bear stack");
      if (s.fast < s.slow) add(12, "LTF EMA bear");
      if (s.momentum6 < 0) add(10, "mom6-");
      if (s.htf5_macdHist < 0) add(8, "HTF5 MACD-");
      if (s.adxProxy > 22) add(6, "ADX");
    } else {
      if (s.htf5_trend > 0 && s.htf15_trend > 0) add(18, "HTF bull stack");
      if (s.fast > s.slow) add(12, "LTF EMA bull");
      if (s.momentum6 > 0) add(10, "mom6+");
      if (s.htf5_macdHist > 0) add(8, "HTF5 MACD+");
      if (s.adxProxy > 22) add(6, "ADX");
    }
  } else if (tpl === "MTF_BREAK") {
    if (short) {
      if (s.price < s.low20 * 1.0005) add(20, "20L break");
      if (s.volRatio > 1.35) add(12, "vol break");
      if (s.htf5_trend <= 0 && s.htf15_trend <= 0) add(12, "HTF bear tail");
      if (s.momentum3 < -s.atr14 * 0.45) add(10, "break impulse-");
    } else {
      if (s.price > s.high20 * 0.9995) add(20, "20H break");
      if (s.volRatio > 1.35) add(12, "vol break");
      if (s.htf5_trend >= 0 && s.htf15_trend >= 0) add(12, "HTF bull tail");
      if (s.momentum3 > s.atr14 * 0.45) add(10, "break impulse+");
    }
  } else if (tpl === "MEAN_REVERT_BB") {
    if (short) {
      if (s.price > s.bbUpper) add(16, "above BB");
      if (s.rsi14 > 64) add(12, "RSI OB");
      if (s.stochK > 78) add(8, "stoch OB");
      if (s.bbWidth > 0.008) add(6, "BB width");
    } else {
      if (s.price < s.bbLower) add(16, "below BB");
      if (s.rsi14 < 36) add(12, "RSI OS");
      if (s.stochK < 22) add(8, "stoch OS");
      if (s.bbWidth > 0.008) add(6, "BB width");
    }
  } else if (tpl === "VWAP_REVERT") {
    if (short) {
      if (vwapPct > 0.014) add(18, "rich vs VWAP");
      if (vwapPct > 0.009) add(10, "VWAP premium");
      if (s.rsi14 > 58) add(8, "RSI firm");
    } else {
      if (vwapPct < -0.014) add(18, "cheap vs VWAP");
      if (vwapPct < -0.009) add(10, "VWAP discount");
      if (s.rsi14 < 42) add(8, "RSI soft");
    }
  } else if (tpl === "MOMENTUM_IMPULSE") {
    if (short) {
      if (s.momentum3 < -s.atr14 * 0.88) add(18, "ATR impulse-");
      if (s.obvSlope < 0 && s.momentum3 < 0) add(12, "OBV agree-");
      if (s.macdHist < s.prevMacdHist && s.macdHist < 0) add(10, "MACD accel-");
      if (s.volRatio > 1.42) add(8, "volume");
    } else {
      if (s.momentum3 > s.atr14 * 0.88) add(18, "ATR impulse+");
      if (s.obvSlope > 0 && s.momentum3 > 0) add(12, "OBV agree+");
      if (s.macdHist > s.prevMacdHist && s.macdHist > 0) add(10, "MACD accel+");
      if (s.volRatio > 1.42) add(8, "volume");
    }
  } else if (tpl === "SESSION_OPEN") {
    const sess = btcFtSessionExpansionHour(utcHourFromMs(s.lastBarTimeMs));
    if (short) {
      if (sess && s.momentum3 < -s.atr14 * 0.42) add(16, "session fade-");
      if (sess && s.volRatio > 1.48) add(12, "session vol");
      if (s.bbWidth > 0.011) add(8, "vol regime");
      if (s.fast < s.slow) add(8, "LTF bias-");
    } else {
      if (sess && s.momentum3 > s.atr14 * 0.42) add(16, "session drive+");
      if (sess && s.volRatio > 1.48) add(12, "session vol");
      if (s.bbWidth > 0.011) add(8, "vol regime");
      if (s.fast > s.slow) add(8, "LTF bias+");
    }
  } else if (tpl === "WYCKOFF_TRAP") {
    const mid = s.donchianMid;
    if (short) {
      if (s.price >= mid * 0.997 && s.cci20 > 35 && s.cci20 < 130) add(16, "CCI thrust OB");
      if (s.volRatio > 1.28) add(10, "vol trap");
      if (s.momentum3 < 0) add(10, "mom-");
      if (s.price > s.mean20) add(8, "above mean");
    } else {
      if (s.price <= mid * 1.003 && s.cci20 < -35 && s.cci20 > -130) add(16, "CCI thrust OS");
      if (s.cci20 < -70) add(6, "CCI spring stress");
      if (s.volRatio > 1.28) add(10, "vol spring");
      if (s.momentum3 > 0) add(10, "mom+");
      if (s.price < s.mean20) add(8, "below mean");
    }
  } else if (tpl === "ORDERFLOW_PROXY") {
    if (short) {
      if (s.momentum3 < -s.atr14 * 1.08 && s.volRatio > 1.65) add(20, "flow burst-");
      if (s.volRatio > 2 && s.momentum3 < 0) add(14, "heavy sell vol");
      if (s.obvSlope < -Math.abs(s.atr14) * 8) add(10, "OBV dump");
    } else {
      if (s.momentum3 > s.atr14 * 1.08 && s.volRatio > 1.65) add(20, "flow burst+");
      if (s.volRatio > 2 && s.momentum3 > 0) add(14, "heavy buy vol");
      if (s.obvSlope > Math.abs(s.atr14) * 8) add(10, "OBV thrust");
    }
  }

  score += Math.max(0, 3 - v) * 2;
  return { score, reason: reasons.slice(0, 3).join(", ") };
}

export function passesBtcFtTemplateConfirmation(s: FuturesSignalInputs, strat: FuturesStratDef): boolean {
  const tpl = strat.btcFtTemplate;
  if (!tpl) return true;

  const short = strat.signalKey.includes("SHORT");
  const px = s.price || 1;
  const vwapPct = s.vwapDev / px;

  if (tpl === "MTF_TREND") {
    if (short) {
      return (
        s.fast < s.slow &&
        Number.isFinite(s.rsi14) &&
        s.rsi14 < 96 &&
        (s.momentum3 < 0 || s.momentum6 < 0)
      );
    }
    return (
      s.fast > s.slow &&
      Number.isFinite(s.rsi14) &&
      s.rsi14 > 16 &&
      (s.momentum3 > 0 || s.momentum6 > 0)
    );
  }

  if (tpl === "MTF_BREAK") {
    if (short) {
      return s.price <= s.low20 * 1.0008 && s.volRatio > 1.06;
    }
    return s.price >= s.high20 * 0.9992 && s.volRatio > 1.06;
  }

  if (tpl === "MEAN_REVERT_BB") {
    if (short) {
      return s.price >= s.bbUpper * 0.998 && s.rsi14 > 56;
    }
    return s.price <= s.bbLower * 1.012 || (s.rsi14 < 46 && s.price < s.mean20);
  }

  if (tpl === "VWAP_REVERT") {
    if (short) {
      return vwapPct > 0.0012 || (s.price > s.mean20 * 1.0012 && s.rsi14 > 46);
    }
    return vwapPct < -0.0045 && s.rsi14 < 48;
  }

  if (tpl === "MOMENTUM_IMPULSE") {
    if (short) {
      return s.momentum3 < -s.atr14 * 0.52 && s.volRatio > 1.18 && s.obvSlope < 0;
    }
    return s.momentum3 > s.atr14 * 0.52 && s.volRatio > 1.18 && s.obvSlope > 0;
  }

  if (tpl === "SESSION_OPEN") {
    const hour = utcHourFromMs(s.lastBarTimeMs);
    if (hour === null) return false;
    if (!btcFtSessionExpansionHour(hour)) return false;
    if (short) {
      return s.momentum3 < -s.atr14 * 0.32 && s.volRatio > 1.22;
    }
    return s.momentum3 > s.atr14 * 0.32 && s.volRatio > 1.22;
  }

  if (tpl === "WYCKOFF_TRAP") {
    const mid = s.donchianMid;
    if (short) {
      return (
        s.volRatio > 1.18 &&
        s.price >= mid * 0.994 &&
        s.cci20 > 28 &&
        s.momentum3 < 0
      );
    }
    return (
      s.volRatio > 1.02 &&
      (s.price <= mid * 1.022 || s.price < s.mean20 * 1.004 || s.cci20 < 55)
    );
  }

  if (tpl === "ORDERFLOW_PROXY") {
    if (short) {
      return s.momentum3 < -s.atr14 * 0.92 && s.volRatio > 1.38;
    }
    return s.momentum3 > s.atr14 * 0.92 && s.volRatio > 1.38;
  }

  return false;
}

// ========== SIGNAL SCORING ==========

export function evalMinuteSignal(s: FuturesSignalInputs, strat: FuturesStratDef): { score: number; reason: string } {
  if (strat.btcFtTemplate) {
    return evalBtcFtTemplateSignal(s, strat);
  }

  let score = 0;
  const reasons: string[] = [];

  const add = (pts: number, desc: string) => { score += pts; if (pts > 0) reasons.push(desc); };
  const short = strat.signalKey.includes("SHORT");

  if (strat.category === "Trend" || strat.category === "MTF Trend") {
    if (short) {
      if (s.fast < s.slow && s.momentum3 < 0) add(10, "EMA bearish");
      if (s.fast < s.slow && s.momentum6 < 0) add(8, "6m down");
      if (s.rsi14 > 25 && s.rsi14 < 45) add(6, "RSI bear zone");
      if (s.adxProxy > 25) add(8, "Trending(ADX)");
      if (s.roc10 < -0.5) add(6, "ROC negative");
    } else {
      if (s.fast > s.slow && s.momentum3 > 0) add(10, "EMA bullish");
      if (s.fast > s.slow && s.momentum6 > 0) add(8, "6m momentum");
      if (s.rsi14 > 55 && s.rsi14 < 75) add(6, "RSI strong");
      if (s.adxProxy > 25) add(8, "Trending(ADX)");
      if (s.roc10 > 0.5) add(6, "ROC positive");
    }
  }

  if (strat.category === "MeanRev" || strat.category === "MR") {
    if (short) {
      if (s.price > s.bbUpper) add(12, "BB upper breach");
      if (s.rsi14 > 65) add(10, "RSI overbought");
      if (s.stochK > 80) add(8, "Stoch overbought");
      if (s.vwapDev > 0.015 * s.price) add(6, "VWAP+ deviation");
    } else {
      if (s.price < s.bbLower) add(12, "BB lower breach");
      if (s.rsi14 < 35) add(10, "RSI oversold");
      if (s.stochK < 20) add(8, "Stoch oversold");
      if (s.vwapDev < -0.015 * s.price) add(6, "VWAP deviation");
    }
  }

  if (strat.category === "Momentum") {
    if (Math.abs(s.momentum3) > s.atr14 * 0.8) add(10, "Strong 3m momentum");
    if (short) {
      if (s.obvSlope < 0) add(8, "OBV falling");
      if (s.macdHist < 0 && s.prevMacdHist < 0 && s.macdHist < s.prevMacdHist) add(8, "MACD accel down");
    } else {
      if (s.obvSlope > 0) add(8, "OBV rising");
      if (s.macdHist > 0 && s.prevMacdHist > 0 && s.macdHist > s.prevMacdHist) add(8, "MACD accel");
    }
  }

  if (strat.category === "RSI") {
    if (short) {
      if (s.rsi14 > 68) add(12, "RSI extreme high");
      if (s.rsi7 > s.rsi14 + 5) add(8, "RSI7 bear div");
    } else {
      if (s.rsi14 < 32) add(12, "RSI extreme low");
      if (s.rsi7 < s.rsi14 - 5) add(8, "RSI7 divergence");
    }
  }

  if (strat.category === "Stoch") {
    if (short) {
      if (s.stochK > 80 && s.stochD > 80) add(12, "Stoch overbought");
      if (s.stochK < s.stochD && s.prevStochK >= s.prevStochD) add(10, "Stoch cross down");
    } else {
      if (s.stochK < 20 && s.stochD < 20) add(12, "Stoch oversold");
      if (s.stochK > s.stochD && s.prevStochK <= s.prevStochD) add(10, "Stoch cross up");
    }
  }

  if (strat.category === "MACD") {
    if (short) {
      if (s.macdLine < s.macdSignal && s.prevMacdLine >= s.prevMacdSignal) add(12, "MACD cross down");
      if (s.macdHist < 0 && s.macdHist < s.prevMacdHist) add(8, "MACD falling");
    } else {
      if (s.macdLine > s.macdSignal && s.prevMacdLine <= s.prevMacdSignal) add(12, "MACD cross");
      if (s.macdHist > 0 && s.macdHist > s.prevMacdHist) add(8, "MACD rising");
    }
  }

  if (strat.category === "OBV") {
    if (short) {
      if (s.obvSlope < 0 && s.momentum3 < 0) add(10, "OBV+price down");
      if (s.obvSlope < -Math.abs(s.atr14) * 10) add(8, "OBV strong down");
    } else {
      if (s.obvSlope > 0 && s.momentum3 > 0) add(10, "OBV+price up");
      if (s.obvSlope > s.atr14 * 10) add(8, "OBV strong");
    }
  }

  if (strat.category === "Confluence") {
    const checks = short
      ? [s.fast < s.slow, s.rsi14 > 30 && s.rsi14 < 50, s.macdLine < s.macdSignal, s.stochK < s.stochD, s.momentum3 < 0]
      : [s.fast > s.slow, s.rsi14 > 50 && s.rsi14 < 70, s.macdLine > s.macdSignal, s.stochK > s.stochD, s.momentum3 > 0];
    const passed = checks.filter(Boolean).length;
    if (passed >= 3) add(10 + passed * 2, `Confluence(${passed})`);
  }

  if (strat.category === "Vol") {
    if (s.volRatio > 1.5) add(10, "Volume spike");
    if (short) {
      if (s.bbWidth < 0.015 && s.momentum3 < -s.atr14) add(12, "Squeeze breakdown");
    } else {
      if (s.bbWidth < 0.015 && s.momentum3 > s.atr14) add(12, "Squeeze breakout");
    }
  }

  if (strat.category === "Breakout" || strat.category === "MTF Break") {
    if (short) {
      if (s.price < s.low20) add(14, "Range breakdown");
      if (s.volRatio > 1.4) add(10, "Breakout volume");
      if (s.adxProxy > 22) add(8, "Trend strength");
      if (s.momentum3 < 0) add(8, "Breakout momentum");
    } else {
      if (s.price > s.high20) add(14, "Range breakout");
      if (s.volRatio > 1.4) add(10, "Breakout volume");
      if (s.adxProxy > 22) add(8, "Trend strength");
      if (s.momentum3 > 0) add(8, "Breakout momentum");
    }
  }

  if (strat.category === "BB") {
    if (s.price < s.bbLower) add(12, "BB lower");
    if (s.price > s.bbUpper) add(10, "BB upper");
    if (s.bbWidth < 0.01) add(8, "BB squeeze");
  }

  if (strat.category === "Williams MR") {
    if (s.williamsR < -80) add(12, "Williams oversold");
    if (s.williamsR > -20) add(10, "Williams overbought");
    if (s.williamsR > -50 && s.prevWilliamsR <= -50) add(8, "Williams cross mid");
  }

  if (strat.category === "CCI MR") {
    if (s.cci20 < -100) add(12, "CCI oversold");
    if (s.cci20 > 100) add(10, "CCI overbought");
    if (Math.abs(s.cci20) < 50) add(6, "CCI neutral");
  }

  if (strat.category === "Williams Trend") {
    if (short) {
      if (s.williamsR > -35 && s.momentum3 < 0) add(12, "Williams bear trend");
      if (s.williamsR > -22) add(10, "Williams elevated");
      if (s.williamsR < s.prevWilliamsR && s.williamsR > -45) add(8, "Williams roll from OB");
    } else {
      if (s.williamsR < -65 && s.momentum3 > 0) add(12, "Williams bull trend");
      if (s.williamsR < -80) add(10, "Williams deep OS");
      if (s.williamsR > s.prevWilliamsR && s.williamsR < -50) add(8, "Williams bounce");
    }
  }

  if (strat.category === "CCI Trend") {
    if (short) {
      if (s.cci20 > 40 && s.momentum3 < 0) add(12, "CCI bear trend");
      if (s.cci20 > 120) add(10, "CCI extended high");
    } else {
      if (s.cci20 < -40 && s.momentum3 > 0) add(12, "CCI bull trend");
      if (s.cci20 < -120) add(10, "CCI extended low");
    }
  }

  if (strat.category === "Keltner MR" || strat.category === "Keltner Trend") {
    if (s.price > s.keltnerUpper) add(10, "Keltner upper breach");
    if (s.price < s.keltnerLower) add(10, "Keltner lower breach");
  }

  if (strat.category === "Donchian Trend" || strat.category === "Donchian MR") {
    if (s.price > s.donchianHigh) add(12, "Donchian high break");
    if (s.price < s.donchianLow) add(12, "Donchian low break");
  }

  if (strat.category === "Ribbon") {
    if (short) {
      if (s.ema5 < s.ema13 && s.prevEma5 >= s.prevEma13) add(12, "Ribbon cross down");
      if (s.ema5 < s.ema13 && s.fast < s.slow) add(8, "Ribbon bear aligned");
    } else {
      if (s.ema5 > s.ema13 && s.prevEma5 <= s.prevEma13) add(12, "Ribbon cross");
      if (s.ema5 > s.ema13 && s.fast > s.slow) add(8, "Ribbon aligned");
    }
  }

  if (strat.category === "Squeeze") {
    if (s.bbWidth < 0.01 && s.adxProxy > 20) add(12, "Squeeze + ADX");
    if (s.volRatio > 2) add(10, "Vol spike after squeeze");
  }

  if (strat.category === "ADX Trend") {
    if (s.adxProxy > 30) add(12, "Strong trend");
    if (short) {
      if (s.adxProxy > 25 && s.fast < s.slow) add(10, "ADX + EMA bear");
    } else {
      if (s.adxProxy > 25 && s.fast > s.slow) add(10, "ADX + EMA");
    }
  }

  if (strat.category === "ROC Trend") {
    if (s.roc10 > 1) add(10, "ROC strong up");
    if (s.roc10 < -1) add(10, "ROC strong down");
  }

  if (strat.category === "MTF Trend") {
    if (short) {
      if (s.fast < s.slow) add(10, "LTF trend down");
      if (s.momentum6 < 0) add(8, "LTF momentum down");
      if (s.adxProxy > 20) add(6, "LTF ADX");
    } else {
      if (s.fast > s.slow) add(10, "LTF trend up");
      if (s.momentum6 > 0) add(8, "LTF momentum up");
      if (s.adxProxy > 20) add(6, "LTF ADX");
    }
  }

  if (strat.category === "MTF MACD") {
    if (short) {
      if (s.macdLine < s.macdSignal) add(10, "LTF MACD bear");
      if (s.macdHist < 0) add(8, "LTF hist below zero");
      if (s.htf5_macdHist < 0) add(8, "HTF5 MACD bear");
      if (s.htf15_macdHist < 0) add(6, "HTF15 MACD bear");
    } else {
      if (s.macdLine > s.macdSignal) add(10, "LTF MACD bull");
      if (s.macdHist > 0) add(8, "LTF hist above zero");
      if (s.htf5_macdHist > 0) add(8, "HTF5 MACD bull");
      if (s.htf15_macdHist > 0) add(6, "HTF15 MACD bull");
    }
  }

  if (strat.category === "MTF ADX") {
    if (short) {
      if (s.adxProxy > 22 && s.fast < s.slow) add(10, "LTF ADX bear");
      if (s.htf5_adx > 24) add(8, "HTF5 ADX");
      if (s.htf15_adx > 24) add(8, "HTF15 ADX");
      if (s.htf5_trend < 0 && s.htf15_trend < 0) add(10, "HTF trend down");
    } else {
      if (s.adxProxy > 22 && s.fast > s.slow) add(10, "LTF ADX bull");
      if (s.htf5_adx > 24) add(8, "HTF5 ADX");
      if (s.htf15_adx > 24) add(8, "HTF15 ADX");
      if (s.htf5_trend > 0 && s.htf15_trend > 0) add(10, "HTF trend up");
    }
  }

  if (strat.requiresHtf) {
    const htf5Trend = htfTrend(s.htf5_fast, s.htf5_slow, s.htf5_momentum);
    const htf15Trend = htfTrend(s.htf15_fast, s.htf15_slow, s.htf15_momentum);

    if (htf5Trend === "UP" && htf15Trend === "UP") add(14, "HTF aligned up");
    if (htf5Trend === "DOWN" && htf15Trend === "DOWN") add(14, "HTF aligned down");
    if (short) {
      if (s.htf5_rsi > 30 && s.htf5_rsi < 50) add(8, "HTF RSI bear zone");
      if (s.htf15_rsi > 30 && s.htf15_rsi < 50) add(6, "HTF15 RSI bear zone");
      if (s.htf5_macdHist < 0) add(8, "HTF MACD bear");
      if (s.htf15_macdHist < 0) add(6, "HTF15 MACD bear");
    } else {
      if (s.htf5_rsi > 50 && s.htf5_rsi < 70) add(8, "HTF RSI healthy");
      if (s.htf15_rsi > 50 && s.htf15_rsi < 70) add(6, "HTF15 RSI healthy");
      if (s.htf5_macdHist > 0) add(8, "HTF MACD bullish");
      if (s.htf15_macdHist > 0) add(6, "HTF15 MACD bullish");
    }
    if (s.htf5_adx > 25) add(8, "HTF trending");
  }

  if (strat.category === "Smart Money") {
    if (short) {
      if (s.volRatio > 2 && s.price > s.mean20) add(14, "Distribution vol");
      if (s.obvSlope < 0 && s.momentum3 < 0) add(12, "Smart money out");
    } else {
      if (s.volRatio > 2 && s.price < s.mean20) add(14, "Accumulation vol");
      if (s.obvSlope > 0 && s.momentum3 > 0) add(12, "Smart money in");
    }
  }

  if (strat.category === "Order Flow") {
    if (short) {
      if (s.momentum3 < -s.atr14 * 1.5) add(14, "Strong flow down");
      if (s.volRatio > 1.8 && s.momentum3 < 0) add(12, "Sell flow");
    } else {
      if (s.momentum3 > s.atr14 * 1.5) add(14, "Strong flow");
      if (s.volRatio > 1.8 && s.price > s.vwapDev + s.mean20) add(12, "Buy flow");
    }
  }

  if (strat.category === "Liquidity") {
    if (short) {
      if (s.price > s.high20 * 0.998) add(14, "Liquidity at highs");
      if (s.williamsR > -15) add(12, "Overbought liquidity");
    } else {
      if (s.price < s.low20 * 1.002) add(14, "Liquidity sweep");
      if (s.williamsR < -85) add(12, "Oversold liquidity");
    }
  }

  if (strat.category === "Stop Hunt") {
    if (short) {
      if (Math.abs(s.price - s.high20) < s.atr14 * 0.3) add(14, "Stop hunt highs");
    } else {
      if (Math.abs(s.price - s.low20) < s.atr14 * 0.3) add(14, "Stop hunt zone");
    }
  }

  if (strat.category === "Wyckoff") {
    if (short) {
      if (s.price < s.donchianMid && s.volRatio > 1.5) add(14, "Wyckoff markdown");
      if (s.cci20 < -100 && s.momentum6 < 0) add(12, "Distribution leg");
    } else {
      if (s.price > s.donchianMid && s.volRatio > 1.5) add(14, "Wyckoff markup");
      if (s.cci20 > 100 && s.momentum6 > 0) add(12, "Spring complete");
    }
  }

  if (strat.category === "Market Structure") {
    if (s.price > s.high20 && s.htf5_trend > 0) add(14, "BOS bullish");
    if (s.price < s.low20 && s.htf5_trend < 0) add(14, "BOS bearish");
  }

  if (strat.category === "Statistical") {
    const zscore = (s.price - s.mean20) / (s.std20 || 1);
    if (Math.abs(zscore) > 2) add(14, "Statistical extreme");
    if (short) {
      if (zscore > 1.5 && s.rsi14 > 60) add(12, "Mean reversion short");
    } else {
      if (zscore < -1.5 && s.rsi14 < 40) add(12, "Mean reversion long");
    }
  }

  if (strat.category === "Institutional") {
    if (s.adxProxy > 30 && s.volRatio > 2) add(14, "Inst activity");
    if (s.htf5_adx > 25 && s.htf15_adx > 25) add(12, "Inst trend");
  }

  if (strat.category === "Session") {
    if (short) {
      if (s.momentum3 < 0 && s.volRatio > 1.5) add(12, "Session sell");
    } else {
      if (s.momentum3 > 0 && s.volRatio > 1.5) add(12, "Session momentum");
    }
  }

  if (strat.category === "Harmonic") {
    if (Math.abs(s.price - s.bbLower) / s.price < 0.005) add(12, "Harmonic support");
    if (s.rsi14 > 30 && s.rsi14 < 50 && s.stochK > s.stochD) add(10, "Harmonic bounce");
  }

  if (strat.category === "Chart Pattern") {
    if (s.bbWidth < 0.015 && s.volRatio > 1.5) add(12, "Pattern breakout");
    if (s.atr14 < s.mean20 * 0.008) add(10, "Consolidation pattern");
  }

  if (strat.category === "Greek-Inspired") {
    if (s.bbWidth < 0.012 && s.adxProxy > 20) add(14, "Gamma squeeze setup");
    if (s.volRatio > 3 && Math.abs(s.momentum3) > s.atr14) add(12, "Delta spike");
  }

  if (strat.category === "Event") {
    if (s.volRatio > 4 && Math.abs(s.momentum3) > s.atr14 * 2) add(16, "Event volatility");
  }

  if (strat.category === "Macro") {
    if (s.htf5_trend > 0 && s.htf15_trend > 0 && s.rsi14 > 50) add(14, "Risk on");
    if (s.htf5_trend < 0 && s.htf15_trend < 0 && s.rsi14 < 50) add(14, "Risk off");
  }

  if (strat.category === "ML-Style") {
    const factors = (
      short
        ? [s.fast < s.slow, s.rsi14 > 25 && s.rsi14 < 55, s.macdHist < 0, s.adxProxy > 20, s.volRatio > 1.2]
        : [s.fast > s.slow, s.rsi14 > 45 && s.rsi14 < 75, s.macdHist > 0, s.adxProxy > 20, s.volRatio > 1.2]
    ).filter(Boolean).length;
    if (factors >= 4) add(16, "ML ensemble strong");
    if (factors === 3) add(10, "ML ensemble medium");
  }

  return { score, reason: reasons.slice(0, 3).join(", ") };
}

// ========== ENTRY CONFIRMATION ==========

export function passesEntryConfirmation(s: FuturesSignalInputs, strat: FuturesStratDef): boolean {
  if (strat.btcFtTemplate) {
    return passesBtcFtTemplateConfirmation(s, strat);
  }

  const isShort = strat.signalKey.includes("SHORT");

  const confluence = (
    isShort
      ? [s.fast < s.slow, Number.isFinite(s.rsi14) && s.rsi14 < 72, s.macdLine < s.macdSignal, s.stochK < s.stochD, s.momentum3 < 0, s.obvSlope < 0, s.atr14 > 0, s.bbWidth > 0]
      : [s.fast > s.slow, s.rsi14 > 40, s.macdLine > s.macdSignal, s.stochK > s.stochD, s.momentum3 > 0, s.obvSlope > 0, s.atr14 > 0, s.bbWidth > 0]
  ).filter(Boolean).length;

  if (strat.category === "Trend" || strat.category === "MTF Trend") {
    if (isShort) {
      if (s.fast >= s.slow) return false;
      if (!Number.isFinite(s.rsi14) || s.rsi14 > 88) return false;
    } else {
      if (s.fast <= s.slow) return false;
      if (!Number.isFinite(s.rsi14) || s.rsi14 < 32) return false;
    }
  }

  if (strat.category === "MeanRev" || strat.category === "MR") {
    if (isShort) {
      if (s.rsi14 < 55) return false;
      if (s.price < s.bbUpper && s.price < s.mean20) return false;
    } else {
      if (s.rsi14 > 45) return false;
      if (s.price > s.bbLower && s.price > s.mean20) return false;
    }
  }

  if (strat.category === "Williams MR") {
    if (isShort) {
      if (s.williamsR < -25) return false;
    } else {
      if (s.williamsR > -70) return false;
    }
  }

  if (strat.category === "CCI MR") {
    if (isShort) {
      if (s.cci20 < 80) return false;
    } else {
      if (s.cci20 > -80) return false;
    }
  }

  if (strat.category === "Keltner MR") {
    if (isShort) {
      if (s.price < s.keltnerUpper) return false;
    } else {
      if (s.price > s.keltnerLower) return false;
    }
  }

  if (strat.category === "VWAP MR") {
    if (isShort) {
      if (s.vwapDev < 0.01 * s.price) return false;
    } else {
      if (s.vwapDev > -0.01 * s.price) return false;
    }
  }

  if (strat.category === "ADX Trend") {
    if (s.adxProxy < 20) return false;
  }

  if (strat.category === "Donchian Trend") {
    if (isShort) {
      if (s.price > s.donchianLow * 1.01) return false;
    } else {
      if (s.price < s.donchianHigh * 0.99) return false;
    }
  }

  if (strat.category === "ROC Trend") {
    if (isShort) {
      if (s.roc10 > -0.3) return false;
    } else {
      if (s.roc10 < 0.3) return false;
    }
  }

  if (strat.category === "Squeeze") {
    if (s.bbWidth > 0.02) return false;
  }

  if (strat.requiresHtf) {
    const htf5T = htfTrend(s.htf5_fast, s.htf5_slow, s.htf5_momentum);
    const htf15T = htfTrend(s.htf15_fast, s.htf15_slow, s.htf15_momentum);
    const ltfBull = s.fast > s.slow && (s.momentum3 > 0 || s.momentum6 > 0);
    const ltfBear = s.fast < s.slow && (s.momentum3 < 0 || s.momentum6 < 0);
    if (isShort) {
      if (htf5T === "UP" || htf15T === "UP") return false;
      if (htf5T === "DOWN" && htf15T === "DOWN") {
        /* strict bearish HTF */
      } else if (!ltfBear) {
        return false;
      }
    } else {
      if (htf5T === "DOWN" || htf15T === "DOWN") return false;
      if (htf5T === "UP" && htf15T === "UP") {
        /* strict bullish HTF */
      } else if (!ltfBull) {
        return false;
      }
    }
  }

  if (strat.category === "Smart Money") {
    if (s.volRatio < 1.5) return false;
    if (isShort) {
      if (s.obvSlope >= 0) return false;
    } else {
      if (s.obvSlope <= 0) return false;
    }
  }

  if (strat.category === "Order Flow") {
    if (isShort) {
      if (s.momentum3 >= -s.atr14) return false;
    } else {
      if (s.momentum3 <= s.atr14) return false;
    }
    if (s.volRatio < 1.5) return false;
  }

  if (strat.category === "Liquidity") {
    if (isShort) {
      if (s.price < s.high20 * 0.995) return false;
    } else {
      if (s.price > s.low20 * 1.005) return false;
    }
  }

  if (strat.category === "Stop Hunt") {
    if (isShort) {
      if (Math.abs(s.price - s.high20) > s.atr14 * 0.5) return false;
    } else {
      if (Math.abs(s.price - s.low20) > s.atr14 * 0.5) return false;
    }
  }

  if (strat.category === "Wyckoff") {
    if (isShort) {
      if (s.cci20 > -80) return false;
      if (s.volRatio < 1.3) return false;
    } else {
      if (s.cci20 < 80) return false;
      if (s.volRatio < 1.3) return false;
    }
  }

  if (strat.category === "Market Structure") {
    if (isShort) {
      if (s.price > s.low20 * 1.002 && s.price < s.high20 * 0.998) return false;
    } else {
      if (s.price < s.high20 * 0.998 && s.price > s.low20 * 1.002) return false;
    }
  }

  if (strat.category === "Statistical") {
    const zscore = Math.abs((s.price - s.mean20) / (s.std20 || 1));
    if (zscore < 1.5) return false;
  }

  if (strat.category === "Institutional") {
    if (s.adxProxy < 25) return false;
    if (s.volRatio < 1.5) return false;
  }

  if (strat.category === "Session") {
    if (s.volRatio < 1.3) return false;
  }

  if (strat.category === "Harmonic") {
    if (s.rsi14 > 60 || s.rsi14 < 20) return false;
  }

  if (strat.category === "Chart Pattern") {
    if (s.bbWidth > 0.02) return false;
  }

  if (strat.category === "Greek-Inspired") {
    if (s.bbWidth > 0.015) return false;
    if (s.adxProxy < 18) return false;
  }

  if (strat.category === "Event") {
    if (s.volRatio < 2.5) return false;
  }

  if (strat.category === "Macro") {
    if (s.htf5_rsi < 45 || s.htf5_rsi > 75) return false;
  }

  if (strat.category === "ML-Style") {
    const factors = (
      isShort
        ? [s.fast < s.slow, s.rsi14 > 25 && s.rsi14 < 55, s.macdHist < 0, s.adxProxy > 20]
        : [s.fast > s.slow, s.rsi14 > 45 && s.rsi14 < 75, s.macdHist > 0, s.adxProxy > 20]
    ).filter(Boolean).length;
    if (factors < 3) return false;
  }

  const strictConfluenceCategory =
    strat.category === "Confluence" || strat.category === "MTF Conf";
  const requiredHits = strictConfluenceCategory
    ? strat.confluenceMin
    : Math.max(2, strat.confluenceMin - 1);
  return confluence >= requiredHits;
}

// ========== THRESHOLD ==========

export function effectiveSignalThreshold(
  baseThreshold: number,
  profileSignalDelta: number,
  clampMin = 18,
  clampMax = 99,
): number {
  return Math.max(clampMin, Math.min(clampMax, baseThreshold + profileSignalDelta));
}
