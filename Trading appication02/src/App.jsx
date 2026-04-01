import React, { useEffect, useRef, useState } from "react";

const MAX_OPEN_TRADES = 5;
const RECENT_COOLDOWN_MS = 7000;
const START_BALANCE = 10000;
const MARKET_SYMBOL = "BTCUSDT";
const MAX_CANDLES = 240;
const RECONNECT_MAX_MS = 15000;

const isBull = (c) => c.close > c.open;
const isBear = (c) => c.close < c.open;
const bodySize = (c) => Math.abs(c.close - c.open);

function calcEMA(data, period) {
  if (!data.length) return [];
  const k = 2 / (period + 1);
  const out = [data[0]];
  for (let i = 1; i < data.length; i++) out.push(data[i] * k + out[i - 1] * (1 - k));
  return out;
}

function calcRSI(closes, period = 14) {
  if (closes.length < period + 1) return closes.map(() => 50);
  const out = [];
  let gains = 0;
  let losses = 0;
  for (let i = 1; i <= period; i++) {
    const d = closes[i] - closes[i - 1];
    if (d > 0) gains += d;
    else losses -= d;
  }
  gains /= period;
  losses /= period;
  for (let i = 0; i < period; i++) out.push(50);
  out.push(losses === 0 ? 100 : 100 - 100 / (1 + gains / losses));
  for (let i = period + 1; i < closes.length; i++) {
    const d = closes[i] - closes[i - 1];
    gains = (gains * (period - 1) + (d > 0 ? d : 0)) / period;
    losses = (losses * (period - 1) + (d < 0 ? -d : 0)) / period;
    out.push(losses === 0 ? 100 : 100 - 100 / (1 + gains / losses));
  }
  return out;
}

function calcMACDHistogram(closes) {
  const fast = calcEMA(closes, 12);
  const slow = calcEMA(closes, 26);
  const macd = fast.map((v, i) => v - slow[i]);
  const signal = calcEMA(macd, 9);
  return macd.map((v, i) => v - signal[i]);
}

function calcATR(candles, period = 14) {
  if (!candles.length) return [];
  const tr = [candles[0].high - candles[0].low];
  for (let i = 1; i < candles.length; i++) {
    const c = candles[i];
    const p = candles[i - 1];
    tr.push(Math.max(c.high - c.low, Math.abs(c.high - p.close), Math.abs(c.low - p.close)));
  }
  const out = [tr[0]];
  for (let i = 1; i < tr.length; i++) out.push((out[i - 1] * (period - 1) + tr[i]) / period);
  return out;
}

function calcVWAP(candles) {
  let cumulativeVolume = 0;
  let cumulativeTurnover = 0;
  return candles.map((c) => {
    cumulativeVolume += c.volume;
    cumulativeTurnover += c.close * c.volume;
    return cumulativeVolume > 0 ? cumulativeTurnover / cumulativeVolume : c.close;
  });
}

function calcBollingerBands(closes, period = 20, mult = 2) {
  return closes.map((_, i) => {
    if (i < period - 1) return { upper: closes[i], middle: closes[i], lower: closes[i], bw: 0 };
    const slice = closes.slice(i - period + 1, i + 1);
    const mean = slice.reduce((a, v) => a + v, 0) / period;
    const std = Math.sqrt(slice.reduce((a, v) => a + (v - mean) ** 2, 0) / period);
    const upper = mean + std * mult;
    const lower = mean - std * mult;
    return { upper, middle: mean, lower, bw: mean > 0 ? (upper - lower) / mean : 0 };
  });
}

function calcStochastic(candles, period = 14, smooth = 3) {
  const k = candles.map((_, i) => {
    if (i < period - 1) return 50;
    const slice = candles.slice(i - period + 1, i + 1);
    const high = Math.max(...slice.map((c) => c.high));
    const low = Math.min(...slice.map((c) => c.low));
    return high === low ? 50 : ((candles[i].close - low) / (high - low)) * 100;
  });
  const d = calcEMA(k, smooth);
  return { k, d };
}

function calcSupertrend(candles, period = 10, mult = 3) {
  const atr = calcATR(candles, period);
  let dir = 1;
  let upperBand = 0;
  let lowerBand = 0;
  return candles.map((c, i) => {
    const hl2 = (c.high + c.low) / 2;
    const a = atr[i] || 0;
    const rawUpper = hl2 + mult * a;
    const rawLower = hl2 - mult * a;
    upperBand = i === 0 ? rawUpper : (rawUpper < upperBand || candles[i - 1].close > upperBand ? rawUpper : upperBand);
    lowerBand = i === 0 ? rawLower : (rawLower > lowerBand || candles[i - 1].close < lowerBand ? rawLower : lowerBand);
    if (i > 0) {
      if (c.close > upperBand) dir = 1;
      else if (c.close < lowerBand) dir = -1;
    }
    return { dir, band: dir === 1 ? lowerBand : upperBand };
  });
}

function calcDonchian(candles, period = 20) {
  return candles.map((_, i) => {
    if (i < period - 1) return { high: candles[i].high, low: candles[i].low };
    const slice = candles.slice(i - period + 1, i + 1);
    return { high: Math.max(...slice.map((c) => c.high)), low: Math.min(...slice.map((c) => c.low)) };
  });
}

function calcHeikinAshi(candles) {
  return candles.map((c, i) => {
    const haClose = (c.open + c.high + c.low + c.close) / 4;
    const haOpen = i === 0 ? (c.open + c.close) / 2 : 0; // computed below via reduce
    return { open: haOpen, close: haClose, high: c.high, low: c.low };
  }).reduce((acc, c, i) => {
    const haOpen = i === 0 ? c.open : (acc[i - 1].open + acc[i - 1].close) / 2;
    const haClose = c.close;
    acc.push({ open: haOpen, close: haClose, high: Math.max(candles[i].high, haOpen, haClose), low: Math.min(candles[i].low, haOpen, haClose) });
    return acc;
  }, []);
}

function calcCCI(candles, period = 14) {
  return candles.map((_, i) => {
    if (i < period - 1) return 0;
    const slice = candles.slice(i - period + 1, i + 1);
    const typicals = slice.map((c) => (c.high + c.low + c.close) / 3);
    const mean = typicals.reduce((a, v) => a + v, 0) / period;
    const mad = typicals.reduce((a, v) => a + Math.abs(v - mean), 0) / period;
    return mad === 0 ? 0 : (typicals[period - 1] - mean) / (0.015 * mad);
  });
}

function calcWilliamsR(candles, period = 14) {
  return candles.map((_, i) => {
    if (i < period - 1) return -50;
    const slice = candles.slice(i - period + 1, i + 1);
    const high = Math.max(...slice.map((c) => c.high));
    const low = Math.min(...slice.map((c) => c.low));
    return high === low ? -50 : ((high - candles[i].close) / (high - low)) * -100;
  });
}

function calcADX(candles, period = 14) {
  if (candles.length < period + 2) return candles.map(() => ({ adx: 0, diPlus: 0, diMinus: 0 }));
  const tr = [0], dmP = [0], dmM = [0];
  for (let i = 1; i < candles.length; i++) {
    const c = candles[i], p = candles[i - 1];
    tr.push(Math.max(c.high - c.low, Math.abs(c.high - p.close), Math.abs(c.low - p.close)));
    const up = c.high - p.high, dn = p.low - c.low;
    dmP.push(up > dn && up > 0 ? up : 0);
    dmM.push(dn > up && dn > 0 ? dn : 0);
  }
  let atr14 = tr.slice(1, period + 1).reduce((a, v) => a + v, 0);
  let sdmP = dmP.slice(1, period + 1).reduce((a, v) => a + v, 0);
  let sdmM = dmM.slice(1, period + 1).reduce((a, v) => a + v, 0);
  const result = new Array(period).fill(null).map(() => ({ adx: 0, diPlus: 0, diMinus: 0 }));
  const getDI = (sm, a) => (a > 0 ? (sm / a) * 100 : 0);
  let dp = getDI(sdmP, atr14), dm = getDI(sdmM, atr14);
  let dx = dp + dm > 0 ? (Math.abs(dp - dm) / (dp + dm)) * 100 : 0;
  let adxVal = dx;
  result.push({ adx: adxVal, diPlus: dp, diMinus: dm });
  for (let i = period + 1; i < candles.length; i++) {
    atr14 = atr14 - atr14 / period + tr[i];
    sdmP = sdmP - sdmP / period + dmP[i];
    sdmM = sdmM - sdmM / period + dmM[i];
    dp = getDI(sdmP, atr14);
    dm = getDI(sdmM, atr14);
    dx = dp + dm > 0 ? (Math.abs(dp - dm) / (dp + dm)) * 100 : 0;
    adxVal = (adxVal * (period - 1) + dx) / period;
    result.push({ adx: adxVal, diPlus: dp, diMinus: dm });
  }
  return result;
}

function calcParabolicSAR(candles, step = 0.02, maxAF = 0.2) {
  const out = [];
  let rising = true;
  let sar = candles[0].low;
  let ep = candles[0].high;
  let af = step;
  for (let i = 0; i < candles.length; i++) {
    if (i === 0) { out.push({ sar, rising }); continue; }
    const prev = candles[i - 1];
    sar = sar + af * (ep - sar);
    if (rising) {
      sar = Math.min(sar, prev.low, i >= 2 ? candles[i - 2].low : prev.low);
      if (candles[i].low < sar) {
        rising = false; sar = ep; ep = candles[i].low; af = step;
      } else if (candles[i].high > ep) {
        ep = candles[i].high; af = Math.min(af + step, maxAF);
      }
    } else {
      sar = Math.max(sar, prev.high, i >= 2 ? candles[i - 2].high : prev.high);
      if (candles[i].high > sar) {
        rising = true; sar = ep; ep = candles[i].high; af = step;
      } else if (candles[i].low < ep) {
        ep = candles[i].low; af = Math.min(af + step, maxAF);
      }
    }
    out.push({ sar, rising });
  }
  return out;
}

// ── ADDITIONAL INDICATOR FUNCTIONS ──────────────────────────────────────────

function calcIchimoku(candles) {
  const mid = (a, b) => (a + b) / 2;
  const hl = (sl) => ({ h: Math.max(...sl.map(c => c.high)), l: Math.min(...sl.map(c => c.low)) });
  return candles.map((_, i) => {
    const hl9  = hl(candles.slice(Math.max(0, i - 8),  i + 1));
    const hl26 = hl(candles.slice(Math.max(0, i - 25), i + 1));
    const hl52 = hl(candles.slice(Math.max(0, i - 51), i + 1));
    const tenkan = mid(hl9.h, hl9.l);
    const kijun  = mid(hl26.h, hl26.l);
    return { tenkan, kijun, senkouA: mid(tenkan, kijun), senkouB: mid(hl52.h, hl52.l) };
  });
}

function calcKeltner(candles, period = 20, mult = 2) {
  const atr = calcATR(candles, period);
  const mid = calcEMA(candles.map(c => c.close), period);
  return candles.map((_, i) => ({ upper: mid[i] + mult * atr[i], middle: mid[i], lower: mid[i] - mult * atr[i] }));
}

function calcCMF(candles, period = 20) {
  return candles.map((_, i) => {
    const sl = candles.slice(Math.max(0, i - period + 1), i + 1);
    let mfv = 0, vol = 0;
    sl.forEach(c => { const r = c.high - c.low; mfv += r > 0 ? ((c.close - c.low) - (c.high - c.close)) / r * c.volume : 0; vol += c.volume; });
    return vol > 0 ? mfv / vol : 0;
  });
}

function calcOBV(candles) {
  return candles.reduce((acc, c, i) => {
    if (i === 0) return [0];
    const prev = acc[i - 1];
    acc.push(c.close > candles[i-1].close ? prev + c.volume : c.close < candles[i-1].close ? prev - c.volume : prev);
    return acc;
  }, []);
}

function calcROC(closes, period = 10) {
  return closes.map((v, i) => i < period ? 0 : closes[i - period] !== 0 ? ((v - closes[i - period]) / closes[i - period]) * 100 : 0);
}

function calcElderRay(candles, period = 13) {
  const ema = calcEMA(candles.map(c => c.close), period);
  return candles.map((c, i) => ({ bullPower: c.high - ema[i], bearPower: c.low - ema[i] }));
}

function calcFibLevels(candles, lookback = 50) {
  const sl = candles.slice(-lookback);
  const swH = Math.max(...sl.map(c => c.high)), swL = Math.min(...sl.map(c => c.low));
  const r = swH - swL;
  return { swingHigh: swH, swingLow: swL, fib382: swH - r * 0.382, fib500: swH - r * 0.5, fib618: swH - r * 0.618, fib786: swH - r * 0.786 };
}

function calcVWAPBands(candles) {
  let cumVol = 0, cumTP = 0, cumTP2 = 0;
  return candles.map(c => {
    const tp = (c.high + c.low + c.close) / 3;
    cumVol += c.volume; cumTP += tp * c.volume; cumTP2 += tp * tp * c.volume;
    const vwap = cumVol > 0 ? cumTP / cumVol : tp;
    const std = Math.sqrt(Math.max(0, cumVol > 0 ? cumTP2 / cumVol - vwap * vwap : 0));
    return { vwap, upper1: vwap + std, lower1: vwap - std, upper2: vwap + 2 * std, lower2: vwap - 2 * std };
  });
}

function calcMACDLine(closes) {
  const fast = calcEMA(closes, 12), slow = calcEMA(closes, 26);
  return fast.map((v, i) => v - slow[i]);
}

function calcPivotPoints(candles) {
  if (candles.length < 2) return null;
  const p = candles[candles.length - 2];
  const pp = (p.high + p.low + p.close) / 3;
  return { pp, r1: 2 * pp - p.low, r2: pp + (p.high - p.low), s1: 2 * pp - p.high, s2: pp - (p.high - p.low) };
}

// ── HIGH-PROFIT INDICATOR FUNCTIONS ────────────────────────────────────────

// Hull Moving Average – reduces EMA lag dramatically
function calcHMA(closes, period = 16) {
  const half = calcEMA(closes, Math.floor(period / 2));
  const full = calcEMA(closes, period);
  const diff = half.map((v, i) => 2 * v - full[i]);
  return calcEMA(diff, Math.round(Math.sqrt(period)));
}

// TRIX – triple-smoothed EMA, filters noise better than MACD
function calcTRIX(closes, period = 14) {
  const e1 = calcEMA(closes, period);
  const e2 = calcEMA(e1, period);
  const e3 = calcEMA(e2, period);
  return e3.map((v, i) => (i === 0 || e3[i - 1] === 0) ? 0 : ((v - e3[i - 1]) / e3[i - 1]) * 100);
}

// Aroon – measures how recently swing highs/lows occurred
function calcAroon(candles, period = 25) {
  return candles.map((_, i) => {
    if (i < period) return { up: 50, down: 50 };
    const sl = candles.slice(i - period, i + 1);
    const hiIdx = sl.reduce((m, c, j) => c.high > sl[m].high ? j : m, 0);
    const loIdx = sl.reduce((m, c, j) => c.low  < sl[m].low  ? j : m, 0);
    return { up: (hiIdx / period) * 100, down: (loIdx / period) * 100 };
  });
}

// Accumulation / Distribution Line
function calcAD(candles) {
  let ad = 0;
  return candles.map(c => {
    const hl = c.high - c.low;
    ad += hl > 0 ? ((c.close - c.low) - (c.high - c.close)) / hl * c.volume : 0;
    return ad;
  });
}

// Relative Volume – how loud this candle is vs the average
function calcRVOL(candles, period = 20) {
  return candles.map((c, i) => {
    if (i < period) return 1;
    const avg = candles.slice(i - period, i).reduce((a, x) => a + x.volume, 0) / period;
    return avg > 0 ? c.volume / avg : 1;
  });
}

