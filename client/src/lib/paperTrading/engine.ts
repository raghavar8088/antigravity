/**
 * The trading engine behind both terminals: place, cancel, close, and tick.
 *
 * ════════════════════════════════════════════════════════════════════════════
 * HOW TIME PASSES ON THESE DESKS
 * ════════════════════════════════════════════════════════════════════════════
 *
 * Neither terminal has a scheduled tick — this project's two Vercel cron slots
 * are already taken, and a third silently breaks webhooks. So a cycle runs when
 * the terminal is read.
 *
 * That is only honest because resting orders and open positions are resolved by
 * REPLAYING THE BARS since the desk last looked, not by comparing the current
 * price to a level. A limit order that filled on Tuesday is filled in Tuesday's
 * bar, at Tuesday's price; a stop that triggered overnight is stamped with the
 * bar that triggered it. Leaving the page shut delays DISCOVERY, not
 * correctness.
 *
 * The exception is carry — funding and swap accrue at the rate observed when
 * the desk next looks rather than the rate live at each settlement in between.
 * Both terminals say so on the page.
 *
 * ════════════════════════════════════════════════════════════════════════════
 * WHAT IS DELIBERATELY REFUSED
 * ════════════════════════════════════════════════════════════════════════════
 *
 * An order is rejected, with the reason shown, when: the market is closed; free
 * margin will not cover it; the size is below the venue's minimum or off its
 * step; a leveraged position's liquidation cannot be computed because the venue
 * published no maintenance margin; or the visible book cannot fill it. Every
 * one of those is a refusal a real venue would make, and swallowing any of them
 * would let this desk report fills nobody could get.
 */

import {
  accrueCarry,
  findLevelHit,
  findLiquidation,
  limitFill,
  liquidationPrice,
  marginUsd,
  notionalUsd,
  pnlUsd,
  roundSize,
  roundToTick,
  stopFill,
  usdPerQuoteUnit,
  walkBook,
} from "./matching";
import {
  accountIdFor,
  accounts,
  acquireLease,
  LIVE,
  listArchive,
  readArchive,
  restoreArchive,
  creditAccount,
  ensureAccount,
  orders as ordersCol,
  positionsCol,
  ptConfigured,
  PaperTradingUnavailable,
  readState,
  releaseLease,
  resetVenue,
  tradesCol,
  updateAccount,
} from "./store";
import type {
  Account,
  AccountSnapshot,
  Candle,
  Instrument,
  Order,
  OrderSide,
  OrderType,
  Position,
  TimeInForce,
  TradeRecord,
  VenueAdapter,
  VenueId,
} from "./types";
import { deltaVenue } from "./venues/delta";
import { forexVenue, marketOpen, nextOpenNote, pipSizeOf } from "./venues/forex";

const VENUES: Record<VenueId, VenueAdapter> = { delta: deltaVenue, forex: forexVenue };

export function getVenue(id: string): VenueAdapter {
  const v = VENUES[id as VenueId];
  if (!v) throw new OrderRejected(`unknown venue ${id}`);
  return v;
}

export class OrderRejected extends Error {}

const TICK_MIN_INTERVAL_MS = 20_000;
const TICK_LEASE_MS = 90_000;
/** Book depth requested for a fill walk. Deep enough for any size this desk allows. */
const FILL_DEPTH = 400;
/** Book depth shown in the terminal. */
export const DISPLAY_DEPTH = 14;

function nowSec(): number {
  return Math.floor(Date.now() / 1000);
}

function r(v: number, dp = 8): number {
  if (!Number.isFinite(v)) return 0;
  const f = 10 ** dp;
  return Math.round(v * f) / f;
}

function uid(prefix: string): string {
  return `${prefix}-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

/** Instruments plus a symbol->price map for quote-currency conversion. */
async function marketState(venue: VenueAdapter): Promise<{
  instruments: Instrument[];
  bySymbol: Map<string, Instrument>;
  quotes: Map<string, number>;
}> {
  const instruments = await venue.listInstruments();
  return {
    instruments,
    bySymbol: new Map(instruments.map((i) => [i.symbol, i])),
    quotes: new Map(instruments.map((i) => [i.symbol, i.last])),
  };
}

function conversion(inst: Instrument, quotes: Map<string, number>): number {
  const c = usdPerQuoteUnit(inst, quotes);
  if (c === null) {
    throw new OrderRejected(
      `no USD rate available for ${inst.symbol}'s quote currency, so a P&L on it cannot be stated ` +
        `in dollars. The order is refused rather than reported in the wrong currency.`,
    );
  }
  return c;
}

// ── snapshot ────────────────────────────────────────────────────────────────

