/**
 * Chart-pattern, candlestick and structure detectors over daily and weekly bars.
 *
 * WHY A NEW LIBRARY HERE. The Go engine has pattern detectors and so does the
 * Python equity backend, but neither is reachable from a Next.js route: one is
 * a compiled binary behind an HTTP surface that does not expose them, the other
 * is a different deployment on a different host. What is portable is the
 * DISCIPLINE those two share, and that is what is reproduced below.
 *
 * DETECTORS FIRE ONLY ON A CLOSE THROUGH THE PATTERN'S OWN BOUNDARY. An
 * unbroken shape is not a signal. A double bottom whose neckline has not been
 * taken out is a drawing, and a screener that reported it as a completed
 * pattern would be manufacturing signals rather than finding them.
 *
 * TRIGGERED VERSUS FORMING — and how FORMING stays honest.
 *
 * A screener has a second legitimate job: showing shapes that are complete and
 * WAITING, so a trader can set an alert at the level instead of learning about
 * it a week late. The dishonest way to do that is to relax the detectors. This
 * does not. It PROBES: it appends one synthetic bar that closes just beyond the
 * recent range and re-runs the unmodified detector. If the pattern then fires,
 * the shape is genuinely complete and only the break is missing — and the probe
 * price is, by construction, the exact level the break would happen at. So a
 * FORMING row carries a real trigger level rather than a vague "watch this". If
 * the detector still does not fire, the shape is not there and nothing is
 * reported.
 *
 * The probe cannot distort the shape it is testing: `pivots()` uses a 3-bar
 * right-hand lookahead, so the last three bars can never be swing points, and
 * one appended bar therefore cannot invent a pivot or move an existing one.
 *
 * ONLY GEOMETRIC PATTERNS ARE PROBED. There is no "forming engulfing candle" —
 * it either happened or it did not — and probing one would manufacture a signal
 * rather than find one. Candlestick and structure templates are TRIGGERED-only.
 */

import type { Bar } from "./delta";

export type PatternFamily = "chart" | "candlestick" | "structure";

export const FAMILY_LABELS: Record<PatternFamily, string> = {
  chart: "Chart",
  candlestick: "Candlestick",
  structure: "Structure",
};

export type PatternSignal = {
  pattern: string;
  side: "BUY" | "SELL";
  entry: number;
  target: number;
  stoploss: number;
  confidence: number;
  rationale: string;
};

type Detector = (bars: Bar[], pv: Pivot[]) => PatternSignal | null;

type Template = {
  family: PatternFamily;
  label: string;
  minBars: number;
  detect: Detector;
};

// ── pivots ──────────────────────────────────────────────────────────────────

export type Pivot = { i: number; price: number; kind: "high" | "low" };

const PIVOT_LEFT = 3;
const PIVOT_RIGHT = 3;

/**
 * Confirmed swing points, oldest first.
 *
 * The right-hand lookahead is what makes a pivot confirmed, and it means the
 * last `PIVOT_RIGHT` bars can never be one. A high is only a swing high once
 * price has turned away from it; treating the newest bar as a pivot is how a
 * pattern gets drawn around a level that is still moving.
 */
export function pivots(bars: Bar[], left = PIVOT_LEFT, right = PIVOT_RIGHT): Pivot[] {
  const out: Pivot[] = [];
  for (let i = left; i < bars.length - right; i++) {
    const b = bars[i]!;
    let isHigh = true;
    let isLow = true;
    for (let j = i - left; j <= i + right; j++) {
      if (j === i) continue;
      if (bars[j]!.high > b.high) isHigh = false;
      if (bars[j]!.low < b.low) isLow = false;
      if (!isHigh && !isLow) break;
    }
    if (isHigh) out.push({ i, price: b.high, kind: "high" });
    else if (isLow) out.push({ i, price: b.low, kind: "low" });
  }
  return out;
}

// ── helpers ─────────────────────────────────────────────────────────────────

const last = <T>(a: T[]): T | undefined => a[a.length - 1];
const closes = (b: Bar[]) => b.map((x) => x.close);

function near(a: number, b: number, tolPct: number): boolean {
  if (a <= 0 || b <= 0) return false;
  return Math.abs(a - b) / ((a + b) / 2) <= tolPct;
}

/**
 * Touches required before a boundary counts as a trendline.
 *
 * THREE, NOT TWO, and this is the difference between a detector that finds
 * shapes and one that finds coincidences. Any two points define a line, so a
 * triangle validated on two highs and two lows fires on essentially any
 * consolidation — the first run of this scanner returned 501 triggered hits
 * across 220 contracts, with symmetrical triangles firing on more than half the
 * venue at confidence 0.8. That is not a market full of triangles; it is a
 * detector claiming a trendline from a pair of points.
 *
 * A third touch is what makes the boundary a claim the market can falsify, and
 * it is the standard the classical definitions have always used: two points to
 * draw the line, a third to confirm it.
 */
const MIN_TRENDLINE_TOUCHES = 3;

function highsOf(pv: Pivot[]): Pivot[] {
  return pv.filter((p) => p.kind === "high");
}
function lowsOf(pv: Pivot[]): Pivot[] {
  return pv.filter((p) => p.kind === "low");
}

/** Slope of a least-squares line through the pivot prices, per bar. */
function slope(points: Pivot[]): number | null {
  if (points.length < 2) return null;
  const n = points.length;
  const mx = points.reduce((s, p) => s + p.i, 0) / n;
  const my = points.reduce((s, p) => s + p.price, 0) / n;
  let num = 0;
  let den = 0;
  for (const p of points) {
    num += (p.i - mx) * (p.price - my);
    den += (p.i - mx) ** 2;
  }
  return den > 0 ? num / den : null;
}

/**
 * The least-squares line through `points`, evaluated at bar index `i`.
 *
 * WHY EXTRAPOLATION AND NOT THE LAST PIVOT'S PRICE. A wedge or a triangle
 * breaks when price closes through the boundary WHERE THE BOUNDARY IS NOW, and
 * the last confirmed pivot is at least three bars old. On a falling boundary
 * that stale level sits well above the real one, so almost any bounce clears
 * it; on a rising boundary it sits below, so almost nothing does.
 *
 * That is not a small bias, and it is not symmetric. Measured on the live
 * venue, the shortcut produced 86 triggered falling wedges against 3 rising
 * ones — a 29:1 split from two detectors that are mirror images of each other.
 * The shape was not in the market; it was in the arithmetic.
 */
function lineAt(points: Pivot[], i: number): number | null {
  const m = slope(points);
  if (m === null) return null;
  const mx = points.reduce((s, p) => s + p.i, 0) / points.length;
  const my = points.reduce((s, p) => s + p.price, 0) / points.length;
  return my + m * (i - mx);
}

/** Fit quality of a line through pivots, 0-1. Used to grade a shape's geometry. */
function fitQuality(points: Pivot[]): number {
  if (points.length < 3) return 0.5;
  const m = slope(points);
  if (m === null) return 0.5;
  const mx = points.reduce((s, p) => s + p.i, 0) / points.length;
  const my = points.reduce((s, p) => s + p.price, 0) / points.length;
  let ss = 0;
  let tot = 0;
  for (const p of points) {
    const pred = my + m * (p.i - mx);
    ss += (p.price - pred) ** 2;
    tot += (p.price - my) ** 2;
  }
  return tot > 0 ? Math.max(0, Math.min(1, 1 - ss / tot)) : 0.5;
}

