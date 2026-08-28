/**
 * Crypto Positions — the domain model.
 *
 * WHAT THIS IS. The F&O Positions desk from the Indian-market app, rebuilt
 * against Delta Exchange: live option chains and perpetuals, buy or sell with
 * real quoted premiums, hedge-aware portfolio margin, across several paper
 * accounts each with its own editable balance. Paper money, real prices.
 *
 * WHAT DIFFERS FROM THE INDIAN DESK, and why each difference is forced rather
 * than chosen:
 *
 *   NO DATED FUTURES.  The Indian desk's Futures tab trades monthly contracts
 *                      with an expiry. Delta India lists none — 220 PERPETUALS
 *                      and nothing else. A perpetual has no expiry and pays
 *                      funding instead, so the tab lists perpetuals and shows
 *                      a funding rate where the Indian one shows an expiry.
 *                      Inventing an expiry column would be inventing a market.
 *
 *   PREMIUM IS PER UNIT OF UNDERLYING, NOT PER CONTRACT. Delta quotes an option
 *                      in USD per 1 unit of the underlying, and each contract
 *                      controls `contractValue` of it — 0.001 BTC, so one
 *                      contract of a $31,045 call costs $31.05. Verified
 *                      against intrinsic: deep-ITM calls mark at 1.005x
 *                      (spot − strike), which only holds on the per-unit
 *                      reading. Treating the quote as the per-contract price
 *                      would overstate every premium by a thousand times, and
 *                      it is the same trap the commodity desk fell into when it
 *                      read a broker lot size as a value multiplier.
 *
 *   CE/PE BECOME CALL/PUT. Same instrument, the venue's own vocabulary.
 *
 *   SETTLEMENT IS USD.  No rupees anywhere on this desk.
 *
 * NOTHING HERE CAN REACH A BROKER. This module holds no API key, signs no
 * request and has no order-routing path. Every price is real and every balance
 * is imaginary.
 */

/** An option, or a perpetual future. Delta India lists no dated futures. */
export type InstrumentKind = "OPTION" | "PERPETUAL";

export type OptionType = "CALL" | "PUT";

export type TransactionType = "BUY" | "SELL";

export type OrderType = "MARKET" | "LIMIT";

export type PositionStatus = "OPEN" | "CLOSED";

export type OrderStatus = "FILLED" | "REJECTED";

/** One tradable contract, resolved from Delta's product and ticker feeds. */
export type Instrument = {
  symbol: string;
  kind: InstrumentKind;
  /** BTC, ETH, XAUT. */
  underlying: string;
  /** ISO date (YYYY-MM-DD). Null on a perpetual, which never expires. */
  expiry: string | null;
  strike: number | null;
  optionType: OptionType | null;

  /**
   * Units of the underlying ONE contract controls. The multiplier for both
   * premium and P&L — see the module docstring.
   */
  contractValue: number;
  tickSize: number;

  /** USD per unit of underlying. */
  markPrice: number;
  bid: number | null;
  ask: number | null;
  spot: number | null;

  openInterest: number | null;
  turnoverUsd: number | null;
  change24hPct: number | null;
  /** Mark implied volatility, as a percent. Options only. */
  ivPct: number | null;
  greeks: Greeks | null;
  /** Percent per 8h settlement, signed: positive means longs pay. Perps only. */
  fundingRatePct8h: number | null;

  /**
   * The venue's own margin parameters, verbatim from the product feed.
   *
   * Not invented and not rounded: `initialMarginPct` is what decides the
   * maximum leverage the venue will allow (200x on BTCUSD, from 0.5%), and
   * `maintenanceMarginPct` is what decides where a position gets liquidated.
   * Guessing either would put the liquidation price somewhere the venue does
   * not agree with, which is the one number on this page that must not be
   * approximated.
   */
  initialMarginPct: number;
  maintenanceMarginPct: number;
  /**
   * Delta raises the requirement as a position grows, so a large book is not
   * margined at the headline rate. Percent added per USD of notional.
   */
  imScalingFactor: number;
  mmScalingFactor: number;
  /** What the venue defaults a new position to. */
  defaultLeverage: number;
  /** 100 / initialMarginPct — the venue's own ceiling, never a number of ours. */
  maxLeverage: number;
  /** Extra charged by the venue when it liquidates, not when you close. */
  penaltyFactor: number;
};