export async function snapshot(venueId: string, tick = true) {
  const venue = getVenue(venueId);
  if (!ptConfigured()) {
    return {
      configured: false as const,
      venue: venueMeta(venue),
      reason:
        "MONGODB_URI is not set on this deployment, so the terminal has nowhere to keep orders and " +
        "positions. Prices and charts still work; placing an order does not.",
    };
  }

  const cycle = tick ? await maybeTick(venueId) : null;
  const account = await ensureAccount(venue.id, defaultsFor(venue));
  const { instruments, bySymbol, quotes } = await marketState(venue);

  const [pos, ords, trades] = await Promise.all([
    (await positionsCol()).find({ accountId: account.accountId, ...LIVE }).toArray(),
    (await ordersCol()).find({ accountId: account.accountId, status: "open", ...LIVE }).sort({ createdAt: -1 }).toArray(),
    (await tradesCol()).find({ accountId: account.accountId, ...LIVE }).sort({ closedAt: -1 }).limit(300).toArray(),
  ]);

  const marked = pos.map((p) => markPosition(p, bySymbol.get(p.symbol), quotes));
  const unrealised = marked.reduce((s, m) => s + (m.unrealisedUsd ?? 0), 0);
  const usedMargin = pos.reduce((s, p) => s + p.marginUsd, 0);
  const equity = account.balance + unrealised;

  const snap: AccountSnapshot = {
    account,
    balance: r(account.balance, 2),
    equity: r(equity, 2),
    usedMargin: r(usedMargin, 2),
    freeMargin: r(equity - usedMargin, 2),
    // Null, not zero, when nothing is open: a margin level of 0% means a
    // stopped-out account, which is the opposite of an idle one.
    marginLevelPct: usedMargin > 0 ? r((equity / usedMargin) * 100, 1) : null,
    unrealisedPnlUsd: r(unrealised, 2),
    openPositions: pos.length,
    openOrders: ords.length,
    totalCarryUsd: r(pos.reduce((s, p) => s + p.carryUsd, 0), 4),
    totalFeesUsd: r(pos.reduce((s, p) => s + p.feesUsd, 0), 4),
  };

  const closedStats = statsOf(trades);

  return {
    configured: true as const,
    venue: venueMeta(venue),
    account: snap,
    marginCall:
      snap.marginLevelPct !== null &&
      venue.marginCallLevelPct > 0 &&
      snap.marginLevelPct < venue.marginCallLevelPct,
    instruments,
    positions: marked,
    orders: ords.map((o) => ({ ...o, _id: undefined })),
    trades: trades.map((t) => ({ ...t, _id: undefined })),
    stats: closedStats,
    tick: await tickMeta(venue.id, cycle),
  };
}

function venueMeta(venue: VenueAdapter) {
  return {
    id: venue.id,
    label: venue.label,
    dataNote: venue.dataNote,
    positionMode: venue.positionMode,
    sizeUnit: venue.sizeUnit,
    leverageChoices: venue.leverageChoices,
    accountTypes: venue.accountTypes,
    stopOutLevelPct: venue.stopOutLevelPct,
    marginCallLevelPct: venue.marginCallLevelPct,
    resolutions: venue.resolutions,
    marketOpen: venue.id === "forex" ? marketOpen() : true,
    marketClosedNote: venue.id === "forex" ? nextOpenNote() : "",
  };
}

function defaultsFor(venue: VenueAdapter) {
  return {
    startingBalance: venue.defaultStartingBalance,
    leverage: venue.defaultLeverage,
    marginMode: "isolated" as const,
    accountType: venue.accountTypes[0]!.key,
    stopOutLevelPct: venue.stopOutLevelPct,
    marginCallLevelPct: venue.marginCallLevelPct,
  };
}

function markPosition(p: Position, inst: Instrument | undefined, quotes: Map<string, number>) {
  if (!inst) return { ...p, _id: undefined, mark: null, unrealisedUsd: null, unrealisedPct: null, pips: null };
  const conv = usdPerQuoteUnit(inst, quotes) ?? 1;
  // Marked at the price the position would CLOSE at — a long exits on the bid.
  const mark = p.side === "long" ? inst.bid : inst.ask;
  const gross = pnlUsd(inst, p.side, p.size, p.entryPrice, mark, conv);
  const net = gross - p.feesUsd - p.carryUsd;
  const pip = inst.venue === "forex" ? pipSizeOf(p.symbol) : inst.tickSize;
  return {
    ...p,
    _id: undefined,
    mark,
    unrealisedUsd: r(net, 2),
    unrealisedPct: p.marginUsd > 0 ? r((net / p.marginUsd) * 100, 2) : null,
    pips: pip > 0 ? r(((p.side === "long" ? mark - p.entryPrice : p.entryPrice - mark) / pip), 1) : null,
    notionalUsd: r(notionalUsd(inst, p.size, mark, conv), 2),
  };
}

function statsOf(trades: TradeRecord[]) {
  const wins = trades.filter((t) => t.netPnlUsd > 0);
  const losses = trades.filter((t) => t.netPnlUsd <= 0);
  const net = trades.reduce((s, t) => s + t.netPnlUsd, 0);
  const gw = wins.reduce((s, t) => s + t.netPnlUsd, 0);
  const gl = Math.abs(losses.reduce((s, t) => s + t.netPnlUsd, 0));
  return {
    trades: trades.length,
    wins: wins.length,
    losses: losses.length,
    winRate: trades.length ? r((wins.length / trades.length) * 100, 1) : null,
    netPnlUsd: r(net, 2),
    profitFactor: gl > 0 ? r(gw / gl, 2) : null,
    expectancyUsd: trades.length ? r(net / trades.length, 2) : null,
    bestUsd: trades.length ? r(Math.max(...trades.map((t) => t.netPnlUsd)), 2) : null,
    worstUsd: trades.length ? r(Math.min(...trades.map((t) => t.netPnlUsd)), 2) : null,
    carryUsd: r(trades.reduce((s, t) => s + t.carryUsd, 0), 2),
    feesUsd: r(trades.reduce((s, t) => s + t.feesUsd, 0), 2),
  };
}

