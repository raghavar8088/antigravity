/**
 * The "why is this moving" engine.
 *
 * THE ONE RULE, inherited unchanged from the equity screener: never emit a
 * reason we cannot point at a number for.
 *
 * That rule is the whole design. It is easy to write something that produces a
 * confident sentence for every row — and a screener that always has an
 * explanation is a screener whose explanations mean nothing, because it has no
 * way of saying "this moved and I do not know why". That answer is common, it
 * is honest, and it is genuinely useful: an uncorroborated 40% move is a very
 * different trade from a 40% move the whole sector is making on rising open
 * interest.
 *
 * WHAT CHANGES ON A PERPETUAL VENUE is which tier the good reasons live in.
 * On the equity side, tier 2 is mostly empty: F&O buildup reads as unavailable
 * for every row, delivery data depends on a bhavcopy that may never have been
 * captured, and news is never checked. Here tier 2 is the strongest tier —
 * funding, open-interest buildup, basis and book imbalance are published for
 * every contract in the same call as the price. So a crypto row routinely
 * carries corroboration an equity row cannot, and the summary sentence says so
 * rather than burying it.
 *
 * Tier 3 is still empty and still says so. No news source is wired up, and
 * inventing a narrative to fill the gap would undo everything above.
 */

export type Reason = {
  code: string;
  tier: 1 | 2 | 3;
  text: string;
  weight: number;
  value: number | null;
  unit: string | null;
};

export type ReasonChip = { label: string; tier: number; code: string };

// ── thresholds ──────────────────────────────────────────────────────────────
const VOL_NOTABLE = 1.5; // x its own 20-day average before volume is worth mentioning
const VOL_STRONG = 3.0;
const RS_STRONG = 3.0; // percentage points of outperformance vs BTC
const CONSISTENCY_STRONG = 65.0;
const EMA_HOLD_STRONG = 80.0;
const NEAR_HIGH_PCT = -2.0;
const SECTOR_MOVE_PCT = 2.5; // crypto sectors move more than equity ones; the bar is higher
const FUNDING_HOT_PCT8H = 0.03; // ~33%/yr — positioning is genuinely lopsided
const FUNDING_EXTREME_PCT8H = 0.08; // ~88%/yr
const BASIS_WIDE_BPS = 25;
const IMBALANCE_STRONG = 0.4;

export type ReasonInputs = {
  symbol: string;
  returnPct: number | null;
  volumeX: number | null;
  breakout: { window: number; date: string } | null;
  pctFromPeriodHigh: number | null;
  emaHoldPct: number | null;
  consistency: number | null;
  upStreak: number | null;
  rsBenchmark: number | null;
  benchmarkSymbol: string;

  funding?: { ratePct8h: number | null; annualisedPct: number | null; payer: string } | null;
  oi?: { buildup: string; oiChangePct6h: number | null } | null;
  basis?: { basisBps: number | null; state: string } | null;
  micro?: { bookImbalance: number | null; spreadBps: number | null } | null;
  sector?: { sector: string; returnPct: number | null; rank: number | null; of: number | null } | null;
  correlation?: { btc: number | null; beta: number | null } | null;
};

/**
 * Assemble the ranked reason stack for one contract.
 *
 * Weights order the stack; they are not a score for the token. A reason's
 * weight answers "how much does this explain the move", which is a different
 * question from "how good is this trade".
 */
