/**
 * Turning the screener's own boards into uniform trade signals.
 *
 * NOTHING IS INVENTED HERE. Every signal is a row that already exists on a tab
 * of this module, carrying the entry, stop and target that tab already
 * published. That is the point: the desk exists to find out whether the numbers
 * on those tabs are worth acting on, and it can only answer that if it takes
 * them at exactly the prices they were shown at. A desk that re-derived its own
 * levels would be measuring a different strategy from the one on screen.
 *
 * FIVE FAMILIES, ONE BOOK PER SYMBOL. Capital is per contract ($10,000 each,
 * the desk's premise) and the family travels on every trade, so the leaderboard
 * can answer both questions from one set of books: which CONTRACT suits this
 * desk, and which KIND OF SIGNAL actually makes money. Splitting capital by
 * family as well would need 220 x 5 books and would answer neither well.
 *
 *   scalp     Setups tab, hours.      Long only.
 *   swing     Setups tab, days.       Long only.
 *   breakout  Setups tab, level break. Long only.
 *   pattern   Chart Patterns tab, TRIGGERED only. LONG AND SHORT.
 *   momentum  Momentum tab, 30-day rank, gated like a swing. Long only.
 *
 * WHY ONLY THE PATTERN FAMILY SHORTS. The three Setups families are built on
 * gates that are directional by construction — up over the horizon, above its
 * own moving average, breaking its own highs — so there is no short version of
 * them to take without inventing one. The pattern detectors already emit SELL
 * signals with their own entry, stop and target, so shorting those is taking a
 * published number rather than manufacturing one. On a perpetual venue a short
 * is a first-class trade, and ignoring half the detector's output would waste it.
 *
 * FORMING PATTERNS ARE NOT TRADED. A shape whose break has not happened is a
 * level to watch, not a fill. Only TRIGGERED rows reach this desk.
 */

import { momentumBoard, patternBoard, setups } from "../engine";
import { gate, swingPlan } from "../plans";
import type { ScreenerRow } from "../universe";
import { getSnapshot } from "../universe";
import { FAMILIES, FAMILY_MAX_HOLD_HOURS, type SignalFamily } from "./store";

export type Signal = {
  symbol: string;
  family: SignalFamily;
  side: "long" | "short";
  /** The price the board published. Slippage is applied by the engine, not here. */
  price: number;
  stop: number;
  target: number;
  netRr: number | null;
  reason: string | null;
  chips: { label: string; tier: number }[];
  pattern: string | null;
  /**
   * Hold for THIS signal, overriding the family default.
   *
   * Exists because a chart pattern's horizon is a property of the timeframe it
   * was drawn on, not of the family. A weekly shape's measured move takes weeks
   * to play out; holding it for the five days a daily shape gets would exit
   * every one of them on the clock and measure nothing but the passage of time.
   */
  maxHoldHours: number;
};

/**
 * Liquidity floor for anything this desk will trade, in 24h USD turnover.
 *
 * Higher than the screener's display floor. A board can honestly SHOW a
 * contract that traded $250k yesterday; a desk that fills a $6,000 position in
 * it is claiming a fill that would have moved the price it filled at.
 */
export const PAPER_MIN_TURNOVER_USD = 1_000_000;

/** Chart patterns below this confidence are not acted on. */
const MIN_PATTERN_CONFIDENCE = 0.55;

/** A pattern whose reward-to-risk is under this is not worth the two taker legs. */
const MIN_PATTERN_RR = 1.2;

/** How far down the momentum board the desk is willing to look. */
const MOMENTUM_DEPTH = 12;

/**
 * Hold for a chart pattern, by the timeframe it was found on.
 *
 * A weekly pattern gets a month. Anything less and the family's record is a
 * record of time stops rather than of the pattern.
 */
const PATTERN_HOLD_HOURS: Record<string, number> = { "1d": 120, "1w": 720 };

/**
 * A level is only worth trading if price can plausibly REACH it inside the hold.
 *
 * Measured against the contract's own daily ATR, scaled to the holding period
 * by square-root-of-time. The first run of this desk opened a weekly-pattern
 * long with its target 188% away and its stop 45% away on a five-day hold:
 * neither level could be touched in that window, so the trade could only ever
 * end on the clock and its "result" would have been a statement about nothing.
 *
 * The two multiples differ because the levels do different jobs. A stop further
 * than 3x the expected move is not protecting the trade, it is a different
 * trade. A target further than 5x is not a plan, it is a hope.
 */