async function tickMeta(venue: VenueId, cycle: unknown) {
  const st = await readState(venue);
  return {
    lastTickAt: st?.lastTickAt ?? null,
    lastTickMs: st?.lastTickMs ?? null,
    ticks: st?.ticks ?? 0,
    lastError: st?.lastError ?? null,
    thisRequest: cycle,
    note:
      "This terminal has no scheduled tick — the project's two Vercel cron slots are taken — so a " +
      "cycle runs when the page is read. Resting orders, take-profits, stops and liquidations are " +
      "resolved by REPLAYING the bars since the last check, so a fill that happened overnight is " +
      "recorded in the bar that produced it, at that price and time. Only carry accrues at the " +
      "rate seen when the desk next looks.",
  };
}

// ── placing ─────────────────────────────────────────────────────────────────

export type PlaceParams = {
  symbol: string;
  side: OrderSide;
  type: OrderType;
  size: number;
  limitPrice?: number | null;
  stopPrice?: number | null;
  leverage?: number | null;
  timeInForce?: TimeInForce;
  reduceOnly?: boolean;
  postOnly?: boolean;
  takeProfit?: number | null;
  stopLoss?: number | null;
};

export async function placeOrder(venueId: string, params: PlaceParams) {
  const venue = getVenue(venueId);
  const account = await ensureAccount(venue.id, defaultsFor(venue));
  const { bySymbol, quotes } = await marketState(venue);

  const inst = bySymbol.get(params.symbol.toUpperCase());
  if (!inst) throw new OrderRejected(`${params.symbol} is not listed on this venue`);

  if (venue.id === "forex" && !marketOpen() && params.type === "market") {
    throw new OrderRejected(nextOpenNote());
  }

  const size = roundSize(params.size, inst.sizeStep);
  if (!(size >= inst.minSize)) {
    throw new OrderRejected(
      `minimum size on ${inst.symbol} is ${inst.minSize} ${inst.sizeUnit}; ${params.size} rounds to ${size}`,
    );
  }

  const leverage = Math.max(1, Math.min(params.leverage ?? account.leverage, inst.maxLeverage));
  const conv = conversion(inst, quotes);

  const now = nowSec();
  const order: Order = {
    orderId: uid("ord"),
    accountId: account.accountId,
    venue: venue.id,
    symbol: inst.symbol,
    side: params.side,
    type: params.type,
    size,
    limitPrice: params.limitPrice != null ? roundToTick(params.limitPrice, inst.tickSize) : null,
    stopPrice: params.stopPrice != null ? roundToTick(params.stopPrice, inst.tickSize) : null,
    timeInForce: params.timeInForce ?? "GTC",
    reduceOnly: Boolean(params.reduceOnly),
    postOnly: Boolean(params.postOnly),
    takeProfit: params.takeProfit != null ? roundToTick(params.takeProfit, inst.tickSize) : null,
    stopLoss: params.stopLoss != null ? roundToTick(params.stopLoss, inst.tickSize) : null,
    status: "open",
    filledSize: 0,
    avgFillPrice: null,
    feeUsd: 0,
    triggered: false,
    createdAt: now,
    updatedAt: now,
    checkedTo: now,
    rejectReason: null,
    fillNote: null,
  };

  validateLevels(order, inst);

  if (order.type === "limit" || order.type === "stop_limit") {
    if (order.limitPrice === null) throw new OrderRejected("a limit order needs a limit price");
  }
  if (order.type === "stop_market" || order.type === "stop_limit") {
    if (order.stopPrice === null) throw new OrderRejected("a stop order needs a stop price");
  }

  // A market order executes now, against the book. Everything else rests and is
  // resolved by the tick's bar replay.
  if (order.type === "market") {
    const book = await venue.getBook(inst.symbol, FILL_DEPTH);
    if (!book) throw new OrderRejected(`no order book available for ${inst.symbol} right now`);
    const fill = walkBook(book, order.side, order.size);
    if (!fill) throw new OrderRejected(`the visible book on ${inst.symbol} has no depth to fill this`);
    if (fill.exhausted) {
      throw new OrderRejected(
        `the visible book on ${inst.symbol} covers only ${fill.filledSize} of ${order.size} ` +
          `${inst.sizeUnit} — the order is refused rather than filled at an invented price beyond ` +
          `the depth the venue published`,
      );
    }
    await executeFill(venue, account, inst, order, fill.avgPrice, leverage, conv, fill.note, true);
    order.status = "filled";
    order.filledSize = fill.filledSize;
    order.avgFillPrice = r(fill.avgPrice, inst.pricePrecision + 2);
    order.fillNote = fill.note;
    order.updatedAt = nowSec();
    const col = await ordersCol();
    await col.insertOne(order);
    return { order, fill };
  }

  // Resting orders reserve nothing up front, exactly as they do on both real
  // venues; the margin check happens when they fill. Rejecting at rest would
  // block an order the account can afford by the time it triggers.
  const col = await ordersCol();
  await col.insertOne(order);
  return { order, fill: null };
}

