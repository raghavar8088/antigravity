/**
 * Hedge-aware portfolio margin.
 *
 * WHY SCENARIOS RATHER THAN A PER-LEG PERCENTAGE. The Indian desk this module
 * clones advertises SPAN-style margin: a bought option that caps a sold
 * option's risk lowers the margin blocked, the way a real broker does it. A
 * flat percentage of notional per leg cannot express that — it charges a bull
 * call spread the full margin of its short leg and reports no benefit for the
 * long one, which makes every hedged structure look unaffordable and pushes the
 * user toward naked positions. That is the opposite of what a margin system is
 * for.
 *
 * So margin is priced the way SPAN prices it: shock the underlying across a
 * range, revalue the WHOLE portfolio at each shock, and hold the worst loss.
 * Legs that offset each other offset in the scenario, and the benefit falls out
 * of the arithmetic instead of being a rule someone has to maintain.
 *
 * HOW OPTIONS ARE REVALUED UNDER A SHOCK. From the venue's own published
 * greeks, second order:
 *
 *     value(S + dS) ~= max(0, mark + delta*dS + 0.5*gamma*dS^2)
 *
 * Delta and gamma are Delta Exchange's, not ours, so the revaluation follows
 * the same model the venue is marking against. The floor at zero matters: the
 * quadratic term turns negative far from the money and a naive curve would
 * price an option below worthless, which would show a short position making
 * more than the premium it collected.
 *
 * WHAT THIS IS NOT. It is not the venue's real margin engine, and a live
 * account would be charged something different. It is a consistent, hedge-aware
 * model that is honest about direction and magnitude, which is what makes the
 * hedge benefit figure on the page mean anything.
 */

import type { Instrument, Liquidation, TransactionType } from "./types";

/* ── leverage and liquidation, the venue's way ───────────────────────────── */

/**
 * Delta raises the margin rate as a position grows, so a big position is not
 * charged the headline rate. Both rates come back as PERCENTS.
 */
export function marginRatesFor(i: Instrument, notionalUsd: number): { imPct: number; mmPct: number } {
  return {
    imPct: i.initialMarginPct + i.imScalingFactor * Math.abs(notionalUsd),
    mmPct: i.maintenanceMarginPct + i.mmScalingFactor * Math.abs(notionalUsd),
  };
}

/** The most leverage the venue permits on this contract at this size. */
export function maxLeverageFor(i: Instrument, notionalUsd: number): number {
  const { imPct } = marginRatesFor(i, notionalUsd);
  return Math.max(1, Math.floor(100 / Math.max(imPct, 0.0001)));
}

/**
 * Initial margin actually posted, in USD.
 *
 * Leverage divides the notional, but never below the venue's own floor: asking
 * for 200x on a contract the venue margins at 1% does not get 200x, it gets
 * 100x, and silently granting the request would understate the requirement and
 * put the liquidation price somewhere the venue does not agree with.
 */
export function initialMarginUsd(i: Instrument, notionalUsd: number, leverage: number): number {
  const { imPct } = marginRatesFor(i, notionalUsd);
  const floor = (Math.abs(notionalUsd) * imPct) / 100;
  const asked = Math.abs(notionalUsd) / Math.max(1, leverage);
  return Math.max(floor, asked);
}

export function maintenanceMarginUsd(i: Instrument, notionalUsd: number): number {
  const { mmPct } = marginRatesFor(i, notionalUsd);
  return (Math.abs(notionalUsd) * mmPct) / 100;
}

/**
 * Where the venue closes the position, and how far that is from here.
 *
 * Returns NULL for a bought option. That is the correct answer, not a missing
 * one: buying pays the premium in full, so there is no borrow, no margin call
 * and no price at which the position is taken away. Delta's own Live Engine
 * copy says the same thing. Printing a number there would invent a risk.
 *
 * For everything that DOES post margin, liquidation is where the loss has eaten
 * the posted margin down to the maintenance floor:
 *
 *     long   liq = entry * (1 - 1/leverage + mm)
 *     short  liq = entry * (1 + 1/leverage - mm)
 *
 * and the bankruptcy price is the same with the maintenance term dropped —
 * where the margin is gone entirely rather than merely insufficient.
 */