// Linear Regression Channel
function calcLinReg(closes, period = 20) {
  return closes.map((_, i) => {
    if (i < period - 1) return { mid: closes[i], upper: closes[i], lower: closes[i], slope: 0 };
    const sl = closes.slice(i - period + 1, i + 1);
    const n = sl.length, sx = n * (n - 1) / 2, sx2 = (n - 1) * n * (2 * n - 1) / 6;
    const sy = sl.reduce((a, v) => a + v, 0), sxy = sl.reduce((a, v, j) => a + j * v, 0);
    const m = (n * sxy - sx * sy) / (n * sx2 - sx * sx);
    const b = (sy - m * sx) / n;
    const pred = sl.map((_, j) => b + m * j);
    const se = Math.sqrt(sl.reduce((a, v, j) => a + (v - pred[j]) ** 2, 0) / n);
    const mid = b + m * (n - 1);
    return { mid, upper: mid + 2 * se, lower: mid - 2 * se, slope: m };
  });
}

// Order Block detection – last opposing candle before impulse move
function detectOrderBlocks(candles, lookback = 40) {
  const bullOBs = [], bearOBs = [];
  for (let i = lookback; i >= 3; i--) {
    const idx = candles.length - i;
    if (idx < 0) continue;
    const c = candles[idx];
    const after = candles.slice(idx + 1, idx + 5);
    if (isBear(c) && after.some(x => x.close > c.high + (c.high - c.low) * 0.1))
      bullOBs.push({ high: c.high, low: c.low, idx });
    if (isBull(c) && after.some(x => x.close < c.low - (c.high - c.low) * 0.1))
      bearOBs.push({ high: c.high, low: c.low, idx });
  }
  return { bullOBs, bearOBs };
}

// Fair Value Gap – price imbalance between candle 1 and candle 3
function detectFVG(candles, lookback = 30) {
  const fvgs = [];
  const start = Math.max(2, candles.length - lookback);
  for (let i = start; i < candles.length; i++) {
    const c1 = candles[i - 2], c3 = candles[i];
    if (c3.low > c1.high)  fvgs.push({ type: 'bull', low: c1.high, high: c3.low, age: candles.length - i });
    if (c3.high < c1.low)  fvgs.push({ type: 'bear', low: c3.high, high: c1.low, age: candles.length - i });
  }
  return fvgs;
}

function initCandles(n = 180) {
  let p = 84200 + Math.random() * 1000;
  const now = Date.now();
  const list = [];
  for (let i = n; i >= 0; i--) {
    const open = p;
    p += (Math.random() - 0.497) * 85;
    list.push({
      time: now - i * 60000,
      open,
      close: p,
      high: Math.max(open, p) + Math.random() * 28,
      low: Math.min(open, p) - Math.random() * 28,
      volume: 20 + Math.random() * 220
    });
  }
  return list;
}

function ensureNumber(v) {
  const n = Number(v);
  return Number.isFinite(n) ? n : null;
}

function normalizeCandle(input) {
  const time = ensureNumber(input.time);
  const open = ensureNumber(input.open);
  const high = ensureNumber(input.high);
  const low = ensureNumber(input.low);
  const close = ensureNumber(input.close);
  const volume = ensureNumber(input.volume ?? 0);
  if ([time, open, high, low, close, volume].some((x) => x === null)) return null;
  return { time, open, high, low, close, volume };
}

function minuteBucket(ts) {
  return Math.floor(ts / 60000) * 60000;
}

function applyTickerToCandles(candles, tickPrice) {
  if (!candles.length || !Number.isFinite(tickPrice)) return candles;
  const nowBucket = minuteBucket(Date.now());
  const last = candles[candles.length - 1];
  if (last.time === nowBucket) {
    const updated = {
      ...last,
      close: tickPrice,
      high: Math.max(last.high, tickPrice),
      low: Math.min(last.low, tickPrice)
    };
    return [...candles.slice(0, -1), updated];
  }
  if (nowBucket > last.time) {
    const open = last.close;
    const next = {
      time: nowBucket,
      open,
      close: tickPrice,
      high: Math.max(open, tickPrice),
      low: Math.min(open, tickPrice),
      volume: 0
    };
    return [...candles.slice(-(MAX_CANDLES - 1)), next];
  }
  return candles;
}

function upsertCandle(candles, incoming) {
  const c = normalizeCandle(incoming);
  if (!c) return candles;
  if (!candles.length) return [c];

  const idx = candles.findIndex((x) => x.time === c.time);
  if (idx >= 0) {
    const next = candles.slice();
    next[idx] = c;
    return next;
  }
  if (c.time > candles[candles.length - 1].time) {
    return [...candles.slice(-(MAX_CANDLES - 1)), c];
  }
  const insertAt = candles.findIndex((x) => x.time > c.time);
  if (insertAt === -1) return [...candles.slice(-(MAX_CANDLES - 1)), c];
  const next = candles.slice();
  next.splice(insertAt, 0, c);
  return next.slice(-MAX_CANDLES);
}

function parseBinanceSocketMessage(msg) {
  const data = msg?.data ?? msg;
  if (!data || typeof data !== "object") return {};
  if (data.e === "24hrTicker") {
    return { ticker: ensureNumber(data.c) };
  }
  if (data.e === "kline" && data.k) {
    return {
      ticker: ensureNumber(data.k.c),
      candle: {
        time: ensureNumber(data.k.t),
        open: ensureNumber(data.k.o),
        high: ensureNumber(data.k.h),
        low: ensureNumber(data.k.l),
        close: ensureNumber(data.k.c),
        volume: ensureNumber(data.k.v)
      }
    };
  }
  return {};
}

function parseBybitSocketMessage(msg) {
  if (!msg || typeof msg !== "object") return {};
  if (msg.op === "pong" || msg.ret_msg === "pong") return {};
  if (msg.topic === `tickers.${MARKET_SYMBOL}`) {
    const payload = Array.isArray(msg.data) ? msg.data[0] : msg.data;
    return { ticker: ensureNumber(payload?.lastPrice) };
  }
  if (msg.topic === `kline.1.${MARKET_SYMBOL}`) {
    const payload = Array.isArray(msg.data) ? msg.data[0] : msg.data;
    if (!payload) return {};
    return {
      ticker: ensureNumber(payload.close),
      candle: {
        time: ensureNumber(payload.start ?? payload.startTime ?? payload.timestamp),
        open: ensureNumber(payload.open),
        high: ensureNumber(payload.high),
        low: ensureNumber(payload.low),
        close: ensureNumber(payload.close),
        volume: ensureNumber(payload.volume)
      }
    };
  }
  return {};
}

async function fetchBootstrapCandles(exchange) {
  if (exchange === "binance") {
    const res = await fetch(
      `https://api.binance.com/api/v3/klines?symbol=${MARKET_SYMBOL}&interval=1m&limit=240`
    );
    if (!res.ok) throw new Error(`Binance bootstrap failed (${res.status})`);
    const rows = await res.json();
    return rows
      .map((r) => normalizeCandle({ time: r[0], open: r[1], high: r[2], low: r[3], close: r[4], volume: r[5] }))
      .filter(Boolean);
  }

  const res = await fetch(
    `https://api.bybit.com/v5/market/kline?category=linear&symbol=${MARKET_SYMBOL}&interval=1&limit=240`
  );
  if (!res.ok) throw new Error(`Bybit bootstrap failed (${res.status})`);
  const payload = await res.json();
  const rows = payload?.result?.list ?? [];
  return rows
    .map((r) =>
      Array.isArray(r)
        ? normalizeCandle({ time: r[0], open: r[1], high: r[2], low: r[3], close: r[4], volume: r[5] })
        : normalizeCandle({
            time: r.startTime ?? r.start ?? r.timestamp,
            open: r.openPrice ?? r.open,
            high: r.highPrice ?? r.high,
            low: r.lowPrice ?? r.low,
            close: r.closePrice ?? r.close,
            volume: r.volume
          })
    )
    .filter(Boolean)
    .sort((a, b) => a.time - b.time);
}

