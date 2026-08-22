/**
 * The four readings a perpetual gives you that an equity screener cannot have:
 * funding, open interest, basis and the order book.
 *
 * THIS FILE IS WHY THE CRYPTO SCREENER IS RICHER THAN THE STOCK ONE, and the
 * reasons are structural rather than a matter of effort.
 *
 *   FUNDING       An equity has no funding rate. A perpetual has no expiry, so
 *                 the venue pins it to spot by making one side pay the other
 *                 every eight hours. That payment is a direct, published,
 *                 continuously-updated price of positioning — it says which
 *                 side is crowded and what it costs them to stay there. The
 *                 equity module has nothing remotely like it.
 *
 *   OPEN INTEREST The Stock Screener lists F&O buildup as a tier-2 reason and
 *                 then reports it as unavailable for every row, because NSE
 *                 stock-level OI is not in that app's data path. Delta
 *                 publishes OI and its 6-hour change for all ~220 contracts in
 *                 the same call as the price. The whole price-versus-OI
 *                 quadrant — long buildup, short buildup, long unwinding,
 *                 short covering — is therefore computed here for the entire
 *                 universe, live.
 *
 *   BASIS         Perp mark against spot. There is no equity analogue for a
 *                 cash instrument.
 *
 *   MICROSTRUCTURE The equity module explicitly refuses to report order-book
 *                 strength, on the grounds that it has no book and a proxy
 *                 dressed up as one would be a lie. Delta returns best bid,
 *                 best ask and both sizes for every contract, so the number it
 *                 declined to invent is simply available here.
 *
 * WINDOW ALIGNMENT IS ENFORCED, NOT ASSUMED. The venue reports OI change over
 * SIX HOURS and price change over TWENTY-FOUR. Pairing those two into a
 * "buildup" would be the exact error the equity module's sector code was fixed
 * for — two different measurements sitting in one row implying they describe
 * the same window. So the buildup classifier takes a 6h price change, derived
 * from hourly bars, and refuses to classify at all when it does not have one.
 */

import type { PerpTicker } from "./delta";
import { round } from "./horizons";

/** Delta settles funding every 8 hours: 3 payments a day, 1,095 a year. */
export const FUNDING_INTERVALS_PER_DAY = 3;
export const FUNDING_INTERVALS_PER_YEAR = FUNDING_INTERVALS_PER_DAY * 365;

/**
 * Taker fee per side on Delta India perpetuals, INCLUDING GST.
 *
 * Kept identical to `delta.PerpTakerFeeRate` in the Go engine so the screener's
 * net reward-to-risk and the live desk's realised P&L are computed against the
 * same schedule. A screener that ranked setups on a cheaper fee than the desk
 * actually pays would promote trades the desk then loses money on — which is
 * precisely how this codebase's equity side once turned +23.5k into -33.6k the
 * day real costs were backfilled.
 */
export const PERP_TAKER_FEE_RATE = 0.00059;

/**
 * Minimum stop width in ticks before a contract is considered able to hold a
 * stop at all.
 *
 * Twenty, matching the live engine's own grid gate. Under it, a single rounding
 * step moves the stop by more than 5% of the planned risk, so the order that
 * reaches the venue is materially not the order the plan described. This is a
 * claim about the CONTRACT, not about the market: a tick size is a property of
 * the instrument and does not improve when conditions do.
 */
export const MIN_STOP_TICKS = 20;

// ── funding ─────────────────────────────────────────────────────────────────

export type FundingRead = {
  /** Percent per 8h funding interval, as the venue publishes it. */
  ratePct8h: number | null;
  /** The same rate compounded out to a year, in percent. Simple, not compounded. */
  annualisedPct: number | null;
  /** Percent per day. */
  dailyPct: number | null;
  /** Who pays: `longs` when positive, `shorts` when negative. */
  payer: "longs" | "shorts" | "flat" | "unknown";
  /** One-line reading in plain words. */
  text: string;
  /** How extreme this rate is against the whole universe, 0-100. Filled by the board. */
  percentile: number | null;
};

