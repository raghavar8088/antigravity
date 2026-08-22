/**
 * Matching, margin and P&L — the arithmetic both terminals share.
 *
 * THREE THINGS HERE ARE EASY TO GET WRONG AND ARE WORTH READING BEFORE
 * CHANGING ANYTHING.
 *
 * 1. A MARKET ORDER IS WALKED THROUGH THE BOOK, not filled at the top of it.
 *    An order larger than the best level eats into worse ones and receives a
 *    size-weighted average. Filling everything at the touch is the difference
 *    between a paper record and a plausible one, and on a thin contract it is
 *    the entire result. Delta publishes thousands of real levels, so this is a
 *    genuine walk; the forex book is modelled and says so.
 *
 * 2. P&L IS CONVERTED OUT OF THE QUOTE CURRENCY. A EURUSD position profits in
 *    dollars, but a USDJPY position profits in YEN and a GBPJPY position also
 *    profits in yen — and an account denominated in USD cannot add those up
 *    without converting. This is the single most common defect in a homemade
 *    forex calculator: it reports a 100-pip win on USDJPY as $1,000 when it is
 *    about $63. Every P&L, margin and carry figure below passes through
 *    `usdPerQuoteUnit`.
 *
 * 3. A PIP IS NOT A TICK, AND ON YEN PAIRS IT IS NOT THE FOURTH DECIMAL. Yen
 *    pairs pip on the second decimal. That is a factor of one hundred, and it
 *    is declared per instrument rather than inferred.
 */

import type { Candle, Instrument, OrderBook, OrderSide, Position } from "./types";
import { quantityOf } from "./types";

// ── currency conversion ─────────────────────────────────────────────────────

/**
 * The USD value of one unit of an instrument's QUOTE currency.
 *
 * Crypto perpetuals and metals quote in USD and return 1. FX pairs quote in
 * whatever the last three letters say, and that currency's USD rate comes from
 * another instrument on the same desk:
 *
 *   USDJPY  quote JPY  ->  1 / USDJPY
 *   EURJPY  quote JPY  ->  1 / USDJPY   (the cross's own price is irrelevant)
 *   EURGBP  quote GBP  ->  GBPUSD
 *
 * Returns null when the rate is genuinely unavailable, and every caller then
 * refuses rather than defaulting to 1 — defaulting would silently report a
 * yen P&L as though it were dollars, inflating it about 159-fold.
 */
export function usdPerQuoteUnit(
  instrument: Instrument,
  quotes: Map<string, number>,
): number | null {
  if (instrument.venue === "delta") return 1;
  const sym = instrument.symbol;
  // Metals, energies, indices and crypto CFDs on this desk are all USD-quoted.
  if (instrument.kind !== "fx") return 1;
  if (sym.length < 6) return 1;
  const quote = sym.slice(3, 6).toUpperCase();
  if (quote === "USD") return 1;

  const direct = quotes.get(`USD${quote}`);
  if (direct && direct > 0) return 1 / direct;
  const inverse = quotes.get(`${quote}USD`);
  if (inverse && inverse > 0) return inverse;
  return null;
}

/** Underlying units controlled by `size` of this instrument. */
export function qtyOf(instrument: Instrument, size: number): number {
  return quantityOf(instrument, size);
}

/**
 * USD notional.
 *
 * `qty x price` is the position's value in the QUOTE currency; converting it
 * gives dollars. On a USD-quoted instrument the conversion is 1 and this
 * reduces to the obvious formula.
 */
export function notionalUsd(
  instrument: Instrument,
  size: number,
  price: number,
  usdPerQuote: number,
): number {
  return qtyOf(instrument, size) * price * usdPerQuote;
}

/** Signed P&L in USD for a position marked at `price`. */
export function pnlUsd(
  instrument: Instrument,
  side: "long" | "short",
  size: number,
  entry: number,
  price: number,
  usdPerQuote: number,
): number {
  const move = side === "long" ? price - entry : entry - price;
  return move * qtyOf(instrument, size) * usdPerQuote;
}

