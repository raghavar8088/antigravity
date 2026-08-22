/**
 * Multi-horizon return maths over daily Delta bars.
 *
 * WHY CALENDAR DAYS HERE, WHEN THE EQUITY SCREENER COUNTS SESSIONS. The Stock
 * Screener this module is modelled on measures every horizon in TRADING
 * SESSIONS, because NSE closes for a dozen-odd holidays a year and a calendar
 * window silently shortens itself around every one — a Diwali week would rank
 * differently from an ordinary week for a reason that has nothing to do with
 * the stocks.
 *
 * Crypto perpetuals have no session, no weekend and no holiday. A calendar day
 * IS a trading day, every week is a seven-day week, and the correction that
 * makes the equity module correct would be a distortion here. So the horizons
 * are 1 / 7 / 30 / 180 calendar days, measured on 00:00 UTC bar boundaries.
 *
 * That is not a simplification of the equity logic; it is the same question
 * answered against a market that is genuinely open all the time. It is written
 * down because the difference is invisible in the output — both screeners show
 * a column called "This Week" — and a reader comparing them is entitled to
 * know that one counts five bars and the other counts seven.
 *
 * THE LAST BAR IS STILL FORMING. The most recent daily candle covers a UTC day
 * that has not ended, so its close is simply the price a few minutes ago. Every
 * horizon here is therefore measured from a COMPLETED past bar to the current
 * price, which `universe.ts` splices onto the series. Measuring to the forming
 * bar's own close would make "24h change" mean "change since some point today",
 * a window that shrinks toward zero every midnight UTC and resets.
 */

import type { Bar } from "./delta";

export type HorizonKey = "1d" | "1w" | "1m" | "6m";

/** Horizon -> number of daily bars back to measure from. */
export const HORIZONS: Record<HorizonKey, number> = { "1d": 1, "1w": 7, "1m": 30, "6m": 180 };

export const HORIZON_LABELS: Record<HorizonKey, string> = {
  "1d": "24 Hours",
  "1w": "7 Days",
  "1m": "30 Days",
  "6m": "6 Months",
};

export const HORIZON_ORDER: HorizonKey[] = ["1d", "1w", "1m", "6m"];

/**
 * Bars required before a horizon can be answered at all. A symbol with less
 * history reports null for that horizon — never a partial-window return
 * presented as if it covered the whole one, which would make a token listed
 * nine days ago look like the best 6-month performer on the venue.
 */
export const MIN_BARS_FOR: Record<HorizonKey, number> = {
  "1d": 2,
  "1w": 8,
  "1m": 31,
  "6m": 181,
};

/** How many days of history to request. 6m + headroom for the deepest maths. */
export const LOOKBACK_DAYS = 400;

export function round(v: number | null | undefined, dp = 2): number | null {
  if (v === null || v === undefined || !Number.isFinite(v)) return null;
  const f = 10 ** dp;
  return Math.round(v * f) / f;
}

/** % change over `days` daily bars, or null when the history is too short. */
export function pctReturn(closes: number[], days: number): number | null {
  if (closes.length < days + 1) return null;
  const then = closes[closes.length - 1 - days]!;
  const now = closes[closes.length - 1]!;
  if (then <= 0) return null;
  return (now / then - 1) * 100;
}

export function allHorizonReturns(closes: number[]): Record<HorizonKey, number | null> {
  return {
    "1d": pctReturn(closes, HORIZONS["1d"]),
    "1w": pctReturn(closes, HORIZONS["1w"]),
    "1m": pctReturn(closes, HORIZONS["1m"]),
    "6m": pctReturn(closes, HORIZONS["6m"]),
  };
}

/**
 * Share of the window's days that closed up.
 *
 * This is what separates a token that ground out +30% over a month from one
 * that gapped +30% on a listing announcement and went sideways. Both show the
 * same return; only one of them is a trend, and a board that ranks them
 * identically is misleading by construction.
 */
export function consistency(closes: number[], days: number): number | null {
  if (closes.length < days + 1) return null;
  const w = closes.slice(-(days + 1));
  let ups = 0;
  for (let i = 1; i < w.length; i++) if (w[i]! > w[i - 1]!) ups++;
  return (ups / (w.length - 1)) * 100;
}

/** Consecutive days closed higher, counting back from the last bar. */
export function upStreak(closes: number[]): number {
  let n = 0;
  for (let i = closes.length - 1; i > 0; i--) {
    if (closes[i]! > closes[i - 1]!) n++;
    else break;
  }
  return n;
}

/**
 * Return spread in percentage points against the benchmark.
 *
 * Deliberately a spread and not a ratio: a ratio explodes whenever the
 * benchmark return is near zero, which on a 24h horizon is most days.
 */