function validateLevels(order: Order, inst: Instrument) {
  const ref = order.limitPrice ?? order.stopPrice ?? (order.side === "buy" ? inst.ask : inst.bid);
  const isLong = order.side === "buy";
  if (order.takeProfit !== null) {
    if (isLong && order.takeProfit <= ref) {
      throw new OrderRejected("a long's take-profit has to sit above the entry");
    }
    if (!isLong && order.takeProfit >= ref) {
      throw new OrderRejected("a short's take-profit has to sit below the entry");
    }
  }
  if (order.stopLoss !== null) {
    if (isLong && order.stopLoss >= ref) {
      throw new OrderRejected("a long's stop-loss has to sit below the entry");
    }
    if (!isLong && order.stopLoss <= ref) {
      throw new OrderRejected("a short's stop-loss has to sit above the entry");
    }
  }
}

/**
 * Apply a fill: open, add to, reduce or flip a position, and charge the fee.
 *
 * NETTING versus HEDGING is decided by the venue. Under netting a fill on the
 * same symbol merges into one position with a weighted-average entry, and an
 * opposing fill reduces or flips it — which is what Delta does. Under hedging
 * every fill is its own ticket and a long and a short coexist, which is what
 * the platform the forex desk clones does.
 */
async function executeFill(
  venue: VenueAdapter,
  account: Account,
  inst: Instrument,
  order: Order,
  price: number,
  leverage: number,
  conv: number,
  note: string,
  taker: boolean,
): Promise<void> {
  const pcol = await positionsCol();
  const side: "long" | "short" = order.side === "buy" ? "long" : "short";
  const feeRate = taker ? inst.takerFeeRate : inst.makerFeeRate;
  const notional = notionalUsd(inst, order.size, price, conv);
  const fee = notional * feeRate + inst.commissionPerLotUsd * order.size;

  const existing = await pcol.find({ accountId: account.accountId, symbol: inst.symbol, ...LIVE }).toArray();

  if (venue.positionMode === "netting" && existing.length > 0) {
    const p = existing[0]!;
    if (p.side === side) {
      // Add: weighted-average entry, margin and fees accumulate.
      const newSize = p.size + order.size;
      const newEntry = (p.entryPrice * p.size + price * order.size) / newSize;
      const newNotional = notionalUsd(inst, newSize, newEntry, conv);
      const newMargin = marginUsd(newNotional, p.leverage);

      // THE ADD PATH NEEDS THE SAME MARGIN CHECK AS A NEW POSITION. It did not
      // have one, and that is a real hole rather than a missing nicety: adding
      // to an existing position is the one way to grow exposure without going
      // through `openPosition`, so a 200,000-contract add worth $15.4m was
      // accepted against $8,441 of free margin. The check is on the INCREMENT,
      // because the margin already posted is not free to post again.
      const { equity, used } = await equityOf(account);
      const extraMargin = newMargin - p.marginUsd;
      if (extraMargin > equity - used) {
        throw new OrderRejected(
          `adding ${order.size} ${inst.sizeUnit} needs a further ${extraMargin.toFixed(2)} USD of ` +
            `margin at ${p.leverage}x, but only ${(equity - used).toFixed(2)} USD is free`,
        );
      }

      await pcol.updateOne(
        { positionId: p.positionId },
        {
          $set: {
            size: newSize,
            entryPrice: r(newEntry, inst.pricePrecision + 4),
            marginUsd: r(newMargin, 2),
            liquidationPrice: liquidationPrice(side, newEntry, p.leverage, inst.maintenanceMarginPct),
            feesUsd: r(p.feesUsd + fee, 6),
          },
        },
      );
      await chargeFee(account.accountId, fee);
      return;
    }

    // Opposing fill: reduce, close, or flip.
    const closeSize = Math.min(p.size, order.size);
    await realise(account, inst, p, closeSize, price, conv, "manual", nowSec(), fee * (closeSize / order.size), false, false);
    const remainder = order.size - closeSize;
    if (remainder > 0) {
      await openPosition(account, inst, side, remainder, price, leverage, conv, order, fee * (remainder / order.size), note);
    } else {
      await chargeFee(account.accountId, 0);
    }
    return;
  }

  await openPosition(account, inst, side, order.size, price, leverage, conv, order, fee, note);
}