/** Price movement expressed in pips, for the forex terminal's readouts. */
export function pipsMoved(instrument: Instrument, from: number, to: number, pipSize: number): number {
  void instrument;
  return pipSize > 0 ? (to - from) / pipSize : 0;
}

// ── book walking ────────────────────────────────────────────────────────────

export type Fill = {
  /** Size-weighted average price actually received. */
  avgPrice: number;
  filledSize: number;
  /** How many book levels the order consumed. */
  levelsConsumed: number;
  /** Difference between the touch and the average received, in price terms. */
  slippage: number;
  /** True when the book ran out before the order was filled. */
  exhausted: boolean;
  note: string;
};

/**
 * Walk `size` through the resting side of the book.
 *
 * A buy consumes asks from the best upward; a sell consumes bids from the best
 * downward. Returns a partial fill with `exhausted: true` when the visible book
 * cannot cover the order — the caller decides whether that is a rejection or a
 * partial, rather than this function inventing depth beyond what the venue
 * published.
 */
export function walkBook(book: OrderBook, side: OrderSide, size: number): Fill | null {
  const levels = side === "buy" ? book.asks : book.bids;
  if (levels.length === 0 || !(size > 0)) return null;

  const touch = levels[0]!.price;
  let remaining = size;
  let cost = 0;
  let consumed = 0;

  for (const lvl of levels) {
    if (remaining <= 0) break;
    const take = Math.min(remaining, lvl.size);
    cost += take * lvl.price;
    remaining -= take;
    consumed++;
  }

  const filled = size - remaining;
  if (filled <= 0) return null;
  const avg = cost / filled;

  return {
    avgPrice: avg,
    filledSize: filled,
    levelsConsumed: consumed,
    slippage: side === "buy" ? avg - touch : touch - avg,
    exhausted: remaining > 0,
    note:
      book.modelled
        ? `Filled against a MODELLED book (${consumed} level${consumed === 1 ? "" : "s"}) — this venue publishes no depth, so the ladder is synthetic and only the mid is real.`
        : `Walked ${consumed} real book level${consumed === 1 ? "" : "s"}; average ${avg.toPrecision(8)} against a touch of ${touch.toPrecision(8)}.`,
  };
}

// ── margin and liquidation ──────────────────────────────────────────────────

export function marginUsd(notional: number, leverage: number): number {
  return leverage > 0 ? notional / leverage : notional;
}

/**
 * Where the VENUE closes a leveraged position, ignoring the trader's own stop.
 *
 * The posted margin absorbs an adverse move of `1/leverage` of notional; the
 * venue keeps `maintenanceMarginPct` back, so the usable cushion is the
 * difference. Returns null when maintenance margin is unknown — the caller then
 * refuses the position rather than drawing a liquidation line it cannot stand
 * behind.
 *
 * Retail forex has no per-position maintenance tier; those accounts are managed
 * on account-wide margin level and stopped out there instead, so this returns
 * null for them by design.
 */
export function liquidationPrice(
  side: "long" | "short",
  entry: number,
  leverage: number,
  maintenanceMarginPct: number,
): number | null {
  if (!(leverage > 0) || !(entry > 0)) return null;
  if (!(maintenanceMarginPct > 0)) return null;
  const cushion = 1 / leverage - maintenanceMarginPct / 100;
  if (cushion <= 0) return null;
  return side === "long" ? entry * (1 - cushion) : entry * (1 + cushion);
}

// ── carry ───────────────────────────────────────────────────────────────────

/** Perpetual funding settles every 8h at 00:00 / 08:00 / 16:00 UTC. */
export const FUNDING_INTERVAL_SEC = 8 * 3_600;

/** Forex rolls over once a day at 21:00 UTC. */
export const ROLLOVER_HOUR_UTC = 21;