/**
 * How much a pair of boundaries has narrowed, as end-width over start-width.
 *
 * THIS IS WHAT MAKES A WEDGE A WEDGE, and its absence was a real defect. The
 * first version of these detectors only checked the ORDER of the two slopes —
 * both falling, highs steeper than lows — which is satisfied by any mildly
 * uneven downward channel. On the live venue that fired 150 triggered falling
 * wedges across 220 contracts, better than one chart in three, in a week when
 * the market had simply been drifting down and bounced.
 *
 * A channel is not a wedge. The defining property is that the boundaries
 * CONVERGE: the distance between them at the end of the shape has to be
 * materially smaller than at the start. Slope ordering is a consequence of
 * that, not a substitute for it.
 *
 * Returns null when either line is undefined or the shape starts with a
 * non-positive width, which would make the ratio meaningless rather than large.
 */
function widthRatio(highs: Pivot[], lows: Pivot[]): number | null {
  const startAt = Math.min(highs[0]!.i, lows[0]!.i);
  const endAt = Math.max(highs[highs.length - 1]!.i, lows[lows.length - 1]!.i);
  if (endAt <= startAt) return null;
  const hStart = lineAt(highs, startAt);
  const lStart = lineAt(lows, startAt);
  const hEnd = lineAt(highs, endAt);
  const lEnd = lineAt(lows, endAt);
  if (hStart === null || lStart === null || hEnd === null || lEnd === null) return null;
  const startWidth = hStart - lStart;
  if (startWidth <= 0) return null;
  return (hEnd - lEnd) / startWidth;
}

/** A wedge or triangle must close to at most this share of its opening width. */
const MAX_CONVERGED_WIDTH_RATIO = 0.65;
/** A broadening formation must open to at least this multiple of its start. */
const MIN_BROADENED_WIDTH_RATIO = 1.5;
/**
 * Both boundaries must actually look like lines. Without this a "trendline"
 * through three scattered pivots is drawn wherever least squares puts it.
 */
const MIN_BOUNDARY_FIT = 0.5;

function atrOf(bars: Bar[], n = 14): number {
  if (bars.length < n + 1) {
    const b = last(bars)!;
    return Math.max(b.high - b.low, b.close * 0.01);
  }
  const trs: number[] = [];
  for (let i = 1; i < bars.length; i++) {
    const c = bars[i]!;
    const p = bars[i - 1]!;
    trs.push(Math.max(c.high - c.low, Math.abs(c.high - p.close), Math.abs(c.low - p.close)));
  }
  let a = trs.slice(0, n).reduce((s, v) => s + v, 0) / n;
  for (let i = n; i < trs.length; i++) a = (a * (n - 1) + trs[i]!) / n;
  return a;
}

function sig(
  pattern: string,
  side: "BUY" | "SELL",
  entry: number,
  target: number,
  stoploss: number,
  confidence: number,
  rationale: string,
): PatternSignal | null {
  if (!(entry > 0) || !(target > 0) || !(stoploss > 0)) return null;
  // A signal whose stop sits the wrong side of its entry, or whose target sits
  // behind it, is not a trade. Returned as null rather than emitted with the
  // levels swapped, which would look like a valid row and price a negative risk.
  if (side === "BUY" && !(stoploss < entry && target > entry)) return null;
  if (side === "SELL" && !(stoploss > entry && target < entry)) return null;
  return {
    pattern,
    side,
    entry: entry,
    target: target,
    stoploss: stoploss,
    confidence: Math.max(0.3, Math.min(0.95, confidence)),
    rationale,
  };
}

// ── chart patterns ──────────────────────────────────────────────────────────

/** Double top / double bottom — the "M" and the "W". */
const doubleTopBottom: Detector = (bars, pv) => {
  const b = last(bars)!;
  const lows = lowsOf(pv);
  const highs = highsOf(pv);

  if (lows.length >= 2) {
    const [l1, l2] = [lows[lows.length - 2]!, lows[lows.length - 1]!];
    if (near(l1.price, l2.price, 0.04) && l2.i - l1.i >= 5) {
      const between = highs.filter((h) => h.i > l1.i && h.i < l2.i);
      const neck = between.length ? Math.max(...between.map((h) => h.price)) : null;
      if (neck && b.close > neck) {
        const height = neck - (l1.price + l2.price) / 2;
        return sig(
          "Double Bottom",
          "BUY",
          b.close,
          neck + height,
          Math.min(l1.price, l2.price),
          0.6 + (near(l1.price, l2.price, 0.015) ? 0.15 : 0),
          `Two lows at ${l1.price.toPrecision(6)} and ${l2.price.toPrecision(6)} (within ` +
            `${((Math.abs(l1.price - l2.price) / l1.price) * 100).toFixed(1)}%), neckline ` +
            `${neck.toPrecision(6)} taken out on the close. Target is the ${height.toPrecision(4)} ` +
            `base height projected up from the break.`,
        );
      }
    }
  }

  if (highs.length >= 2) {
    const [h1, h2] = [highs[highs.length - 2]!, highs[highs.length - 1]!];
    if (near(h1.price, h2.price, 0.04) && h2.i - h1.i >= 5) {
      const between = lows.filter((l) => l.i > h1.i && l.i < h2.i);
      const neck = between.length ? Math.min(...between.map((l) => l.price)) : null;
      if (neck && b.close < neck) {
        const height = (h1.price + h2.price) / 2 - neck;
        return sig(
          "Double Top",
          "SELL",
          b.close,
          neck - height,
          Math.max(h1.price, h2.price),
          0.6 + (near(h1.price, h2.price, 0.015) ? 0.15 : 0),
          `Two highs at ${h1.price.toPrecision(6)} and ${h2.price.toPrecision(6)}, neckline ` +
            `${neck.toPrecision(6)} broken on the close. Target is the base height projected down.`,
        );
      }
    }
  }
  return null;
};

/** Head and shoulders, and its inverse. */
const headShoulders: Detector = (bars, pv) => {
  const b = last(bars)!;
  const highs = highsOf(pv);
  const lows = lowsOf(pv);

  if (highs.length >= 3) {
    const [ls, head, rs] = highs.slice(-3) as [Pivot, Pivot, Pivot];
    if (
      head.price > ls.price &&
      head.price > rs.price &&
      near(ls.price, rs.price, 0.06) &&
      head.i > ls.i &&
      rs.i > head.i
    ) {
      const troughs = lows.filter((l) => l.i > ls.i && l.i < rs.i);
      if (troughs.length >= 1) {
        const neck = troughs.reduce((s, t) => s + t.price, 0) / troughs.length;
        if (b.close < neck) {
          const height = head.price - neck;
          return sig(
            "Head & Shoulders",
            "SELL",
            b.close,
            neck - height,
            head.price,
            0.62 + (troughs.length >= 2 ? 0.1 : 0),
            `Left shoulder ${ls.price.toPrecision(6)}, head ${head.price.toPrecision(6)}, right ` +
              `shoulder ${rs.price.toPrecision(6)} with shoulders within ` +
              `${((Math.abs(ls.price - rs.price) / ls.price) * 100).toFixed(1)}%. Neckline ` +
              `${neck.toPrecision(6)} broken on the close; target is the head-to-neck height ` +
              `projected down.`,
          );
        }
      }
    }
  }

  if (lows.length >= 3) {
    const [ls, head, rs] = lows.slice(-3) as [Pivot, Pivot, Pivot];
    if (head.price < ls.price && head.price < rs.price && near(ls.price, rs.price, 0.06)) {
      const peaks = highs.filter((h) => h.i > ls.i && h.i < rs.i);
      if (peaks.length >= 1) {
        const neck = peaks.reduce((s, p) => s + p.price, 0) / peaks.length;
        if (b.close > neck) {
          const height = neck - head.price;
          return sig(
            "Inverse Head & Shoulders",
            "BUY",
            b.close,
            neck + height,
            head.price,
            0.62 + (peaks.length >= 2 ? 0.1 : 0),
            `Inverted shoulders at ${ls.price.toPrecision(6)} and ${rs.price.toPrecision(6)} around ` +
              `a head at ${head.price.toPrecision(6)}. Neckline ${neck.toPrecision(6)} taken out on ` +
              `the close; target is the head-to-neck height projected up.`,
          );
        }
      }
    }
  }
  return null;
};