async function openPosition(
  account: Account,
  inst: Instrument,
  side: "long" | "short",
  size: number,
  price: number,
  leverage: number,
  conv: number,
  order: Order,
  fee: number,
  note: string,
): Promise<void> {
  const notional = notionalUsd(inst, size, price, conv);
  const margin = marginUsd(notional, leverage);

  // Free margin is checked at the fill, not at rest. A position that cannot be
  // collateralised is refused rather than opened into a negative balance.
  const { equity, used } = await equityOf(account);
  if (margin > equity - used) {
    throw new OrderRejected(
      `needs ${margin.toFixed(2)} USD of margin at ${leverage}x but only ${(equity - used).toFixed(2)} ` +
        `USD is free`,
    );
  }

  const liq = liquidationPrice(side, price, leverage, inst.maintenanceMarginPct);
  if (inst.maintenanceMarginPct > 0 && liq === null) {
    throw new OrderRejected(
      `${leverage}x on ${inst.symbol} leaves no cushion before the venue's ${inst.maintenanceMarginPct}% ` +
        `maintenance requirement — the position would open already liquidatable`,
    );
  }

  const now = nowSec();
  const pcol = await positionsCol();
  await pcol.insertOne({
    positionId: uid("pos"),
    accountId: account.accountId,
    venue: inst.venue,
    symbol: inst.symbol,
    side,
    size,
    entryPrice: r(price, inst.pricePrecision + 4),
    leverage,
    marginUsd: r(margin, 2),
    liquidationPrice: liq === null ? null : r(liq, inst.pricePrecision + 4),
    takeProfit: order.takeProfit,
    stopLoss: order.stopLoss,
    carryUsd: 0,
    carryTo: now,
    feesUsd: r(fee, 6),
    openedAt: now,
    checkedTo: now,
    note,
  });
  await chargeFee(account.accountId, fee);
}

async function chargeFee(accountId: string, fee: number): Promise<void> {
  if (fee !== 0) await creditAccount(accountId, -fee);
}

async function equityOf(account: Account): Promise<{ equity: number; used: number }> {
  const pcol = await positionsCol();
  const pos = await pcol.find({ accountId: account.accountId, ...LIVE }).toArray();
  const used = pos.reduce((s, p) => s + p.marginUsd, 0);
  const acc = await (await accounts()).findOne({ accountId: account.accountId });
  const balance = acc?.balance ?? account.balance;
  // Unrealised P&L is deliberately NOT added here. Sizing a new position off
  // paper profit is how an account opens more risk than it can carry the moment
  // the profit evaporates; free margin for a NEW order is measured on realised
  // cash. The account-wide margin level in the snapshot does use equity,
  // because that is what the stop-out is measured against.
  return { equity: balance, used };
}

/** Close a position, book the trade, and return the cash to the account. */
async function realise(
  account: Account,
  inst: Instrument,
  p: Position,
  size: number,
  exitPrice: number,
  conv: number,
  reason: TradeRecord["exitReason"],
  at: number,
  extraFee: number,
  gapped: boolean,
  ambiguous: boolean,
): Promise<TradeRecord> {
  const portion = size / p.size;
  const gross = pnlUsd(inst, p.side, size, p.entryPrice, exitPrice, conv);
  const fees = p.feesUsd * portion + extraFee;
  const carry = p.carryUsd * portion;
  const net = gross - extraFee - carry;

  const trade: TradeRecord = {
    ...p,
    size,
    marginUsd: r(p.marginUsd * portion, 2),
    feesUsd: r(fees, 6),
    carryUsd: r(carry, 6),
    exitPrice: r(exitPrice, inst.pricePrecision + 4),
    exitReason: reason,
    closedAt: at,
    grossPnlUsd: r(gross, 4),
    netPnlUsd: r(net, 4),
    returnPct: p.marginUsd > 0 ? r((net / (p.marginUsd * portion)) * 100, 3) : 0,
    holdHours: r((at - p.openedAt) / 3_600, 2),
    gapped,
    ambiguous,
  };

  const tcol = await tradesCol();
  await tcol.insertOne(trade);

  // The entry fee was already taken from the balance when the position opened,
  // so only the exit fee, the carry and the gross move settle here. Charging
  // the entry fee twice is the easiest way to make a paper book drift.
  await creditAccount(account.accountId, gross - extraFee - carry);

  const pcol = await positionsCol();
  if (size >= p.size - 1e-9) {
    await pcol.deleteOne({ positionId: p.positionId });
  } else {
    await pcol.updateOne(
      { positionId: p.positionId },
      {
        $set: {
          size: r(p.size - size, 8),
          marginUsd: r(p.marginUsd * (1 - portion), 2),
          feesUsd: r(p.feesUsd * (1 - portion), 6),
          carryUsd: r(p.carryUsd * (1 - portion), 6),
        },
      },
    );
  }
  return trade;
}

// ── explicit actions ────────────────────────────────────────────────────────

export async function cancelOrder(venueId: string, orderId: string) {
  const venue = getVenue(venueId);
  const col = await ordersCol();
  const res = await col.findOneAndUpdate(
    { orderId, accountId: accountIdFor(venue.id), status: "open" },
    { $set: { status: "cancelled", updatedAt: nowSec() } },
    { returnDocument: "after" },
  );
  if (!res) throw new OrderRejected(`no open order ${orderId} on this desk`);
  return { order: { ...res, _id: undefined } };
}