export function build(m: ReasonInputs): Reason[] {
  const out: Reason[] = [];
  const push = (
    code: string,
    tier: 1 | 2 | 3,
    text: string,
    weight: number,
    value: number | null = null,
    unit: string | null = null,
  ) => out.push({ code, tier, text, weight, value: value === null ? null : round2(value), unit });

  // ── tier 1: mechanical ────────────────────────────────────────────────────
  if (m.volumeX !== null && m.volumeX >= VOL_NOTABLE) {
    const strength = m.volumeX >= VOL_STRONG ? "far above" : "above";
    push(
      "volume",
      1,
      `Trading ${m.volumeX.toFixed(1)}x its 20-day average volume — participation is ${strength} normal`,
      Math.min(1, m.volumeX / 5) * 0.9,
      m.volumeX,
      "x",
    );
  }

  if (m.breakout) {
    push(
      "breakout",
      1,
      `Closed through its ${m.breakout.window}-day high on ${m.breakout.date}`,
      m.breakout.window >= 50 ? 0.85 : 0.65,
      m.breakout.window,
      "days",
    );
  }

  if (m.pctFromPeriodHigh !== null && m.pctFromPeriodHigh >= NEAR_HIGH_PCT) {
    push(
      "at_highs",
      1,
      `Within ${Math.abs(m.pctFromPeriodHigh).toFixed(1)}% of its 1-year high`,
      0.7,
      m.pctFromPeriodHigh,
      "%",
    );
  }

  if (m.emaHoldPct !== null && m.emaHoldPct >= EMA_HOLD_STRONG) {
    push(
      "ema_hold",
      1,
      `Held above its 9 EMA on ${m.emaHoldPct.toFixed(0)}% of the last 30 days`,
      0.6,
      m.emaHoldPct,
      "%",
    );
  }

  if (m.consistency !== null && m.consistency >= CONSISTENCY_STRONG) {
    push(
      "consistency",
      1,
      `${m.consistency.toFixed(0)}% of days in this window closed up — a grind, not one candle`,
      0.65,
      m.consistency,
      "%",
    );
  }

  if (m.upStreak !== null && m.upStreak >= 3) {
    push("streak", 1, `${m.upStreak} consecutive up days`, 0.5, m.upStreak, "days");
  }

  if (m.rsBenchmark !== null && m.rsBenchmark >= RS_STRONG) {
    push(
      "rs_benchmark",
      1,
      `Outperforming ${m.benchmarkSymbol} by ${m.rsBenchmark.toFixed(1)} points over this horizon`,
      0.8,
      m.rsBenchmark,
      "pp",
    );
  }

  // ── tier 2: corroborating — and on this venue it is the strong tier ───────

  // Open interest first. It is the single most informative corroboration here,
  // because it separates the two green candles that mean opposite things.
  if (m.oi && m.oi.buildup !== "unclassified" && m.oi.buildup !== "flat") {
    const pct = m.oi.oiChangePct6h;
    const pctTxt = pct !== null ? ` (${pct > 0 ? "+" : ""}${pct.toFixed(1)}% in 6h)` : "";
    if (m.oi.buildup === "long_buildup") {
      push(
        "oi_long_buildup",
        2,
        `Open interest rising with price${pctTxt} — new longs, not a squeeze. The move has ` +
          `positions behind it`,
        0.95,
        pct,
        "%",
      );
    } else if (m.oi.buildup === "short_covering") {
      push(
        "oi_short_covering",
        2,
        `Price up while open interest FALLS${pctTxt} — this is shorts closing, not longs opening. ` +
          `Squeezes exhaust when the shorts run out`,
        0.9,
        pct,
        "%",
      );
    } else if (m.oi.buildup === "short_buildup") {
      push(
        "oi_short_buildup",
        2,
        `Open interest rising as price falls${pctTxt} — new shorts. The decline is being ` +
          `positioned into`,
        0.9,
        pct,
        "%",
      );
    } else if (m.oi.buildup === "long_unwinding") {
      push(
        "oi_long_unwinding",
        2,
        `Open interest falling with price${pctTxt} — longs closing out. Capitulation rather ` +
          `than fresh selling`,
        0.85,
        pct,
        "%",
      );
    }
  }

  if (m.funding && m.funding.ratePct8h !== null) {
    const r = m.funding.ratePct8h;
    const ann = m.funding.annualisedPct;
    const annTxt = ann !== null ? ` (${ann > 0 ? "+" : ""}${ann.toFixed(0)}%/yr)` : "";
    if (Math.abs(r) >= FUNDING_EXTREME_PCT8H) {
      push(
        "funding_extreme",
        2,
        `Funding is at an extreme: ${r > 0 ? "longs" : "shorts"} pay ${Math.abs(r).toFixed(3)}% ` +
          `every 8h${annTxt}. Positioning this lopsided is what unwinds violently`,
        0.9,
        r,
        "%/8h",
      );
    } else if (Math.abs(r) >= FUNDING_HOT_PCT8H) {
      push(
        "funding_hot",
        2,
        `${r > 0 ? "Longs" : "Shorts"} are paying ${Math.abs(r).toFixed(3)}% every 8h${annTxt} — ` +
          `the crowded side is on the ${r > 0 ? "long" : "short"} book`,
        0.75,
        r,
        "%/8h",
      );
    }
  }

  if (m.basis && m.basis.basisBps !== null && Math.abs(m.basis.basisBps) >= BASIS_WIDE_BPS) {
    const b = m.basis.basisBps;
    push(
      b > 0 ? "basis_premium" : "basis_discount",
      2,
      `The perp trades ${Math.abs(b).toFixed(0)} bps ${b > 0 ? "above" : "below"} spot — ` +
        `leverage is ${b > 0 ? "chasing" : "fading"} this, and the derivative is moving ahead of ` +
        `the cash market`,
      0.7,
      b,
      "bps",
    );
  }

  if (m.micro && m.micro.bookImbalance !== null && Math.abs(m.micro.bookImbalance) >= IMBALANCE_STRONG) {
    const i = m.micro.bookImbalance;
    push(
      "book_imbalance",
      2,
      `Top of book is ${i > 0 ? "bid" : "ask"}-heavy (${(i * 100).toFixed(0)}% skew) — ` +
        `resting size is stacked on the ${i > 0 ? "buy" : "sell"} side right now`,
      0.55,
      i,
      "",
    );
  }

  if (m.sector) {
    const sr = m.sector.returnPct;
    if (sr !== null && Math.abs(sr) >= SECTOR_MOVE_PCT) {
      const rankTxt =
        m.sector.rank && m.sector.of ? `, rank ${m.sector.rank} of ${m.sector.of}` : "";
      push(
        "sector_rotation",
        2,
        `${m.sector.sector} is ${sr >= 0 ? "+" : ""}${sr.toFixed(1)}% over this horizon${rankTxt} — ` +
          `this is sector rotation, not a lone move`,
        0.9,
        sr,
        "%",
      );
    } else if (sr !== null) {
      push(
        "sector_flat",
        2,
        `${m.sector.sector} is only ${sr >= 0 ? "+" : ""}${sr.toFixed(1)}% — the move is ` +
          `specific to this contract`,
        0.55,
        sr,
        "%",
      );
    }
  }

  // Correlation is corroboration in the negative direction: a 0.95-correlated
  // alt "outperforming BTC by 3 points" is mostly beta, and saying so is more
  // useful than another bullish chip.
  if (m.correlation && m.correlation.btc !== null && m.correlation.btc >= 0.85) {
    const b = m.correlation.beta;
    push(
      "btc_beta",
      2,
      `${(m.correlation.btc * 100).toFixed(0)}% correlated to BTC over 30 days` +
        (b !== null ? ` at ${b.toFixed(2)}x beta` : "") +
        ` — most of this move is the market, not the token`,
      0.6,
      m.correlation.btc,
      "",
    );
  }

  out.sort((a, b) => b.weight - a.weight || a.tier - b.tier);
  return out;
}