const MAX_STOP_MOVE_X = 3;
const MAX_TARGET_MOVE_X = 5;

function expectedMovePct(atrPct: number | null, holdHours: number): number | null {
  if (atrPct === null || atrPct <= 0 || holdHours <= 0) return null;
  return atrPct * Math.sqrt(holdHours / 24);
}

/**
 * Refusal string when a signal's levels cannot be reached inside its hold, or
 * null when both can.
 *
 * An unknown ATR PASSES rather than blocks. Refusing every contract whose
 * volatility could not be measured would quietly narrow the desk to its
 * best-covered symbols and bias the leaderboard toward them.
 */
function unreachable(
  row: ScreenerRow,
  entry: number,
  stop: number,
  target: number,
  holdHours: number,
): string | null {
  const expected = expectedMovePct(row.atrPct, holdHours);
  if (expected === null) return null;
  const stopPct = (Math.abs(entry - stop) / entry) * 100;
  const targetPct = (Math.abs(target - entry) / entry) * 100;
  if (stopPct > expected * MAX_STOP_MOVE_X) {
    return (
      "stop is " + stopPct.toFixed(0) + "% away but this contract moves about " +
      expected.toFixed(0) + "% in " + Math.round(holdHours) + "h, so it could not be reached inside the hold"
    );
  }
  if (targetPct > expected * MAX_TARGET_MOVE_X) {
    return (
      "target is " + targetPct.toFixed(0) + "% away but this contract moves about " +
      expected.toFixed(0) + "% in " + Math.round(holdHours) + "h, so the trade could only end on the clock"
    );
  }
  return null;
}

function tradable(row: ScreenerRow | undefined): row is ScreenerRow {
  if (!row) return false;
  if (row.turnoverUsd24h === null || row.turnoverUsd24h < PAPER_MIN_TURNOVER_USD) return false;
  if (!row.micro.stopExpressible) return false;
  if (row.price === null || row.price <= 0) return false;
  return true;
}

/**
 * Every signal the desk would act on right now, across all five families.
 *
 * Reads the same cached snapshot every tab reads, so a signal's price is the
 * price the page was showing at that moment rather than a fresh quote taken
 * microseconds later.
 */