/** Triple top / triple bottom. */
const tripleTopBottom: Detector = (bars, pv) => {
  const b = last(bars)!;
  const highs = highsOf(pv);
  const lows = lowsOf(pv);

  if (lows.length >= 3) {
    const t = lows.slice(-3) as [Pivot, Pivot, Pivot];
    const avg = (t[0].price + t[1].price + t[2].price) / 3;
    if (t.every((p) => near(p.price, avg, 0.05))) {
      const between = highs.filter((h) => h.i > t[0].i && h.i < t[2].i);
      const neck = between.length ? Math.max(...between.map((h) => h.price)) : null;
      if (neck && b.close > neck) {
        return sig(
          "Triple Bottom",
          "BUY",
          b.close,
          neck + (neck - avg),
          Math.min(...t.map((p) => p.price)),
          0.7,
          `Three lows within 5% of ${avg.toPrecision(6)} rejected the same level, and the ` +
            `${neck.toPrecision(6)} neckline has now closed through. Three tests carry more ` +
            `weight than two.`,
        );
      }
    }
  }

  if (highs.length >= 3) {
    const t = highs.slice(-3) as [Pivot, Pivot, Pivot];
    const avg = (t[0].price + t[1].price + t[2].price) / 3;
    if (t.every((p) => near(p.price, avg, 0.05))) {
      const between = lows.filter((l) => l.i > t[0].i && l.i < t[2].i);
      const neck = between.length ? Math.min(...between.map((l) => l.price)) : null;
      if (neck && b.close < neck) {
        return sig(
          "Triple Top",
          "SELL",
          b.close,
          neck - (avg - neck),
          Math.max(...t.map((p) => p.price)),
          0.7,
          `Three highs within 5% of ${avg.toPrecision(6)} failed at the same level, and the ` +
            `${neck.toPrecision(6)} neckline has now broken.`,
        );
      }
    }
  }
  return null;
};

/** Ascending triangle: flat resistance, rising lows. */
const ascendingTriangle: Detector = (bars, pv) => {
  const b = last(bars)!;
  const highs = highsOf(pv).slice(-4);
  const lows = lowsOf(pv).slice(-4);
  if (highs.length < MIN_TRENDLINE_TOUCHES || lows.length < MIN_TRENDLINE_TOUCHES) return null;

  const resist = highs.reduce((s, h) => s + h.price, 0) / highs.length;
  const flat = highs.every((h) => near(h.price, resist, 0.03));
  const ls = slope(lows);
  if (!flat || ls === null || ls <= 0) return null;
  if (!(lows[lows.length - 1]!.price > lows[0]!.price)) return null;
  if (!(b.close > resist)) return null;

  const height = resist - lows[0]!.price;
  return sig(
    "Ascending Triangle",
    "BUY",
    b.close,
    resist + height,
    lows[lows.length - 1]!.price,
    0.55 + fitQuality(lows) * 0.25,
    `Flat resistance at ${resist.toPrecision(6)} tested ${highs.length} times while lows rose ` +
      `from ${lows[0]!.price.toPrecision(6)} to ${lows[lows.length - 1]!.price.toPrecision(6)}. ` +
      `Resistance has now closed through; target is the ${height.toPrecision(4)} triangle height ` +
      `projected up.`,
  );
};

/** Descending triangle: flat support, falling highs. */
const descendingTriangle: Detector = (bars, pv) => {
  const b = last(bars)!;
  const lows = lowsOf(pv).slice(-4);
  const highs = highsOf(pv).slice(-4);
  if (lows.length < MIN_TRENDLINE_TOUCHES || highs.length < MIN_TRENDLINE_TOUCHES) return null;

  const support = lows.reduce((s, l) => s + l.price, 0) / lows.length;
  const flat = lows.every((l) => near(l.price, support, 0.03));
  const hs = slope(highs);
  if (!flat || hs === null || hs >= 0) return null;
  if (!(b.close < support)) return null;

  const height = highs[0]!.price - support;
  return sig(
    "Descending Triangle",
    "SELL",
    b.close,
    support - height,
    highs[highs.length - 1]!.price,
    0.55 + fitQuality(highs) * 0.25,
    `Flat support at ${support.toPrecision(6)} tested ${lows.length} times while highs fell. ` +
      `Support has now broken on the close; target is the triangle height projected down.`,
  );
};

/** Symmetrical triangle: converging highs and lows, break either way. */
const symmetricalTriangle: Detector = (bars, pv) => {
  const b = last(bars)!;
  const highs = highsOf(pv).slice(-4);
  const lows = lowsOf(pv).slice(-4);
  if (highs.length < MIN_TRENDLINE_TOUCHES || lows.length < MIN_TRENDLINE_TOUCHES) return null;

  const hs = slope(highs);
  const ls = slope(lows);
  if (hs === null || ls === null || hs >= 0 || ls <= 0) return null;
  const wr = widthRatio(highs, lows);
  if (wr === null || wr > MAX_CONVERGED_WIDTH_RATIO || wr <= 0) return null;
  if (fitQuality(highs) < MIN_BOUNDARY_FIT || fitQuality(lows) < MIN_BOUNDARY_FIT) return null;

  const at = bars.length - 1;
  const upper = lineAt(highs, at);
  const lower = lineAt(lows, at);
  if (upper === null || lower === null || upper <= lower) return null;
  const height = highs[0]!.price - lows[0]!.price;
  if (height <= 0) return null;

  if (b.close > upper) {
    return sig(
      "Symmetrical Triangle",
      "BUY",
      b.close,
      upper + height,
      lower,
      0.5 + (fitQuality(highs) + fitQuality(lows)) * 0.15,
      `Highs falling and lows rising into an apex; the upper boundary ${upper.toPrecision(6)} has ` +
        `resolved UP on the close. Target is the ${height.toPrecision(4)} opening width projected ` +
        `from the break.`,
    );
  }
  if (b.close < lower) {
    return sig(
      "Symmetrical Triangle",
      "SELL",
      b.close,
      lower - height,
      upper,
      0.5 + (fitQuality(highs) + fitQuality(lows)) * 0.15,
      `Converging boundaries resolved DOWN through ${lower.toPrecision(6)} on the close.`,
    );
  }
  return null;
};