export function relativeStrength(assetRet: number | null, benchRet: number | null): number | null {
  if (assetRet === null || benchRet === null) return null;
  return assetRet - benchRet;
}

/** Where `value` sits in `population`, 0-100. Ties resolve to the midpoint. */
export function percentileRank(value: number | null, population: number[]): number | null {
  if (value === null || population.length === 0) return null;
  let below = 0;
  let equal = 0;
  for (const v of population) {
    if (v < value) below++;
    else if (v === value) equal++;
  }
  return ((below + equal / 2) / population.length) * 100;
}

export function sma(vals: number[], n: number): number | null {
  if (vals.length < n) return null;
  let s = 0;
  for (let i = vals.length - n; i < vals.length; i++) s += vals[i]!;
  return s / n;
}

/** EMA at the final bar, seeded with the first n-value SMA. */
export function emaLast(vals: number[], n: number): number | null {
  if (vals.length < n) return null;
  const k = 2 / (n + 1);
  let e = 0;
  for (let i = 0; i < n; i++) e += vals[i]!;
  e /= n;
  for (let i = n; i < vals.length; i++) e = vals[i]! * k + e * (1 - k);
  return e;
}

/**
 * Share of the last `window` days that closed above the n-period EMA.
 *
 * Deliberately a SHARE and not an unbroken streak. A 9 EMA is fast enough that
 * even a textbook uptrend clips it every week or two, so gating on an unbroken
 * month empties the screen permanently — a lesson this codebase has already
 * paid for once on the equity side.
 */
export function daysAboveEma(closes: number[], n = 9, window = 30): number | null {
  if (closes.length < n + window) return null;
  const k = 2 / (n + 1);
  let e = 0;
  for (let i = 0; i < n; i++) e += closes[i]!;
  e /= n;
  const series: number[] = [];
  for (let i = n; i < closes.length; i++) {
    e = closes[i]! * k + e * (1 - k);
    series.push(e);
  }
  const tail = closes.slice(n);
  const from = Math.max(0, tail.length - window);
  let hits = 0;
  let total = 0;
  for (let i = from; i < tail.length; i++) {
    total++;
    if (tail[i]! > series[i]!) hits++;
  }
  return total > 0 ? (hits / total) * 100 : null;
}

/**
 * Last day's volume against its own trailing average.
 *
 * The average EXCLUDES the last bar, so a huge day does not dilute the very
 * baseline it is being judged against.
 */
export function volumeRatio(bars: Bar[], window = 20): number | null {
  if (bars.length < window + 1) return null;
  const prior = bars.slice(-(window + 1), -1).filter((b) => b.volume > 0);
  if (prior.length === 0) return null;
  const avg = prior.reduce((s, b) => s + b.volume, 0) / prior.length;
  return avg > 0 ? bars[bars.length - 1]!.volume / avg : null;
}

/**
 * (volume over the last `days`, the volume those days would have carried at the
 * contract's own recent daily average).
 *
 * THE BASELINE IS A TRAILING AVERAGE, NOT THE PREVIOUS BLOCK, and the
 * difference matters twice over.
 *
 * Comparing a window against the single window immediately before it makes the
 * ratio hostage to one arbitrary period: a quiet Tuesday turns an ordinary
 * Wednesday into a "4x volume surge", and the same contract reads as 1.1x if
 * the comparison had started a day earlier. Averaging over `baselineDays`
 * removes that.
 *
 * It also makes this agree with `volumeRatio` at days = 1, which is what the
 * momentum board's Vol× column uses. Two tabs printing different multiples for
 * the same contract on the same day — 123x on one and 120x on the other —
 * gives a reader no way to know which to believe, and there is no reason for
 * them to differ.
 *
 * The baseline window sits strictly BEFORE the measured one, so a volume surge
 * is never compared against a baseline it is itself inflating.
 */
export function windowVolume(
  bars: Bar[],
  days: number,
  baselineDays = 20,
): [number | null, number | null] {
  if (bars.length < days + baselineDays) return [null, null];
  const recent = bars.slice(-days);
  const baseline = bars.slice(-(days + baselineDays), -days).filter((b) => b.volume > 0);
  if (baseline.length === 0) return [null, null];
  const perDay = baseline.reduce((s, b) => s + b.volume, 0) / baseline.length;
  return [recent.reduce((s, b) => s + b.volume, 0), perDay * days];
}

/**
 * Distance from the rolling high and low of `window` days, as percentages.
 *
 * `livePrice` is measured against, not the last bar's stored close. Every other
 * column in a row is quoted against the live traded price, and a "distance from
 * the 1-year high" computed off a stale close would be the one number in the
 * row describing a different moment — visibly so on a contract that has moved
 * several percent since the bar opened.
 */