export async function collectSignals(): Promise<{ signals: Signal[]; skipped: Map<string, number> }> {
  const snap = await getSnapshot();
  const bySymbol = new Map(snap.rows.map((r) => [r.symbol, r]));
  const skipped = new Map<string, number>();
  const note = (reason: string) => skipped.set(reason, (skipped.get(reason) ?? 0) + 1);

  const out: Signal[] = [];

  // ── the three Setups families ─────────────────────────────────────────────
  for (const kind of ["scalp", "swing", "breakout"] as const) {
    let board;
    try {
      board = await setups(kind, 60);
    } catch {
      note(`${kind} board could not be built`);
      continue;
    }
    for (const r of board.rows) {
      const plan = r.plan;
      // worthTaking already means: tradable grid, net R:R at or above 1 after
      // real taker fees AND the funding this hold would pay. Rows that fail it
      // are shown on the tab and are deliberately not traded here.
      if (!plan.worthTaking) {
        note(`${kind}: plan does not clear its costs`);
        continue;
      }
      const row = bySymbol.get(r.symbol);
      if (!tradable(row)) {
        note(`${kind}: below the desk's liquidity floor or ungriddable`);
        continue;
      }
      const hold = FAMILY_MAX_HOLD_HOURS[kind];
      const bad = unreachable(row, plan.entry, plan.stop, plan.target, hold);
      if (bad) {
        note(kind + ": " + bad);
        continue;
      }
      out.push({
        symbol: r.symbol,
        family: kind,
        side: "long",
        price: plan.entry,
        stop: plan.stop,
        target: plan.target,
        netRr: plan.netRr,
        reason: r.whySummary,
        chips: r.why ?? [],
        pattern: null,
        maxHoldHours: hold,
      });
    }
  }

  // ── chart patterns, both directions ───────────────────────────────────────
  try {
    const pat = await patternBoard({ state: "TRIGGERED", limit: 400 });
    for (const h of pat.rows) {
      if (h.confidence < MIN_PATTERN_CONFIDENCE) {
        note("pattern: confidence below the desk's floor");
        continue;
      }
      if (h.rewardRisk === null || h.rewardRisk < MIN_PATTERN_RR) {
        note("pattern: reward-to-risk below the desk's floor");
        continue;
      }
      const row = bySymbol.get(h.symbol);
      if (!tradable(row)) {
        note("pattern: below the desk's liquidity floor or ungriddable");
        continue;
      }
      const side = h.direction === "bullish" ? "long" : "short";
      // The detector's own levels, used as published. Sanity-checked for
      // direction because a signal with its stop the wrong side of entry would
      // size to a negative risk.
      if (side === "long" && !(h.stoploss < h.entry && h.target > h.entry)) continue;
      if (side === "short" && !(h.stoploss > h.entry && h.target < h.entry)) continue;

      const hold = PATTERN_HOLD_HOURS[h.timeframe] ?? FAMILY_MAX_HOLD_HOURS.pattern;
      const bad = unreachable(row, h.entry, h.stoploss, h.target, hold);
      if (bad) {
        note("pattern: " + bad);
        continue;
      }
      out.push({
        symbol: h.symbol,
        family: "pattern",
        side,
        price: h.entry,
        stop: h.stoploss,
        target: h.target,
        netRr: h.rewardRisk,
        reason: h.rationale,
        chips: [{ label: `${h.pattern} · ${h.timeframeLabel}`, tier: 1 }],
        pattern: `${h.pattern} (${h.timeframeLabel})`,
        maxHoldHours: hold,
      });
    }
  } catch {
    note("pattern board could not be built");
  }

  // ── momentum rank ─────────────────────────────────────────────────────────
  //
  // Deliberately gated the same way the swing family is, and priced with the
  // same swing plan. That makes this book a test of RANK as a signal rather
  // than a second copy of the swing setup: the entry rules are identical, so
  // any difference in outcome is attributable to how the candidates were chosen.
  try {
    const board = await momentumBoard({ horizon: "1m", limit: MOMENTUM_DEPTH });
    for (const r of board.rows) {
      const row = bySymbol.get(r.symbol);
      if (!tradable(row)) {
        note("momentum: below the desk's liquidity floor or ungriddable");
        continue;
      }
      const [passes, why] = gate(row, "swing");
      if (!passes) {
        note(`momentum: ${why}`);
        continue;
      }
      const plan = swingPlan(row);
      if (!plan || !plan.worthTaking) {
        note("momentum: swing plan does not clear its costs");
        continue;
      }
      const hold = FAMILY_MAX_HOLD_HOURS.momentum;
      const bad = unreachable(row, plan.entry, plan.stop, plan.target, hold);
      if (bad) {
        note("momentum: " + bad);
        continue;
      }
      out.push({
        symbol: r.symbol,
        family: "momentum",
        side: "long",
        price: plan.entry,
        stop: plan.stop,
        target: plan.target,
        netRr: plan.netRr,
        reason: r.whySummary,
        chips: r.why ?? [],
        pattern: null,
        maxHoldHours: hold,
      });
    }
  } catch {
    note("momentum board could not be built");
  }

  // ORDERED ROUND-ROBIN ACROSS FAMILIES, best-first within each.
  //
  // A flat sort by reward-to-risk looks fairer and is not. Swing plans target
  // 2.5R by construction and breakout plans target a measured move, so a flat
  // sort hands almost every slot to one family: the first run of this desk
  // filled 12 of 12 openings with swing. The leaderboard would then be
  // comparing a family with twelve trades against families with one, and the
  // difference would be allocation rather than edge — which is exactly the
  // question this desk exists to answer.
  out.sort((a, b) => (b.netRr ?? 0) - (a.netRr ?? 0));
  const queues = new Map<SignalFamily, Signal[]>();
  for (const sig of out) {
    const q = queues.get(sig.family);
    if (q) q.push(sig);
    else queues.set(sig.family, [sig]);
  }
  const interleaved: Signal[] = [];
  for (let round = 0; interleaved.length < out.length; round++) {
    let progressed = false;
    for (const f of FAMILIES) {
      const q = queues.get(f);
      if (q && q.length > round) {
        interleaved.push(q[round]!);
        progressed = true;
      }
    }
    if (!progressed) break;
  }
  return { signals: interleaved, skipped };
}

export function familyOrder(f: SignalFamily): number {
  return FAMILIES.indexOf(f);
}
