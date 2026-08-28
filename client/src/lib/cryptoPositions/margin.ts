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

import type { Instrument, TransactionType } from "./types";

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
  const netPremium = legs.reduce((s, l) => s + legPremium(l), 0);
  const debitPaid = Math.max(0, -netPremium);
  const worst = worstCaseLoss(legs);
  const marginRequired = Math.max(0, worst - debitPaid);

  let standalone = 0;
  for (const leg of legs) {
    const p = legPremium(leg);
    const legDebit = Math.max(0, -p);
    standalone += Math.max(0, worstCaseLoss([leg]) - legDebit);
  }

  return {
    marginRequired,
    standaloneMargin: standalone,
    marginBenefit: Math.max(0, standalone - marginRequired),
    netPremium,
    worstCaseLossUsd: worst,
  };
}