function detectSignal(candles) {
  if (candles.length < 40) return null;
  const closes = candles.map((c) => c.close);
  const last = closes.length - 1;
  const rsi = calcRSI(closes);
  const macdHist = calcMACDHistogram(closes);
  const ema9 = calcEMA(closes, 9);
  const ema21 = calcEMA(closes, 21);
  const ema34 = calcEMA(closes, 34);
  const ema50 = calcEMA(closes, 50);
  const ema55 = calcEMA(closes, 55);
  const ema89 = calcEMA(closes, 89);
  const vwap = calcVWAP(candles);
  const bb = calcBollingerBands(closes);
  const stoch = calcStochastic(candles);
  const supertrend = calcSupertrend(candles);
  const donchian = calcDonchian(candles);
  const ha = calcHeikinAshi(candles);
  const cci = calcCCI(candles);
  const willR = calcWilliamsR(candles);
  const adx = calcADX(candles);
  const psar = calcParabolicSAR(candles);
  const ichimoku = calcIchimoku(candles);
  const keltner = calcKeltner(candles);
  const cmf = calcCMF(candles);
  const obv = calcOBV(candles);
  const roc = calcROC(closes);
  const elder = calcElderRay(candles);
  const fib = calcFibLevels(candles);
  const vwapBands = calcVWAPBands(candles);
  const macdLine = calcMACDLine(closes);
  const ema200 = calcEMA(closes, 200);
  const pivots = calcPivotPoints(candles);
  const hma = calcHMA(closes);
  const trix = calcTRIX(closes);
  const aroon = calcAroon(candles);
  const adLine = calcAD(candles);
  const rvol = calcRVOL(candles);
  const linReg = calcLinReg(closes);
  const orderBlocks = detectOrderBlocks(candles);
  const fvgs = detectFVG(candles);
  const recent = candles.slice(-3);
  const avgBody = candles.slice(-20).reduce((a, c) => a + bodySize(c), 0) / 20;
  const vol5 = candles.slice(-5).reduce((a, c) => a + c.volume, 0) / 5;
  const volPrev5 = candles.slice(-10, -5).reduce((a, c) => a + c.volume, 0) / 5;
  const prev = candles[last - 1];
  const curr = candles[last];

  let buy = 0;
  let sell = 0;
  const reasons = [];
  const strategies = [];

  // ── Strategy 1: EMA 9/21 Crossover ─────────────────────────────────
  if (ema9[last] > ema21[last]) {
    buy += 2;
    reasons.push("EMA trend up");
    strategies.push("EMA Cross");
  } else {
    sell += 2;
    reasons.push("EMA trend down");
    strategies.push("EMA Cross");
  }

  // ── Strategy 2: MACD Histogram ──────────────────────────────────────
  if (macdHist[last] > 0 && macdHist[last] > macdHist[last - 1]) {
    buy += 2;
    reasons.push("MACD bullish & rising");
  } else if (macdHist[last] < 0 && macdHist[last] < macdHist[last - 1]) {
    sell += 2;
    reasons.push("MACD bearish & falling");
  } else if (macdHist[last] > 0) {
    buy += 1;
    reasons.push("MACD bullish");
  } else {
    sell += 1;
    reasons.push("MACD bearish");
  }

  // ── Strategy 3: RSI Mean Reversion ──────────────────────────────────
  if (rsi[last] < 30) {
    buy += 4;
    reasons.push(`RSI deeply oversold (${rsi[last].toFixed(0)})`);
    strategies.push("RSI Revert");
  } else if (rsi[last] < 38) {
    buy += 2;
    reasons.push(`RSI oversold (${rsi[last].toFixed(0)})`);
    strategies.push("RSI Revert");
  } else if (rsi[last] > 70) {
    sell += 4;
    reasons.push(`RSI deeply overbought (${rsi[last].toFixed(0)})`);
    strategies.push("RSI Revert");
  } else if (rsi[last] > 62) {
    sell += 2;
    reasons.push(`RSI overbought (${rsi[last].toFixed(0)})`);
    strategies.push("RSI Revert");
  }

  // ── Strategy 4: 3-Candle Momentum ───────────────────────────────────
  if (recent.every((c) => isBull(c) && bodySize(c) > avgBody * 0.25)) {
    buy += 2;
    reasons.push("3-candle bullish momentum");
    strategies.push("Momentum");
  }
  if (recent.every((c) => isBear(c) && bodySize(c) > avgBody * 0.25)) {
    sell += 2;
    reasons.push("3-candle bearish momentum");
    strategies.push("Momentum");
  }

  // ── Strategy 5: VWAP Position ───────────────────────────────────────
  if (closes[last] > vwap[last] * 1.0005) {
    buy += 1;
    reasons.push("Above VWAP");
  } else if (closes[last] < vwap[last] * 0.9995) {
    sell += 1;
    reasons.push("Below VWAP");
  }

  // ── Strategy 6: Volume Expansion ────────────────────────────────────
  if (volPrev5 > 0 && vol5 > volPrev5 * 1.2) {
    if (buy >= sell) buy += 1;
    else sell += 1;
    reasons.push("Volume surge");
  }

  // ── Strategy 7: Bollinger Band Bounce ───────────────────────────────
  const bbLast = bb[last];
  const bbRange = bbLast.upper - bbLast.lower;
  const bbPct = bbRange > 0 ? (closes[last] - bbLast.lower) / bbRange : 0.5;
  if (bbPct < 0.1) {
    buy += 3;
    reasons.push(`BB lower bounce (${(bbPct * 100).toFixed(0)}%)`);
    strategies.push("BB Bounce");
  } else if (bbPct < 0.2) {
    buy += 1;
    reasons.push("Near BB lower band");
  } else if (bbPct > 0.9) {
    sell += 3;
    reasons.push(`BB upper bounce (${(bbPct * 100).toFixed(0)}%)`);
    strategies.push("BB Bounce");
  } else if (bbPct > 0.8) {
    sell += 1;
    reasons.push("Near BB upper band");
  }

  // ── Strategy 8: BB Squeeze Breakout ─────────────────────────────────
  if (last >= 5) {
    const prevBw = bb.slice(last - 5, last).map((b) => b.bw);
    const avgPrevBw = prevBw.reduce((a, v) => a + v, 0) / prevBw.length;
    const squeezed = avgPrevBw > 0 && avgPrevBw < 0.015;
    if (squeezed && bbLast.bw > avgPrevBw * 1.5) {
      if (isBull(curr)) {
        buy += 2;
        reasons.push("BB squeeze breakout up");
        strategies.push("BB Squeeze");
      } else {
        sell += 2;
        reasons.push("BB squeeze breakout down");
        strategies.push("BB Squeeze");
      }
    }
  }

  // ── Strategy 9: Stochastic Crossover ────────────────────────────────
  const kNow = stoch.k[last];
  const kPrev = stoch.k[last - 1] || kNow;
  const dNow = stoch.d[last];
  const dPrev = stoch.d[last - 1] || dNow;
  if (kNow < 25 && dNow < 25) {
    buy += 2;
    reasons.push(`Stoch oversold (K:${kNow.toFixed(0)})`);
    strategies.push("Stoch");
  } else if (kNow > 75 && dNow > 75) {
    sell += 2;
    reasons.push(`Stoch overbought (K:${kNow.toFixed(0)})`);
    strategies.push("Stoch");
  }
  if (kPrev < dPrev && kNow > dNow && kNow < 40) {
    buy += 2;
    reasons.push("Stoch K/D bullish cross");
    strategies.push("Stoch Cross");
  } else if (kPrev > dPrev && kNow < dNow && kNow > 60) {
    sell += 2;
    reasons.push("Stoch K/D bearish cross");
    strategies.push("Stoch Cross");
  }

  // ── Strategy 10: EMA 50 Trend & Bounce ──────────────────────────────
  if (ema50.length > last) {
    const e50 = ema50[last];
    if (closes[last] > e50) {
      buy += 1;
      reasons.push("Above EMA50");
    } else {
      sell += 1;
      reasons.push("Below EMA50");
    }
    const distPct = Math.abs(closes[last] - e50) / e50;
    if (distPct < 0.002) {
      if (isBull(curr) && ema9[last] > ema21[last]) {
        buy += 2;
        reasons.push("EMA50 bounce up");
        strategies.push("EMA50 Bounce");
      } else if (isBear(curr) && ema9[last] < ema21[last]) {
        sell += 2;
        reasons.push("EMA50 bounce down");
        strategies.push("EMA50 Bounce");
      }
    }
  }

  // ── Strategy 11: Engulfing Candle ───────────────────────────────────
  if (prev && bodySize(curr) > bodySize(prev) * 1.5 && bodySize(prev) > 0) {
    if (isBull(curr) && isBear(prev) && curr.close > prev.open) {
      buy += 3;
      reasons.push("Bullish engulfing");
      strategies.push("Engulfing");
    } else if (isBear(curr) && isBull(prev) && curr.close < prev.open) {
      sell += 3;
      reasons.push("Bearish engulfing");
      strategies.push("Engulfing");
    }
  }

  // ── Strategy 12: Pin Bar / Hammer ───────────────────────────────────
  if (curr) {
    const totalRange = curr.high - curr.low;
    const body = bodySize(curr);
    const upperWick = curr.high - Math.max(curr.open, curr.close);
    const lowerWick = Math.min(curr.open, curr.close) - curr.low;
    if (totalRange > 0 && body < totalRange * 0.35) {
      if (lowerWick > body * 2.5 && lowerWick > upperWick * 2) {
        buy += 3;
        reasons.push("Hammer/pin bar bullish");
        strategies.push("Pin Bar");
      } else if (upperWick > body * 2.5 && upperWick > lowerWick * 2) {
        sell += 3;
        reasons.push("Shooting star bearish");
        strategies.push("Pin Bar");
      }
    }
  }

  // ── Strategy 13: Supertrend Direction ───────────────────────────────
  const st = supertrend[last];
  const stPrev = supertrend[last - 1];
  if (st.dir === 1) {
    buy += 2;
    reasons.push("Supertrend bullish");
    strategies.push("Supertrend");
  } else {
    sell += 2;
    reasons.push("Supertrend bearish");
    strategies.push("Supertrend");
  }
  if (stPrev && stPrev.dir !== st.dir) {
    if (st.dir === 1) { buy += 2; reasons.push("Supertrend flipped UP"); }
    else { sell += 2; reasons.push("Supertrend flipped DOWN"); }
  }

  // ── Strategy 14: Donchian Channel Breakout ───────────────────────────
  if (last >= 1) {
    const dcPrev = donchian[last - 1];
    if (closes[last] > dcPrev.high && closes[last - 1] <= dcPrev.high) {
      buy += 3;
      reasons.push("Donchian channel breakout UP");
      strategies.push("Donchian");
    } else if (closes[last] < dcPrev.low && closes[last - 1] >= dcPrev.low) {
      sell += 3;
      reasons.push("Donchian channel breakdown");
      strategies.push("Donchian");
    }
  }

  // ── Strategy 15: Heikin Ashi Trend Confirmation ───────────────────────
  if (ha.length > last + 1) {
    const haSlice = ha.slice(-5);
    const haBullCount = haSlice.filter((c) => c.close > c.open).length;
    const haBearCount = haSlice.filter((c) => c.close < c.open).length;
    const haLast = ha[last];
    const noUpperShadow = haLast.high <= Math.max(haLast.open, haLast.close) + 1;
    const noLowerShadow = haLast.low >= Math.min(haLast.open, haLast.close) - 1;
    if (haBullCount >= 4) {
      buy += 2;
      reasons.push(`HA ${haBullCount}/5 bull candles`);
      strategies.push("HA Trend");
      if (noLowerShadow) { buy += 1; reasons.push("HA strong bull (no lower shadow)"); }
    } else if (haBearCount >= 4) {
      sell += 2;
      reasons.push(`HA ${haBearCount}/5 bear candles`);
      strategies.push("HA Trend");
      if (noUpperShadow) { sell += 1; reasons.push("HA strong bear (no upper shadow)"); }
    }
  }

  // ── Strategy 16: CCI Extremes ────────────────────────────────────────
  const cciNow = cci[last];
  const cciPrev = cci[last - 1] || 0;
  if (cciNow < -150) {
    buy += 4;
    reasons.push(`CCI deeply oversold (${cciNow.toFixed(0)})`);
    strategies.push("CCI");
  } else if (cciNow < -100) {
    buy += 2;
    reasons.push(`CCI oversold (${cciNow.toFixed(0)})`);
    strategies.push("CCI");
  } else if (cciNow > 150) {
    sell += 4;
    reasons.push(`CCI deeply overbought (${cciNow.toFixed(0)})`);
    strategies.push("CCI");
  } else if (cciNow > 100) {
    sell += 2;
    reasons.push(`CCI overbought (${cciNow.toFixed(0)})`);
    strategies.push("CCI");
  }
  // CCI zero-line cross
  if (cciPrev < 0 && cciNow > 0) { buy += 2; reasons.push("CCI crossed above zero"); }
  else if (cciPrev > 0 && cciNow < 0) { sell += 2; reasons.push("CCI crossed below zero"); }

  // ── Strategy 17: Williams %R ─────────────────────────────────────────
  const wrNow = willR[last];
  const wrPrev = willR[last - 1] || -50;
  if (wrNow < -85) {
    buy += 3;
    reasons.push(`Williams %R oversold (${wrNow.toFixed(0)})`);
    strategies.push("Williams %R");
  } else if (wrNow < -70) {
    buy += 1;
    reasons.push(`Williams %R near oversold (${wrNow.toFixed(0)})`);
  } else if (wrNow > -15) {
    sell += 3;
    reasons.push(`Williams %R overbought (${wrNow.toFixed(0)})`);
    strategies.push("Williams %R");
  } else if (wrNow > -30) {
    sell += 1;
    reasons.push(`Williams %R near overbought (${wrNow.toFixed(0)})`);
  }
  // %R exit from extreme
  if (wrPrev < -80 && wrNow > -80) { buy += 2; reasons.push("Williams %R exiting oversold"); }
  else if (wrPrev > -20 && wrNow < -20) { sell += 2; reasons.push("Williams %R exiting overbought"); }

  // ── Strategy 18: ADX + DI Cross ─────────────────────────────────────
  const adxNow = adx[last];
  const adxPrev = adx[last - 1];
  if (adxNow && adxPrev) {
    const strongTrend = adxNow.adx > 25;
    const veryStrong = adxNow.adx > 40;
    if (strongTrend) {
      if (adxNow.diPlus > adxNow.diMinus) {
        buy += veryStrong ? 3 : 2;
        reasons.push(`ADX trend up (${adxNow.adx.toFixed(0)})`);
        strategies.push("ADX");
      } else {
        sell += veryStrong ? 3 : 2;
        reasons.push(`ADX trend down (${adxNow.adx.toFixed(0)})`);
        strategies.push("ADX");
      }
    }
    // DI crossover
    if (adxPrev.diPlus < adxPrev.diMinus && adxNow.diPlus > adxNow.diMinus && strongTrend) {
      buy += 3;
      reasons.push("ADX DI+ crossed above DI-");
      strategies.push("ADX Cross");
    } else if (adxPrev.diPlus > adxPrev.diMinus && adxNow.diPlus < adxNow.diMinus && strongTrend) {
      sell += 3;
      reasons.push("ADX DI- crossed above DI+");
      strategies.push("ADX Cross");
    }
  }

  // ── Strategy 19: Parabolic SAR ───────────────────────────────────────
  const psarNow = psar[last];
  const psarPrev = psar[last - 1];
  if (psarNow) {
    if (psarNow.rising) {
      buy += 2;
      reasons.push("PSAR bullish");
      strategies.push("PSAR");
    } else {
      sell += 2;
      reasons.push("PSAR bearish");
      strategies.push("PSAR");
    }
    if (psarPrev && psarPrev.rising !== psarNow.rising) {
      if (psarNow.rising) {
        buy += 3;
        reasons.push("PSAR flipped UP");
        strategies.push("PSAR Flip");
      } else {
        sell += 3;
        reasons.push("PSAR flipped DOWN");
        strategies.push("PSAR Flip");
      }
    }
  }

  // ── Strategy 20: RSI Divergence ─────────────────────────────────────
  if (last >= 10) {
    const lookback = 10;
    const priceSlice = closes.slice(last - lookback, last + 1);
    const rsiSlice = rsi.slice(last - lookback, last + 1);
    const priceMin = Math.min(...priceSlice.slice(0, -1));
    const priceMax = Math.max(...priceSlice.slice(0, -1));
    const rsiMin = Math.min(...rsiSlice.slice(0, -1));
    const rsiMax = Math.max(...rsiSlice.slice(0, -1));
    // Bullish divergence: price lower low, RSI higher low
    if (closes[last] < priceMin && rsiSlice[lookback] > rsiMin && rsiSlice[lookback] < 45) {
      buy += 4;
      reasons.push("Bullish RSI divergence");
      strategies.push("RSI Divergence");
    }
    // Bearish divergence: price higher high, RSI lower high
    if (closes[last] > priceMax && rsiSlice[lookback] < rsiMax && rsiSlice[lookback] > 55) {
      sell += 4;
      reasons.push("Bearish RSI divergence");
      strategies.push("RSI Divergence");
    }
  }

  // ── Strategy 21: Market Structure Break (MSB) ────────────────────────
  if (last >= 15) {
    const swing = candles.slice(last - 15, last - 3);
    const swingHighs = swing.map((c) => c.high);
    const swingLows = swing.map((c) => c.low);
    const highestSwing = Math.max(...swingHighs);
    const lowestSwing = Math.min(...swingLows);
    const pullback3 = candles.slice(last - 3, last - 1);
    const pulledBackDown = pullback3.some((c) => c.close < pullback3[0].open);
    const pulledBackUp = pullback3.some((c) => c.close > pullback3[0].open);
    // BOS up: broke structure high, pulled back, now bullish candle
    if (closes[last - 3] > highestSwing && pulledBackDown && isBull(curr) && ema9[last] > ema21[last]) {
      buy += 4;
      reasons.push("Market structure break UP");
      strategies.push("MSB");
    }
    // BOS down: broke structure low, pulled back up, now bearish candle
    if (closes[last - 3] < lowestSwing && pulledBackUp && isBear(curr) && ema9[last] < ema21[last]) {
      sell += 4;
      reasons.push("Market structure break DOWN");
      strategies.push("MSB");
    }
  }

  // ── Strategy 22: EMA Ribbon Alignment (21/34/55/89) ──────────────────
  if (ema89.length > last) {
    const e21 = ema21[last], e34 = ema34[last], e55 = ema55[last], e89 = ema89[last];
    const price = closes[last];
    if (price > e21 && e21 > e34 && e34 > e55 && e55 > e89) {
      buy += 3;
      reasons.push("EMA ribbon fully bullish");
      strategies.push("EMA Ribbon");
    } else if (price < e21 && e21 < e34 && e34 < e55 && e55 < e89) {
      sell += 3;
      reasons.push("EMA ribbon fully bearish");
      strategies.push("EMA Ribbon");
    } else if (price > e21 && e21 > e34) {
      buy += 1;
      reasons.push("EMA ribbon partial bull");
    } else if (price < e21 && e21 < e34) {
      sell += 1;
      reasons.push("EMA ribbon partial bear");
    }
  }

  // ── Strategy 23: VWAP Retest ─────────────────────────────────────────
  if (last >= 3) {
    const vwapNow = vwap[last];
    const distFromVwap = (closes[last] - vwapNow) / vwapNow;
    const prev3Closes = closes.slice(last - 3, last);
    const wasAboveVwap = prev3Closes.every((c) => c > vwap[last - 3]);
    const wasBelowVwap = prev3Closes.every((c) => c < vwap[last - 3]);
    // Price retested VWAP from above and bounced
    if (wasAboveVwap && Math.abs(distFromVwap) < 0.001 && isBull(curr)) {
      buy += 3;
      reasons.push("VWAP retest bounce UP");
      strategies.push("VWAP Retest");
    } else if (wasBelowVwap && Math.abs(distFromVwap) < 0.001 && isBear(curr)) {
      sell += 3;
      reasons.push("VWAP retest rejection DOWN");
      strategies.push("VWAP Retest");
    }
  }

  // ─────────────────────────────────────────────────────────────────────
  // HIGH-PROFIT STRATEGIES (44–63)
  // ─────────────────────────────────────────────────────────────────────

  // ── Strategy 44: Order Block ─────────────────────────────────────────
  const curClose = closes[last];
  orderBlocks.bullOBs.slice(0, 3).forEach(ob => {
    if (curClose >= ob.low && curClose <= ob.high * 1.002) {
      buy += 5; reasons.push(`In Bullish OB zone ($${ob.low.toFixed(0)}-$${ob.high.toFixed(0)})`);
      strategies.push("Order Block");
    }
  });
  orderBlocks.bearOBs.slice(0, 3).forEach(ob => {
    if (curClose >= ob.low * 0.998 && curClose <= ob.high) {
      sell += 5; reasons.push(`In Bearish OB zone ($${ob.low.toFixed(0)}-$${ob.high.toFixed(0)})`);
      strategies.push("Order Block");
    }
  });

  // ── Strategy 45: Fair Value Gap Fill ─────────────────────────────────
  fvgs.filter(g => g.age <= 20).forEach(g => {
    if (g.type === 'bull' && curClose >= g.low && curClose <= g.high) {
      buy += 4; reasons.push(`Filling Bull FVG ($${g.low.toFixed(0)}-$${g.high.toFixed(0)})`);
      strategies.push("FVG");
    }
    if (g.type === 'bear' && curClose >= g.low && curClose <= g.high) {
      sell += 4; reasons.push(`Filling Bear FVG ($${g.low.toFixed(0)}-$${g.high.toFixed(0)})`);
      strategies.push("FVG");
    }
  });

  // ── Strategy 46: Liquidity Sweep Reversal ────────────────────────────
  if (last >= 3) {
    const recentLows  = candles.slice(last - 10, last - 1).map(c => c.low);
    const recentHighs = candles.slice(last - 10, last - 1).map(c => c.high);
    const structLow  = Math.min(...recentLows);
    const structHigh = Math.max(...recentHighs);
    const sweptLow  = candles[last - 1].low < structLow && curr.close > structLow;
    const sweptHigh = candles[last - 1].high > structHigh && curr.close < structHigh;
    if (sweptLow  && isBull(curr)) { buy  += 5; reasons.push("Liquidity sweep below lows"); strategies.push("Liq Sweep"); }
    if (sweptHigh && isBear(curr)) { sell += 5; reasons.push("Liquidity sweep above highs"); strategies.push("Liq Sweep"); }
  }

  // ── Strategy 47: Hull MA Cross ───────────────────────────────────────
  if (hma.length > last && last >= 1) {
    const hmaNow = hma[last], hmaPrev = hma[last - 1];
    if (hmaPrev < closes[last - 1] && hmaNow > curClose) {
      // HMA above price: bearish (HMA acts as resistance flipping)
    }
    if (hmaNow > hmaPrev && closes[last] > hmaNow) {
      buy += 3; reasons.push(`HMA rising (${hmaNow.toFixed(0)})`); strategies.push("HMA");
    } else if (hmaNow < hmaPrev && closes[last] < hmaNow) {
      sell += 3; reasons.push(`HMA falling (${hmaNow.toFixed(0)})`); strategies.push("HMA");
    }
    if (closes[last - 1] < hma[last - 1] && closes[last] > hmaNow) {
      buy += 3; reasons.push("Price crossed above HMA"); strategies.push("HMA Cross");
    } else if (closes[last - 1] > hma[last - 1] && closes[last] < hmaNow) {
      sell += 3; reasons.push("Price crossed below HMA"); strategies.push("HMA Cross");
    }
  }

  // ── Strategy 48: TRIX Zero Cross & Momentum ──────────────────────────
  const trixNow  = trix[last];
  const trixPrev = trix[last - 1] || 0;
  if (trixPrev < 0 && trixNow > 0) { buy  += 4; reasons.push("TRIX bullish zero cross"); strategies.push("TRIX"); }
  else if (trixPrev > 0 && trixNow < 0) { sell += 4; reasons.push("TRIX bearish zero cross"); strategies.push("TRIX"); }
  else if (trixNow > 0 && trixNow > trixPrev) { buy  += 2; reasons.push(`TRIX rising (${trixNow.toFixed(3)})`); }
  else if (trixNow < 0 && trixNow < trixPrev) { sell += 2; reasons.push(`TRIX falling (${trixNow.toFixed(3)})`); }

  // ── Strategy 49: Aroon Cross & Extremes ──────────────────────────────
  const arNow  = aroon[last];
  const arPrev = aroon[last - 1] || { up: 50, down: 50 };
  if (arNow.up > 70 && arNow.down < 30) { buy  += 3; reasons.push(`Aroon bullish (U:${arNow.up.toFixed(0)} D:${arNow.down.toFixed(0)})`); strategies.push("Aroon"); }
  else if (arNow.down > 70 && arNow.up < 30) { sell += 3; reasons.push(`Aroon bearish (U:${arNow.up.toFixed(0)} D:${arNow.down.toFixed(0)})`); strategies.push("Aroon"); }
  if (arPrev.up < arPrev.down && arNow.up > arNow.down) { buy  += 3; reasons.push("Aroon bullish cross"); strategies.push("Aroon Cross"); }
  else if (arPrev.up > arPrev.down && arNow.up < arNow.down) { sell += 3; reasons.push("Aroon bearish cross"); strategies.push("Aroon Cross"); }

  // ── Strategy 50: A/D Line Trend ──────────────────────────────────────
  if (last >= 5) {
    const adNow  = adLine[last];
    const adPast = adLine[last - 5];
    const adTrend = adNow - adPast;
    const priceTrend5 = closes[last] - closes[last - 5];
    if (adTrend > 0 && priceTrend5 > 0) { buy  += 2; reasons.push("A/D line rising"); strategies.push("A/D Line"); }
    else if (adTrend < 0 && priceTrend5 < 0) { sell += 2; reasons.push("A/D line falling"); strategies.push("A/D Line"); }
    // Divergence
    if (adTrend > 0 && priceTrend5 < 0) { buy  += 3; reasons.push("Bullish A/D divergence"); strategies.push("A/D Div"); }
    if (adTrend < 0 && priceTrend5 > 0) { sell += 3; reasons.push("Bearish A/D divergence"); strategies.push("A/D Div"); }
  }

  // ── Strategy 51: Relative Volume Confirmation ────────────────────────
  const rvolNow = rvol[last];
  if (rvolNow >= 3.0) {
    if (isBull(curr)) { buy  += 4; reasons.push(`RVOL spike ${rvolNow.toFixed(1)}x bull`); strategies.push("RVOL Spike"); }
    else               { sell += 4; reasons.push(`RVOL spike ${rvolNow.toFixed(1)}x bear`); strategies.push("RVOL Spike"); }
  } else if (rvolNow >= 2.0) {
    if (isBull(curr)) { buy  += 2; reasons.push(`High RVOL ${rvolNow.toFixed(1)}x bull`); }
    else               { sell += 2; reasons.push(`High RVOL ${rvolNow.toFixed(1)}x bear`); }
  } else if (rvolNow < 0.5) {
    // Low volume — reduce confidence in whichever side is leading
    if (buy > sell) buy  = Math.max(0, buy  - 2);
    else            sell = Math.max(0, sell - 2);
    reasons.push(`Low RVOL (${rvolNow.toFixed(1)}x) signal weakened`);
  }

  // ── Strategy 52: Linear Regression Channel ───────────────────────────
  const lr = linReg[last];
  if (lr) {
    if (closes[last] < lr.lower) { buy  += 4; reasons.push("Below LinReg channel lower"); strategies.push("LinReg"); }
    else if (closes[last] > lr.upper) { sell += 4; reasons.push("Above LinReg channel upper"); strategies.push("LinReg"); }
    else if (closes[last] < lr.mid && lr.slope > 0) { buy  += 2; reasons.push("LinReg pullback in uptrend"); }
    else if (closes[last] > lr.mid && lr.slope < 0) { sell += 2; reasons.push("LinReg pullback in downtrend"); }
    if (lr.slope > 0) { buy  += 1; reasons.push("LinReg slope positive"); }
    else              { sell += 1; reasons.push("LinReg slope negative"); }
  }

  // ── Strategy 53: NR4 / NR7 Squeeze Breakout ──────────────────────────
  const currRange = curr.high - curr.low;
  if (last >= 7) {
    const ranges4 = candles.slice(last - 4, last).map(c => c.high - c.low);
    const ranges7 = candles.slice(last - 7, last).map(c => c.high - c.low);
    const isNR4 = currRange < Math.min(...ranges4);
    const isNR7 = currRange < Math.min(...ranges7);
    if (isNR7 && last >= 1) {
      if (isBull(curr)) { buy  += 4; reasons.push("NR7 squeeze breakout UP"); strategies.push("NR7"); }
      else               { sell += 4; reasons.push("NR7 squeeze breakout DOWN"); strategies.push("NR7"); }
    } else if (isNR4) {
      if (isBull(curr)) { buy  += 2; reasons.push("NR4 squeeze UP"); strategies.push("NR4"); }
      else               { sell += 2; reasons.push("NR4 squeeze DOWN"); strategies.push("NR4"); }
    }
  }

  // ── Strategy 54: Smart Money Distribution (High Vol + Small Body) ────
  if (rvolNow >= 2 && currRange > 0 && bodySize(curr) / currRange < 0.25) {
    const upperWick = curr.high - Math.max(curr.open, curr.close);
    const lowerWick = Math.min(curr.open, curr.close) - curr.low;
    if (lowerWick > upperWick * 2) { buy  += 4; reasons.push("Smart money accumulation"); strategies.push("SMC Accum"); }
    if (upperWick > lowerWick * 2) { sell += 4; reasons.push("Smart money distribution"); strategies.push("SMC Dist"); }
  }

  // ── Strategy 55: Change of Character (CHoCH) ─────────────────────────
  if (last >= 15) {
    const prevTrend = candles.slice(last - 15, last - 5);
    const recentAction = candles.slice(last - 5, last + 1);
    const prevLows  = prevTrend.map(c => c.low);
    const prevHighs = prevTrend.map(c => c.high);
    const wasDowntrend = prevLows[prevLows.length - 1] < prevLows[0];
    const wasUptrend   = prevHighs[prevHighs.length - 1] > prevHighs[0];
    const recentHigh = Math.max(...recentAction.map(c => c.high));
    const recentLow  = Math.min(...recentAction.map(c => c.low));
    if (wasDowntrend && recentHigh > Math.max(...prevHighs) && isBull(curr)) {
      buy  += 5; reasons.push("CHoCH: downtrend structure broken UP"); strategies.push("CHoCH");
    }
    if (wasUptrend && recentLow < Math.min(...prevLows) && isBear(curr)) {
      sell += 5; reasons.push("CHoCH: uptrend structure broken DOWN"); strategies.push("CHoCH");
    }
  }

  // ── Strategy 56: Volume-Confirmed Breakout ───────────────────────────
  if (last >= 20 && rvolNow >= 2.5) {
    const high20 = Math.max(...candles.slice(last - 20, last).map(c => c.high));
    const low20  = Math.min(...candles.slice(last - 20, last).map(c => c.low));
    if (closes[last] > high20) { buy  += 5; reasons.push(`Vol-confirmed breakout above $${high20.toFixed(0)}`); strategies.push("Vol Breakout"); }
    if (closes[last] < low20)  { sell += 5; reasons.push(`Vol-confirmed breakdown below $${low20.toFixed(0)}`); strategies.push("Vol Breakout"); }
  }

  // ── Strategy 57: Candle Body Expansion ──────────────────────────────
  if (last >= 5) {
    const bodies = candles.slice(last - 4, last + 1).map(c => bodySize(c));
    const expanding = bodies.every((b, i) => i === 0 || b >= bodies[i - 1] * 0.9);
    const allBullExp = expanding && candles.slice(last - 4, last + 1).every(c => isBull(c));
    const allBearExp = expanding && candles.slice(last - 4, last + 1).every(c => isBear(c));
    if (allBullExp) { buy  += 3; reasons.push("Bull candle body expansion"); strategies.push("Body Exp"); }
    if (allBearExp) { sell += 3; reasons.push("Bear candle body expansion"); strategies.push("Body Exp"); }
  }

  // ── Strategy 58: Price Velocity Acceleration ─────────────────────────
  if (last >= 6) {
    const vel1 = closes[last]     - closes[last - 3];
    const vel2 = closes[last - 3] - closes[last - 6];
    if (vel1 > 0 && vel2 > 0 && vel1 > vel2 * 1.5) { buy  += 3; reasons.push("Bull velocity accelerating"); strategies.push("Velocity"); }
    if (vel1 < 0 && vel2 < 0 && Math.abs(vel1) > Math.abs(vel2) * 1.5) { sell += 3; reasons.push("Bear velocity accelerating"); strategies.push("Velocity"); }
    // Deceleration = potential reversal
    if (vel2 > 0 && vel1 < 0) { sell += 2; reasons.push("Bull momentum reversing"); }
    if (vel2 < 0 && vel1 > 0) { buy  += 2; reasons.push("Bear momentum reversing"); }
  }

  // ── Strategy 59: Multi-Oscillator Confluence ─────────────────────────
  let bullOscCount = 0, bearOscCount = 0;
  if (rsi[last] < 45)     bullOscCount++; else if (rsi[last] > 55)     bearOscCount++;
  if (cciNow < -50)       bullOscCount++; else if (cciNow > 50)        bearOscCount++;
  if (wrNow < -60)        bullOscCount++; else if (wrNow > -40)        bearOscCount++;
  if (stoch.k[last] < 45) bullOscCount++; else if (stoch.k[last] > 55) bearOscCount++;
  if (trixNow > 0)        bullOscCount++; else if (trixNow < 0)        bearOscCount++;
  if (bullOscCount >= 4) { buy  += 4; reasons.push(`${bullOscCount}/5 oscillators bearish→bullish`); strategies.push("Osc Confluence"); }
  if (bearOscCount >= 4) { sell += 4; reasons.push(`${bearOscCount}/5 oscillators overbought`);      strategies.push("Osc Confluence"); }

  // ── Strategy 60: Rejection Candle at Key Level ───────────────────────
  if (curr) {
    const wick = curr.high - curr.low;
    const body = bodySize(curr);
    const sr = findSupportResistance(candles);
    const tolerance = wick * 0.15 + 10;
    if (wick > 0 && body / wick < 0.3) {
      const nearSupport    = sr.support    && Math.abs(curr.low  - sr.support)    < tolerance;
      const nearResistance = sr.resistance && Math.abs(curr.high - sr.resistance) < tolerance;
      const lowerWick = Math.min(curr.open, curr.close) - curr.low;
      const upperWick = curr.high - Math.max(curr.open, curr.close);
      if (nearSupport    && lowerWick > upperWick * 2) { buy  += 4; reasons.push("Rejection candle at support");    strategies.push("Rejection"); }
      if (nearResistance && upperWick > lowerWick * 2) { sell += 4; reasons.push("Rejection candle at resistance"); strategies.push("Rejection"); }
    }
  }

  // ── Strategy 61: Trend Exhaustion (Climax) ───────────────────────────
  if (last >= 5) {
    const last5 = candles.slice(last - 4, last + 1);
    const allBull = last5.every(c => isBull(c));
    const allBear = last5.every(c => isBear(c));
    const bigVol = rvolNow >= 2;
    const smallBody = bodySize(curr) < avgBody * 0.5;
    // Exhaustion: multi-candle run + climax volume + small final candle
    if (allBull && bigVol && smallBody) { sell += 4; reasons.push("Bull climax exhaustion"); strategies.push("Climax"); }
    if (allBear && bigVol && smallBody) { buy  += 4; reasons.push("Bear climax exhaustion"); strategies.push("Climax"); }
  }

  // ── Strategy 62: EMA Fan Divergence ──────────────────────────────────
  // When fast EMAs start pulling away from slow EMAs = trend strength
  if (last >= 3) {
    const spread9_21_now  = Math.abs(ema9[last]     - ema21[last]);
    const spread9_21_prev = Math.abs(ema9[last - 3] - ema21[last - 3]);
    const expanding = spread9_21_now > spread9_21_prev * 1.3;
    if (expanding) {
      if (ema9[last] > ema21[last]) { buy  += 2; reasons.push("EMA fan expanding bullish"); strategies.push("EMA Fan"); }
      else                           { sell += 2; reasons.push("EMA fan expanding bearish"); strategies.push("EMA Fan"); }
    }
  }

  // ── Strategy 63: HMA + Supertrend Agreement ──────────────────────────
  const hmaNow2   = hma[last];
  const hmaPrev2  = hma[last - 1] || hmaNow2;
  const hmaRising = hmaNow2 > hmaPrev2;
  const stBull    = supertrend[last]?.dir === 1;
  if (hmaRising && stBull && closes[last] > hmaNow2) {
    buy  += 4; reasons.push("HMA rising + Supertrend bullish"); strategies.push("HMA+ST");
  } else if (!hmaRising && !stBull && closes[last] < hmaNow2) {
    sell += 4; reasons.push("HMA falling + Supertrend bearish"); strategies.push("HMA+ST");
  }

  // ── Strategy 24: Ichimoku Cloud ──────────────────────────────────────
  const ichi = ichimoku[last];
  const ichiPrev = ichimoku[last - 1];
  if (ichi && ichiPrev) {
    const aboveCloud = closes[last] > Math.max(ichi.senkouA, ichi.senkouB);
    const belowCloud = closes[last] < Math.min(ichi.senkouA, ichi.senkouB);
    if (aboveCloud) { buy += 2; reasons.push("Above Ichimoku cloud"); strategies.push("Ichimoku"); }
    else if (belowCloud) { sell += 2; reasons.push("Below Ichimoku cloud"); strategies.push("Ichimoku"); }
    // TK Cross
    if (ichiPrev.tenkan < ichiPrev.kijun && ichi.tenkan > ichi.kijun) {
      buy += 3; reasons.push("Ichimoku TK bullish cross"); strategies.push("Ichimoku TK");
    } else if (ichiPrev.tenkan > ichiPrev.kijun && ichi.tenkan < ichi.kijun) {
      sell += 3; reasons.push("Ichimoku TK bearish cross"); strategies.push("Ichimoku TK");
    }
    // Price crossing cloud
    if (closes[last - 1] < Math.max(ichiPrev.senkouA, ichiPrev.senkouB) && aboveCloud) {
      buy += 3; reasons.push("Price broke above Ichimoku cloud");
    } else if (closes[last - 1] > Math.min(ichiPrev.senkouA, ichiPrev.senkouB) && belowCloud) {
      sell += 3; reasons.push("Price broke below Ichimoku cloud");
    }
  }

  // ── Strategy 25: Keltner Channel ─────────────────────────────────────
  const kc = keltner[last];
  if (kc) {
    const kcPct = (kc.upper - kc.lower) > 0 ? (closes[last] - kc.lower) / (kc.upper - kc.lower) : 0.5;
    if (closes[last] < kc.lower) { buy += 3; reasons.push("Below Keltner lower band"); strategies.push("Keltner"); }
    else if (closes[last] > kc.upper) { sell += 3; reasons.push("Above Keltner upper band"); strategies.push("Keltner"); }
    else if (kcPct < 0.15) { buy += 1; reasons.push("Near Keltner lower"); }
    else if (kcPct > 0.85) { sell += 1; reasons.push("Near Keltner upper"); }
    // KC+BB Squeeze: BB inside KC = squeeze setup
    if (bbLast.upper < kc.upper && bbLast.lower > kc.lower) {
      if (isBull(curr)) { buy += 2; reasons.push("KC+BB squeeze BUY"); strategies.push("KC Squeeze"); }
      else { sell += 2; reasons.push("KC+BB squeeze SELL"); strategies.push("KC Squeeze"); }
    }
  }

  // ── Strategy 26: Chaikin Money Flow ──────────────────────────────────
  const cmfNow = cmf[last];
  const cmfPrev = cmf[last - 1] || 0;
  if (cmfNow > 0.15) { buy += 3; reasons.push(`CMF strong inflow (${cmfNow.toFixed(2)})`); strategies.push("CMF"); }
  else if (cmfNow > 0.05) { buy += 1; reasons.push(`CMF inflow (${cmfNow.toFixed(2)})`); }
  else if (cmfNow < -0.15) { sell += 3; reasons.push(`CMF strong outflow (${cmfNow.toFixed(2)})`); strategies.push("CMF"); }
  else if (cmfNow < -0.05) { sell += 1; reasons.push(`CMF outflow (${cmfNow.toFixed(2)})`); }
  if (cmfPrev < 0 && cmfNow > 0) { buy += 2; reasons.push("CMF flipped positive"); }
  else if (cmfPrev > 0 && cmfNow < 0) { sell += 2; reasons.push("CMF flipped negative"); }

  // ── Strategy 27: On-Balance Volume Trend ────────────────────────────
  if (last >= 5) {
    const obvSlice = obv.slice(last - 5, last + 1);
    const obvTrend = obvSlice[obvSlice.length - 1] - obvSlice[0];
    const priceTrend = closes[last] - closes[last - 5];
    if (obvTrend > 0 && priceTrend > 0) { buy += 2; reasons.push("OBV rising with price"); strategies.push("OBV"); }
    else if (obvTrend < 0 && priceTrend < 0) { sell += 2; reasons.push("OBV falling with price"); strategies.push("OBV"); }
    // OBV divergence
    else if (obvTrend > 0 && priceTrend < 0) { buy += 2; reasons.push("Bullish OBV divergence"); strategies.push("OBV Div"); }
    else if (obvTrend < 0 && priceTrend > 0) { sell += 2; reasons.push("Bearish OBV divergence"); strategies.push("OBV Div"); }
  }

  // ── Strategy 28: Rate of Change ─────────────────────────────────────
  const rocNow = roc[last];
  const rocPrev = roc[last - 1] || 0;
  if (rocNow > 1.5) { buy += 2; reasons.push(`ROC bullish momentum (${rocNow.toFixed(1)}%)`); strategies.push("ROC"); }
  else if (rocNow < -1.5) { sell += 2; reasons.push(`ROC bearish momentum (${rocNow.toFixed(1)}%)`); strategies.push("ROC"); }
  if (rocPrev < 0 && rocNow > 0) { buy += 1; reasons.push("ROC turned positive"); }
  else if (rocPrev > 0 && rocNow < 0) { sell += 1; reasons.push("ROC turned negative"); }

  // ── Strategy 29: Elder Ray (Bull/Bear Power) ─────────────────────────
  const erNow = elder[last];
  const erPrev = elder[last - 1];
  if (erNow && erPrev) {
    if (erNow.bullPower > 0 && erNow.bearPower < 0 && erNow.bullPower > erPrev.bullPower) {
      buy += 2; reasons.push(`Elder: Bull power rising (+${erNow.bullPower.toFixed(0)})`); strategies.push("Elder Ray");
    } else if (erNow.bearPower < 0 && erNow.bullPower > 0 && erNow.bearPower < erPrev.bearPower) {
      sell += 2; reasons.push(`Elder: Bear power falling (${erNow.bearPower.toFixed(0)})`); strategies.push("Elder Ray");
    }
    if (erNow.bullPower > 0 && erPrev.bullPower <= 0) { buy += 2; reasons.push("Bull power crossed positive"); }
    if (erNow.bearPower > 0 && erPrev.bearPower <= 0) { buy += 1; reasons.push("Bear power crossed positive (bull)"); }
  }

  // ── Strategy 30: Fibonacci Retracement Bounce ───────────────────────
  const fibTol = fib.swingHigh * 0.0015;
  const fibLevels = [
    { label: "Fib 38.2%", price: fib.fib382 },
    { label: "Fib 50%",   price: fib.fib500 },
    { label: "Fib 61.8%", price: fib.fib618 },
    { label: "Fib 78.6%", price: fib.fib786 }
  ];
  fibLevels.forEach(({ label, price: fp }) => {
    if (Math.abs(closes[last] - fp) < fibTol) {
      const uptrend = ema9[last] > ema21[last];
      if (isBull(curr) && uptrend) { buy += 3; reasons.push(`${label} bounce UP`); strategies.push("Fibonacci"); }
      else if (isBear(curr) && !uptrend) { sell += 3; reasons.push(`${label} bounce DOWN`); strategies.push("Fibonacci"); }
    }
  });

  // ── Strategy 31: Morning Star / Evening Star ─────────────────────────
  if (last >= 2) {
    const c0 = candles[last - 2], c1 = candles[last - 1], c2 = candles[last];
    const avgB = avgBody || 1;
    // Morning Star: big bear, small doji/body, big bull
    if (isBear(c0) && bodySize(c0) > avgB * 1.2 &&
        bodySize(c1) < avgB * 0.5 &&
        isBull(c2) && bodySize(c2) > avgB * 1.2 && c2.close > (c0.open + c0.close) / 2) {
      buy += 4; reasons.push("Morning star pattern"); strategies.push("Morning Star");
    }
    // Evening Star: big bull, small doji, big bear
    if (isBull(c0) && bodySize(c0) > avgB * 1.2 &&
        bodySize(c1) < avgB * 0.5 &&
        isBear(c2) && bodySize(c2) > avgB * 1.2 && c2.close < (c0.open + c0.close) / 2) {
      sell += 4; reasons.push("Evening star pattern"); strategies.push("Evening Star");
    }
  }

  // ── Strategy 32: Three White Soldiers / Three Black Crows ────────────
  if (last >= 2) {
    const c0 = candles[last - 2], c1 = candles[last - 1], c2 = candles[last];
    const avgB = avgBody || 1;
    if (isBull(c0) && isBull(c1) && isBull(c2) &&
        bodySize(c0) > avgB * 0.8 && bodySize(c1) > avgB * 0.8 && bodySize(c2) > avgB * 0.8 &&
        c1.open > c0.open && c1.open < c0.close &&
        c2.open > c1.open && c2.open < c1.close) {
      buy += 4; reasons.push("Three white soldiers"); strategies.push("3 Soldiers");
    }
    if (isBear(c0) && isBear(c1) && isBear(c2) &&
        bodySize(c0) > avgB * 0.8 && bodySize(c1) > avgB * 0.8 && bodySize(c2) > avgB * 0.8 &&
        c1.open < c0.open && c1.open > c0.close &&
        c2.open < c1.open && c2.open > c1.close) {
      sell += 4; reasons.push("Three black crows"); strategies.push("3 Crows");
    }
  }

  // ── Strategy 33: Tweezer Top / Bottom ───────────────────────────────
  if (prev && curr) {
    const tolerance = (curr.high - curr.low) * 0.05 + 1;
    if (Math.abs(curr.low - prev.low) < tolerance && isBear(prev) && isBull(curr)) {
      buy += 3; reasons.push("Tweezer bottom reversal"); strategies.push("Tweezer");
    }
    if (Math.abs(curr.high - prev.high) < tolerance && isBull(prev) && isBear(curr)) {
      sell += 3; reasons.push("Tweezer top reversal"); strategies.push("Tweezer");
    }
  }

  // ── Strategy 34: Inside Bar Breakout ────────────────────────────────
  if (last >= 2) {
    const mother = candles[last - 2], inside = candles[last - 1];
    const isInsideBar = inside.high < mother.high && inside.low > mother.low;
    if (isInsideBar) {
      if (curr.close > mother.high) { buy += 3; reasons.push("Inside bar breakout UP"); strategies.push("Inside Bar"); }
      else if (curr.close < mother.low) { sell += 3; reasons.push("Inside bar breakdown"); strategies.push("Inside Bar"); }
    }
  }

  // ── Strategy 35: Marubozu (Strong Conviction Candle) ────────────────
  if (curr) {
    const range = curr.high - curr.low;
    const body = bodySize(curr);
    if (range > 0 && body / range > 0.9 && body > avgBody * 1.5) {
      if (isBull(curr)) { buy += 3; reasons.push("Bullish marubozu"); strategies.push("Marubozu"); }
      else { sell += 3; reasons.push("Bearish marubozu"); strategies.push("Marubozu"); }
    }
  }

  // ── Strategy 36: ATR Expansion (Volatility Breakout) ────────────────
  const atrArr = calcATR(candles);
  const atrNow = atrArr[last];
  const atrAvg = atrArr.slice(Math.max(0, last - 14), last).reduce((a, v) => a + v, 0) / Math.min(14, last);
  if (atrAvg > 0 && atrNow > atrAvg * 1.8) {
    if (isBull(curr)) { buy += 2; reasons.push(`ATR expansion bull (${(atrNow / atrAvg).toFixed(1)}x)`); strategies.push("ATR Breakout"); }
    else { sell += 2; reasons.push(`ATR expansion bear (${(atrNow / atrAvg).toFixed(1)}x)`); strategies.push("ATR Breakout"); }
  }

  // ── Strategy 37: MACD Line Zero Cross ───────────────────────────────
  const mlNow = macdLine[last];
  const mlPrev = macdLine[last - 1] || 0;
  if (mlPrev < 0 && mlNow > 0) { buy += 3; reasons.push("MACD line crossed above zero"); strategies.push("MACD Zero"); }
  else if (mlPrev > 0 && mlNow < 0) { sell += 3; reasons.push("MACD line crossed below zero"); strategies.push("MACD Zero"); }

  // ── Strategy 38: Golden / Death Cross (EMA50 / EMA200) ───────────────
  if (ema200.length > last && ema200[last - 1]) {
    const goldCross = ema50[last - 1] < ema200[last - 1] && ema50[last] > ema200[last];
    const deathCross = ema50[last - 1] > ema200[last - 1] && ema50[last] < ema200[last];
    if (goldCross) { buy += 5; reasons.push("Golden cross (EMA50 > EMA200)"); strategies.push("Golden Cross"); }
    else if (deathCross) { sell += 5; reasons.push("Death cross (EMA50 < EMA200)"); strategies.push("Death Cross"); }
    else if (ema50[last] > ema200[last]) { buy += 1; reasons.push("Above EMA200 (bull market)"); }
    else { sell += 1; reasons.push("Below EMA200 (bear market)"); }
  }

  // ── Strategy 39: Consecutive Candles (5+) ───────────────────────────
  if (last >= 5) {
    const c5 = candles.slice(last - 4, last + 1);
    const allBull = c5.every(c => isBull(c) && bodySize(c) > avgBody * 0.3);
    const allBear = c5.every(c => isBear(c) && bodySize(c) > avgBody * 0.3);
    if (allBull) { buy += 3; reasons.push("5 consecutive bull candles"); strategies.push("Consec Bull"); }
    else if (allBear) { sell += 3; reasons.push("5 consecutive bear candles"); strategies.push("Consec Bear"); }
  }

  // ── Strategy 40: VWAP Standard Deviation Bands ───────────────────────
  const vb = vwapBands[last];
  if (vb) {
    if (closes[last] < vb.lower2) { buy += 4; reasons.push("Below VWAP -2σ band"); strategies.push("VWAP Band"); }
    else if (closes[last] < vb.lower1) { buy += 2; reasons.push("Below VWAP -1σ band"); strategies.push("VWAP Band"); }
    else if (closes[last] > vb.upper2) { sell += 4; reasons.push("Above VWAP +2σ band"); strategies.push("VWAP Band"); }
    else if (closes[last] > vb.upper1) { sell += 2; reasons.push("Above VWAP +1σ band"); strategies.push("VWAP Band"); }
  }

  // ── Strategy 41: CMF + OBV Confluence ───────────────────────────────
  if (last >= 5) {
    const obvTrend = obv[last] - obv[last - 5];
    if (cmfNow > 0.05 && obvTrend > 0) { buy += 3; reasons.push("CMF+OBV bull confluence"); strategies.push("CMF+OBV"); }
    else if (cmfNow < -0.05 && obvTrend < 0) { sell += 3; reasons.push("CMF+OBV bear confluence"); strategies.push("CMF+OBV"); }
  }

  // ── Strategy 42: Pivot Point Bounce ──────────────────────────────────
  if (pivots) {
    const ptol = closes[last] * 0.001;
    if (Math.abs(closes[last] - pivots.s1) < ptol && isBull(curr)) {
      buy += 3; reasons.push("Bounce at Pivot S1"); strategies.push("Pivot");
    } else if (Math.abs(closes[last] - pivots.s2) < ptol && isBull(curr)) {
      buy += 4; reasons.push("Bounce at Pivot S2"); strategies.push("Pivot");
    } else if (Math.abs(closes[last] - pivots.r1) < ptol && isBear(curr)) {
      sell += 3; reasons.push("Rejection at Pivot R1"); strategies.push("Pivot");
    } else if (Math.abs(closes[last] - pivots.r2) < ptol && isBear(curr)) {
      sell += 4; reasons.push("Rejection at Pivot R2"); strategies.push("Pivot");
    } else if (Math.abs(closes[last] - pivots.pp) < ptol) {
      if (isBull(curr)) { buy += 2; reasons.push("Bounce at Pivot PP"); }
      else { sell += 2; reasons.push("Rejection at Pivot PP"); }
    }
  }

  // ── Strategy 43: Double Bottom / Double Top Pattern ──────────────────
  if (last >= 20) {
    const lookback = candles.slice(last - 20, last - 2);
    const recentLow = Math.min(...lookback.map(c => c.low));
    const recentHigh = Math.max(...lookback.map(c => c.high));
    const tol = closes[last] * 0.003;
    if (Math.abs(curr.low - recentLow) < tol && isBull(curr) && rsi[last] < 50) {
      buy += 4; reasons.push("Double bottom reversal"); strategies.push("Double Bottom");
    }
    if (Math.abs(curr.high - recentHigh) < tol && isBear(curr) && rsi[last] > 50) {
      sell += 4; reasons.push("Double top reversal"); strategies.push("Double Top");
    }
  }

  const side = buy > sell ? "BUY" : sell > buy ? "SELL" : "NEUTRAL";
  const dominance = Math.max(buy, sell);
  const confidence = Math.min(95, Math.round(40 + dominance * 4 + Math.abs(buy - sell) * 2));

  // Pick the primary strategy label for the trade tag
  const primaryStrategy = strategies.length > 0 ? strategies[strategies.length - 1] : null;
  const tagMap = {
    "Engulfing":      side === "BUY" ? "Engulf Long"       : "Engulf Short",
    "Pin Bar":        side === "BUY" ? "Pin Bar Long"       : "Pin Bar Short",
    "BB Bounce":      side === "BUY" ? "BB Bounce Long"     : "BB Bounce Short",
    "BB Squeeze":     side === "BUY" ? "BB Squeeze Long"    : "BB Squeeze Short",
    "Stoch Cross":    side === "BUY" ? "Stoch Cross Long"   : "Stoch Cross Short",
    "Stoch":          side === "BUY" ? "Stoch OB Long"      : "Stoch OB Short",
    "Donchian":       side === "BUY" ? "Donchian Long"      : "Donchian Short",
    "Supertrend":     side === "BUY" ? "Supertrend Long"    : "Supertrend Short",
    "EMA50 Bounce":   side === "BUY" ? "EMA50 Bounce Long"  : "EMA50 Bounce Short",
    "RSI Revert":     side === "BUY" ? "RSI Revert Long"    : "RSI Revert Short",
    "Momentum":       side === "BUY" ? "Momentum Long"      : "Momentum Short",
    "EMA Cross":      side === "BUY" ? "EMA Scalp Long"     : "EMA Scalp Short",
    "HA Trend":       side === "BUY" ? "HA Trend Long"      : "HA Trend Short",
    "CCI":            side === "BUY" ? "CCI Revert Long"    : "CCI Revert Short",
    "Williams %R":    side === "BUY" ? "WillR Long"         : "WillR Short",
    "ADX":            side === "BUY" ? "ADX Trend Long"     : "ADX Trend Short",
    "ADX Cross":      side === "BUY" ? "ADX Cross Long"     : "ADX Cross Short",
    "PSAR":           side === "BUY" ? "PSAR Long"          : "PSAR Short",
    "PSAR Flip":      side === "BUY" ? "PSAR Flip Long"     : "PSAR Flip Short",
    "RSI Divergence": side === "BUY" ? "RSI Div Long"       : "RSI Div Short",
    "MSB":            side === "BUY" ? "MSB Long"           : "MSB Short",
    "EMA Ribbon":     side === "BUY" ? "Ribbon Long"        : "Ribbon Short",
    "VWAP Retest":    side === "BUY" ? "VWAP Retest Long"   : "VWAP Retest Short",
    "Ichimoku":       side === "BUY" ? "Ichimoku Long"      : "Ichimoku Short",
    "Ichimoku TK":    side === "BUY" ? "TK Cross Long"      : "TK Cross Short",
    "Keltner":        side === "BUY" ? "Keltner Long"       : "Keltner Short",
    "KC Squeeze":     side === "BUY" ? "KC Squeeze Long"    : "KC Squeeze Short",
    "CMF":            side === "BUY" ? "CMF Long"           : "CMF Short",
    "OBV":            side === "BUY" ? "OBV Long"           : "OBV Short",
    "OBV Div":        side === "BUY" ? "OBV Div Long"       : "OBV Div Short",
    "ROC":            side === "BUY" ? "ROC Long"           : "ROC Short",
    "Elder Ray":      side === "BUY" ? "Elder Ray Long"     : "Elder Ray Short",
    "Fibonacci":      side === "BUY" ? "Fib Bounce Long"    : "Fib Bounce Short",
    "Morning Star":   side === "BUY" ? "Morning Star Long"  : "Morning Star Short",
    "Evening Star":   side === "BUY" ? "Evening Star Long"  : "Evening Star Short",
    "3 Soldiers":     side === "BUY" ? "3 Soldiers Long"    : "3 Soldiers Short",
    "3 Crows":        side === "BUY" ? "3 Crows Long"       : "3 Crows Short",
    "Tweezer":        side === "BUY" ? "Tweezer Bottom"     : "Tweezer Top",
    "Inside Bar":     side === "BUY" ? "IB Breakout Long"   : "IB Breakout Short",
    "Marubozu":       side === "BUY" ? "Marubozu Long"      : "Marubozu Short",
    "ATR Breakout":   side === "BUY" ? "ATR Break Long"     : "ATR Break Short",
    "MACD Zero":      side === "BUY" ? "MACD Zero Long"     : "MACD Zero Short",
    "Golden Cross":   side === "BUY" ? "Golden Cross Long"  : "Golden Cross Short",
    "Death Cross":    side === "BUY" ? "Death Cross Long"   : "Death Cross Short",
    "Consec Bull":    side === "BUY" ? "Consec Bull Long"   : "Consec Bull Short",
    "Consec Bear":    side === "BUY" ? "Consec Bear Long"   : "Consec Bear Short",
    "VWAP Band":      side === "BUY" ? "VWAP Band Long"     : "VWAP Band Short",
    "CMF+OBV":        side === "BUY" ? "CMF+OBV Long"       : "CMF+OBV Short",
    "Pivot":          side === "BUY" ? "Pivot Long"         : "Pivot Short",
    "Double Bottom":  side === "BUY" ? "Double Bottom Long" : "Double Bottom Short",
    "Double Top":     side === "BUY" ? "Double Top Long"    : "Double Top Short",
    "Order Block":    side === "BUY" ? "OB Long"            : "OB Short",
    "FVG":            side === "BUY" ? "FVG Long"           : "FVG Short",
    "Liq Sweep":      side === "BUY" ? "Liq Sweep Long"     : "Liq Sweep Short",
    "HMA":            side === "BUY" ? "HMA Long"           : "HMA Short",
    "HMA Cross":      side === "BUY" ? "HMA Cross Long"     : "HMA Cross Short",
    "TRIX":           side === "BUY" ? "TRIX Long"          : "TRIX Short",
    "Aroon":          side === "BUY" ? "Aroon Long"         : "Aroon Short",
    "Aroon Cross":    side === "BUY" ? "Aroon Cross Long"   : "Aroon Cross Short",
    "A/D Line":       side === "BUY" ? "A/D Long"           : "A/D Short",
    "A/D Div":        side === "BUY" ? "A/D Div Long"       : "A/D Div Short",
    "RVOL Spike":     side === "BUY" ? "RVOL Long"          : "RVOL Short",
    "LinReg":         side === "BUY" ? "LinReg Long"        : "LinReg Short",
    "NR7":            side === "BUY" ? "NR7 Long"           : "NR7 Short",
    "NR4":            side === "BUY" ? "NR4 Long"           : "NR4 Short",
    "SMC Accum":      side === "BUY" ? "SMC Accum Long"     : "SMC Accum Short",
    "SMC Dist":       side === "BUY" ? "SMC Dist Long"      : "SMC Dist Short",
    "CHoCH":          side === "BUY" ? "CHoCH Long"         : "CHoCH Short",
    "Vol Breakout":   side === "BUY" ? "Vol Break Long"     : "Vol Break Short",
    "Body Exp":       side === "BUY" ? "Body Exp Long"      : "Body Exp Short",
    "Velocity":       side === "BUY" ? "Velocity Long"      : "Velocity Short",
    "Osc Confluence": side === "BUY" ? "Osc Bull"           : "Osc Bear",
    "Rejection":      side === "BUY" ? "Rejection Long"     : "Rejection Short",
    "Climax":         side === "BUY" ? "Climax Reversal L"  : "Climax Reversal S",
    "EMA Fan":        side === "BUY" ? "EMA Fan Long"       : "EMA Fan Short",
    "HMA+ST":         side === "BUY" ? "HMA+ST Long"        : "HMA+ST Short",
  };
  const tag = side === "NEUTRAL"
    ? "No Edge"
    : primaryStrategy && tagMap[primaryStrategy]
      ? tagMap[primaryStrategy]
      : side === "BUY" ? (buy >= 12 ? "Aggressive Long" : "Scalp Long") : (sell >= 12 ? "Aggressive Short" : "Scalp Short");

  return { side, confidence, reasons, tag, scoreBuy: buy, scoreSell: sell, strategies };
}