export function readFunding(t: PerpTicker): FundingRead {
  const r = t.fundingRatePct8h;
  if (r === null) {
    return {
      ratePct8h: null,
      annualisedPct: null,
      dailyPct: null,
      payer: "unknown",
      text: "The venue did not publish a funding rate for this contract.",
      percentile: null,
    };
  }
  const annual = r * FUNDING_INTERVALS_PER_YEAR;
  const daily = r * FUNDING_INTERVALS_PER_DAY;
  const payer: FundingRead["payer"] = r > 0.0005 ? "longs" : r < -0.0005 ? "shorts" : "flat";

  let text: string;
  if (payer === "flat") {
    text = "Funding is effectively flat — neither side is paying to hold this contract.";
  } else if (payer === "longs") {
    text =
      `Longs pay shorts ${Math.abs(r).toFixed(4)}% every 8h (${Math.abs(annual).toFixed(1)}%/yr). ` +
      `The crowd is long and it is costing them to stay there.`;
  } else {
    text =
      `Shorts pay longs ${Math.abs(r).toFixed(4)}% every 8h (${Math.abs(annual).toFixed(1)}%/yr). ` +
      `The crowd is short and it is costing them to stay there.`;
  }

  return {
    ratePct8h: round(r, 5),
    annualisedPct: round(annual, 2),
    dailyPct: round(daily, 4),
    payer,
    text,
    percentile: null,
  };
}

/**
 * Funding actually paid or received over a hold, as a percentage of notional.
 *
 * Signed from the LONG's point of view: positive means the position paid.
 * Rounded up to whole intervals, because funding is charged at the settlement
 * stamp and not pro-rata — a position open for one hour across a settlement
 * pays a full interval, and a plan that assumed one eighth of one would
 * understate the cost of exactly the short holds this desk favours.
 */
export function fundingCostPct(ratePct8h: number | null, holdHours: number, side: "long" | "short"): number | null {
  if (ratePct8h === null || holdHours <= 0) return null;
  const intervals = Math.max(1, Math.ceil(holdHours / 8));
  const paid = ratePct8h * intervals;
  return round(side === "long" ? paid : -paid, 5);
}

// ── open interest ───────────────────────────────────────────────────────────

export type BuildupKind =
  | "long_buildup"
  | "short_buildup"
  | "long_unwinding"
  | "short_covering"
  | "flat"
  | "unclassified";

export type OiRead = {
  oi: number | null;
  oiContracts: number | null;
  oiValueUsd: number | null;
  oiChangeUsd6h: number | null;
  /** 6h OI change as a share of total OI notional, in percent. */
  oiChangePct6h: number | null;
  /** The 6h price change this classification was actually made against. */
  priceChangePct6h: number | null;
  buildup: BuildupKind;
  buildupLabel: string;
  text: string;
  /** OI notional divided by 24h turnover — how sticky the open book is. */
  oiToTurnover: number | null;
};

/**
 * The 6h open-interest change as a percentage OF THE BOOK IT STARTED FROM.
 *
 * Dividing by the CURRENT open interest is the obvious mistake and it produces
 * impossible numbers: a contract whose OI fell from $487k to $201k reports a
 * -$287k change against a $201k book, which prints as -143%. Open interest
 * cannot fall by more than all of itself, so any reader seeing that correctly
 * concludes the column is broken. The denominator has to be the OI at the START
 * of the window — current minus the change — which turns that same contract
 * into a legible -59%.
 *
 * Returns null rather than a clamped figure when the implied starting book is
 * non-positive, which would mean the venue's two fields disagree.
 */
export function oiChangePct6h(
  oiChangeUsd6h: number | null,
  oiValueUsd: number | null,
): number | null {
  if (oiChangeUsd6h === null || oiValueUsd === null) return null;
  const started = oiValueUsd - oiChangeUsd6h;
  if (started <= 0) return null;
  return (oiChangeUsd6h / started) * 100;
}

const BUILDUP_LABELS: Record<BuildupKind, string> = {
  long_buildup: "Long buildup",
  short_buildup: "Short buildup",
  long_unwinding: "Long unwinding",
  short_covering: "Short covering",
  flat: "No change",
  unclassified: "Not classified",
};

/**
 * The price-versus-open-interest quadrant.
 *
 * Price up + OI up      new money is going long. The move has positions behind it.
 * Price down + OI up    new money is going short. The decline has conviction.
 * Price up + OI down    shorts are closing. A squeeze, which exhausts rather than trends.
 * Price down + OI down  longs are closing. Liquidation or capitulation, not fresh selling.
 *
 * The distinction between the first and third is the single most useful thing
 * on this page: both are green candles, and they mean opposite things about
 * what happens next.
 *
 * `priceChangePct6h` is REQUIRED. Passing null returns `unclassified` rather
 * than falling back to the 24h change that happens to be at hand — a 6h OI
 * delta judged against a 24h price move is a statement about neither window.
 */