/** Rising wedge — both boundaries up, lows steeper. Bearish. */
const risingWedge: Detector = (bars, pv) => {
  const b = last(bars)!;
  const highs = highsOf(pv).slice(-4);
  const lows = lowsOf(pv).slice(-4);
  if (highs.length < MIN_TRENDLINE_TOUCHES || lows.length < MIN_TRENDLINE_TOUCHES) return null;
  const hs = slope(highs);
  const ls = slope(lows);
  if (hs === null || ls === null || hs <= 0 || ls <= 0 || ls <= hs) return null;
  const wr = widthRatio(highs, lows);
  if (wr === null || wr > MAX_CONVERGED_WIDTH_RATIO || wr <= 0) return null;
  if (fitQuality(highs) < MIN_BOUNDARY_FIT || fitQuality(lows) < MIN_BOUNDARY_FIT) return null;

  const lower = lineAt(lows, bars.length - 1);
  if (lower === null || !(b.close < lower)) return null;
  const height = highs[highs.length - 1]!.price - lows[0]!.price;
  return sig(
    "Rising Wedge",
    "SELL",
    b.close,
    b.close - height,
    highs[highs.length - 1]!.price,
    0.55 + fitQuality(lows) * 0.2,
    `Both boundaries rising with the lows steeper than the highs — a narrowing advance running ` +
      `out of room. The lower boundary ${lower.toPrecision(6)} has broken on the close.`,
  );
};

/** Falling wedge — both boundaries down, highs steeper. Bullish. */
const fallingWedge: Detector = (bars, pv) => {
  const b = last(bars)!;
  const highs = highsOf(pv).slice(-4);
  const lows = lowsOf(pv).slice(-4);
  if (highs.length < MIN_TRENDLINE_TOUCHES || lows.length < MIN_TRENDLINE_TOUCHES) return null;
  const hs = slope(highs);
  const ls = slope(lows);
  if (hs === null || ls === null || hs >= 0 || ls >= 0 || hs >= ls) return null;
  const wr = widthRatio(highs, lows);
  if (wr === null || wr > MAX_CONVERGED_WIDTH_RATIO || wr <= 0) return null;
  if (fitQuality(highs) < MIN_BOUNDARY_FIT || fitQuality(lows) < MIN_BOUNDARY_FIT) return null;

  const upper = lineAt(highs, bars.length - 1);
  if (upper === null || !(b.close > upper)) return null;
  const height = highs[0]!.price - lows[lows.length - 1]!.price;
  return sig(
    "Falling Wedge",
    "BUY",
    b.close,
    b.close + height,
    lows[lows.length - 1]!.price,
    0.55 + fitQuality(highs) * 0.2,
    `Both boundaries falling with the highs steeper — a narrowing decline losing momentum. The ` +
      `upper boundary ${upper.toPrecision(6)} has closed through.`,
  );
};

/** Bull flag: a sharp pole, then a shallow counter-trend drift, then a break. */
const bullFlag: Detector = (bars) => {
  if (bars.length < 25) return null;
  const b = last(bars)!;
  const poleEnd = bars.length - 11;
  const poleStart = bars.length - 25;
  const poleLow = Math.min(...bars.slice(poleStart, poleEnd).map((x) => x.low));
  const poleHigh = Math.max(...bars.slice(poleStart, poleEnd).map((x) => x.high));
  const pole = poleHigh - poleLow;
  if (pole <= 0 || pole / poleLow < 0.08) return null;

  const flag = bars.slice(poleEnd);
  const flagHigh = Math.max(...flag.map((x) => x.high));
  const flagLow = Math.min(...flag.map((x) => x.low));
  // The consolidation must be a PAUSE, not a reversal: no more than half the
  // pole retraced. A drift that gives back most of the move is a failed rally.
  if (flagHigh - flagLow > pole * 0.5) return null;
  if (flagLow < poleHigh - pole * 0.6) return null;
  if (!(b.close > flagHigh)) return null;

  return sig(
    "Bull Flag",
    "BUY",
    b.close,
    b.close + pole,
    flagLow,
    0.6,
    `A ${((pole / poleLow) * 100).toFixed(1)}% pole into a consolidation that retraced only ` +
      `${(((flagHigh - flagLow) / pole) * 100).toFixed(0)}% of it, now broken upward through ` +
      `${flagHigh.toPrecision(6)}. Target is the pole length measured from the break.`,
  );
};

/** Bear flag: the mirror. */
const bearFlag: Detector = (bars) => {
  if (bars.length < 25) return null;
  const b = last(bars)!;
  const poleEnd = bars.length - 11;
  const poleStart = bars.length - 25;
  const poleHigh = Math.max(...bars.slice(poleStart, poleEnd).map((x) => x.high));
  const poleLow = Math.min(...bars.slice(poleStart, poleEnd).map((x) => x.low));
  const pole = poleHigh - poleLow;
  if (pole <= 0 || pole / poleHigh < 0.08) return null;

  const flag = bars.slice(poleEnd);
  const flagHigh = Math.max(...flag.map((x) => x.high));
  const flagLow = Math.min(...flag.map((x) => x.low));
  if (flagHigh - flagLow > pole * 0.5) return null;
  if (flagHigh > poleLow + pole * 0.6) return null;
  if (!(b.close < flagLow)) return null;

  return sig(
    "Bear Flag",
    "SELL",
    b.close,
    b.close - pole,
    flagHigh,
    0.6,
    `A ${((pole / poleHigh) * 100).toFixed(1)}% decline into a shallow drift, now broken down ` +
      `through ${flagLow.toPrecision(6)}. Target is the pole length measured from the break.`,
  );
};

/** Cup and handle: a rounded base, a shallow handle, then a break of the rim. */
const cupHandle: Detector = (bars) => {
  if (bars.length < 60) return null;
  const b = last(bars)!;
  const cup = bars.slice(-60, -8);
  const handle = bars.slice(-8);
  const leftRim = Math.max(...cup.slice(0, 10).map((x) => x.high));
  const rightRim = Math.max(...cup.slice(-10).map((x) => x.high));
  const bottom = Math.min(...cup.map((x) => x.low));
  const mid = Math.floor(cup.length / 2);
  const midLow = Math.min(...cup.slice(mid - 5, mid + 5).map((x) => x.low));

  // The low must sit in the MIDDLE of the cup — that is what makes it a cup
  // rather than a descending staircase that happens to end higher.
  if (!near(leftRim, rightRim, 0.06)) return null;
  if (midLow > bottom * 1.02) return null;
  const depth = leftRim - bottom;
  if (depth <= 0 || depth / leftRim < 0.12 || depth / leftRim > 0.6) return null;

  const handleHigh = Math.max(...handle.map((x) => x.high));
  const handleLow = Math.min(...handle.map((x) => x.low));
  // A handle deeper than a third of the cup is a second leg down, not a handle.
  if (leftRim - handleLow > depth * 0.35) return null;
  if (!(b.close > Math.max(handleHigh, rightRim))) return null;

  return sig(
    "Cup & Handle",
    "BUY",
    b.close,
    b.close + depth,
    handleLow,
    0.65,
    `Rims at ${leftRim.toPrecision(6)} and ${rightRim.toPrecision(6)} around a base ` +
      `${((depth / leftRim) * 100).toFixed(0)}% deep whose low sits in the middle of the shape, ` +
      `then a handle retracing under a third of it. The rim has now closed through; target is the ` +
      `cup depth projected up.`,
  );
};