export function liquidationFor(
  i: Instrument,
  side: TransactionType,
  lots: number,
  entryPrice: number,
  leverage: number,
  markPrice: number | null,
): Liquidation | null {
  // A long option cannot be liquidated. Nothing is borrowed against it.
  if (i.kind === "OPTION" && side === "BUY") return null;

  const qty = lots * i.contractValue;
  const notional = Math.abs(qty * entryPrice);
  if (notional <= 0) return null;

  const im = initialMarginUsd(i, notional, leverage);
  const mm = maintenanceMarginUsd(i, notional);
  // The leverage the position is ACTUALLY carrying after the venue's floor.
  const effLev = notional / Math.max(im, 1e-9);
  const mmFrac = mm / notional;

  const long = side === "BUY";
  const price = long
    ? entryPrice * (1 - 1 / effLev + mmFrac)
    : entryPrice * (1 + 1 / effLev - mmFrac);
  const bankruptcyPrice = long ? entryPrice * (1 - 1 / effLev) : entryPrice * (1 + 1 / effLev);

  const ref = markPrice ?? entryPrice;
  return {
    price: Math.max(0, price),
    bankruptcyPrice: Math.max(0, bankruptcyPrice),
    distancePct: ref > 0 ? ((price - ref) / ref) * 100 : 0,
    maintenanceMarginUsd: mm,
    initialMarginUsd: im,
    leverage: effLev,
    penaltyFactor: i.penaltyFactor,
  };
}

/**
 * How far spot is shocked, and in how many steps.
 *
 * +/-20% because this is crypto: an equity scan range of 6% would leave a short
 * option looking nearly free to hold, and BTC has moved more than 20% in a day
 * inside the history this desk quotes from. Odd step count so that zero — the
 * unshocked book — is one of the points evaluated.
 */
export const SCAN_RANGE_PCT = 20;
export const SCAN_STEPS = 17;

export type MarginLeg = {
  symbol: string;
  underlying: string;
  side: TransactionType;
  /** Contracts. */
  lots: number;
  contractValue: number;
  /** Entry price for an existing position, or the live mark for a new leg. */
  price: number;
  instrument: Instrument;
  /** Chosen leverage. Ignored on a bought option, which is always 1x. */
  leverage: number;
};

/** The shocks applied, as fractions of spot. */
export function scanGrid(): number[] {
  const out: number[] = [];
  const half = (SCAN_STEPS - 1) / 2;
  for (let i = -half; i <= half; i++) out.push((i / half) * (SCAN_RANGE_PCT / 100));
  return out;
}

/** An option's value after a spot shock, floored at zero. */
function shockedOptionValue(i: Instrument, dS: number): number {
  const g = i.greeks;
  if (!g) {
    // No greeks published. Intrinsic is the only defensible fallback: assuming
    // the price does not move would price a short option as riskless.
    if (i.strike === null) return i.markPrice;
    const s = (i.spot ?? 0) + dS;
    return i.optionType === "CALL" ? Math.max(0, s - i.strike) : Math.max(0, i.strike - s);
  }
  return Math.max(0, i.markPrice + g.delta * dS + 0.5 * g.gamma * dS * dS);
}

/** A contract's value after a spot shock, in USD per unit of underlying. */
function shockedPrice(i: Instrument, dS: number): number {
  // A perpetual tracks spot one for one; there is nothing to model.
  if (i.kind === "PERPETUAL") return Math.max(0, i.markPrice + dS);
  return shockedOptionValue(i, dS);
}

/**
 * The portfolio's P&L at one shock, in USD, measured from the given prices.
 *
 * Signed by side: a short gains when the contract cheapens.
 */
export function portfolioPnlAtShock(legs: MarginLeg[], shockFraction: number): number {
  let pnl = 0;
  for (const leg of legs) {
    const spot = leg.instrument.spot ?? 0;
    const dS = spot * shockFraction;
    const next = shockedPrice(leg.instrument, dS);
    const qty = leg.lots * leg.contractValue;
    const move = (next - leg.price) * qty;
    pnl += leg.side === "BUY" ? move : -move;
  }
  return pnl;
}