export function classifyBuildup(
  priceChangePct6h: number | null,
  oiChangeUsd6h: number | null,
  oiValueUsd: number | null,
): { kind: BuildupKind; text: string } {
  if (priceChangePct6h === null || oiChangeUsd6h === null) {
    return {
      kind: "unclassified",
      text:
        "Not classified. Open interest is reported over 6 hours, and this contract has no " +
        "aligned 6-hour price change — pairing it with the 24h move would describe neither window.",
    };
  }

  // A threshold on both axes, so noise does not get a label. The OI threshold is
  // relative to the contract's own book: $50k of OI change is a rounding error on
  // BTC and a regime change on a small alt.
  const oiPct = oiChangePct6h(oiChangeUsd6h, oiValueUsd);
  const oiMoved = oiPct !== null ? Math.abs(oiPct) >= 1 : Math.abs(oiChangeUsd6h) > 0;
  const priceMoved = Math.abs(priceChangePct6h) >= 0.25;

  if (!oiMoved || !priceMoved) {
    return {
      kind: "flat",
      text:
        "Neither price nor open interest moved enough over the last 6 hours to read as " +
        "positioning. Nothing is being built or unwound here.",
    };
  }

  const up = priceChangePct6h > 0;
  const oiUp = oiChangeUsd6h > 0;

  if (up && oiUp) {
    return {
      kind: "long_buildup",
      text:
        `Price +${priceChangePct6h.toFixed(2)}% over 6h with open interest UP ` +
        `${oiPct !== null ? oiPct.toFixed(1) + "%" : "$" + Math.abs(oiChangeUsd6h).toLocaleString()} — ` +
        `new money going long. The move has positions behind it rather than being a squeeze.`,
    };
  }
  if (!up && oiUp) {
    return {
      kind: "short_buildup",
      text:
        `Price ${priceChangePct6h.toFixed(2)}% over 6h with open interest UP ` +
        `${oiPct !== null ? oiPct.toFixed(1) + "%" : ""} — new money going short. ` +
        `This decline is being positioned into, not just sold into.`,
    };
  }
  if (up && !oiUp) {
    return {
      kind: "short_covering",
      text:
        `Price +${priceChangePct6h.toFixed(2)}% over 6h with open interest FALLING ` +
        `${oiPct !== null ? Math.abs(oiPct).toFixed(1) + "%" : ""} — shorts closing, not longs opening. ` +
        `A squeeze tends to exhaust when the shorts run out rather than continue.`,
    };
  }
  return {
    kind: "long_unwinding",
    text:
      `Price ${priceChangePct6h.toFixed(2)}% over 6h with open interest FALLING ` +
      `${oiPct !== null ? Math.abs(oiPct).toFixed(1) + "%" : ""} — longs closing out. ` +
      `Positions leaving, which is capitulation rather than fresh selling pressure.`,
  };
}

export function readOi(t: PerpTicker, priceChangePct6h: number | null): OiRead {
  const { kind, text } = classifyBuildup(priceChangePct6h, t.oiChangeUsd6h, t.oiValueUsd);
  const oiPct = oiChangePct6h(t.oiChangeUsd6h, t.oiValueUsd);
  return {
    oi: t.oi,
    oiContracts: t.oiContracts,
    oiValueUsd: t.oiValueUsd,
    oiChangeUsd6h: t.oiChangeUsd6h,
    oiChangePct6h: round(oiPct, 2),
    priceChangePct6h: round(priceChangePct6h, 2),
    buildup: kind,
    buildupLabel: BUILDUP_LABELS[kind],
    text,
    oiToTurnover:
      t.oiValueUsd && t.turnoverUsd24h && t.turnoverUsd24h > 0
        ? round(t.oiValueUsd / t.turnoverUsd24h, 2)
        : null,
  };
}

// ── basis ───────────────────────────────────────────────────────────────────

export type BasisRead = {
  markPrice: number | null;
  spotPrice: number | null;
  /** (mark/spot - 1) in basis points. Positive means the perp trades rich. */
  basisBps: number | null;
  state: "premium" | "discount" | "at_par" | "unknown";
  text: string;
};