/** Rounding bottom: a long, smooth base with no handle. */
const roundingBottom: Detector = (bars) => {
  if (bars.length < 50) return null;
  const b = last(bars)!;
  const win = bars.slice(-50);
  const cl = win.map((x) => x.close);
  const third = Math.floor(win.length / 3);
  const first = cl.slice(0, third).reduce((s, v) => s + v, 0) / third;
  const middle = cl.slice(third, 2 * third).reduce((s, v) => s + v, 0) / third;
  const lastT = cl.slice(2 * third).reduce((s, v) => s + v, 0) / (cl.length - 2 * third);

  if (!(middle < first * 0.94 && lastT > middle * 1.06)) return null;
  const rim = Math.max(first, lastT);
  if (!(b.close > rim)) return null;
  const depth = rim - Math.min(...win.map((x) => x.low));

  return sig(
    "Rounding Bottom",
    "BUY",
    b.close,
    b.close + depth,
    Math.min(...bars.slice(-12).map((x) => x.low)),
    0.55,
    `Fifty days shaped as a saucer — first third averaging ${first.toPrecision(6)}, middle ` +
      `${middle.toPrecision(6)}, last third ${lastT.toPrecision(6)} — with the rim now taken out. ` +
      `A base built slowly rather than a V.`,
  );
};

/** Broadening formation: diverging boundaries — expanding volatility. */
const broadening: Detector = (bars, pv) => {
  const b = last(bars)!;
  const highs = highsOf(pv).slice(-4);
  const lows = lowsOf(pv).slice(-4);
  if (highs.length < MIN_TRENDLINE_TOUCHES || lows.length < MIN_TRENDLINE_TOUCHES) return null;
  const hs = slope(highs);
  const ls = slope(lows);
  if (hs === null || ls === null || hs <= 0 || ls >= 0) return null;
  const wr = widthRatio(highs, lows);
  if (wr === null || wr < MIN_BROADENED_WIDTH_RATIO) return null;
  if (fitQuality(highs) < MIN_BOUNDARY_FIT || fitQuality(lows) < MIN_BOUNDARY_FIT) return null;

  const at = bars.length - 1;
  const upper = lineAt(highs, at);
  const lower = lineAt(lows, at);
  if (upper === null || lower === null) return null;
  const width = upper - lower;
  if (width <= 0) return null;

  if (b.close > upper) {
    return sig(
      "Broadening Formation",
      "BUY",
      b.close,
      b.close + width * 0.6,
      lower,
      0.45,
      `Highs rising and lows falling — volatility expanding rather than coiling. Resolved up ` +
        `through ${upper.toPrecision(6)}. Confidence is deliberately low: broadening patterns ` +
        `whipsaw more than they trend.`,
    );
  }
  if (b.close < lower) {
    return sig(
      "Broadening Formation",
      "SELL",
      b.close,
      b.close - width * 0.6,
      upper,
      0.45,
      `Diverging boundaries resolved down through ${lower.toPrecision(6)}. Low confidence by ` +
        `construction — this shape is as often noise as signal.`,
    );
  }
  return null;
};

// ── candlestick patterns ────────────────────────────────────────────────────

const engulfing: Detector = (bars) => {
  if (bars.length < 3) return null;
  const c = last(bars)!;
  const p = bars[bars.length - 2]!;
  const a = atrOf(bars);
  const body = Math.abs(c.close - c.open);
  const prevBody = Math.abs(p.close - p.open);
  if (body < a * 0.5 || prevBody <= 0) return null;

  if (c.close > c.open && p.close < p.open && c.close > p.open && c.open < p.close) {
    return sig(
      "Bullish Engulfing",
      "BUY",
      c.close,
      c.close + 2 * body,
      Math.min(c.low, p.low),
      0.5 + Math.min(0.25, body / (a * 4)),
      `Today's up candle opened below and closed above the whole of yesterday's down candle. ` +
        `Body is ${(body / a).toFixed(1)}x ATR, so this is a real reversal bar and not a doji.`,
    );
  }
  if (c.close < c.open && p.close > p.open && c.close < p.open && c.open > p.close) {
    return sig(
      "Bearish Engulfing",
      "SELL",
      c.close,
      c.close - 2 * body,
      Math.max(c.high, p.high),
      0.5 + Math.min(0.25, body / (a * 4)),
      `Today's down candle swallowed the whole of yesterday's up candle, body ` +
        `${(body / a).toFixed(1)}x ATR.`,
    );
  }
  return null;
};

const hammerStar: Detector = (bars) => {
  if (bars.length < 6) return null;
  const c = last(bars)!;
  const range = c.high - c.low;
  if (range <= 0) return null;
  const body = Math.abs(c.close - c.open);
  const upper = c.high - Math.max(c.close, c.open);
  const lower = Math.min(c.close, c.open) - c.low;
  const prior = bars.slice(-6, -1);
  const downTrend = prior[0]!.close > prior[prior.length - 1]!.close;
  const upTrend = prior[0]!.close < prior[prior.length - 1]!.close;

  if (downTrend && lower > body * 2 && upper < body && body / range < 0.4) {
    return sig(
      "Hammer",
      "BUY",
      c.close,
      c.close + range * 2,
      c.low,
      0.5,
      `A lower wick ${(lower / Math.max(body, range * 0.01)).toFixed(1)}x the body after five ` +
        `down days — sellers pushed it down and were rejected inside the day.`,
    );
  }
  if (upTrend && upper > body * 2 && lower < body && body / range < 0.4) {
    return sig(
      "Shooting Star",
      "SELL",
      c.close,
      c.close - range * 2,
      c.high,
      0.5,
      `A long upper wick after five up days — buyers pushed it up and could not hold it.`,
    );
  }
  return null;
};

const starPattern: Detector = (bars) => {
  if (bars.length < 4) return null;
  const [a, b, c] = bars.slice(-3) as [Bar, Bar, Bar];
  const bodyA = Math.abs(a.close - a.open);
  const bodyB = Math.abs(b.close - b.open);
  const bodyC = Math.abs(c.close - c.open);
  const atr = atrOf(bars);
  // The middle candle must be SMALL — that indecision is the pattern. Without
  // the size check this fires on any three-bar reversal.
  if (bodyB > bodyA * 0.5 || bodyB > atr * 0.4) return null;

  if (a.close < a.open && c.close > c.open && bodyC > bodyA * 0.6 && c.close > (a.open + a.close) / 2) {
    return sig(
      "Morning Star",
      "BUY",
      c.close,
      c.close + bodyC * 2,
      Math.min(a.low, b.low, c.low),
      0.58,
      `A big down candle, then indecision (body ${(bodyB / atr).toFixed(2)}x ATR), then a strong ` +
        `up candle closing back above the midpoint of the first. A three-bar reversal.`,
    );
  }
  if (a.close > a.open && c.close < c.open && bodyC > bodyA * 0.6 && c.close < (a.open + a.close) / 2) {
    return sig(
      "Evening Star",
      "SELL",
      c.close,
      c.close - bodyC * 2,
      Math.max(a.high, b.high, c.high),
      0.58,
      `A big up candle, a small indecisive one, then a strong down candle closing back below the ` +
        `first candle's midpoint.`,
    );
  }
  return null;
};