function findSupportResistance(candles) {
  const lookback = candles.slice(-50);
  if (!lookback.length) return { support: null, resistance: null };
  const lows = lookback.map((c) => c.low).sort((a, b) => a - b);
  const highs = lookback.map((c) => c.high).sort((a, b) => a - b);
  return {
    support: lows[Math.floor(lows.length * 0.2)] || null,
    resistance: highs[Math.floor(highs.length * 0.8)] || null
  };
}

function buildTradeLevels(side, candles, support, resistance) {
  const entry = candles[candles.length - 1].close;
  const atr = calcATR(candles);
  const a = atr[atr.length - 1] || 55;
  if (side === "BUY") {
    let sl = Math.min(...candles.slice(-5).map((c) => c.low)) - a * 0.25;
    if (support && support < entry) sl = Math.max(sl, support - a * 0.1);
    if (sl >= entry) sl = entry - a;
    const r = entry - sl;
    const tp2Base = entry + r * 2.2;
    // Only use resistance as TP2 cap if it is strictly ABOVE entry
    const tp2 = (resistance && resistance > entry) ? Math.min(tp2Base, resistance) : tp2Base;
    return {
      entry,
      sl,
      tp1: entry + r * 1.3,
      tp2: Math.max(tp2, entry + r * 1.5), // never let TP2 fall below TP1
      tp3: entry + r * 3.0
    };
  }
  let sl = Math.max(...candles.slice(-5).map((c) => c.high)) + a * 0.25;
  if (resistance && resistance > entry) sl = Math.min(sl, resistance + a * 0.1);
  if (sl <= entry) sl = entry + a;
  const r = sl - entry;
  const tp2Base = entry - r * 2.2;
  // Only use support as TP2 floor if it is strictly BELOW entry
  const tp2 = (support && support < entry) ? Math.max(tp2Base, support) : tp2Base;
  return {
    entry,
    sl,
    tp1: entry - r * 1.3,
    tp2: Math.min(tp2, entry - r * 1.5), // never let TP2 rise above TP1
    tp3: entry - r * 3.0
  };
}