function round2(v: number): number {
  return Math.round(v * 100) / 100;
}

/**
 * One honest sentence for the row tooltip and the drawer header.
 *
 * When nothing corroborates the move, say exactly that. This is the branch that
 * makes the rest of the engine trustworthy.
 */
export function summarise(symbol: string, horizonReturn: number | null, reasons: Reason[]): string {
  const move =
    horizonReturn !== null ? `${horizonReturn >= 0 ? "+" : ""}${horizonReturn.toFixed(1)}%` : "moving";
  const tier1 = reasons.filter((r) => r.tier === 1);
  const tier2 = reasons.filter((r) => r.tier === 2);

  if (tier1.length === 0 && tier2.length === 0) {
    return (
      `${symbol} ${move} with nothing in the data explaining it — no unusual volume, no breakout, ` +
      `no open-interest change, no funding skew, no sector move. Unexplained.`
    );
  }

  // Built by joining only the parts that exist. A contract with corroboration
  // but no mechanical signal — a sector-wide move on ordinary volume — would
  // otherwise render as "SYMBOL +9.1%. . defi is up".
  const parts: string[] = [`${symbol} ${move}`];
  if (tier1.length > 0) parts.push(tier1.slice(0, 2).map((r) => r.text).join("; "));

  if (tier2.length > 0) {
    parts.push(tier2[0]!.text);
    return `${parts.join(". ")}.`;
  }

  parts.push(
    "Otherwise uncorroborated — open interest flat, funding neutral, no sector move, news not checked",
  );
  return `${parts.join(". ")}.`;
}