const harami: Detector = (bars) => {
  if (bars.length < 3) return null;
  const c = last(bars)!;
  const p = bars[bars.length - 2]!;
  const prevBody = Math.abs(p.close - p.open);
  const body = Math.abs(c.close - c.open);
  if (prevBody < atrOf(bars) * 0.8 || body > prevBody * 0.6) return null;
  const inside = Math.max(c.open, c.close) < Math.max(p.open, p.close) &&
    Math.min(c.open, c.close) > Math.min(p.open, p.close);
  if (!inside) return null;

  if (p.close < p.open) {
    return sig(
      "Bullish Harami",
      "BUY",
      c.close,
      c.close + prevBody,
      p.low,
      0.45,
      `A small candle entirely inside yesterday's large down candle — the selling stopped rather ` +
        `than reversed, which is why confidence is modest.`,
    );
  }
  return sig(
    "Bearish Harami",
    "SELL",
    c.close,
    c.close - prevBody,
    p.high,
    0.45,
    `A small candle entirely inside yesterday's large up candle — momentum paused.`,
  );
};

const marubozu: Detector = (bars) => {
  if (bars.length < 3) return null;
  const c = last(bars)!;
  const range = c.high - c.low;
  if (range <= 0) return null;
  const body = Math.abs(c.close - c.open);
  if (body / range < 0.9 || body < atrOf(bars)) return null;

  if (c.close > c.open) {
    return sig(
      "Bullish Marubozu",
      "BUY",
      c.close,
      c.close + body * 1.5,
      c.low,
      0.5,
      `A candle that is ${((body / range) * 100).toFixed(0)}% body — it opened at the low and ` +
        `closed at the high. One side controlled the entire session.`,
    );
  }
  return sig(
    "Bearish Marubozu",
    "SELL",
    c.close,
    c.close - body * 1.5,
    c.high,
    0.5,
    `A near-bodiless-wick down candle: opened at the high, closed at the low.`,
  );
};

const threeSoldiers: Detector = (bars) => {
  if (bars.length < 5) return null;
  const t = bars.slice(-3) as [Bar, Bar, Bar];
  const atr = atrOf(bars);
  const allUp = t.every((x) => x.close > x.open && x.close - x.open > atr * 0.4);
  const allDown = t.every((x) => x.close < x.open && x.open - x.close > atr * 0.4);
  const rising = t[1].close > t[0].close && t[2].close > t[1].close;
  const falling = t[1].close < t[0].close && t[2].close < t[1].close;

  if (allUp && rising) {
    const span = t[2].close - t[0].open;
    return sig(
      "Three White Soldiers",
      "BUY",
      t[2].close,
      t[2].close + span,
      t[0].low,
      0.6,
      `Three consecutive full-bodied up candles, each closing higher than the last. Sustained ` +
        `buying rather than a single spike.`,
    );
  }
  if (allDown && falling) {
    const span = t[0].open - t[2].close;
    return sig(
      "Three Black Crows",
      "SELL",
      t[2].close,
      t[2].close - span,
      t[0].high,
      0.6,
      `Three consecutive full-bodied down candles, each closing lower. Sustained selling.`,
    );
  }
  return null;
};

// ── structure ───────────────────────────────────────────────────────────────

const maStack: Detector = (bars) => {
  const cl = closes(bars);
  if (cl.length < 205) return null;
  const s = (n: number) => cl.slice(-n).reduce((a, v) => a + v, 0) / n;
  const s20 = s(20);
  const s50 = s(50);
  const s200 = s(200);
  const c = last(bars)!.close;
  const atr = atrOf(bars);

  if (c > s20 && s20 > s50 && s50 > s200) {
    return sig(
      "Bullish MA Stack",
      "BUY",
      c,
      c + atr * 4,
      s50,
      0.55,
      `Price > 20MA (${s20.toPrecision(6)}) > 50MA (${s50.toPrecision(6)}) > 200MA ` +
        `(${s200.toPrecision(6)}). Every timeframe is aligned the same way.`,
    );
  }
  if (c < s20 && s20 < s50 && s50 < s200) {
    return sig(
      "Bearish MA Stack",
      "SELL",
      c,
      c - atr * 4,
      s50,
      0.55,
      `Price < 20MA < 50MA < 200MA — a fully inverted stack.`,
    );
  }
  return null;
};

const cross: Detector = (bars) => {
  const cl = closes(bars);
  if (cl.length < 205) return null;
  const sAt = (n: number, back: number) =>
    cl.slice(cl.length - n - back, cl.length - back).reduce((a, v) => a + v, 0) / n;
  const now50 = sAt(50, 0);
  const now200 = sAt(200, 0);
  const then50 = sAt(50, 3);
  const then200 = sAt(200, 3);
  const c = last(bars)!.close;
  const atr = atrOf(bars);

  if (then50 <= then200 && now50 > now200) {
    return sig(
      "Golden Cross",
      "BUY",
      c,
      c + atr * 6,
      now200,
      0.5,
      `The 50-day average crossed above the 200-day within the last three days ` +
        `(${now50.toPrecision(6)} vs ${now200.toPrecision(6)}). A slow signal, reported for the ` +
        `regime it marks rather than for the entry.`,
    );
  }
  if (then50 >= then200 && now50 < now200) {
    return sig(
      "Death Cross",
      "SELL",
      c,
      c - atr * 6,
      now200,
      0.5,
      `The 50-day average crossed below the 200-day within the last three days.`,
    );
  }
  return null;
};

const insideBarBreak: Detector = (bars) => {
  if (bars.length < 4) return null;
  const c = last(bars)!;
  const inside = bars[bars.length - 2]!;
  const mother = bars[bars.length - 3]!;
  if (!(inside.high < mother.high && inside.low > mother.low)) return null;
  const range = mother.high - mother.low;
  if (range <= 0) return null;

  if (c.close > mother.high) {
    return sig(
      "Inside Bar Break",
      "BUY",
      c.close,
      c.close + range,
      inside.low,
      0.5,
      `A compressed inside day inside a ${range.toPrecision(4)} range, resolved UP through ` +
        `${mother.high.toPrecision(6)}. Contraction then expansion.`,
    );
  }
  if (c.close < mother.low) {
    return sig(
      "Inside Bar Break",
      "SELL",
      c.close,
      c.close - range,
      inside.high,
      0.5,
      `An inside day resolved DOWN through ${mother.low.toPrecision(6)}.`,
    );
  }
  return null;
};

const outsideBar: Detector = (bars) => {
  if (bars.length < 3) return null;
  const c = last(bars)!;
  const p = bars[bars.length - 2]!;
  if (!(c.high > p.high && c.low < p.low)) return null;
  const range = c.high - c.low;
  if (range < atrOf(bars) * 1.5) return null;

  if (c.close > (c.high + c.low) / 2) {
    return sig(
      "Outside Bar",
      "BUY",
      c.close,
      c.close + range,
      c.low,
      0.48,
      `Today took out both of yesterday's extremes on a range ${(range / atrOf(bars)).toFixed(1)}x ` +
        `ATR and closed in the upper half — both stops ran and buyers finished on top.`,
    );
  }
  return sig(
    "Outside Bar",
    "SELL",
    c.close,
    c.close - range,
    c.high,
    0.48,
    `An expansion bar engulfing yesterday's range that closed in the LOWER half.`,
  );
};