function beep(side) {
  try {
    const ctx = new (window.AudioContext || window.webkitAudioContext)();
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();
    osc.connect(gain);
    gain.connect(ctx.destination);
    osc.frequency.setValueAtTime(side === "BUY" ? 880 : 520, ctx.currentTime);
    gain.gain.setValueAtTime(0.1, ctx.currentTime);
    gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.25);
    osc.start();
    osc.stop(ctx.currentTime + 0.25);
  } catch (err) {
    // ignore audio failures
  }
}

function addFeed(setFeed, msg, type = "info") {
  setFeed((prev) => [{ msg, type, time: Date.now() }, ...prev].slice(0, 60));
}

function Chart({ candles, trades, height = 260 }) {
  const ref = useRef(null);
  const [width, setWidth] = useState(720);

  useEffect(() => {
    const ro = new ResizeObserver((entries) => {
      for (const e of entries) setWidth(e.contentRect.width);
    });
    if (ref.current?.parentElement) ro.observe(ref.current.parentElement);
    return () => ro.disconnect();
  }, []);

  useEffect(() => {
    const cv = ref.current;
    if (!cv || candles.length < 20) return;
    const ctx = cv.getContext("2d");
    const dpr = window.devicePixelRatio || 1;
    cv.width = width * dpr;
    cv.height = height * dpr;
    ctx.scale(dpr, dpr);
    ctx.clearRect(0, 0, width, height);

    const pad = { t: 8, r: 8, b: 14, l: 44 };
    const w = width - pad.l - pad.r;
    const h = height - pad.t - pad.b;
    const visible = candles.slice(-65);
    const prices = visible.flatMap((c) => [c.high, c.low]);
    const openTrades = trades.filter((t) => t.status === "OPEN");
    let min = Math.min(...prices);
    let max = Math.max(...prices);
    openTrades.forEach((t) => {
      min = Math.min(min, t.sl);
      max = Math.max(max, t.tp2);
    });
    min -= 20;
    max += 20;
    const range = max - min || 1;
    const candleW = w / visible.length;
    const toY = (p) => pad.t + h - ((p - min) / range) * h;

    for (let i = 0; i <= 4; i++) {
      const y = pad.t + (h / 4) * i;
      ctx.beginPath();
      ctx.moveTo(pad.l, y);
      ctx.lineTo(width - pad.r, y);
      ctx.strokeStyle = "rgba(0,0,0,0.08)";
      ctx.lineWidth = 0.6;
      ctx.stroke();
      ctx.fillStyle = "rgba(15,23,42,0.35)";
      ctx.font = "10px monospace";
      ctx.textAlign = "right";
      ctx.fillText((max - (range / 4) * i).toFixed(0), pad.l - 3, y + 3);
    }

    visible.forEach((c, i) => {
      const x = pad.l + i * candleW + candleW / 2;
      const col = c.close >= c.open ? "#0f9d76" : "#d33f49";
      ctx.strokeStyle = col;
      ctx.fillStyle = col;
      ctx.beginPath();
      ctx.moveTo(x, toY(c.high));
      ctx.lineTo(x, toY(c.low));
      ctx.stroke();
      const yOpen = toY(c.open);
      const yClose = toY(c.close);
      ctx.fillRect(
        x - candleW * 0.33,
        Math.min(yOpen, yClose),
        candleW * 0.66,
        Math.max(1, Math.abs(yClose - yOpen))
      );
    });

    const closes = visible.map((c) => c.close);
    const ema9 = calcEMA(closes, 9);
    const ema21 = calcEMA(closes, 21);
    [
      [ema9, "rgba(37,99,235,0.75)"],
      [ema21, "rgba(217,119,6,0.75)"]
    ].forEach(([arr, color]) => {
      ctx.beginPath();
      arr.forEach((v, i) => {
        const x = pad.l + i * candleW + candleW / 2;
        if (i === 0) ctx.moveTo(x, toY(v));
        else ctx.lineTo(x, toY(v));
      });
      ctx.strokeStyle = color;
      ctx.lineWidth = 1.15;
      ctx.stroke();
    });

    openTrades.forEach((t, idx) => {
      const colors = ["#2563eb", "#7c3aed", "#0891b2", "#059669", "#be185d"];
      const c = colors[idx % colors.length];
      const lines = [
        { p: t.entry, label: `E${idx + 1}`, col: c },
        { p: t.sl, label: "SL", col: "#dc2626" },
        { p: t.tp1, label: "TP1", col: "#10b981" },
        { p: t.tp2, label: "TP2", col: "#059669" }
      ];
      lines.forEach((ln) => {
        const y = toY(ln.p);
        ctx.beginPath();
        ctx.moveTo(pad.l, y);
        ctx.lineTo(width - pad.r, y);
        ctx.strokeStyle = `${ln.col}88`;
        ctx.setLineDash([5, 3]);
        ctx.lineWidth = 0.7;
        ctx.stroke();
        ctx.setLineDash([]);
        ctx.fillStyle = ln.col;
        ctx.font = "bold 9px monospace";
        ctx.textAlign = "right";
        ctx.fillText(ln.label, width - 10, y - 2);
      });
    });
  }, [candles, trades, width, height]);

  return <canvas ref={ref} style={{ width: "100%", height, borderRadius: 10, display: "block" }} />;
}

