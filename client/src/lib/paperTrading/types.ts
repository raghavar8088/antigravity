/**
 * The domain model shared by both paper-trading terminals.
 *
 * TWO VENUES, ONE ENGINE. Delta Paper Trading and Forex Paper Trading are the
 * same terminal pointed at different markets, so order handling, matching,
 * margin and liquidation are written once here and the venues supply only what
 * genuinely differs: where quotes come from, how size is expressed, and how the
 * cost of carry is charged.
 *
 * WHAT ACTUALLY DIFFERS BETWEEN THEM, and why each difference is real rather
 * than cosmetic:
 *
 *   SIZE.       Delta trades CONTRACTS whose underlying quantity is fixed per
 *               product (one BTCUSD contract is 0.001 BTC, one ADAUSD contract
 *               is 1 ADA — a thousandfold spread). Forex trades LOTS, where a
 *               standard lot is 100,000 units of the base currency. Neither can
 *               be expressed in the other's unit without lying about the tick.
 *
 *   POSITIONS.  Delta NETS: buying then selling the same contract leaves one
 *               position with an averaged entry. MetaTrader-style forex HEDGES:
 *               each fill is its own ticket and a long and a short on the same
 *               pair coexist. Collapsing the second into the first would make
 *               the forex terminal unable to do the thing its users do most.
 *
 *   CARRY.      A perpetual charges FUNDING every eight hours at a published
 *               rate. Forex charges a SWAP once a day at rollover, tripled on
 *               Wednesday to cover the weekend. Different mechanics, different
 *               schedule, different sign conventions.
 *
 *   SPREAD.     Delta publishes a real order book, so the spread is quoted and
 *               a market order can be walked through actual depth. No free
 *               forex feed publishes a broker's bid and ask, so the forex
 *               spread is MODELLED from a per-instrument table and every
 *               instrument carries `spreadIsModelled: true` so the UI can say
 *               so. A modelled spread presented as a quoted one would be the
 *               single most misleading thing either terminal could do.
 *
 * PAPER MONEY, REAL PRICES. Neither terminal holds an API key, signs a
 * request, or has any path to an order endpoint. Every price is real and every
 * balance is imaginary.
 */

export type VenueId = "delta" | "forex";

export type InstrumentKind = "perp" | "fx" | "metal" | "index" | "commodity" | "crypto";

/** How a venue expresses position size. */
export type SizeUnit = "contracts" | "lots";

/** Netting merges fills into one position per symbol; hedging keeps each ticket. */
export type PositionMode = "netting" | "hedging";

export type CarryKind = "funding" | "swap";

export type Instrument = {
  venue: VenueId;
  symbol: string;
  displayName: string;
  kind: InstrumentKind;
  /** What one unit of size controls: contract value, or units per lot. */
  contractSize: number;
  sizeUnit: SizeUnit;
  minSize: number;
  sizeStep: number;
  tickSize: number;
  /**
   * The unit a spread is quoted in. A pip on FX (the FOURTH decimal, or the
   * second on yen pairs), and one tick on a crypto perpetual. Carried per
   * instrument because it cannot be derived from the tick: brokers quote a
   * fractional pip, so tick is a tenth of pip on this desk.
   */
  pipSize: number;
  pricePrecision: number;
  maxLeverage: number;
  /** Percent of notional the venue keeps before it liquidates. */
  maintenanceMarginPct: number;

  takerFeeRate: number;
  makerFeeRate: number;
  /**
   * Per-lot, per-side commission in USD. Forex only, and zero on account types
   * that price their execution into the spread instead.
   */
  commissionPerLotUsd: number;

  carryKind: CarryKind;
  /** Perpetuals: percent per 8h settlement, signed (positive = longs pay). */
  fundingRatePct8h: number | null;
  /** Forex: points per day, per lot, signed from the position's own side. */
  swapLongPointsPerDay: number | null;
  swapShortPointsPerDay: number | null;

  last: number;
  bid: number;
  ask: number;
  markPrice: number;
  change24hPct: number | null;
  high24h: number | null;
  low24h: number | null;
  /** Rupee-free: everything on both desks settles in USD. */
  quoteCurrency: string;

  /**
   * TRUE when the bid and ask were derived from a mid price and a spread table
   * rather than quoted by the venue. The UI must surface this — see the module
   * docstring.
   */
  spreadIsModelled: boolean;
  /** Where the price came from, shown verbatim in the terminal. */
  source: string;
};

export type OrderSide = "buy" | "sell";

export type OrderType = "market" | "limit" | "stop_market" | "stop_limit";

export type TimeInForce = "GTC" | "IOC" | "FOK";

export type OrderStatus = "open" | "filled" | "partial" | "cancelled" | "rejected";