/**
 * Worst loss across the scan, as a positive number. Zero when nothing loses.
 *
 * Shocks are applied per underlying but summed across the book, so a BTC option
 * and an ETH option are not assumed to move together. Correlating them would
 * hand out a diversification benefit this desk has not measured and cannot
 * stand behind.
 */
export function worstCaseLoss(legs: MarginLeg[]): number {
  if (legs.length === 0) return 0;
  const byUnderlying = new Map<string, MarginLeg[]>();
  for (const l of legs) {
    const arr = byUnderlying.get(l.underlying) ?? [];
    arr.push(l);
    byUnderlying.set(l.underlying, arr);
  }
  let total = 0;
  for (const group of byUnderlying.values()) {
    let worst = 0;
    for (const shock of scanGrid()) {
      const pnl = portfolioPnlAtShock(group, shock);
      if (pnl < worst) worst = pnl;
    }
    total += -worst;
  }
  return total;
}

/** Signed premium in USD for one leg: negative paid, positive received. */
export function legPremium(leg: MarginLeg): number {
  const cash = leg.price * leg.lots * leg.contractValue;
  return leg.side === "BUY" ? -cash : cash;
}

export type MarginResult = {
  /** Netted across the whole book. This is what is actually blocked. */
  marginRequired: number;
  /** What each leg would need alone, added up. */
  standaloneMargin: number;
  /** standaloneMargin - marginRequired. Never negative. */
  marginBenefit: number;
  netPremium: number;
  worstCaseLossUsd: number;
};

/**
 * Margin for a set of legs held together.
 *
 * The upfront debit is subtracted because it has already left the account as
 * cash. Without that, a long call would be charged margin equal to the premium
 * it just paid — the money would be counted against the account twice, and a
 * simple bought option would look twice as expensive as it is.
 */
export function marginFor(legs: MarginLeg[]): MarginResult {
  // PERPETUALS ARE MARGINED BY LEVERAGE, OPTIONS BY SCENARIO.
  //
  // This is what Delta actually does, and it is why the leverage control means
  // anything: a perpetual's requirement is its notional divided by the chosen
  // leverage, floored at the venue's own rate. Running perps through the option
  // scan instead would make the leverage setting decorative.
  //
  // The cost is that a perpetual hedging an option earns no offset here. That
  // matches Delta's default isolated margin, where the two are margined apart;
  // Delta's own portfolio-margin mode would net them, and this desk does not
  // model that mode.
  const perps = legs.filter((l) => l.instrument.kind === "PERPETUAL");
  const options = legs.filter((l) => l.instrument.kind === "OPTION");

  const perpMargin = perps.reduce(
    (s, l) => s + initialMarginUsd(l.instrument, l.lots * l.contractValue * l.price, l.leverage),
    0,
  );

  const netPremium = options.reduce((s, l) => s + legPremium(l), 0);
  const debitPaid = Math.max(0, -netPremium);
  const worst = worstCaseLoss(options);
  const optionMargin = Math.max(0, worst - debitPaid);

  let standalone = perpMargin;
  for (const leg of options) {
    const p = legPremium(leg);
    standalone += Math.max(0, worstCaseLoss([leg]) - Math.max(0, -p));
  }

  const marginRequired = perpMargin + optionMargin;
  return {
    marginRequired,
    standaloneMargin: standalone,
    marginBenefit: Math.max(0, standalone - marginRequired),
    netPremium,
    worstCaseLossUsd: worst,
  };
}

/** Total maintenance margin across a book — the floor the account must hold. */
export function bookMaintenanceMargin(legs: MarginLeg[]): number {
  return legs.reduce((s, l) => {
    // A bought option posts nothing and so has no maintenance floor.
    if (l.instrument.kind === "OPTION" && l.side === "BUY") return s;
    return s + maintenanceMarginUsd(l.instrument, l.lots * l.contractValue * l.price);
  }, 0);
}