export type Greeks = {
  delta: number;
  gamma: number;
  theta: number;
  vega: number;
  rho: number;
};

/** One strike's call and put, side by side, as the chain renders them. */
export type ChainRow = {
  strike: number;
  call: Instrument | null;
  put: Instrument | null;
};

export type OptionChain = {
  underlying: string;
  expiry: string;
  spot: number | null;
  rows: ChainRow[];
  /** Strike nearest spot, so the UI can scroll the ladder to the money. */
  atmStrike: number | null;
  asOf: number;
};

export type Account = {
  accountId: string;
  name: string;
  /** Editable base capital. Default is DEFAULT_ACCOUNT_CAPITAL_USD. */
  initialCapital: number;
  createdAt: number;
};

export type Position = {
  positionId: string;
  accountId: string;
  kind: InstrumentKind;
  symbol: string;
  displayName: string;
  underlying: string;
  expiry: string | null;
  strike: number | null;
  optionType: OptionType | null;
  side: TransactionType;

  /** Contracts. `lots * contractValue` is the underlying quantity. */
  lots: number;
  contractValue: number;
  /** USD per unit of underlying, as quoted. */
  entryPrice: number;
  exitPrice: number | null;
  /** Signed premium in USD: negative paid, positive received. */
  premiumUsd: number;

  status: PositionStatus;
  /** Margin this position contributes standalone, before hedge netting. */
  standaloneMarginUsd: number;
  realizedPnl: number;
  feesUsd: number;

  /**
   * Leverage the position was opened at, as Delta records it per position.
   *
   * Meaningful on a perpetual and on a SHORT option, both of which post margin.
   * A BOUGHT option is always 1x: the premium is paid in full, there is nothing
   * borrowed, and no leverage setting changes that.
   */
  leverage: number;

  openedAt: number;
  closedAt: number | null;
};

/** A position with its live mark folded in. Never stored — always derived. */
export type LivePosition = Position & {
  markPrice: number | null;
  unrealizedPnl: number;
  /**
   * Market value of the CONTRACTS — mark x quantity, signed by side.
   *
   * On an option this is the premium, which is a small number: 0.125 BTC of
   * 77,600 puts is about $51 of premium. It is what the position is worth to
   * close, and it is NOT what the position controls.
   */
  valueUsd: number;
  /**
   * What the position actually controls, in USD: quantity x UNDERLYING spot.
   *
   * The number that answers "how big is this bet". The same 0.125 BTC of puts
   * above controls $9,700 of Bitcoin — nearly 200 times its own premium — and
   * reporting only the premium made a book running at ten times the account
   * look like it was risking a hundred dollars.
   *
   * On a perpetual the contract IS the underlying, so the two coincide.
   */
  notionalUsd: number;
  /** notionalUsd / account equity. How many times the account this position is. */
  accountMultiple: number | null;
  /**
   * The listed strike nearest spot for this leg's expiry, or null on a
   * perpetual. Equal to `strike` means a roll would do nothing.
   */
  atmStrike: number | null;
  /** Liquidation detail, or null where the position cannot be liquidated. */
  liquidation: Liquidation | null;
};

/**
 * Where a position gets closed out by the venue, and how far away that is.
 *
 * NULL FOR A BOUGHT OPTION, and that is not a gap in the data. Buying an option
 * pays the premium in full: there is no borrow, no margin call and no price at
 * which the venue takes the position away. The worst case is the premium, which
 * is already paid. Printing a liquidation price there would invent a risk that
 * does not exist.
 */