export function highLowContext(
  bars: Bar[],
  window = 365,
  livePrice: number | null = null,
): { high: number | null; low: number | null; pctFromHigh: number | null; pctFromLow: number | null } {
  if (bars.length < 2) return { high: null, low: null, pctFromHigh: null, pctFromLow: null };
  const win = bars.slice(-window);
  // The live price can exceed the stored high — it is a price that has traded
  // since the bar was written — so it is folded into the extremes. Without
  // this, a contract making a new high right now reports a positive distance
  // from a high it has already passed.
  const price = livePrice && livePrice > 0 ? livePrice : bars[bars.length - 1]!.close;
  const hi = Math.max(...win.map((b) => b.high), price);
  const lo = Math.min(...win.map((b) => b.low), price);
  return {
    high: hi,
    low: lo,
    pctFromHigh: hi > 0 ? (price / hi - 1) * 100 : null,
    pctFromLow: lo > 0 ? (price / lo - 1) * 100 : null,
  };
}

/**
 * Wilder's ATR at the final bar — the volatility unit every stop here is sized
 * in.
 *
 * A fixed percentage stop is either too tight on a meme coin or too loose on a
 * stablecoin-adjacent pair, and one number cannot be both. On this venue that
 * spread is far wider than on NSE: daily ranges run from well under 1% to over
 * 30% inside the same universe.
 */
export function atr(bars: Bar[], n = 14): number | null {
  if (bars.length < n + 1) return null;
  const trs: number[] = [];
  for (let i = 1; i < bars.length; i++) {
    const cur = bars[i]!;
    const prev = bars[i - 1]!;
    trs.push(
      Math.max(cur.high - cur.low, Math.abs(cur.high - prev.close), Math.abs(cur.low - prev.close)),
    );
  }
  if (trs.length < n) return null;
  let a = 0;
  for (let i = 0; i < n; i++) a += trs[i]!;
  a /= n;
  for (let i = n; i < trs.length; i++) a = (a * (n - 1) + trs[i]!) / n;
  return a;
}

/** Annualised realised volatility from daily log returns, in percent. */
export function realisedVol(closes: number[], window = 30): number | null {
  if (closes.length < window + 1) return null;
  const w = closes.slice(-(window + 1));
  const rets: number[] = [];
  for (let i = 1; i < w.length; i++) {
    if (w[i - 1]! <= 0 || w[i]! <= 0) continue;
    rets.push(Math.log(w[i]! / w[i - 1]!));
  }
  if (rets.length < 5) return null;
  const mean = rets.reduce((s, r) => s + r, 0) / rets.length;
  const varr = rets.reduce((s, r) => s + (r - mean) ** 2, 0) / (rets.length - 1);
  return Math.sqrt(varr) * Math.sqrt(365) * 100;
}

/** Highest high over the `window` days BEFORE the last bar. */
export function donchianHigh(bars: Bar[], window: number): number | null {
  if (bars.length < window + 1) return null;
  return Math.max(...bars.slice(-(window + 1), -1).map((b) => b.high));
}

export function donchianLow(bars: Bar[], window: number): number | null {
  if (bars.length < window + 1) return null;
  return Math.min(...bars.slice(-(window + 1), -1).map((b) => b.low));
}

/**
 * The UTC date the last close broke above its own `window`-day high, or null.
 *
 * The high is measured over the bars BEFORE the breakout bar — including the
 * breakout bar's own high in its own resistance level means nothing ever breaks
 * out.
 */
export function donchianBreak(bars: Bar[], window: number): string | null {
  const prior = donchianHigh(bars, window);
  if (prior === null) return null;
  const last = bars[bars.length - 1]!;
  if (last.close > prior) return new Date(last.ts * 1000).toISOString().slice(0, 10);
  return null;
}

/**
 * The most recent CONFIRMED swing low — a bar whose low is the lowest within
 * `left` bars before and `right` after it.
 *
 * The right-hand lookahead is what makes it confirmed, and it means the last
 * `right` bars can never qualify. That is correct: a low is only a swing low
 * once price has turned away from it, and treating the newest bar as one is
 * how a stop gets placed under a level that is still falling.
 */
export function lastSwingLow(bars: Bar[], left = 3, right = 3): number | null {
  for (let i = bars.length - right - 1; i >= left; i--) {
    const low = bars[i]!.low;
    let ok = true;
    for (let j = i - left; j < i && ok; j++) if (bars[j]!.low < low) ok = false;
    for (let j = i + 1; j <= i + right && ok; j++) if (bars[j]!.low < low) ok = false;
    if (ok) return low;
  }
  return null;
}