const rangeBreak: Detector = (bars) => {
  if (bars.length < 60) return null;
  const c = last(bars)!;
  const prior = bars.slice(-56, -1);
  const hi = Math.max(...prior.map((x) => x.high));
  const lo = Math.min(...prior.map((x) => x.low));
  const width = hi - lo;
  if (width <= 0) return null;

  if (c.close > hi) {
    return sig(
      "55-Day Range Break",
      "BUY",
      c.close,
      c.close + width * 0.5,
      hi - width * 0.15,
      0.55,
      `Closed above a 55-day high of ${hi.toPrecision(6)} — the Donchian break the trend-following ` +
        `literature is built on.`,
    );
  }
  if (c.close < lo) {
    return sig(
      "55-Day Range Break",
      "SELL",
      c.close,
      c.close - width * 0.5,
      lo + width * 0.15,
      0.55,
      `Closed below a 55-day low of ${lo.toPrecision(6)}.`,
    );
  }
  return null;
};

const volumeThrust: Detector = (bars) => {
  if (bars.length < 25) return null;
  const c = last(bars)!;
  const prior = bars.slice(-21, -1).filter((x) => x.volume > 0);
  if (prior.length < 10) return null;
  const avg = prior.reduce((s, x) => s + x.volume, 0) / prior.length;
  if (avg <= 0 || c.volume < avg * 3) return null;
  const range = c.high - c.low;
  if (range <= 0) return null;
  const atr = atrOf(bars);

  if (c.close > c.open && c.close > (c.high + c.low) / 2) {
    return sig(
      "Volume Thrust",
      "BUY",
      c.close,
      c.close + atr * 3,
      c.low,
      0.55,
      `Volume ${(c.volume / avg).toFixed(1)}x its 20-day average on an up close in the top half of ` +
        `the range. Participation and direction agree.`,
    );
  }
  if (c.close < c.open && c.close < (c.high + c.low) / 2) {
    return sig(
      "Volume Thrust",
      "SELL",
      c.close,
      c.close - atr * 3,
      c.high,
      0.55,
      `Volume ${(c.volume / avg).toFixed(1)}x average on a down close in the bottom half of the range.`,
    );
  }
  return null;
};

const higherHighsLows: Detector = (bars, pv) => {
  const highs = highsOf(pv).slice(-4);
  const lows = lowsOf(pv).slice(-4);
  if (highs.length < MIN_TRENDLINE_TOUCHES || lows.length < MIN_TRENDLINE_TOUCHES) return null;
  const c = last(bars)!;
  const atr = atrOf(bars);

  const hh = highs[highs.length - 1]!.price > highs[highs.length - 2]!.price;
  const hl = lows[lows.length - 1]!.price > lows[lows.length - 2]!.price;
  const lh = highs[highs.length - 1]!.price < highs[highs.length - 2]!.price;
  const ll = lows[lows.length - 1]!.price < lows[lows.length - 2]!.price;

  if (hh && hl && c.close > highs[highs.length - 1]!.price) {
    return sig(
      "Higher Highs & Higher Lows",
      "BUY",
      c.close,
      c.close + atr * 3,
      lows[lows.length - 1]!.price,
      0.55,
      `Swing structure is intact: last high ${highs[highs.length - 1]!.price.toPrecision(6)} above ` +
        `the previous, last low ${lows[lows.length - 1]!.price.toPrecision(6)} above the previous, ` +
        `and price has now taken out the last high.`,
    );
  }
  if (lh && ll && c.close < lows[lows.length - 1]!.price) {
    return sig(
      "Lower Highs & Lower Lows",
      "SELL",
      c.close,
      c.close - atr * 3,
      highs[highs.length - 1]!.price,
      0.55,
      `Swing structure is breaking down: lower high, lower low, and the last low is now gone.`,
    );
  }
  return null;
};

// ── catalogue ───────────────────────────────────────────────────────────────

export const TEMPLATES: Record<string, Template> = {
  double_top_bottom: { family: "chart", label: "Double Top / Bottom", minBars: 45, detect: doubleTopBottom },
  head_shoulders: { family: "chart", label: "Head & Shoulders", minBars: 55, detect: headShoulders },
  triple_top_bottom: { family: "chart", label: "Triple Top / Bottom", minBars: 60, detect: tripleTopBottom },
  ascending_triangle: { family: "chart", label: "Ascending Triangle", minBars: 45, detect: ascendingTriangle },
  descending_triangle: { family: "chart", label: "Descending Triangle", minBars: 45, detect: descendingTriangle },
  symmetrical_triangle: { family: "chart", label: "Symmetrical Triangle", minBars: 45, detect: symmetricalTriangle },
  rising_wedge: { family: "chart", label: "Rising Wedge", minBars: 45, detect: risingWedge },
  falling_wedge: { family: "chart", label: "Falling Wedge", minBars: 45, detect: fallingWedge },
  bull_flag: { family: "chart", label: "Bull Flag", minBars: 30, detect: bullFlag },
  bear_flag: { family: "chart", label: "Bear Flag", minBars: 30, detect: bearFlag },
  cup_handle: { family: "chart", label: "Cup & Handle", minBars: 70, detect: cupHandle },
  rounding_bottom: { family: "chart", label: "Rounding Bottom", minBars: 55, detect: roundingBottom },
  broadening: { family: "chart", label: "Broadening Formation", minBars: 45, detect: broadening },

  engulfing: { family: "candlestick", label: "Engulfing", minBars: 5, detect: engulfing },
  hammer_star: { family: "candlestick", label: "Hammer / Shooting Star", minBars: 8, detect: hammerStar },
  star: { family: "candlestick", label: "Morning / Evening Star", minBars: 6, detect: starPattern },
  harami: { family: "candlestick", label: "Harami", minBars: 5, detect: harami },
  marubozu: { family: "candlestick", label: "Marubozu", minBars: 5, detect: marubozu },
  three_soldiers: { family: "candlestick", label: "Three Soldiers / Crows", minBars: 6, detect: threeSoldiers },

  ma_stack: { family: "structure", label: "Moving-Average Stack", minBars: 205, detect: maStack },
  ma_cross: { family: "structure", label: "Golden / Death Cross", minBars: 205, detect: cross },
  inside_bar: { family: "structure", label: "Inside Bar Break", minBars: 6, detect: insideBarBreak },
  outside_bar: { family: "structure", label: "Outside Bar", minBars: 5, detect: outsideBar },
  range_break: { family: "structure", label: "55-Day Range Break", minBars: 60, detect: rangeBreak },
  volume_thrust: { family: "structure", label: "Volume Thrust", minBars: 25, detect: volumeThrust },
  swing_structure: { family: "structure", label: "Swing Structure", minBars: 30, detect: higherHighsLows },
};

/**
 * Never assert an exact catalogue size.
 *
 * An exact-count assert on a template catalogue once crash-looped an entire
 * backend in this codebase the day a colleague added seven templates. The
 * check that matters is that the catalogue is not EMPTY — that is the failure
 * that would silently return a blank patterns tab.
 */
export const TEMPLATE_KEYS = Object.keys(TEMPLATES);
if (TEMPLATE_KEYS.length < 10) {
  throw new Error("crypto screener pattern catalogue looks empty — check the template map");
}