export type Liquidation = {
  /** Price at which maintenance margin is exhausted. */
  price: number;
  /** Price at which the whole posted margin is gone — past liquidation. */
  bankruptcyPrice: number;
  /** How far spot must move to get there, as a percent of the mark. */
  distancePct: number;
  maintenanceMarginUsd: number;
  initialMarginUsd: number;
  leverage: number;
  /** Charged by the venue on a liquidation, from liquidation_penalty_factor. */
  penaltyFactor: number;
  /**
   * What `price` is a price OF.
   *
   * "contract" for a perpetual, where the contract and the underlying are the
   * same thing. "underlying" for a short option, where the level that matters
   * is where SPOT has to go — quoting an option's own price there produced a
   * liquidation half a percent from the mark, which reads as imminent and means
   * nothing.
   */
  basis: "contract" | "underlying";
};

export type Order = {
  orderId: string;
  accountId: string;
  positionId: string | null;
  kind: InstrumentKind;
  symbol: string;
  displayName: string;
  transactionType: TransactionType;
  orderType: OrderType;
  lots: number;
  /** Null until filled. */
  fillPrice: number | null;
  limitPrice: number | null;
  status: OrderStatus;
  rejectReason: string | null;
  /** "ENTRY" or "EXIT", so the blotter can be read without inference. */
  intent: "ENTRY" | "EXIT";
  createdAt: number;
};

/** The account header: what the tiles across the top of the page show. */
export type PositionsSummary = {
  accountId: string;
  initialCapital: number;
  /** initialCapital + realized P&L. */
  balance: number;
  /** balance + unrealized P&L. */
  equity: number;
  realizedPnl: number;
  unrealizedPnl: number;
  roiPct: number;
  /** Null when nothing has closed yet — not zero, which would read as 0% wins. */
  winPct: number | null;
  openPositions: number;
  closedPositions: number;
  /** Portfolio margin actually held, after hedges net against each other. */
  deployedMargin: number;
  /** What the hedges saved: sum of standalone margins minus the netted figure. */
  marginBenefit: number;
  availableCash: number;
  totalFeesUsd: number;
  /**
   * Full notional of the open book: quantity x UNDERLYING spot, summed.
   *
   * Deliberately the underlying, not the contracts' own market value. Those
   * differ by orders of magnitude on options — a two-leg straddle worth $111 of
   * premium controls $19,400 of Bitcoin — and summing the premium made a book
   * running at 9.7x the account report 0.06x, which is the opposite of a
   * warning.
   *
   * Shown next to margin because the two answer different questions and are
   * routinely confused: margin is what the account has POSTED, exposure is what
   * it is HOLDING.
   */
  contractExposureUsd: number;
  /** Market value of the contracts themselves — premium, on an option book. */
  premiumValueUsd: number;
  /** Distinct underlyings with an open position. */
  underlyingsOpen: number;

  /** Total maintenance margin across the book — the account-level floor. */
  maintenanceMarginUsd: number;
  /**
   * equity / maintenance margin, as a percent. Null when nothing is posted.
   *
   * Null rather than zero or infinity: an account holding nothing has no margin
   * level, and rendering that as 0% would read as "about to be liquidated"
   * exactly when the account is safest.
   */
  marginLevelPct: number | null;
  /** Effective leverage of the whole book: exposure / equity. */
  accountLeverage: number | null;
  /** How many open positions could be liquidated at all. */
  liquidatablePositions: number;
};

/** What one contract of an underlying actually controls. */
export type ContractSpec = {
  underlying: string;
  /** Units of the underlying per contract, e.g. 0.001 BTC. */
  contractValue: number;
  /** What the quote means, spelled out. */
  priceUnit: string;
  optionCount: number;
  perpetualCount: number;
  expiryCount: number;
  spot: number | null;
  tickSize: number | null;
  /** Value of one contract at the current spot. */
  contractValueUsd: number | null;
  /** The venue's ceiling, from its published initial-margin rate. */
  maxLeverage: number;
  defaultLeverage: number;
  initialMarginPct: number;
  maintenanceMarginPct: number;
};

/** One leg's outcome inside a roll. */
export type RollLeg = {
  positionId: string;
  from: string;
  to: string;
  exitPrice: number;
  entryPrice: number;
  realizedPnl: number;
};

export type RollResult = {
  rolled: RollLeg[];
  realizedPnl: number;
  marginDelta: number;
  feesUsd: number;
  /** Groups left untouched, with the reason. */
  failed: { underlying: string; expiry: string; reason: string }[];
  note: string;
};