/** Every confirmed swing high, oldest first. Same confirmation rule. */
export function swingHighs(bars: Bar[], left = 3, right = 3): number[] {
  const out: number[] = [];
  for (let i = left; i < bars.length - right; i++) {
    const high = bars[i]!.high;
    let ok = true;
    for (let j = i - left; j < i && ok; j++) if (bars[j]!.high > high) ok = false;
    for (let j = i + 1; j <= i + right && ok; j++) if (bars[j]!.high > high) ok = false;
    if (ok) out.push(high);
  }
  return out;
}

/**
 * The lowest confirmed swing high still above `price` — where the market last
 * turned back, and therefore the first level that has to give way.
 *
 * Returns null when price is above every prior swing high, which is the honest
 * answer for a token at new highs: there is no overhead level, so a target has
 * to come from somewhere else and say so.
 */
export function nearestResistanceAbove(bars: Bar[], price: number): number | null {
  const above = swingHighs(bars).filter((h) => h > price);
  return above.length > 0 ? Math.min(...above) : null;
}

/**
 * Pearson correlation of two daily return series over the last `window` days.
 *
 * Returns null unless both series cover the window: correlating a 20-day token
 * against a 400-day BTC series over whatever they happen to share produces a
 * number computed on a different sample for every row, which cannot be ranked
 * against itself.
 */
export function correlation(a: number[], b: number[], window = 30): number | null {
  const n = Math.min(a.length, b.length);
  if (n < window + 1) return null;
  const ra: number[] = [];
  const rb: number[] = [];
  for (let i = n - window; i < n; i++) {
    const a0 = a[a.length - n + i - 1];
    const a1 = a[a.length - n + i];
    const b0 = b[b.length - n + i - 1];
    const b1 = b[b.length - n + i];
    if (!a0 || !a1 || !b0 || !b1 || a0 <= 0 || b0 <= 0) continue;
    ra.push(a1 / a0 - 1);
    rb.push(b1 / b0 - 1);
  }
  if (ra.length < 10) return null;
  const ma = ra.reduce((s, v) => s + v, 0) / ra.length;
  const mb = rb.reduce((s, v) => s + v, 0) / rb.length;
  let num = 0;
  let da = 0;
  let db = 0;
  for (let i = 0; i < ra.length; i++) {
    const x = ra[i]! - ma;
    const y = rb[i]! - mb;
    num += x * y;
    da += x * x;
    db += y * y;
  }
  if (da <= 0 || db <= 0) return null;
  return num / Math.sqrt(da * db);
}

/**
 * Beta of `a` against `b` over `window` days — how much the asset moves for a
 * 1% move in the benchmark.
 *
 * Correlation says whether they move together; beta says by how much. A token
 * 0.9 correlated to BTC with a beta of 2.4 is not "the same trade as BTC", it
 * is a leveraged version of it, and only one of the two numbers says so.
 */
export function beta(a: number[], b: number[], window = 30): number | null {
  const n = Math.min(a.length, b.length);
  if (n < window + 1) return null;
  const ra: number[] = [];
  const rb: number[] = [];
  for (let i = n - window; i < n; i++) {
    const a0 = a[a.length - n + i - 1];
    const a1 = a[a.length - n + i];
    const b0 = b[b.length - n + i - 1];
    const b1 = b[b.length - n + i];
    if (!a0 || !a1 || !b0 || !b1 || a0 <= 0 || b0 <= 0) continue;
    ra.push(a1 / a0 - 1);
    rb.push(b1 / b0 - 1);
  }
  if (ra.length < 10) return null;
  const mb = rb.reduce((s, v) => s + v, 0) / rb.length;
  const ma = ra.reduce((s, v) => s + v, 0) / ra.length;
  let cov = 0;
  let varb = 0;
  for (let i = 0; i < ra.length; i++) {
    cov += (ra[i]! - ma) * (rb[i]! - mb);
    varb += (rb[i]! - mb) ** 2;
  }
  return varb > 0 ? cov / varb : null;
}

/**
 * How much of the universe can actually answer this horizon. Surfaced in the
 * UI so a thin backfill reads as "not enough history" rather than as "nothing
 * is trending".
 */
export function coverage(
  barsBySymbol: Map<string, Bar[]>,
  horizon: HorizonKey,
): { symbols: number; withHistory: number; pct: number; barsNeeded: number } {
  const need = MIN_BARS_FOR[horizon];
  const total = barsBySymbol.size;
  let have = 0;
  for (const b of barsBySymbol.values()) if (b.length >= need) have++;
  return {
    symbols: total,
    withHistory: have,
    pct: total > 0 ? round((have / total) * 100, 1)! : 0,
    barsNeeded: need,
  };
}