export async function closePosition(venueId: string, positionId: string, size?: number | null) {
  const venue = getVenue(venueId);
  const account = await ensureAccount(venue.id, defaultsFor(venue));
  const pcol = await positionsCol();
  const p = await pcol.findOne({ positionId, accountId: account.accountId });
  if (!p) throw new OrderRejected(`no open position ${positionId} on this desk`);

  const { bySymbol, quotes } = await marketState(venue);
  const inst = bySymbol.get(p.symbol);
  if (!inst) throw new OrderRejected(`${p.symbol} is no longer listed, so it cannot be priced to close`);
  if (venue.id === "forex" && !marketOpen()) throw new OrderRejected(nextOpenNote());

  const conv = conversion(inst, quotes);
  const closing = size && size > 0 ? Math.min(roundSize(size, inst.sizeStep), p.size) : p.size;
  if (!(closing > 0)) throw new OrderRejected("nothing to close at that size");

  const book = await venue.getBook(inst.symbol, FILL_DEPTH);
  if (!book) throw new OrderRejected(`no order book available for ${inst.symbol} right now`);
  const exitSide: OrderSide = p.side === "long" ? "sell" : "buy";
  const fill = walkBook(book, exitSide, closing);
  if (!fill) throw new OrderRejected(`the visible book on ${inst.symbol} has no depth to close this`);

  const notional = notionalUsd(inst, closing, fill.avgPrice, conv);
  const fee = notional * inst.takerFeeRate + inst.commissionPerLotUsd * closing;

  // Carry is brought up to the moment of the close before the trade is booked,
  // so a position that sat through a settlement pays for it.
  const carry = accrueCarry(inst, p, nowSec(), conv, inst.markPrice);
  if (carry.usd !== 0) {
    await pcol.updateOne({ positionId: p.positionId }, { $set: { carryUsd: r(p.carryUsd + carry.usd, 6), carryTo: nowSec() } });
    p.carryUsd = r(p.carryUsd + carry.usd, 6);
  }

  const trade = await realise(account, inst, p, closing, fill.avgPrice, conv, "manual", nowSec(), fee, false, false);
  return { trade: { ...trade, _id: undefined }, fill };
}

export async function modifyPosition(
  venueId: string,
  positionId: string,
  patch: { takeProfit?: number | null; stopLoss?: number | null },
) {
  const venue = getVenue(venueId);
  const pcol = await positionsCol();
  const p = await pcol.findOne({ positionId, accountId: accountIdFor(venue.id) });
  if (!p) throw new OrderRejected(`no open position ${positionId} on this desk`);
  const inst = await venue.getInstrument(p.symbol);
  if (!inst) throw new OrderRejected(`${p.symbol} is no longer listed`);

  const set: Partial<Position> = {};
  if (patch.takeProfit !== undefined) {
    if (patch.takeProfit === null) set.takeProfit = null;
    else {
      const tp = roundToTick(patch.takeProfit, inst.tickSize);
      if (p.side === "long" && tp <= p.entryPrice) throw new OrderRejected("a long's take-profit has to sit above the entry");
      if (p.side === "short" && tp >= p.entryPrice) throw new OrderRejected("a short's take-profit has to sit below the entry");
      set.takeProfit = tp;
    }
  }
  if (patch.stopLoss !== undefined) {
    if (patch.stopLoss === null) set.stopLoss = null;
    else {
      const sl = roundToTick(patch.stopLoss, inst.tickSize);
      if (p.side === "long" && sl >= p.entryPrice) throw new OrderRejected("a long's stop-loss has to sit below the entry");
      if (p.side === "short" && sl <= p.entryPrice) throw new OrderRejected("a short's stop-loss has to sit above the entry");
      set.stopLoss = sl;
    }
  }
  await pcol.updateOne({ positionId }, { $set: set });
  return { positionId, ...set };
}

export async function setAccountSettings(
  venueId: string,
  patch: { leverage?: number; accountType?: Account["accountType"]; marginMode?: Account["marginMode"] },
) {
  const venue = getVenue(venueId);
  const account = await ensureAccount(venue.id, defaultsFor(venue));
  const set: Partial<Account> = {};
  if (patch.leverage) {
    if (!venue.leverageChoices.includes(patch.leverage)) {
      throw new OrderRejected(`this venue offers ${venue.leverageChoices.join(", ")}x`);
    }
    set.leverage = patch.leverage;
  }
  if (patch.accountType) {
    if (!venue.accountTypes.some((a) => a.key === patch.accountType)) {
      throw new OrderRejected(`this venue has no ${patch.accountType} account type`);
    }
    set.accountType = patch.accountType;
  }
  if (patch.marginMode) set.marginMode = patch.marginMode;
  await updateAccount(account.accountId, set);
  return { ...account, ...set };
}

export async function resetAccount(venueId: string) {
  const venue = getVenue(venueId);
  await ensureAccount(venue.id, defaultsFor(venue));
  return resetVenue(venue.id, venue.defaultStartingBalance);
}

/** Past lives of this desk, and the trades inside one of them. */
export async function archive(venueId: string, generation: number | null) {
  const venue = getVenue(venueId);
  if (generation === null) return { generations: await listArchive(venue.id) };
  return {
    generation,
    trades: (await readArchive(venue.id, generation)).map((t) => ({ ...t, _id: undefined })),
  };
}