/** Only geometric chart shapes are probed for a FORMING state. */
const PROBE_FAMILIES: ReadonlySet<PatternFamily> = new Set<PatternFamily>(["chart"]);
const PROBE_EDGE = 0.001;

export type PatternHit = {
  symbol: string;
  sector: string;
  pattern: string;
  template: string;
  family: PatternFamily;
  familyLabel: string;
  timeframe: "1d" | "1w";
  timeframeLabel: string;
  state: "TRIGGERED" | "FORMING";
  side: "BUY" | "SELL";
  direction: "bullish" | "bearish";
  entry: number;
  target: number;
  stoploss: number;
  triggerLevel: number | null;
  confidence: number;
  rationale: string;
  rewardRisk: number | null;
  asOf: string;
};

export const TIMEFRAME_LABELS = { "1d": "Daily", "1w": "Weekly" } as const;

/**
 * A synthetic bar that closes just beyond the extreme of the last `window`
 * bars.
 *
 * THE WINDOW MUST COVER THE PATTERN'S OWN STRUCTURE, which is why it is a
 * parameter rather than a constant. A double bottom's neckline is the peak
 * BETWEEN its two lows, and on a 70-bar shape that peak can sit thirty bars
 * back — probing only past the last 20 bars would clear a level the pattern
 * does not care about while leaving its real boundary untouched, so the shape
 * would never be found. Passing each template's own `minBars` probes every
 * pattern just past the structure it actually reads.
 */
function probeBar(bars: Bar[], direction: "up" | "down", window: number): Bar | null {
  if (bars.length < 5) return null;
  const span = bars.slice(-Math.min(window, bars.length));
  const l = last(bars)!;
  if (direction === "up") {
    const level = Math.max(...span.map((b) => b.high)) * (1 + PROBE_EDGE);
    return { ts: l.ts, open: l.close, high: level, low: Math.min(l.close, level * 0.995), close: level, volume: l.volume };
  }
  const level = Math.min(...span.map((b) => b.low)) * (1 - PROBE_EDGE);
  return { ts: l.ts, open: l.close, high: Math.max(l.close, level * 1.005), low: level, close: level, volume: l.volume };
}

function rr(entry: number, target: number, stop: number): number | null {
  const risk = Math.abs(entry - stop);
  return risk > 0 ? Math.round((Math.abs(target - entry) / risk) * 100) / 100 : null;
}

/** Every pattern hit for one symbol on one timeframe. */
export function scanSymbol(
  symbol: string,
  sector: string,
  bars: Bar[],
  timeframe: "1d" | "1w",
): PatternHit[] {
  const hits: PatternHit[] = [];
  if (bars.length < 30) return hits;

  const asOf = new Date(last(bars)!.ts * 1000).toISOString().slice(0, 10);
  const pv = pivots(bars);
  const probeCache = new Map<string, Bar | null>();
  const probePivots = new Map<string, Pivot[]>();

  for (const [key, tpl] of Object.entries(TEMPLATES)) {
    if (bars.length < tpl.minBars) continue;

    const hit = tpl.detect(bars, pv);
    if (hit) {
      hits.push(row(symbol, sector, key, tpl, hit, "TRIGGERED", timeframe, asOf, null));
      continue;
    }
    if (!PROBE_FAMILIES.has(tpl.family)) continue;

    for (const dir of ["up", "down"] as const) {
      const ck = `${dir}:${tpl.minBars}`;
      if (!probeCache.has(ck)) probeCache.set(ck, probeBar(bars, dir, tpl.minBars));
      const probe = probeCache.get(ck);
      if (!probe) continue;

      const probed = bars.concat(probe);
      if (!probePivots.has(ck)) probePivots.set(ck, pivots(probed));
      const found = tpl.detect(probed, probePivots.get(ck)!);
      if (!found) continue;
      // The probe must have produced a signal in the direction it probed. An
      // up-probe yielding a SELL means the shape resolved the other way and the
      // probe told us nothing about it.
      if ((dir === "up") !== (found.side === "BUY")) continue;

      hits.push(row(symbol, sector, key, tpl, found, "FORMING", timeframe, asOf, probe.close));
      break;
    }
  }
  return hits;
}

function row(
  symbol: string,
  sector: string,
  key: string,
  tpl: Template,
  s: PatternSignal,
  state: "TRIGGERED" | "FORMING",
  timeframe: "1d" | "1w",
  asOf: string,
  triggerLevel: number | null,
): PatternHit {
  return {
    symbol,
    sector,
    pattern: s.pattern,
    template: key,
    family: tpl.family,
    familyLabel: FAMILY_LABELS[tpl.family],
    timeframe,
    timeframeLabel: TIMEFRAME_LABELS[timeframe],
    state,
    side: s.side,
    direction: s.side === "BUY" ? "bullish" : "bearish",
    entry: s.entry,
    target: s.target,
    stoploss: s.stoploss,
    triggerLevel,
    confidence: Math.round(s.confidence * 100) / 100,
    rationale: s.rationale,
    rewardRisk: rr(s.entry, s.target, s.stoploss),
    asOf,
  };
}

/**
 * Resample daily bars into ISO-week buckets. Open = first, high = max, low =
 * min, close = last, volume = sum. Stamped with the week's LAST day.
 *
 * Bucketed by ISO week rather than by a fixed factor of seven bars. Crypto has
 * no holidays, so the two agree today — but a single missing bar from a venue
 * outage would put a fixed-factor resample permanently out of phase with the
 * real calendar, and every "weekly" bar after it would straddle two actual
 * weeks.
 */
export function toWeekly(bars: Bar[]): Bar[] {
  if (bars.length === 0) return [];
  const buckets = new Map<string, Bar[]>();
  for (const b of bars) {
    const d = new Date(b.ts * 1000);
    const key = isoWeekKey(d);
    const g = buckets.get(key);
    if (g) g.push(b);
    else buckets.set(key, [b]);
  }
  const out: Bar[] = [];
  for (const key of [...buckets.keys()].sort()) {
    const g = buckets.get(key)!;
    out.push({
      ts: g[g.length - 1]!.ts,
      open: g[0]!.open,
      high: Math.max(...g.map((x) => x.high)),
      low: Math.min(...g.map((x) => x.low)),
      close: g[g.length - 1]!.close,
      volume: g.reduce((s, x) => s + x.volume, 0),
    });
  }
  return out;
}

function isoWeekKey(d: Date): string {
  const t = new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate()));
  const day = t.getUTCDay() || 7;
  t.setUTCDate(t.getUTCDate() + 4 - day);
  const yearStart = new Date(Date.UTC(t.getUTCFullYear(), 0, 1));
  const week = Math.ceil(((t.getTime() - yearStart.getTime()) / 86_400_000 + 1) / 7);
  return `${t.getUTCFullYear()}-${String(week).padStart(2, "0")}`;
}

/** The selectable pattern list for the UI filter. */
export function catalog(): {
  key: string;
  label: string;
  family: PatternFamily;
  familyLabel: string;
  probeable: boolean;
}[] {
  return Object.entries(TEMPLATES)
    .map(([key, t]) => ({
      key,
      label: t.label,
      family: t.family,
      familyLabel: FAMILY_LABELS[t.family],
      probeable: PROBE_FAMILIES.has(t.family),
    }))
    .sort((a, b) => a.family.localeCompare(b.family) || a.label.localeCompare(b.label));
}