const styles = {
  page: {
    minHeight: "100vh",
    background: "linear-gradient(135deg,#f8fafc 0%,#eef2ff 100%)",
    color: "#0f172a",
    padding: "10px",
    fontFamily: "system-ui,Segoe UI,sans-serif"
  },
  card: {
    background: "#ffffff",
    border: "1px solid #e2e8f0",
    borderRadius: 12,
    boxShadow: "0 1px 2px rgba(15,23,42,0.06)",
    padding: 10,
    marginBottom: 8
  }
};

const Stat = ({ label, value, color = "#0f172a" }) => (
  <div style={{ background: "#f8fafc", borderRadius: 8, padding: "5px 6px", minWidth: 76, flex: "1 1 auto" }}>
    <div style={{ fontSize: 10, color: "#64748b", fontWeight: 600 }}>{label}</div>
    <div style={{ fontSize: 14, fontWeight: 800, color }}>{value}</div>
  </div>
);

function lsGet(key, fallback) {
  try { const v = localStorage.getItem(key); return v !== null ? JSON.parse(v) : fallback; }
  catch { return fallback; }
}
function lsSet(key, value) {
  try { localStorage.setItem(key, JSON.stringify(value)); } catch {}
}

export default function App() {
  const [candles, setCandles] = useState(() => initCandles(180));
  const [trades, setTrades] = useState(() => lsGet("btc_trades", []));
  const [feed, setFeed] = useState(() => lsGet("btc_feed", []));
  const [activeTab, setActiveTab] = useState("trade");
  const [exchange, setExchange] = useState(() => lsGet("btc_exchange", "binance"));
  const [connectionState, setConnectionState] = useState("connecting");
  const [connectionError, setConnectionError] = useState("");
  const [lastMarketEventAt, setLastMarketEventAt] = useState(null);
  const [tickerPrice, setTickerPrice] = useState(null);
  const [speed, setSpeed] = useState(() => lsGet("btc_speed", 1));
  const [isAutoOn, setIsAutoOn] = useState(() => lsGet("btc_autoOn", true));
  const [isSoundOn, setIsSoundOn] = useState(() => lsGet("btc_soundOn", true));
  const [scans, setScans] = useState(() => lsGet("btc_scans", 0));
  const [latestSignal, setLatestSignal] = useState(null);
  const [startedAt] = useState(() => lsGet("btc_startedAt", Date.now()));

  const candlesRef = useRef(candles);
  const tickerPriceRef = useRef(tickerPrice);

  useEffect(() => {
    candlesRef.current = candles;
  }, [candles]);

  useEffect(() => {
    tickerPriceRef.current = tickerPrice;
  }, [tickerPrice]);

  // ── One-time migration: fix corrupt trades from old bug ─────────────────
  useEffect(() => {
    if (lsGet("btc_migrated_v3", false)) return;
    setTrades(prev => {
      const fixed = prev.map(t => {
        const updated = { ...t };
        // Fix tag direction mismatch (e.g. "Double Top Short" on a BUY trade)
        if (t.side === "BUY" && t.tag && t.tag.endsWith("Short")) {
          updated.tag = t.tag.replace(/Short$/, "Long");
        } else if (t.side === "SELL" && t.tag && t.tag.endsWith("Long")) {
          updated.tag = t.tag.replace(/Long$/, "Short");
        }
        // Recalculate P&L from stored entry/exit prices
        if (t.exit != null && t.entry != null) {
          updated.pnl = t.side === "BUY" ? t.exit - t.entry : t.entry - t.exit;
        }
        // Fix status to match actual P&L direction
        if (updated.status === "WIN"  && (updated.pnl || 0) < 0) updated.status = "LOSS";
        if (updated.status === "LOSS" && (updated.pnl || 0) > 0) updated.status = "WIN";
        return updated;
      });
      lsSet("btc_migrated_v3", true);
      return fixed;
    });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // ── Persist state to localStorage ──────────────────────────────────────
  useEffect(() => { lsSet("btc_trades", trades); }, [trades]);
  useEffect(() => { lsSet("btc_feed", feed); }, [feed]);
  useEffect(() => { lsSet("btc_scans", scans); }, [scans]);
  useEffect(() => { lsSet("btc_exchange", exchange); }, [exchange]);
  useEffect(() => { lsSet("btc_speed", speed); }, [speed]);
  useEffect(() => { lsSet("btc_autoOn", isAutoOn); }, [isAutoOn]);
  useEffect(() => { lsSet("btc_soundOn", isSoundOn); }, [isSoundOn]);
  useEffect(() => { lsSet("btc_startedAt", startedAt); }, [startedAt]);

  const price = tickerPrice ?? (candles[candles.length - 1]?.close || 0);
  const prev = candles[candles.length - 2]?.close || price;
  const delta = price - prev;

  const closedTrades = trades.filter((t) => t.status !== "OPEN");
  const openTrades = trades.filter((t) => t.status === "OPEN");
  const wins = closedTrades.filter((t) => t.status === "WIN");
  const losses = closedTrades.filter((t) => t.status === "LOSS");
  const closedPnl = closedTrades.reduce((a, t) => a + (t.pnl || 0), 0);
  const unrealized = openTrades.reduce(
    (a, t) => a + (t.side === "BUY" ? price - t.entry : t.entry - price),
    0
  );
  const equity = START_BALANCE + closedPnl;
  const elapsedSec = Math.floor((Date.now() - startedAt) / 1000);
  const uptime = `${Math.floor(elapsedSec / 60)}:${String(elapsedSec % 60).padStart(2, "0")}`;
  const winRate = closedTrades.length ? (wins.length / closedTrades.length) * 100 : 0;
  const totalReturnPct = ((equity - START_BALANCE) / START_BALANCE) * 100;
  const lastTickAgeSec = lastMarketEventAt ? Math.floor((Date.now() - lastMarketEventAt) / 1000) : null;
  const connectionColor =
    connectionState === "live"
      ? "#059669"
      : connectionState === "connecting" ||
          connectionState === "reconnecting" ||
          connectionState === "bootstrapping"
        ? "#d97706"
        : "#dc2626";

  useEffect(() => {
    let active = true;
    setConnectionState("bootstrapping");
    setConnectionError("");

    fetchBootstrapCandles(exchange)
      .then((rows) => {
        if (!active) return;
        if (rows.length >= 40) {
          setCandles(rows.slice(-MAX_CANDLES));
          setTickerPrice(rows[rows.length - 1].close);
          setLastMarketEventAt(Date.now());
          addFeed(setFeed, `Loaded ${rows.length} bootstrap candles from ${exchange}`, "info");
        } else {
          throw new Error(`Insufficient bootstrap candles from ${exchange}`);
        }
      })
      .catch((err) => {
        if (!active) return;
        setConnectionError(err?.message || "Bootstrap failed");
        addFeed(
          setFeed,
          `Bootstrap fetch failed for ${exchange}. Keeping local fallback candles.`,
          "loss"
        );
      });

    return () => {
      active = false;
    };
  }, [exchange]);

  useEffect(() => {
    let ws = null;
    let pingId = null;
    let reconnectId = null;
    let closing = false;
    let attempt = 0;

    const cleanupTimers = () => {
      if (pingId) clearInterval(pingId);
      if (reconnectId) clearTimeout(reconnectId);
      pingId = null;
      reconnectId = null;
    };

    const connect = () => {
      if (closing) return;
      setConnectionState(attempt > 0 ? "reconnecting" : "connecting");
      setConnectionError("");

      const isBinance = exchange === "binance";
      const url = isBinance
        ? `wss://stream.binance.com:9443/stream?streams=${MARKET_SYMBOL.toLowerCase()}@ticker/${MARKET_SYMBOL.toLowerCase()}@kline_1m`
        : "wss://stream.bybit.com/v5/public/linear";

      ws = new WebSocket(url);

      ws.onopen = () => {
        if (closing) return;
        attempt = 0;
        setConnectionState("live");
        setConnectionError("");
        addFeed(setFeed, `Connected to ${exchange} live stream`, "info");

        if (!isBinance) {
          ws.send(
            JSON.stringify({
              op: "subscribe",
              args: [`tickers.${MARKET_SYMBOL}`, `kline.1.${MARKET_SYMBOL}`]
            })
          );
          pingId = setInterval(() => {
            if (ws && ws.readyState === WebSocket.OPEN) {
              ws.send(JSON.stringify({ op: "ping" }));
            }
          }, 20000);
        }
      };

      ws.onmessage = (event) => {
        let parsed;
        try {
          const payload = JSON.parse(event.data);
          parsed = exchange === "binance" ? parseBinanceSocketMessage(payload) : parseBybitSocketMessage(payload);
        } catch {
          return;
        }

        const ticker = ensureNumber(parsed?.ticker);
        if (ticker !== null) {
          setTickerPrice(ticker);
          setCandles((prev) => applyTickerToCandles(prev, ticker));
          setLastMarketEventAt(Date.now());
        }

        if (parsed?.candle) {
          setCandles((prev) => upsertCandle(prev, parsed.candle));
          setLastMarketEventAt(Date.now());
        }
      };

      ws.onerror = () => {
        if (closing) return;
        setConnectionState("error");
        setConnectionError(`${exchange} websocket error`);
      };

      ws.onclose = (event) => {
        cleanupTimers();
        if (closing) return;
        setConnectionState("reconnecting");
        setConnectionError(`${exchange} socket closed (${event.code})`);
        const delay = Math.min(RECONNECT_MAX_MS, 1200 * 1.6 ** attempt);
        attempt += 1;
        reconnectId = setTimeout(connect, delay);
      };
    };

    connect();
    return () => {
      closing = true;
      cleanupTimers();
      if (ws && ws.readyState <= WebSocket.OPEN) ws.close();
    };
  }, [exchange]);

  useEffect(() => {
    if (!isAutoOn) return undefined;
    const id = setInterval(() => {
      const liveCandles = candlesRef.current;
      const currentPrice = tickerPriceRef.current ?? liveCandles[liveCandles.length - 1]?.close;
      if (!liveCandles.length || !Number.isFinite(currentPrice)) return;

      setScans((v) => v + 1);

      setTrades((prevTrades) =>
        prevTrades.map((t) => {
          if (t.status !== "OPEN") return t;
          const isLong = t.side === "BUY";
          if ((isLong && currentPrice <= t.sl) || (!isLong && currentPrice >= t.sl)) {
            const pnl = isLong ? t.sl - t.entry : t.entry - t.sl;
            addFeed(setFeed, `Stop hit: ${t.tag} ${pnl >= 0 ? "+" : ""}$${pnl.toFixed(1)}`, "loss");
            return { ...t, status: "LOSS", exit: t.sl, exitTime: Date.now(), pnl };
          }
          if ((isLong && currentPrice >= t.tp2) || (!isLong && currentPrice <= t.tp2)) {
            const pnl = isLong ? t.tp2 - t.entry : t.entry - t.tp2;
            addFeed(setFeed, `TP2 hit: ${t.tag} +$${pnl.toFixed(1)}`, "win");
            return { ...t, status: "WIN", exit: t.tp2, exitTime: Date.now(), pnl };
          }
          if (!t.tp1Hit && ((isLong && currentPrice >= t.tp1) || (!isLong && currentPrice <= t.tp1))) {
            addFeed(setFeed, `TP1 touched: ${t.tag} stop moved to BE`, "tp1");
            return { ...t, tp1Hit: true, sl: t.entry };
          }
          return t;
        })
      );

      const signal = detectSignal(liveCandles);
      setLatestSignal(signal);
      if (!signal || signal.side === "NEUTRAL" || signal.confidence < 60) return;

      setTrades((prevTrades) => {
        const openCount = prevTrades.filter((t) => t.status === "OPEN").length;
        if (openCount >= MAX_OPEN_TRADES) return prevTrades;

        const openTagSet = new Set(prevTrades.filter((t) => t.status === "OPEN").map((t) => t.tag));
        const recentlyClosedTagSet = new Set(
          prevTrades
            .filter((t) => t.status !== "OPEN" && t.exitTime && Date.now() - t.exitTime < RECENT_COOLDOWN_MS)
            .map((t) => t.tag)
        );
        if (openTagSet.has(signal.tag) || recentlyClosedTagSet.has(signal.tag)) return prevTrades;

        const bySide = prevTrades.filter((t) => t.status === "OPEN" && t.side === signal.side).length;
        if (bySide >= 3) return prevTrades;

        const sr = findSupportResistance(liveCandles);
        const lv = buildTradeLevels(signal.side, liveCandles, sr.support, sr.resistance);
        const risk = Math.abs(lv.entry - lv.sl);
        const reward = Math.abs(lv.tp2 - lv.entry);
        const rr = risk > 0 ? reward / risk : 0;
        if (rr < 1.2) return prevTrades;

        const trade = {
          id: `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
          tag: signal.tag,
          side: signal.side,
          confidence: signal.confidence,
          score: signal.side === "BUY" ? signal.scoreBuy : signal.scoreSell,
          entry: lv.entry,
          sl: lv.sl,
          tp1: lv.tp1,
          tp2: lv.tp2,
          tp3: lv.tp3,
          rr: rr.toFixed(2),
          tp1Hit: false,
          status: "OPEN",
          openTime: Date.now(),
          pnl: 0
        };

        addFeed(
          setFeed,
          `${trade.side} ${trade.tag} @ $${trade.entry.toFixed(0)} | conf ${trade.confidence}%`,
          trade.side === "BUY" ? "buy" : "sell"
        );
        if (isSoundOn) beep(trade.side);
        return [...prevTrades, trade];
      });
    }, Math.round(1200 / speed));

    return () => clearInterval(id);
  }, [isAutoOn, speed, isSoundOn]);

  const streak = (() => {
    if (!closedTrades.length) return "0";
    let count = 1;
    let type = closedTrades[closedTrades.length - 1].status;
    for (let i = closedTrades.length - 2; i >= 0; i--) {
      if (closedTrades[i].status === type) count++;
      else break;
    }
    return `${count}${type === "WIN" ? "W" : "L"}`;
  })();

  const closeAllNow = () => {
    setTrades((prevTrades) =>
      prevTrades.map((t) => {
        if (t.status !== "OPEN") return t;
        const pnl = t.side === "BUY" ? price - t.entry : t.entry - price;
        addFeed(setFeed, `Manually closed: ${t.tag} ${pnl >= 0 ? "+" : ""}$${pnl.toFixed(1)}`, "close");
        return { ...t, status: pnl >= 0 ? "WIN" : "LOSS", exit: price, exitTime: Date.now(), pnl };
      })
    );
  };

  const clearHistory = () => {
    setTrades([]);
    setFeed([]);
    setScans(0);
    ["btc_trades", "btc_feed", "btc_scans", "btc_startedAt"].forEach((k) => localStorage.removeItem(k));
    lsSet("btc_startedAt", Date.now());
  };

  return (
    <div style={styles.page}>
      <div style={{ ...styles.card, marginBottom: 8 }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 6, flexWrap: "wrap" }}>
          <div>
            <div style={{ fontSize: 12, color: "#64748b", fontWeight: 700 }}>
              BTC Scalper Autonomous Trader | {exchange.toUpperCase()} {MARKET_SYMBOL} | Uptime {uptime} | Scans {scans}
            </div>
            <div style={{ display: "flex", alignItems: "baseline", gap: 8 }}>
              <div style={{ fontSize: 30, fontWeight: 900 }}>${price.toFixed(2)}</div>
              <div style={{ color: delta >= 0 ? "#059669" : "#dc2626", fontWeight: 800 }}>
                {delta >= 0 ? "+" : "-"}${Math.abs(delta).toFixed(2)}
              </div>
              <div style={{ color: totalReturnPct >= 0 ? "#059669" : "#dc2626", fontWeight: 700 }}>
                Equity ${equity.toFixed(0)} ({totalReturnPct.toFixed(2)}%)
              </div>
            </div>
            <div style={{ display: "flex", gap: 6, marginTop: 3, flexWrap: "wrap" }}>
              <span
                style={{
                  fontSize: 11,
                  fontWeight: 800,
                  color: "#fff",
                  background: connectionColor,
                  borderRadius: 999,
                  padding: "2px 8px"
                }}
              >
                {connectionState.toUpperCase()}
              </span>
              <span
                style={{
                  fontSize: 11,
                  fontWeight: 700,
                  color: "#334155",
                  background: "#e2e8f0",
                  borderRadius: 999,
                  padding: "2px 8px"
                }}
              >
                Last Tick {lastTickAgeSec === null ? "—" : `${lastTickAgeSec}s ago`}
              </span>
              {!!connectionError && (
                <span
                  style={{
                    fontSize: 11,
                    fontWeight: 700,
                    color: "#7f1d1d",
                    background: "#fee2e2",
                    borderRadius: 999,
                    padding: "2px 8px",
                    maxWidth: 330,
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap"
                  }}
                  title={connectionError}
                >
                  {connectionError}
                </span>
              )}
            </div>
          </div>

          <div style={{ display: "flex", gap: 6, alignItems: "center", flexWrap: "wrap" }}>
            {["binance", "bybit"].map((name) => (
              <button
                key={name}
                onClick={() => setExchange(name)}
                style={{
                  border: "1px solid #cbd5e1",
                  borderRadius: 8,
                  padding: "4px 8px",
                  background: exchange === name ? "#111827" : "#fff",
                  color: exchange === name ? "#fff" : "#334155",
                  fontWeight: 800,
                  cursor: "pointer"
                }}
              >
                {name === "binance" ? "Binance" : "Bybit"}
              </button>
            ))}
            {[1, 2, 5].map((v) => (
              <button
                key={v}
                onClick={() => setSpeed(v)}
                style={{
                  border: "1px solid #cbd5e1",
                  borderRadius: 8,
                  padding: "4px 8px",
                  background: speed === v ? "#0f172a" : "#fff",
                  color: speed === v ? "#fff" : "#334155",
                  fontWeight: 700,
                  cursor: "pointer"
                }}
              >
                {v}x
              </button>
            ))}
            <button
              onClick={() => setIsSoundOn((v) => !v)}
              style={{
                border: "1px solid #cbd5e1",
                borderRadius: 8,
                padding: "4px 8px",
                background: "#fff",
                fontWeight: 700,
                cursor: "pointer"
              }}
            >
              {isSoundOn ? "Sound On" : "Sound Off"}
            </button>
            <button
              onClick={() => setIsAutoOn((v) => !v)}
              style={{
                border: "none",
                borderRadius: 8,
                padding: "6px 10px",
                background: isAutoOn ? "#059669" : "#dc2626",
                color: "#fff",
                fontWeight: 800,
                cursor: "pointer"
              }}
            >
              {isAutoOn ? "AUTO RUNNING" : "AUTO STOPPED"}
            </button>
            {!!openTrades.length && (
              <button
                onClick={closeAllNow}
                style={{
                  border: "1px solid #fecaca",
                  borderRadius: 8,
                  padding: "6px 10px",
                  background: "#fff1f2",
                  color: "#b91c1c",
                  fontWeight: 800,
                  cursor: "pointer"
                }}
              >
                Close All
              </button>
            )}
            <button
              onClick={() => { if (window.confirm("Clear all trade history and feed? This cannot be undone.")) clearHistory(); }}
              style={{
                border: "1px solid #e2e8f0",
                borderRadius: 8,
                padding: "6px 10px",
                background: "#f8fafc",
                color: "#64748b",
                fontWeight: 700,
                cursor: "pointer",
                fontSize: 12
              }}
            >
              Clear History
            </button>
          </div>
        </div>
      </div>

      <div style={{ display: "flex", gap: 6, marginBottom: 8, flexWrap: "wrap" }}>
        <Stat label="Closed P&L" value={`${closedPnl >= 0 ? "+" : ""}$${closedPnl.toFixed(1)}`} color={closedPnl >= 0 ? "#059669" : "#dc2626"} />
        <Stat label="Unrealized" value={`${unrealized >= 0 ? "+" : ""}$${unrealized.toFixed(1)}`} color={unrealized >= 0 ? "#059669" : "#dc2626"} />
        <Stat label="Win Rate" value={`${winRate.toFixed(0)}%`} color={winRate >= 50 ? "#059669" : "#dc2626"} />
        <Stat label="Open Trades" value={`${openTrades.length}/${MAX_OPEN_TRADES}`} color="#1d4ed8" />
        <Stat label="Streak" value={streak} color="#7c3aed" />
      </div>

      <div style={{ display: "flex", gap: 6, marginBottom: 8 }}>
        {[
          ["trade", "Trade"],
          ["stats", "Stats"],
          ["history", `History (${closedTrades.length})`],
          ["feed", "Feed"]
        ].map(([k, lbl]) => (
          <button
            key={k}
            onClick={() => setActiveTab(k)}
            style={{
              border: "none",
              borderRadius: 999,
              padding: "6px 12px",
              background: activeTab === k ? "#0f172a" : "#e2e8f0",
              color: activeTab === k ? "#fff" : "#334155",
              fontWeight: 800,
              cursor: "pointer"
            }}
          >
            {lbl}
          </button>
        ))}
      </div>

      {activeTab === "trade" && (
        <>
          <div style={styles.card}>
            <div style={{ fontSize: 12, fontWeight: 700, color: "#64748b", marginBottom: 4 }}>
              Market Chart (EMA + Active Trade Levels)
            </div>
            <Chart candles={candles} trades={trades} height={240} />
          </div>
          {latestSignal && (
            <div style={styles.card}>
              <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                <div style={{ fontWeight: 900, fontSize: 16, color: latestSignal.side === "BUY" ? "#059669" : latestSignal.side === "SELL" ? "#dc2626" : "#334155" }}>
                  {latestSignal.side === "BUY" ? "▲" : latestSignal.side === "SELL" ? "▼" : "–"} {latestSignal.tag}
                </div>
                <span style={{ fontSize: 12, fontWeight: 800, background: latestSignal.confidence >= 75 ? "#dcfce7" : latestSignal.confidence >= 60 ? "#fef9c3" : "#fee2e2", color: latestSignal.confidence >= 75 ? "#166534" : latestSignal.confidence >= 60 ? "#713f12" : "#991b1b", borderRadius: 999, padding: "2px 10px" }}>
                  {latestSignal.confidence}% conf
                </span>
                <span style={{ fontSize: 12, color: "#64748b" }}>B:{latestSignal.scoreBuy} S:{latestSignal.scoreSell}</span>
              </div>
              {latestSignal.strategies && latestSignal.strategies.length > 0 && (
                <div style={{ marginTop: 4, display: "flex", gap: 4, flexWrap: "wrap" }}>
                  {latestSignal.strategies.map((s, i) => (
                    <span key={i} style={{ fontSize: 11, background: "#0f172a", color: "#e2e8f0", borderRadius: 999, padding: "2px 8px", fontWeight: 700 }}>
                      {s}
                    </span>
                  ))}
                </div>
              )}
              <div style={{ marginTop: 5, display: "flex", gap: 4, flexWrap: "wrap" }}>
                {latestSignal.reasons.slice(0, 8).map((r, i) => (
                  <span key={i} style={{ fontSize: 11, background: "#eef2ff", color: "#3730a3", borderRadius: 999, padding: "2px 8px", fontWeight: 700 }}>
                    {r}
                  </span>
                ))}
              </div>
            </div>
          )}
          {!!openTrades.length && (
            <div style={styles.card}>
              <div style={{ fontSize: 12, fontWeight: 800, color: "#475569", marginBottom: 6 }}>Active Positions ({openTrades.length})</div>
              {openTrades.map((t) => {
                const floating = t.side === "BUY" ? price - t.entry : t.entry - price;
                const durSec = Math.floor((Date.now() - t.openTime) / 1000);
                return (
                  <div key={t.id} style={{ borderBottom: "1px solid #f1f5f9", padding: "5px 0" }}>
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 6 }}>
                      <div style={{ fontWeight: 800, color: t.side === "BUY" ? "#059669" : "#dc2626" }}>
                        {t.side} {t.tag}
                      </div>
                      <div style={{ fontWeight: 800, color: floating >= 0 ? "#059669" : "#dc2626" }}>
                        {floating >= 0 ? "+" : ""}${floating.toFixed(2)}
                      </div>
                    </div>
                    <div style={{ fontSize: 12, color: "#64748b" }}>
                      E: ${t.entry.toFixed(0)} | SL: ${t.sl.toFixed(0)} {t.tp1Hit ? "(BE)" : ""} | TP1: ${t.tp1.toFixed(0)} | TP2: ${t.tp2.toFixed(0)} | RR: {t.rr} | Conf: {t.confidence}% | {durSec}s
                    </div>
                  </div>
                );
              })}
            </div>
          )}

          <div style={styles.card}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
              <div style={{ fontSize: 12, fontWeight: 800, color: "#475569" }}>
                Closed Trades ({closedTrades.length})
              </div>
              <div style={{ display: "flex", gap: 10, fontSize: 12, fontWeight: 700 }}>
                <span style={{ color: "#059669" }}>{wins.length}W</span>
                <span style={{ color: "#dc2626" }}>{losses.length}L</span>
                <span style={{ color: closedPnl >= 0 ? "#059669" : "#dc2626" }}>
                  {closedPnl >= 0 ? "+" : ""}${closedPnl.toFixed(2)}
                </span>
              </div>
            </div>
            {!closedTrades.length ? (
              <div style={{ color: "#94a3b8", fontSize: 12, padding: "8px 0" }}>No closed trades yet.</div>
            ) : (
              <div style={{ maxHeight: 320, overflowY: "auto" }}>
                {[...closedTrades].reverse().map((t, i) => {
                  const isWin = t.status === "WIN";
                  const durSec = t.exitTime && t.openTime ? Math.floor((t.exitTime - t.openTime) / 1000) : null;
                  const durStr = durSec !== null ? (durSec >= 60 ? `${Math.floor(durSec / 60)}m ${durSec % 60}s` : `${durSec}s`) : "—";
                  return (
                    <div key={t.id} style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "5px 0", borderBottom: "1px solid #f1f5f9", gap: 6 }}>
                      <div style={{ display: "flex", flexDirection: "column", gap: 2, flex: 1, minWidth: 0 }}>
                        <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                          <span style={{ fontWeight: 800, fontSize: 11, background: isWin ? "#dcfce7" : "#fee2e2", color: isWin ? "#166534" : "#991b1b", borderRadius: 4, padding: "1px 6px" }}>
                            {t.status}
                          </span>
                          <span style={{ fontWeight: 700, color: t.side === "BUY" ? "#059669" : "#dc2626", fontSize: 12 }}>
                            {t.side}
                          </span>
                          <span style={{ fontWeight: 700, fontSize: 12, color: "#334155" }}>{t.tag}</span>
                        </div>
                        <div style={{ fontSize: 11, color: "#94a3b8" }}>
                          E: ${t.entry?.toFixed(0)} → ${t.exit?.toFixed(0) ?? "—"} | SL: ${t.sl?.toFixed(0)} | TP2: ${t.tp2?.toFixed(0)} | RR: {t.rr}x | {t.confidence}% | {durStr}
                        </div>
                      </div>
                      <div style={{ fontWeight: 900, fontSize: 14, color: isWin ? "#059669" : "#dc2626", whiteSpace: "nowrap" }}>
                        {(t.pnl || 0) >= 0 ? "+" : ""}${(t.pnl || 0).toFixed(2)}
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </>
      )}

      {activeTab === "stats" && (
        <div style={styles.card}>
          <div style={{ fontSize: 14, fontWeight: 900, marginBottom: 8 }}>Performance Snapshot</div>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(3, minmax(0,1fr))", gap: 6 }}>
            <Stat label="Total Trades" value={closedTrades.length} />
            <Stat label="W / L" value={`${wins.length} / ${losses.length}`} color={wins.length >= losses.length ? "#059669" : "#dc2626"} />
            <Stat label="Avg P&L" value={closedTrades.length ? `${(closedPnl / closedTrades.length).toFixed(2)}` : "0.00"} color="#1d4ed8" />
            <Stat label="Best Win" value={wins.length ? `+$${Math.max(...wins.map((t) => t.pnl)).toFixed(2)}` : "-"} color="#059669" />
            <Stat label="Worst Loss" value={losses.length ? `$${Math.min(...losses.map((t) => t.pnl)).toFixed(2)}` : "-"} color="#dc2626" />
            <Stat label="Trades / Min" value={elapsedSec > 0 ? (closedTrades.length / (elapsedSec / 60)).toFixed(2) : "0.00"} color="#7c3aed" />
          </div>
        </div>
      )}

      {activeTab === "history" && (
        <div style={styles.card}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 10, flexWrap: "wrap", gap: 6 }}>
            <div style={{ fontSize: 14, fontWeight: 900 }}>Trade History — {closedTrades.length} Closed</div>
            <div style={{ display: "flex", gap: 8 }}>
              <span style={{ fontSize: 12, fontWeight: 700, color: "#059669" }}>
                {wins.length}W / {losses.length}L
              </span>
              <span style={{ fontSize: 12, fontWeight: 700, color: closedPnl >= 0 ? "#059669" : "#dc2626" }}>
                {closedPnl >= 0 ? "+" : ""}${closedPnl.toFixed(2)} total
              </span>
            </div>
          </div>

          {/* Strategy breakdown */}
          {closedTrades.length > 0 && (() => {
            const byTag = {};
            closedTrades.forEach((t) => {
              const key = t.tag.replace(/ Long$| Short$/, "");
              if (!byTag[key]) byTag[key] = { w: 0, l: 0, pnl: 0 };
              if (t.status === "WIN") byTag[key].w++;
              else byTag[key].l++;
              byTag[key].pnl += t.pnl || 0;
            });
            const sorted = Object.entries(byTag).sort((a, b) => b[1].pnl - a[1].pnl);
            return (
              <div style={{ marginBottom: 12 }}>
                <div style={{ fontSize: 11, fontWeight: 700, color: "#94a3b8", marginBottom: 4 }}>STRATEGY BREAKDOWN</div>
                <div style={{ display: "flex", flexWrap: "wrap", gap: 4 }}>
                  {sorted.map(([tag, s]) => (
                    <div key={tag} style={{ fontSize: 11, background: s.pnl >= 0 ? "#f0fdf4" : "#fff1f2", border: `1px solid ${s.pnl >= 0 ? "#bbf7d0" : "#fecaca"}`, borderRadius: 6, padding: "3px 8px", fontWeight: 700 }}>
                      <span style={{ color: "#334155" }}>{tag}</span>
                      <span style={{ color: "#64748b", marginLeft: 4 }}>{s.w}W/{s.l}L</span>
                      <span style={{ color: s.pnl >= 0 ? "#059669" : "#dc2626", marginLeft: 4 }}>
                        {s.pnl >= 0 ? "+" : ""}${s.pnl.toFixed(1)}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            );
          })()}

          {!closedTrades.length && (
            <div style={{ color: "#94a3b8", padding: "20px 0", textAlign: "center" }}>No closed trades yet. Waiting for first exits...</div>
          )}

          {closedTrades.length > 0 && (
            <div style={{ overflowX: "auto" }}>
              <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
                <thead>
                  <tr style={{ background: "#f8fafc", borderBottom: "2px solid #e2e8f0" }}>
                    {["#", "Strategy", "Side", "Entry", "Exit", "SL", "TP2", "RR", "Conf", "Duration", "P&L", "Result"].map((h) => (
                      <th key={h} style={{ padding: "6px 8px", textAlign: "left", fontWeight: 700, color: "#64748b", whiteSpace: "nowrap" }}>{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {[...closedTrades].reverse().map((t, i) => {
                    const durSec = t.exitTime && t.openTime ? Math.floor((t.exitTime - t.openTime) / 1000) : null;
                    const durStr = durSec !== null
                      ? durSec >= 60 ? `${Math.floor(durSec / 60)}m ${durSec % 60}s` : `${durSec}s`
                      : "—";
                    const isWin = t.status === "WIN";
                    return (
                      <tr key={t.id} style={{ borderBottom: "1px solid #f1f5f9", background: i % 2 === 0 ? "#fff" : "#fafafa" }}>
                        <td style={{ padding: "5px 8px", color: "#94a3b8", fontWeight: 600 }}>{closedTrades.length - i}</td>
                        <td style={{ padding: "5px 8px", fontWeight: 700, color: "#334155", whiteSpace: "nowrap" }}>{t.tag}</td>
                        <td style={{ padding: "5px 8px" }}>
                          <span style={{ fontWeight: 800, color: t.side === "BUY" ? "#059669" : "#dc2626", background: t.side === "BUY" ? "#f0fdf4" : "#fff1f2", borderRadius: 4, padding: "1px 6px" }}>
                            {t.side}
                          </span>
                        </td>
                        <td style={{ padding: "5px 8px", fontFamily: "monospace" }}>${t.entry?.toFixed(0)}</td>
                        <td style={{ padding: "5px 8px", fontFamily: "monospace" }}>${t.exit?.toFixed(0) ?? "—"}</td>
                        <td style={{ padding: "5px 8px", fontFamily: "monospace", color: "#dc2626" }}>${t.sl?.toFixed(0)}</td>
                        <td style={{ padding: "5px 8px", fontFamily: "monospace", color: "#059669" }}>${t.tp2?.toFixed(0)}</td>
                        <td style={{ padding: "5px 8px", color: "#7c3aed", fontWeight: 700 }}>{t.rr}x</td>
                        <td style={{ padding: "5px 8px", color: "#1d4ed8" }}>{t.confidence}%</td>
                        <td style={{ padding: "5px 8px", color: "#64748b", whiteSpace: "nowrap" }}>{durStr}</td>
                        <td style={{ padding: "5px 8px", fontWeight: 800, color: isWin ? "#059669" : "#dc2626" }}>
                          {(t.pnl || 0) >= 0 ? "+" : ""}${(t.pnl || 0).toFixed(2)}
                        </td>
                        <td style={{ padding: "5px 8px" }}>
                          <span style={{ fontWeight: 800, fontSize: 11, background: isWin ? "#dcfce7" : "#fee2e2", color: isWin ? "#166534" : "#991b1b", borderRadius: 4, padding: "2px 7px" }}>
                            {t.status}
                          </span>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {activeTab === "feed" && (
        <div style={styles.card}>
          <div style={{ fontSize: 14, fontWeight: 900, marginBottom: 8 }}>Activity Feed ({feed.length})</div>
          {!feed.length && <div style={{ color: "#94a3b8" }}>Waiting for events...</div>}
          {!!feed.length && (
            <div style={{ maxHeight: 360, overflowY: "auto" }}>
              {feed.map((f, i) => (
                <div key={`${f.time}-${i}`} style={{ padding: "4px 0", borderBottom: "1px solid #f8fafc", display: "flex", justifyContent: "space-between", alignItems: "center", gap: 6 }}>
                  <div
                    style={{
                      fontSize: 13,
                      color:
                        f.type === "win"
                          ? "#059669"
                          : f.type === "loss"
                            ? "#dc2626"
                            : f.type === "buy"
                              ? "#1d4ed8"
                              : f.type === "sell"
                                ? "#be185d"
                                : "#334155"
                    }}
                  >
                    {f.msg}
                  </div>
                  <div style={{ fontSize: 11, color: "#94a3b8" }}>{new Date(f.time).toLocaleTimeString()}</div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      <div style={{ textAlign: "center", fontSize: 11, color: "#64748b" }}>
        Live market mode via {exchange === "binance" ? "Binance Spot" : "Bybit Linear"} WebSocket ticker + 1m candles for {MARKET_SYMBOL}.
      </div>
    </div>
  );
}