export async function restoreGeneration(venueId: string, generation: number) {
  const venue = getVenue(venueId);
  try {
    return { restored: await restoreArchive(venue.id, generation) };
  } catch (e) {
    throw new OrderRejected(e instanceof Error ? e.message : "restore failed");
  }
}

// ── the tick ────────────────────────────────────────────────────────────────

export type Cycle = {
  ran: boolean;
  skippedReason?: string;
  ordersFilled: number;
  positionsClosed: number;
  carryCharged: number;
  liquidations: number;
  stopOuts: number;
  elapsedMs: number;
};

export async function maybeTick(venueId: string): Promise<Cycle> {
  try {
    return await runCycle(venueId, false);
  } catch (e) {
    return {
      ran: false,
      skippedReason: e instanceof Error ? e.message : "cycle failed",
      ordersFilled: 0,
      positionsClosed: 0,
      carryCharged: 0,
      liquidations: 0,
      stopOuts: 0,
      elapsedMs: 0,
    };
  }
}

export async function runCycle(venueId: string, force: boolean): Promise<Cycle> {
  const started = Date.now();
  const venue = getVenue(venueId);
  if (!ptConfigured()) throw new PaperTradingUnavailable("MONGODB_URI is not set on this deployment");

  const st = await readState(venue.id);
  if (!force && st && Date.now() - st.lastTickAt < TICK_MIN_INTERVAL_MS) {
    return { ran: false, skippedReason: "a cycle ran moments ago", ordersFilled: 0, positionsClosed: 0, carryCharged: 0, liquidations: 0, stopOuts: 0, elapsedMs: 0 };
  }
  if (!(await acquireLease(venue.id, TICK_LEASE_MS))) {
    return { ran: false, skippedReason: "another request is running the cycle", ordersFilled: 0, positionsClosed: 0, carryCharged: 0, liquidations: 0, stopOuts: 0, elapsedMs: 0 };
  }

  let ordersFilled = 0;
  let positionsClosed = 0;
  let carryCharged = 0;
  let liquidations = 0;
  let stopOuts = 0;
  let lastError: string | null = null;

  try {
    const account = await ensureAccount(venue.id, defaultsFor(venue));
    const { bySymbol, quotes } = await marketState(venue);
    const to = nowSec();

    const pcol = await positionsCol();
    const ocol = await ordersCol();
    const open = await pcol.find({ accountId: account.accountId, ...LIVE }).toArray();
    const resting = await ocol.find({ accountId: account.accountId, status: "open", ...LIVE }).toArray();

    // One bar series per symbol, shared by every order and position on it.
    const symbols = [...new Set([...open.map((p) => p.symbol), ...resting.map((o) => o.symbol)])];
    const earliest = Math.min(
      ...open.map((p) => p.checkedTo),
      ...resting.map((o) => o.checkedTo),
      to,
    );
    const bars = new Map<string, Candle[]>();
    await Promise.all(
      symbols.map(async (s) => {
        try {
          bars.set(s, await venue.getCandles(s, "5m", earliest, to));
        } catch {
          bars.set(s, []);
        }
      }),
    );

    // ── resting orders ────────────────────────────────────────────────────
    for (const o of resting) {
      const inst = bySymbol.get(o.symbol);
      const series = (bars.get(o.symbol) ?? []).filter((b) => b.time >= o.checkedTo);
      if (!inst || series.length === 0) continue;

      let triggered = o.triggered;
      let fillPrice: number | null = null;
      let at = 0;

      for (const bar of series) {
        if ((o.type === "stop_market" || o.type === "stop_limit") && !triggered) {
          const t = stopFill(bar, o.side, o.stopPrice!);
          if (t === null) continue;
          triggered = true;
          if (o.type === "stop_market") {
            fillPrice = t;
            at = bar.time;
            break;
          }
          // A stop-limit becomes a resting limit at the trigger; it may fill in
          // the same bar, but only if that bar also traded through the limit.
        }
        if (o.type === "limit" || (o.type === "stop_limit" && triggered)) {
          const f = limitFill(bar, o.side, o.limitPrice!);
          if (f !== null) {
            fillPrice = f;
            at = bar.time;
            break;
          }
        }
      }

      if (fillPrice === null) {
        await ocol.updateOne({ orderId: o.orderId }, { $set: { checkedTo: to, triggered, updatedAt: to } });
        continue;
      }

      try {
        const conv = conversion(inst, quotes);
        // A resting limit that fills is a MAKER fill; a triggered stop crosses
        // the book and pays taker. Charging one rate for both would misprice
        // every stop on the desk.
        const taker = o.type === "stop_market" || o.type === "stop_limit";
        await executeFill(venue, account, inst, o, fillPrice, account.leverage, conv,
          `Filled from the resting book by bar replay at ${new Date(at * 1000).toISOString()}.`, taker);
        await ocol.updateOne(
          { orderId: o.orderId },
          {
            $set: {
              status: "filled",
              filledSize: o.size,
              avgFillPrice: r(fillPrice, inst.pricePrecision + 2),
              triggered,
              checkedTo: to,
              updatedAt: at,
              fillNote: `Bar replay: ${o.type} filled at ${fillPrice} in the 5m bar stamped ${at}.`,
            },
          },
        );
        ordersFilled++;
      } catch (e) {
        await ocol.updateOne(
          { orderId: o.orderId },
          { $set: { status: "rejected", rejectReason: e instanceof Error ? e.message : "fill failed", checkedTo: to, updatedAt: to } },
        );
      }
    }

    // ── open positions ────────────────────────────────────────────────────
    for (const p of open) {
      const inst = bySymbol.get(p.symbol);
      if (!inst) continue;
      const conv = usdPerQuoteUnit(inst, quotes);
      if (conv === null) continue;
      const series = (bars.get(p.symbol) ?? []).filter((b) => b.time >= p.checkedTo);

      // Liquidation is checked FIRST: the venue closes the position before any
      // of the trader's own levels can be reached, and checking the stop first
      // would record an exit the trader never actually got.
      const liq = findLiquidation(series, p.side, p.liquidationPrice, p.checkedTo);
      if (liq) {
        const carry = accrueCarry(inst, p, liq.at, conv, inst.markPrice);
        p.carryUsd = r(p.carryUsd + carry.usd, 6);
        const fee = notionalUsd(inst, p.size, liq.fill, conv) * inst.takerFeeRate;
        await realise(account, inst, p, p.size, liq.fill, conv, "liquidation", liq.at, fee, true, false);
        liquidations++;
        positionsClosed++;
        continue;
      }

      const hit = findLevelHit(series, p.side, p.takeProfit, p.stopLoss, p.checkedTo);
      if (hit) {
        const carry = accrueCarry(inst, p, hit.at, conv, inst.markPrice);
        p.carryUsd = r(p.carryUsd + carry.usd, 6);
        const fee = notionalUsd(inst, p.size, hit.fill, conv) * inst.takerFeeRate + inst.commissionPerLotUsd * p.size;
        await realise(account, inst, p, p.size, hit.fill, conv, hit.reason, hit.at, fee, hit.gapped, hit.ambiguous);
        positionsClosed++;
        continue;
      }

      const carry = accrueCarry(inst, p, to, conv, inst.markPrice);
      if (carry.charges > 0) {
        await pcol.updateOne(
          { positionId: p.positionId },
          { $set: { carryUsd: r(p.carryUsd + carry.usd, 6), carryTo: to, checkedTo: to } },
        );
        carryCharged += carry.charges;
      } else {
        await pcol.updateOne({ positionId: p.positionId }, { $set: { checkedTo: to } });
      }
    }

    // ── account-wide stop out ─────────────────────────────────────────────
    //
    // Forex is managed on margin level rather than per-position maintenance:
    // below the stop-out threshold the broker closes the WORST position and
    // re-checks, repeating until the level recovers. Closing everything at once
    // would be a margin call, not a stop out, and would overstate the damage.
    if (venue.stopOutLevelPct > 0) {
      for (let guard = 0; guard < 20; guard++) {
        const cur = await pcol.find({ accountId: account.accountId, ...LIVE }).toArray();
        if (cur.length === 0) break;
        const used = cur.reduce((s, x) => s + x.marginUsd, 0);
        if (used <= 0) break;
        const acc = await (await accounts()).findOne({ accountId: account.accountId });
        const balance = acc?.balance ?? account.balance;
        const marked = cur.map((x) => {
          const inst = bySymbol.get(x.symbol);
          if (!inst) return { p: x, pnl: 0 };
          const c = usdPerQuoteUnit(inst, quotes) ?? 1;
          const mark = x.side === "long" ? inst.bid : inst.ask;
          return { p: x, pnl: pnlUsd(inst, x.side, x.size, x.entryPrice, mark, c) - x.feesUsd - x.carryUsd };
        });
        const equity = balance + marked.reduce((s, m) => s + m.pnl, 0);
        const level = (equity / used) * 100;
        if (level >= venue.stopOutLevelPct) break;

        const worst = marked.reduce((a, b) => (b.pnl < a.pnl ? b : a));
        const inst = bySymbol.get(worst.p.symbol);
        if (!inst) break;
        const c = usdPerQuoteUnit(inst, quotes) ?? 1;
        const exit = worst.p.side === "long" ? inst.bid : inst.ask;
        const fee = notionalUsd(inst, worst.p.size, exit, c) * inst.takerFeeRate + inst.commissionPerLotUsd * worst.p.size;
        await realise(account, inst, worst.p, worst.p.size, exit, c, "stop_out", to, fee, false, false);
        stopOuts++;
        positionsClosed++;
      }
    }
  } catch (e) {
    lastError = e instanceof Error ? e.message : String(e);
  } finally {
    await releaseLease(venue.id, { lastTickAt: Date.now(), lastTickMs: Date.now() - started, lastError });
  }

  return { ran: true, ordersFilled, positionsClosed, carryCharged, liquidations, stopOuts, elapsedMs: Date.now() - started };
}

export { PaperTradingUnavailable, ptConfigured };
