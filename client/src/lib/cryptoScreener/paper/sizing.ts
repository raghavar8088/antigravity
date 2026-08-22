/**
 * Position sizing for the paper desk: risk, leverage, and the liquidation check.
 *
 * EACH SYMBOL GETS ITS OWN $10,000 AND SPENDS ONLY FROM IT. Per-symbol rather
 * than one shared pool, because the question this desk exists to answer is
 * which CONTRACT suits these signals. A shared balance would let BTC's results
 * pay for a loss on DOGE and hide it — the same reasoning the Top Crypto
 * Trading desk uses for its per-instrument books.
 *
 * SIZE IS DERIVED FROM RISK, NOT FROM A FIXED TICKET. A fixed $1,000 notional
 * risks $5 on a contract with a 0.5% stop and $150 on one with a 15% stop, so a
 * leaderboard built on it would rank stop width rather than signal quality. Here
 * every position risks the same fraction of its book, and the notional falls out
 * of the stop distance. That makes an R multiple comparable across contracts,
 * which is the whole point of recording one.
 *
 * THE LIQUIDATION CHECK IS NOT OPTIONAL, and it is why this file reads
 * `/v2/products`. A leveraged perpetual has a price at which the VENUE closes
 * the position, regardless of where the strategy put its stop. If that price
 * sits inside the stop, the stop is decorative — the trade is closed out from
 * under it and the recorded loss is not the loss the plan described. This
 * codebase has already had exactly that failure once, from an ASSUMED
 * maintenance margin. Maintenance margins on this venue run from 0.25% to 2.5%,
 * a tenfold spread, so a position whose margin spec is missing is refused
 * rather than sized on a guess.
 *
 * LEVERAGE IS MINIMISED, NOT MAXIMISED. The desk takes the lowest leverage that
 * still funds the position out of the book's free equity. Lower leverage pushes
 * the liquidation price further away at no cost to the P&L — leverage changes
 * how much margin is posted, not how much the position makes or loses.
 */

import type { ScreenerRow } from "../universe";

/** Every symbol's book starts here. */
export const BOOK_STARTING_EQUITY_USD = 10_000;

/**
 * Fraction of a book's CURRENT equity risked per position.
 *
 * Of current equity, not of the starting $10,000: a book down 30% then sizes
 * off $7,000, so losses shrink the next bet instead of compounding at a fixed
 * dollar risk into a drawdown.
 */
export const RISK_PER_TRADE_PCT = 2;

/** A single position may not exceed this multiple of its book's equity. */
export const MAX_NOTIONAL_X_EQUITY = 3;

/** The desk will not use more leverage than this, whatever the venue allows. */
export const DESK_MAX_LEVERAGE = 10;

/**
 * Margin one position may post, as a percent of its book's equity.
 *
 * Exists because "use the least leverage possible" — the first version of this
 * rule — is safe and useless. A scalp with a 2.6% stop sizes to about 76% of
 * the book's notional, and at 1x that posts 76% of the book as collateral for a
 * single trade. The book then has no room for a second position, and the
 * per-symbol ceiling can never be reached.
 *
 * Leverage does not change what a position makes or loses. It changes how much
 * collateral is posted and therefore how far away the venue's liquidation sits.
 * So the rule is: post at most this share of the book, take whatever leverage
 * that implies, and then REFUSE if that leverage would put liquidation inside
 * the stop. Safety is enforced by the liquidation check, not by starving the
 * book of capacity.
 */
export const MAX_MARGIN_PCT_OF_EQUITY = 25;

/**
 * The liquidation price must sit at least this multiple of the stop distance
 * away from entry.
 *
 * 1.5x rather than 1.0x. At exactly 1.0 the two coincide, and a wick that
 * touches the stop touches liquidation in the same instant — the order in which
 * they resolve then depends on the venue's mark price rather than on the last
 * trade, which is not something this desk can model or should pretend to.
 */
export const LIQUIDATION_BUFFER_X = 1.5;

export type SizingInput = {
  row: ScreenerRow;
  side: "long" | "short";
  entry: number;
  stop: number;
  /** Book equity available to commit right now (equity minus margin already posted). */
  availableEquityUsd: number;
  /** Book equity, for the risk and notional caps. */
  bookEquityUsd: number;
};