export function fundingSettlements(fromSec: number, toSec: number): number {
  if (!(toSec > fromSec)) return 0;
  return Math.floor(toSec / FUNDING_INTERVAL_SEC) - Math.floor(fromSec / FUNDING_INTERVAL_SEC);
}

/**
 * Rollovers crossed, counting Wednesday's as THREE.
 *
 * A spot FX trade settles in two business days, so the position rolled over on
 * Wednesday night carries a value date over the weekend and is charged three
 * days of swap for it. Charging one would understate the cost of every trade
 * held midweek, which is most of them.
 */
export function swapCharges(fromSec: number, toSec: number): number {
  if (!(toSec > fromSec)) return 0;
  const DAY = 86_400;
  const offset = ROLLOVER_HOUR_UTC * 3_600;
  const firstAfter = Math.floor((fromSec - offset) / DAY) + 1;
  const lastAtOrBefore = Math.floor((toSec - offset) / DAY);
  let charges = 0;
  for (let d = firstAfter; d <= lastAtOrBefore; d++) {
    const at = d * DAY + offset;
    // getUTCDay on the rollover instant: 3 is Wednesday.
    charges += new Date(at * 1000).getUTCDay() === 3 ? 3 : 1;
  }
  return charges;
}

/**
 * Carry owed since `position.carryTo`, signed from the position's own side.
 *
 * Positive means the position PAID. A short perpetual with positive funding is
 * credited, and clamping that to zero would misprice every position on the
 * discount side of the venue.
 */
export function accrueCarry(
  instrument: Instrument,
  position: Pick<Position, "side" | "size" | "carryTo">,
  toSec: number,
  usdPerQuote: number,
  markPrice: number,
): { usd: number; charges: number } {
  if (instrument.carryKind === "funding") {
    const rate = instrument.fundingRatePct8h;
    if (rate === null) return { usd: 0, charges: 0 };
    const n = fundingSettlements(position.carryTo, toSec);
    if (n <= 0) return { usd: 0, charges: 0 };
    const notional = notionalUsd(instrument, position.size, markPrice, usdPerQuote);
    const sign = position.side === "long" ? 1 : -1;
    return { usd: notional * (rate / 100) * n * sign, charges: n };
  }

  // Swap: quoted in POINTS per lot per day, so it converts through the tick
  // size and the quote currency exactly as a price move does.
  const points = position.side === "long" ? instrument.swapLongPointsPerDay : instrument.swapShortPointsPerDay;
  if (points === null || points === undefined) return { usd: 0, charges: 0 };
  const n = swapCharges(position.carryTo, toSec);
  if (n <= 0) return { usd: 0, charges: 0 };
  // A NEGATIVE swap rate is a cost, so the sign flips: `carryUsd` is what the
  // position paid.
  const perDayUsd = points * instrument.tickSize * qtyOf(instrument, position.size) * usdPerQuote;
  return { usd: -perDayUsd * n, charges: n };
}

// ── bar replay ──────────────────────────────────────────────────────────────

/**
 * Did a resting LIMIT order fill inside this bar, and at what price?
 *
 * A limit order needs price to trade THROUGH its level, not merely touch it:
 * at the touch you are at the back of a queue that may never clear. Requiring
 * the bar to trade strictly beyond the limit is the conservative reading, and
 * it is the one that does not hand a paper account fills a real one would have
 * missed. A bar that OPENS beyond the limit fills at the open, which is better
 * than the limit and is what actually happens on a gap.
 */
export function limitFill(bar: Candle, side: OrderSide, limit: number): number | null {
  if (side === "buy") {
    if (bar.open <= limit) return bar.open;
    return bar.low < limit ? limit : null;
  }
  if (bar.open >= limit) return bar.open;
  return bar.high > limit ? limit : null;
}

/**
 * Did a STOP trigger inside this bar, and at what price would it have filled?
 *
 * A stop triggers on TOUCH — unlike a limit, it does not need to trade through,
 * because it becomes a market order the moment the level prints. It then fills
 * at the worse of the level or the bar's open, which is what a stop order
 * actually does when price gaps past it.
 */