export type Order = {
  orderId: string;
  accountId: string;
  venue: VenueId;
  symbol: string;
  side: OrderSide;
  type: OrderType;
  /** In the venue's own unit: contracts for Delta, lots for forex. */
  size: number;
  limitPrice: number | null;
  stopPrice: number | null;
  timeInForce: TimeInForce;
  reduceOnly: boolean;
  postOnly: boolean;
  /** Bracket levels attached at entry; become position-level once filled. */
  takeProfit: number | null;
  stopLoss: number | null;

  status: OrderStatus;
  filledSize: number;
  avgFillPrice: number | null;
  feeUsd: number;
  /** True once a stop order's trigger has been touched. */
  triggered: boolean;

  createdAt: number;
  updatedAt: number;
  /** Bars have been replayed up to here, for resting orders. Unix seconds. */
  checkedTo: number;
  rejectReason: string | null;
  /** How the order filled, kept so a fill can be argued with rather than trusted. */
  fillNote: string | null;
};

export type Position = {
  positionId: string;
  accountId: string;
  venue: VenueId;
  symbol: string;
  side: "long" | "short";
  size: number;
  entryPrice: number;
  /** Weighted-average entry survives partial adds under netting. */
  leverage: number;
  marginUsd: number;
  liquidationPrice: number | null;
  takeProfit: number | null;
  stopLoss: number | null;

  /** Carry paid so far, signed from this position's side. */
  carryUsd: number;
  carryTo: number;
  feesUsd: number;

  openedAt: number;
  checkedTo: number;
  note: string | null;
};

export type TradeRecord = Omit<Position, "checkedTo"> & {
  exitPrice: number;
  exitReason: "manual" | "take_profit" | "stop_loss" | "liquidation" | "stop_out";
  closedAt: number;
  grossPnlUsd: number;
  netPnlUsd: number;
  returnPct: number;
  holdHours: number;
  /** Set when the exit level and the fill differ because price gapped past it. */
  gapped: boolean;
  ambiguous: boolean;
};

export type MarginMode = "isolated" | "cross";

/** Exness-style account tiers. Delta accounts always read `standard`. */
export type AccountType = "standard" | "raw" | "zero" | "pro";

export type Account = {
  accountId: string;
  venue: VenueId;
  currency: string;
  /** Realised cash. Equity is this plus open P&L, and is derived, never stored. */
  balance: number;
  startingBalance: number;
  /** Account-wide leverage cap. Forex sets this; Delta uses per-order leverage. */
  leverage: number;
  marginMode: MarginMode;
  accountType: AccountType;
  /**
   * Margin level below which open positions are closed out, in percent.
   * Zero disables it, which is how some real accounts are configured.
   */
  stopOutLevelPct: number;
  marginCallLevelPct: number;
  createdAt: number;
  resetCount: number;
};

/** Everything the terminal needs to render an account's state at one instant. */
export type AccountSnapshot = {
  account: Account;
  balance: number;
  equity: number;
  usedMargin: number;
  freeMargin: number;
  /** equity / usedMargin x 100. Null when nothing is open, not zero. */
  marginLevelPct: number | null;
  unrealisedPnlUsd: number;
  openPositions: number;
  openOrders: number;
  totalCarryUsd: number;
  totalFeesUsd: number;
};

/** One level of an order book. */
export type BookLevel = { price: number; size: number };

export type OrderBook = {
  symbol: string;
  bids: BookLevel[];
  asks: BookLevel[];
  /** Unix ms. */
  asOf: number;
  /** True when the book was derived from a mid and a spread rather than quoted. */
  modelled: boolean;
};

export type Candle = {
  time: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
};

/** What a venue adapter must provide for the shared engine to run on it. */
export type VenueAdapter = {
  id: VenueId;
  label: string;
  /** One-line description of where the numbers come from, shown in the UI. */
  dataNote: string;
  positionMode: PositionMode;
  sizeUnit: SizeUnit;
  defaultStartingBalance: number;
  defaultLeverage: number;
  leverageChoices: number[];
  accountTypes: { key: AccountType; label: string; note: string }[];
  stopOutLevelPct: number;
  marginCallLevelPct: number;

  listInstruments(): Promise<Instrument[]>;
  getInstrument(symbol: string): Promise<Instrument | null>;
  getBook(symbol: string, depth: number): Promise<OrderBook | null>;
  getCandles(symbol: string, resolution: string, from: number, to: number): Promise<Candle[]>;
  /** Resolutions this venue can answer, coarsest last. */
  resolutions: { key: string; label: string; seconds: number }[];
};

/** Size in the instrument's own unit converted to units of the underlying. */
export function quantityOf(instrument: Instrument, size: number): number {
  return size * instrument.contractSize;
}

/** USD notional of a position. */
export function notionalOf(instrument: Instrument, size: number, price: number): number {
  return quantityOf(instrument, size) * price;
}

/**
 * The value of one price point for one unit of size, in USD.
 *
 * This is what makes a forex P&L legible: on EURUSD one pip on one standard lot
 * is $10, and the number falls out of `contractSize x tickSize` rather than
 * being tabulated per pair.
 */
export function tickValueUsd(instrument: Instrument, size: number): number {
  return quantityOf(instrument, size) * instrument.tickSize;
}