/**
 * Basis reads as premium or discount at 5 bps, not at zero.
 *
 * Mark and spot are computed from different inputs and are never exactly equal;
 * a zero threshold would label every contract on the venue as being at a
 * premium or a discount, which tells the reader nothing.
 */
const BASIS_FLAT_BPS = 5;

export function readBasis(t: PerpTicker): BasisRead {
  let bps: number | null = null;
  if (t.markBasis !== null) bps = t.markBasis * 10_000;
  else if (t.markPrice !== null && t.spotPrice !== null && t.spotPrice > 0) {
    bps = (t.markPrice / t.spotPrice - 1) * 10_000;
  }

  if (bps === null) {
    return {
      markPrice: t.markPrice,
      spotPrice: t.spotPrice,
      basisBps: null,
      state: "unknown",
      text: "No basis available — the venue did not publish both a mark and a spot price.",
    };
  }

  const state: BasisRead["state"] =
    bps > BASIS_FLAT_BPS ? "premium" : bps < -BASIS_FLAT_BPS ? "discount" : "at_par";

  const text =
    state === "premium"
      ? `The perp trades ${bps.toFixed(1)} bps ABOVE spot. Leveraged demand is on the long side; ` +
        `funding is the mechanism that pulls it back.`
      : state === "discount"
        ? `The perp trades ${Math.abs(bps).toFixed(1)} bps BELOW spot. Leveraged demand is on the ` +
          `short side — the derivative is more bearish than the cash market.`
        : `Perp and spot are within ${BASIS_FLAT_BPS} bps of each other — no meaningful ` +
          `derivative premium either way.`;

  return { markPrice: t.markPrice, spotPrice: t.spotPrice, basisBps: round(bps, 2), state, text };
}

// ── microstructure ──────────────────────────────────────────────────────────

export type MicroRead = {
  bestBid: number | null;
  bestAsk: number | null;
  spreadBps: number | null;
  bidSize: number | null;
  askSize: number | null;
  /** (bid - ask) / (bid + ask), -1 to +1. Positive = more size resting on the bid. */
  bookImbalance: number | null;
  imbalanceLabel: string;

  tickSize: number | null;
  /** Tick size as basis points of price. The grid's coarseness. */
  tickBps: number | null;
  /** How many ticks wide a stop of `stopPct` would be on this contract. */
  stopTicks: number | null;
  /** False when the grid cannot express that stop within MIN_STOP_TICKS. */
  stopExpressible: boolean;
  gridNote: string;

  contractValue: number | null;
  /** USD value of one contract at the current price — the smallest ticket. */
  notionalPerContract: number | null;
  maxLeverage: number | null;

  /** Distance to the venue's own price limit band, in percent. */
  bandHeadroomUpPct: number | null;
  bandHeadroomDownPct: number | null;

  /** Round-trip taker fee as a percentage of notional. */
  roundTripFeePct: number;
  /**
   * The move needed just to clear the spread and both taker fees. Below this,
   * a trade is not a trade — it is a donation to the venue.
   */
  breakEvenMovePct: number | null;
};

/**
 * The stop width every grid check is measured at, in percent.
 *
 * 0.35% is the tight end of what the scalp desks on this venue actually use,
 * so it is the width that decides whether a contract is tradable AT ALL by the
 * strategies this app runs. Measuring the grid at a comfortable 3% stop would
 * pass almost every contract and answer a question nobody asked.
 */
export const GRID_PROBE_STOP_PCT = 0.35;