/** One leg of a basket, before it is priced. */
export type BasketLeg = {
  symbol: string;
  transactionType: TransactionType;
  lots: number;
  /** Omit to take the venue's default for the contract. */
  leverage?: number;
};

/** A basket priced against the account, without placing anything. */
export type BasketPreview = {
  legs: {
    symbol: string;
    displayName: string;
    side: TransactionType;
    lots: number;
    /** Underlying units: lots * contractValue. */
    quantity: number;
    price: number;
    /** Signed USD: negative paid, positive received. */
    premiumUsd: number;
    /** Effective leverage after the venue's own floor is applied. */
    leverage: number;
    maxLeverage: number;
    /** Null on a bought option, which cannot be liquidated. */
    liquidation: Liquidation | null;
  }[];
  /** Combined requirement, netted against what the account already holds. */
  marginRequired: number;
  /** Sum of the legs' standalone margins, for showing what hedging saved. */
  standaloneMargin: number;
  marginBenefit: number;
  /** Positive is a net credit received, negative a net debit paid. */
  netPremium: number;
  feesUsd: number;
  availableCash: number;
  affordable: boolean;
  /** Worst loss across the shocked spot grid, which is what margin prices. */
  worstCaseLossUsd: number;
  label: string;
};

/** A row of the Top Movers tab. */
export type TopMover = {
  symbol: string;
  displayName: string;
  underlying: string;
  expiry: string | null;
  strike: number | null;
  optionType: OptionType | null;
  markPrice: number;
  change24hPct: number | null;
  turnoverUsd: number | null;
  openInterest: number | null;
};

/** Starting balance for a new paper account. */
export const DEFAULT_ACCOUNT_CAPITAL_USD = 100_000;

/**
 * Delta's taker fee for options is capped at 10% of the premium.
 *
 * Without the cap a cheap option is untradeable on paper in a way it is not in
 * reality: 0.03% of NOTIONAL on a $0.50 premium whose notional is $78,000 would
 * charge $23 to buy 50 cents of optionality. The venue caps it for exactly that
 * reason, and a paper desk that skips the cap makes every far-OTM option look
 * like a loser on fees alone.
 */
export const OPTION_TAKER_FEE_RATE = 0.0003;
export const OPTION_FEE_CAP_FRACTION_OF_PREMIUM = 0.1;
export const PERP_TAKER_FEE_RATE = 0.0005;

/** Human label for a contract, e.g. "BTC 26SEP26 80000 CALL". */
export function displayNameOf(i: {
  underlying: string;
  kind: InstrumentKind;
  expiry: string | null;
  strike: number | null;
  optionType: OptionType | null;
  symbol: string;
}): string {
  if (i.kind === "PERPETUAL") return `${i.symbol} PERP`;
  const d = i.expiry ? formatExpiry(i.expiry) : "";
  return `${i.underlying} ${d} ${i.strike ?? ""} ${i.optionType ?? ""}`.replace(/\s+/g, " ").trim();
}

/** "2026-09-26" -> "26SEP26". */
export function formatExpiry(iso: string): string {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  if (!m) return iso;
  const months = ["JAN", "FEB", "MAR", "APR", "MAY", "JUN", "JUL", "AUG", "SEP", "OCT", "NOV", "DEC"];
  const mon = months[Number(m[2]) - 1] ?? m[2];
  return `${m[3]}${mon}${m[1].slice(2)}`;
}

/**
 * The fee for one fill, in USD.
 *
 * Options are charged on notional and capped at a fraction of the premium;
 * perpetuals are charged on notional outright.
 */
export function feeFor(
  kind: InstrumentKind,
  quantity: number,
  price: number,
  spot: number | null,
): number {
  if (kind === "PERPETUAL") return Math.abs(quantity * price) * PERP_TAKER_FEE_RATE;
  const notional = Math.abs(quantity * (spot ?? price));
  const premium = Math.abs(quantity * price);
  return Math.min(notional * OPTION_TAKER_FEE_RATE, premium * OPTION_FEE_CAP_FRACTION_OF_PREMIUM);
}