export type Sizing = {
  ok: true;
  contracts: number;
  /** Underlying units: contracts x contractValue. */
  quantity: number;
  notionalUsd: number;
  leverage: number;
  marginUsd: number;
  riskUsd: number;
  stopDistancePct: number;
  liquidationPrice: number;
  liquidationDistancePct: number;
  maintenanceMarginPct: number;
};

export type SizingRefusal = { ok: false; reason: string };

export type SizingResult = Sizing | SizingRefusal;

const refuse = (reason: string): SizingRefusal => ({ ok: false, reason });

/**
 * Size one position, or refuse with a stated reason.
 *
 * Refusals are returned rather than thrown and are recorded by the caller, so
 * "the desk took no trades today" can be told apart from "the desk wanted to
 * and could not" — which are very different facts about a strategy.
 */
export function sizePosition(input: SizingInput): SizingResult {
  const { row, side, entry, stop, availableEquityUsd, bookEquityUsd } = input;

  if (!(entry > 0)) return refuse("no entry price");
  if (!(stop > 0)) return refuse("no stop price");
  if (side === "long" && stop >= entry) return refuse("long stop is at or above entry");
  if (side === "short" && stop <= entry) return refuse("short stop is at or below entry");
  if (bookEquityUsd <= 0) return refuse("book equity is exhausted");
  if (availableEquityUsd <= 0) return refuse("book has no free equity — all of it is posted as margin");

  const contractValue = row.micro.contractValue;
  if (!contractValue || contractValue <= 0) {
    return refuse("venue did not report a contract value — size cannot be computed");
  }

  // The margin spec decides where the venue liquidates. Without it there is no
  // honest way to check the stop survives, so the position is refused rather
  // than sized against an assumption.
  const mm = row.maintenanceMarginPct;
  if (mm === null || mm <= 0) {
    return refuse("no maintenance margin published for this contract — cannot verify the stop sits inside liquidation");
  }

  const stopDistanceFrac = Math.abs(entry - stop) / entry;
  if (!(stopDistanceFrac > 0)) return refuse("stop is at the entry price");

  // A stop tighter than the tick grid cannot be expressed as an order, which is
  // the same gate the Setups tab applies. Repeated here because a signal can
  // reach this function from a family that does not run that gate.
  if (!row.micro.stopExpressible) {
    return refuse(`tick grid cannot hold this stop (${row.micro.stopTicks ?? "?"} ticks)`);
  }

  const riskUsd = (bookEquityUsd * RISK_PER_TRADE_PCT) / 100;
  const idealNotional = riskUsd / stopDistanceFrac;
  const cappedNotional = Math.min(idealNotional, bookEquityUsd * MAX_NOTIONAL_X_EQUITY);

  const perContractUsd = entry * contractValue;
  const contracts = Math.floor(cappedNotional / perContractUsd);
  if (contracts < 1) {
    return refuse(
      `one contract is ${perContractUsd.toFixed(2)} USD of notional, more than this book's ` +
        `${cappedNotional.toFixed(0)} USD budget allows`,
    );
  }

  const quantity = contracts * contractValue;
  const notionalUsd = quantity * entry;

  // ── leverage ──────────────────────────────────────────────────────────────
  //
  // Two constraints pull in opposite directions and both are hard.
  //
  // From ABOVE: the highest leverage at which the venue's liquidation still
  // sits a safe multiple of the stop away. Solving
  //   1/L - mm/100 >= stopFrac * buffer
  // for L gives the ceiling below. Beyond it the venue closes the position
  // before the stop can, and the stop is decorative.
  //
  // From BELOW: the leverage needed to keep the posted margin inside this
  // position's share of the book. Less leverage than that ties up capital the
  // book needs for its other positions.
  //
  // When the two cross, SAFETY WINS and the position simply posts more margin.
  // If that no longer fits in free equity, it is refused — never sized at a
  // leverage that puts liquidation inside the stop.
  const denom = stopDistanceFrac * LIQUIDATION_BUFFER_X + mm / 100;
  const maxSafeLeverage = denom > 0 ? 1 / denom : 0;
  if (maxSafeLeverage < 1) {
    return refuse(
      `a ${(stopDistanceFrac * 100).toFixed(1)}% stop cannot be held even unleveraged against a ` +
        `${mm}% maintenance requirement — the venue would liquidate before the stop`,
    );
  }

  const venueMax = row.venueMaxLeverage ?? DESK_MAX_LEVERAGE;
  const marginBudget = Math.min(
    availableEquityUsd,
    (bookEquityUsd * MAX_MARGIN_PCT_OF_EQUITY) / 100,
  );
  const leverageForBudget = marginBudget > 0 ? Math.ceil(notionalUsd / marginBudget) : Infinity;

  const leverage = Math.max(
    1,
    Math.min(leverageForBudget, DESK_MAX_LEVERAGE, Math.max(1, venueMax), Math.floor(maxSafeLeverage)),
  );
  const marginUsd = notionalUsd / leverage;

  if (marginUsd > availableEquityUsd) {
    return refuse(
      `needs ${marginUsd.toFixed(0)} USD margin at ${leverage}x — the most this contract can ` +
        `safely take against a ${(stopDistanceFrac * 100).toFixed(1)}% stop — but only ` +
        `${availableEquityUsd.toFixed(0)} USD is free in this book`,
    );
  }

  // Distance to liquidation as a fraction of entry: the posted margin, less what
  // the venue keeps as maintenance. At 10x with a 2.5% maintenance requirement
  // that is 10% - 2.5% = 7.5%, and a 3% stop clears it; at 20x it is 2.5%, and
  // the same stop does not. Re-asserted after the clamping above rather than
  // assumed from it, because several caps interact and an off-by-one in any of
  // them would be silent.
  const liqDistanceFrac = 1 / leverage - mm / 100;
  if (liqDistanceFrac < stopDistanceFrac * LIQUIDATION_BUFFER_X) {
    return refuse(
      `liquidation sits ${(liqDistanceFrac * 100).toFixed(2)}% away but the stop is ` +
        `${(stopDistanceFrac * 100).toFixed(2)}% away — the venue would close this position ` +
        `before the stop could, so the stop would be decorative`,
    );
  }

  const liquidationPrice =
    side === "long" ? entry * (1 - liqDistanceFrac) : entry * (1 + liqDistanceFrac);

  return {
    ok: true,
    contracts,
    quantity,
    notionalUsd: round(notionalUsd, 2),
    leverage,
    marginUsd: round(marginUsd, 2),
    riskUsd: round(Math.abs(entry - stop) * quantity, 2),
    stopDistancePct: round(stopDistanceFrac * 100, 3),
    liquidationPrice: round(liquidationPrice, 8),
    liquidationDistancePct: round(liqDistanceFrac * 100, 3),
    maintenanceMarginPct: mm,
  };
}

function round(v: number, dp: number): number {
  const f = 10 ** dp;
  return Math.round(v * f) / f;
}

/**
 * Half the quoted spread, as a fraction — the slippage a taker pays crossing it.
 *
 * Charged on BOTH legs and in addition to the taker fee, because they are
 * different costs: the fee is what the venue takes, the spread is what the book
 * takes. Filling paper trades at the mid would flatter this desk by exactly the
 * amount that makes thin contracts look tradable — and spreads on this venue run
 * to 160 basis points, which is larger than most of the targets on the board.
 *
 * Falls back to a deliberately pessimistic 25 bps when no book is quoted: a
 * contract the venue will not quote is not one to assume a tight fill on.
 */
export function halfSpreadFraction(row: ScreenerRow): number {
  const bps = row.micro.spreadBps;
  if (bps === null || bps < 0) return 0.0025;
  return bps / 2 / 10_000;
}

/** Apply slippage against the position: a taker always fills on the wrong side. */
export function slipped(price: number, side: "long" | "short", entering: boolean, halfSpread: number): number {
  // Entering long and exiting short both BUY, so both pay the ask.
  const buying = (side === "long") === entering;
  return buying ? price * (1 + halfSpread) : price * (1 - halfSpread);
}