export function readMicro(t: PerpTicker, stopPct = GRID_PROBE_STOP_PCT): MicroRead {
  const bid = t.quotes.bestBid;
  const ask = t.quotes.bestAsk;
  const price = t.markPrice ?? t.close;

  let spreadBps: number | null = null;
  if (bid !== null && ask !== null && ask > bid && bid > 0) {
    spreadBps = ((ask - bid) / ((ask + bid) / 2)) * 10_000;
  }

  const bs = t.quotes.bidSize;
  const as = t.quotes.askSize;
  let imbalance: number | null = null;
  if (bs !== null && as !== null && bs + as > 0) imbalance = (bs - as) / (bs + as);

  const imbalanceLabel =
    imbalance === null
      ? "no book"
      : imbalance >= 0.33
        ? "bid heavy"
        : imbalance <= -0.33
          ? "ask heavy"
          : "balanced";

  const tick = t.tickSize;
  let tickBps: number | null = null;
  let stopTicks: number | null = null;
  if (tick !== null && tick > 0 && price !== null && price > 0) {
    tickBps = (tick / price) * 10_000;
    stopTicks = (price * (stopPct / 100)) / tick;
  }

  const expressible = stopTicks !== null && stopTicks >= MIN_STOP_TICKS;
  const gridNote =
    stopTicks === null
      ? "No tick size or price available — the grid cannot be assessed."
      : expressible
        ? `A ${stopPct}% stop is ${stopTicks.toFixed(1)} ticks wide. Comfortably expressible.`
        : `A ${stopPct}% stop is only ${stopTicks.toFixed(1)} ticks wide, under the ${MIN_STOP_TICKS}-tick ` +
          `floor. Rounding to the grid moves the stop by more than 5% of the intended risk, so the ` +
          `order that reaches the venue is not the order the plan described. This is a property of ` +
          `the contract and does not improve when the market calms down.`;

  const notionalPerContract =
    t.contractValue !== null && price !== null ? t.contractValue * price : null;

  let bandUp: number | null = null;
  let bandDown: number | null = null;
  if (price !== null && price > 0) {
    if (t.priceBandUpper !== null) bandUp = (t.priceBandUpper / price - 1) * 100;
    if (t.priceBandLower !== null) bandDown = (t.priceBandLower / price - 1) * 100;
  }

  const roundTripFeePct = PERP_TAKER_FEE_RATE * 2 * 100;
  const breakEven = spreadBps !== null ? spreadBps / 100 + roundTripFeePct : null;

  return {
    bestBid: bid,
    bestAsk: ask,
    spreadBps: round(spreadBps, 2),
    bidSize: bs,
    askSize: as,
    bookImbalance: round(imbalance, 3),
    imbalanceLabel,
    tickSize: tick,
    tickBps: round(tickBps, 2),
    stopTicks: round(stopTicks, 1),
    stopExpressible: expressible,
    gridNote,
    contractValue: t.contractValue,
    notionalPerContract: round(notionalPerContract, 4),
    maxLeverage: t.maxLeverage,
    bandHeadroomUpPct: round(bandUp, 2),
    bandHeadroomDownPct: round(bandDown, 2),
    roundTripFeePct: round(roundTripFeePct, 4)!,
    breakEvenMovePct: round(breakEven, 4),
  };
}

/**
 * A blunt 0-100 tradability score built only from contract properties.
 *
 * Deliberately NOT a view on the token. It answers a narrower question the
 * momentum board cannot: if this signal fires, can the trade actually be put
 * on at a sane cost. A contract can be the best mover on the venue and still
 * score near zero here, and that combination is exactly the trap the score
 * exists to expose.
 */
export function tradabilityScore(m: MicroRead, turnoverUsd24h: number | null): {
  score: number | null;
  blockers: string[];
} {
  const blockers: string[] = [];
  let score = 100;

  if (!m.stopExpressible) {
    score -= 45;
    blockers.push(
      `tick grid too coarse — a ${GRID_PROBE_STOP_PCT}% stop is ${m.stopTicks ?? "?"} ticks`,
    );
  }
  if (m.spreadBps === null) {
    score -= 20;
    blockers.push("no top of book quoted");
  } else if (m.spreadBps > 25) {
    score -= 30;
    blockers.push(`spread ${m.spreadBps.toFixed(0)} bps — crossing it costs more than most targets`);
  } else if (m.spreadBps > 10) {
    score -= 15;
    blockers.push(`spread ${m.spreadBps.toFixed(1)} bps is wide`);
  }

  if (turnoverUsd24h === null) {
    score -= 20;
    blockers.push("no 24h turnover reported");
  } else if (turnoverUsd24h < 50_000) {
    score -= 35;
    blockers.push(`24h turnover only $${Math.round(turnoverUsd24h).toLocaleString()} — a single order moves it`);
  } else if (turnoverUsd24h < 500_000) {
    score -= 15;
    blockers.push(`24h turnover $${Math.round(turnoverUsd24h).toLocaleString()} is thin`);
  }

  return { score: Math.max(0, Math.min(100, score)), blockers };
}