export function stopFill(bar: Candle, side: OrderSide, stop: number): number | null {
  if (side === "buy") {
    if (bar.open >= stop) return bar.open;
    return bar.high >= stop ? stop : null;
  }
  if (bar.open <= stop) return bar.open;
  return bar.low <= stop ? stop : null;
}

export type LevelHit = {
  reason: "take_profit" | "stop_loss";
  level: number;
  fill: number;
  at: number;
  gapped: boolean;
  ambiguous: boolean;
};

/**
 * Walk bars looking for a position's own take-profit or stop-loss.
 *
 * SAME-BAR AMBIGUITY IS RESOLVED AGAINST THE POSITION. When one bar contains
 * both levels, OHLC cannot say which printed first, and picking the profitable
 * one is how a paper record invents a win rate. The stop is assumed and the
 * trade is flagged, so the reader can see how often it mattered.
 */
export function findLevelHit(
  bars: Candle[],
  side: "long" | "short",
  takeProfit: number | null,
  stopLoss: number | null,
  fromSec: number,
): LevelHit | null {
  if (takeProfit === null && stopLoss === null) return null;
  const exitSide: OrderSide = side === "long" ? "sell" : "buy";

  for (const bar of bars) {
    if (bar.time < fromSec) continue;
    const tpFill = takeProfit !== null ? limitFillTouch(bar, exitSide, takeProfit, side, "tp") : null;
    const slFill = stopLoss !== null ? stopFill(bar, exitSide, stopLoss) : null;

    if (tpFill !== null && slFill !== null) {
      return {
        reason: "stop_loss",
        level: stopLoss!,
        fill: slFill,
        at: bar.time,
        gapped: slFill !== stopLoss,
        ambiguous: true,
      };
    }
    if (slFill !== null) {
      return { reason: "stop_loss", level: stopLoss!, fill: slFill, at: bar.time, gapped: slFill !== stopLoss, ambiguous: false };
    }
    if (tpFill !== null) {
      return { reason: "take_profit", level: takeProfit!, fill: tpFill, at: bar.time, gapped: tpFill !== takeProfit, ambiguous: false };
    }
  }
  return null;
}

/**
 * A take-profit is a limit order, but one attached to an open position rather
 * than resting in the queue — the broker fills it on touch. Modelled on touch
 * for that reason, while an unattached limit order still requires a
 * trade-through.
 */
function limitFillTouch(bar: Candle, exitSide: OrderSide, level: number, _posSide: "long" | "short", _k: string): number | null {
  void _posSide;
  void _k;
  if (exitSide === "sell") {
    if (bar.open >= level) return bar.open;
    return bar.high >= level ? level : null;
  }
  if (bar.open <= level) return bar.open;
  return bar.low <= level ? level : null;
}

/** Did price reach a liquidation level inside these bars? */
export function findLiquidation(
  bars: Candle[],
  side: "long" | "short",
  liquidation: number | null,
  fromSec: number,
): { at: number; fill: number } | null {
  if (liquidation === null) return null;
  for (const bar of bars) {
    if (bar.time < fromSec) continue;
    if (side === "long" && bar.low <= liquidation) {
      return { at: bar.time, fill: Math.min(bar.open, liquidation) };
    }
    if (side === "short" && bar.high >= liquidation) {
      return { at: bar.time, fill: Math.max(bar.open, liquidation) };
    }
  }
  return null;
}

/** Round a price to the instrument's grid, which is what the venue would accept. */
export function roundToTick(price: number, tickSize: number): number {
  if (!(tickSize > 0)) return price;
  return Math.round(price / tickSize) * tickSize;
}

/** Round a size to the instrument's step, never up past what was asked. */
export function roundSize(size: number, step: number): number {
  if (!(step > 0)) return size;
  const n = Math.floor(size / step + 1e-9) * step;
  // Floating point: 0.01 steps accumulate error that shows up in the UI.
  return Math.round(n * 1e8) / 1e8;
}