/** A one-word character for the move, used for the UI chip colour. */
export function classify(reasons: Reason[]): string {
  const codes = new Set(reasons.map((r) => r.code));
  if (codes.has("oi_short_covering")) return "short-covering";
  if (codes.has("oi_long_unwinding")) return "unwinding";
  if (codes.has("funding_extreme")) return "crowded";
  if (codes.has("oi_long_buildup") && codes.has("volume")) return "accumulation";
  if (codes.has("sector_rotation")) return "rotation";
  if (codes.has("breakout") && codes.has("volume")) return "breakout";
  if (codes.has("btc_beta")) return "beta";
  if (codes.has("volume") || codes.has("streak") || codes.has("consistency")) return "momentum";
  return "unexplained";
}

/** Compact labels for the board's Why column — the drawer carries the full stack. */
export function chips(reasons: Reason[], limit = 3): ReasonChip[] {
  return reasons.slice(0, limit).map((r) => {
    const v = r.value;
    let label: string;
    switch (r.code) {
      case "volume":
        label = `Vol ${v?.toFixed(1)}x`;
        break;
      case "breakout":
        label = `${v ?? "?"}d breakout`;
        break;
      case "at_highs":
        label = "At 1y high";
        break;
      case "rs_benchmark":
        label = `RS ${v !== null && v >= 0 ? "+" : ""}${v?.toFixed(1)}`;
        break;
      case "streak":
        label = `${v ?? "?"}d streak`;
        break;
      case "consistency":
        label = "Consistent";
        break;
      case "ema_hold":
        label = "Above 9EMA";
        break;
      case "oi_long_buildup":
        label = "Long buildup";
        break;
      case "oi_short_buildup":
        label = "Short buildup";
        break;
      case "oi_short_covering":
        label = "Short covering";
        break;
      case "oi_long_unwinding":
        label = "Long unwinding";
        break;
      case "funding_hot":
        label = `Funding ${v !== null && v >= 0 ? "+" : ""}${v?.toFixed(3)}%`;
        break;
      case "funding_extreme":
        label = `Funding extreme`;
        break;
      case "basis_premium":
        label = `Premium ${v?.toFixed(0)}bp`;
        break;
      case "basis_discount":
        label = `Discount ${Math.abs(v ?? 0).toFixed(0)}bp`;
        break;
      case "book_imbalance":
        label = v !== null && v > 0 ? "Bid heavy" : "Ask heavy";
        break;
      case "sector_rotation":
        label = `Sector ${v !== null && v >= 0 ? "+" : ""}${v?.toFixed(1)}%`;
        break;
      case "sector_flat":
        label = "Contract-specific";
        break;
      case "btc_beta":
        label = "BTC beta";
        break;
      default:
        label = r.code.replace(/_/g, " ");
    }
    return { label, tier: r.tier, code: r.code };
  });
}
