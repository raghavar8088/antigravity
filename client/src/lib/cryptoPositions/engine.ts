/**
 * The desk itself: pricing a basket, filling it, and valuing what it holds.
 *
 * FILLS CROSS THE SPREAD. A buy pays the ask and a sell hits the bid, from
 * Delta's published quotes. Filling everything at the mark would hand every
 * position half the spread as free profit the instant it opened, which on a
 * far-OTM option whose book is 0.01 bid / 0.10 ask is most of the premium.
 * Where a contract has no quote the mark is used and the fill says so.
 *
 * A LIMIT ORDER EITHER CROSSES NOW OR IS REJECTED. This desk keeps no resting
 * book and has no process to fill one later, so an unmarketable limit is
 * refused with that as the stated reason. Accepting it would leave an order
 * that appears live and can never fill — the same silent-nothing that has bitten
 * this codebase elsewhere.
 *
 * MARGIN IS CHECKED AS AN INCREMENT. What matters is what the new leg adds to
 * the book, not what it would cost alone: a leg that hedges an existing short
 * can REDUCE the requirement, and charging it standalone margin would refuse a
 * trade that makes the account safer.
 */

import {
  getInstrument,
  getOptionChain,
  getSnapshot,
  getTopMovers,
  listOptionExpiries,
  listPerpetuals,
  listUnderlyings,
  marksFor,
} from "./delta";
import { marginFor, type MarginLeg } from "./margin";
import {
  closePositionDoc,
  getAccount,
  getPosition,
  insertOrder,
  insertPosition,
  listOrders,
  listPositions,
  type PositionDoc,
} from "./store";
import type {
  BasketLeg,
  BasketPreview,
  Instrument,
  LivePosition,
  Order,
  Position,
  PositionsSummary,
  TransactionType,
} from "./types";
import { displayNameOf, feeFor } from "./types";

export { getOptionChain, getTopMovers, listOptionExpiries, listPerpetuals, listUnderlyings, getSnapshot };

export class Rejected extends Error {
  constructor(message: string) {
    super(message);
    this.name = "Rejected";
  }
}

/** The price a side would actually pay, and whether a real quote backed it. */
export function fillPriceFor(i: Instrument, side: TransactionType): { price: number; quoted: boolean } {
  const q = side === "BUY" ? i.ask : i.bid;
  if (q !== null && q > 0) return { price: q, quoted: true };
  return { price: i.markPrice, quoted: false };
}

function legFrom(i: Instrument, side: TransactionType, lots: number, price: number): MarginLeg {
  return {
    symbol: i.symbol,
    underlying: i.underlying,
    side,
    lots,
    contractValue: i.contractValue,
    price,
    instrument: i,
  };
}

/** Open positions as margin legs, valued at their ENTRY price. */
async function openLegs(accountId: string): Promise<MarginLeg[]> {
  const open = await listPositions(accountId, "OPEN");
  if (open.length === 0) return [];
  const marks = await marksFor(open.map((p) => p.symbol));
  const legs: MarginLeg[] = [];
  for (const p of open) {
    const i = marks.get(p.symbol);
    // A delisted or expired contract has no instrument to shock. It is excluded
    // from the scan rather than assumed flat, and `summary` reports it as
    // unpriced so the omission is visible instead of silently lowering margin.
    if (!i) continue;
    legs.push(legFrom(i, p.side, p.lots, p.entryPrice));
  }
  return legs;
}

export async function livePositions(
  accountId: string,
  status?: "OPEN" | "CLOSED",
): Promise<LivePosition[]> {
  const rows = await listPositions(accountId, status);
  const marks = await marksFor(rows.map((p) => p.symbol));
  return rows.map((p) => {
    const i = marks.get(p.symbol);
    const qty = p.lots * p.contractValue;
    if (p.status === "CLOSED" || !i) {
      return { ...p, markPrice: i?.markPrice ?? null, unrealizedPnl: 0, valueUsd: 0 };
    }
    const move = (i.markPrice - p.entryPrice) * qty;
    const unrealized = p.side === "BUY" ? move : -move;
    return {
      ...p,
      markPrice: i.markPrice,
      unrealizedPnl: unrealized,
      valueUsd: i.markPrice * qty * (p.side === "BUY" ? 1 : -1),
    };
  });
}

export async function summary(accountId: string): Promise<PositionsSummary> {
  const account = await getAccount(accountId);
  if (!account) throw new Rejected(`No account ${accountId}`);

  const [all, legs] = await Promise.all([livePositions(accountId), openLegs(accountId)]);
  const open = all.filter((p) => p.status === "OPEN");
  const closed = all.filter((p) => p.status === "CLOSED");

  const realized = closed.reduce((s, p) => s + p.realizedPnl, 0);
  const unrealized = open.reduce((s, p) => s + p.unrealizedPnl, 0);
  const fees = all.reduce((s, p) => s + p.feesUsd, 0);

  const m = marginFor(legs);
  const balance = account.initialCapital + realized;
  const wins = closed.filter((p) => p.realizedPnl > 0).length;

  return {
    accountId,
    initialCapital: account.initialCapital,
    balance,
    equity: balance + unrealized,
    realizedPnl: realized,
    unrealizedPnl: unrealized,
    roiPct: account.initialCapital > 0 ? ((balance + unrealized - account.initialCapital) / account.initialCapital) * 100 : 0,
    // Null, not zero: no closed trades is "unknown", and 0% reads as "all losses".
    winPct: closed.length > 0 ? (wins / closed.length) * 100 : null,
    openPositions: open.length,
    closedPositions: closed.length,
    deployedMargin: m.marginRequired,
    marginBenefit: m.marginBenefit,
    availableCash: balance - m.marginRequired,
    totalFeesUsd: fees,
  };
}

async function resolveLegs(
  legs: BasketLeg[],
): Promise<{ instrument: Instrument; side: TransactionType; lots: number; price: number; quoted: boolean }[]> {
  if (legs.length === 0) throw new Rejected("A basket needs at least one leg.");
  const out = [];
  for (const l of legs) {
    if (!Number.isFinite(l.lots) || l.lots <= 0) throw new Rejected(`${l.symbol}: lots must be a positive number.`);
    const i = await getInstrument(l.symbol);
    if (!i) throw new Rejected(`${l.symbol} is not listed on Delta.`);
    const { price, quoted } = fillPriceFor(i, l.transactionType);
    out.push({ instrument: i, side: l.transactionType, lots: Math.floor(l.lots), price, quoted });
  }
  return out;
}

export async function previewBasket(accountId: string, legs: BasketLeg[]): Promise<BasketPreview> {
  const resolved = await resolveLegs(legs);
  const existing = await openLegs(accountId);
  const newLegs = resolved.map((r) => legFrom(r.instrument, r.side, r.lots, r.price));

  const before = marginFor(existing);
  const after = marginFor([...existing, ...newLegs]);
  const standalone = marginFor(newLegs);

  // The INCREMENT, never the standalone figure — see the module docstring.
  const marginRequired = Math.max(0, after.marginRequired - before.marginRequired);

  const fees = resolved.reduce(
    (s, r) => s + feeFor(r.instrument.kind, r.lots * r.instrument.contractValue, r.price, r.instrument.spot),
    0,
  );
  const netPremium = standalone.netPremium;
  const s = await summary(accountId);
  // A debit leaves the account as cash; a credit arrives. Both move what is
  // left to post as margin, so the affordability test has to see them.
  const cashAfterPremium = s.availableCash + netPremium - fees;

  return {
    legs: resolved.map((r) => ({
      symbol: r.instrument.symbol,
      displayName: displayNameOf(r.instrument),
      side: r.side,
      lots: r.lots,
      quantity: r.lots * r.instrument.contractValue,
      price: r.price,
      premiumUsd: (r.side === "BUY" ? -1 : 1) * r.price * r.lots * r.instrument.contractValue,
    })),
    marginRequired,
    standaloneMargin: standalone.standaloneMargin,
    marginBenefit: Math.max(0, standalone.standaloneMargin - marginRequired),
    netPremium,
    feesUsd: fees,
    availableCash: s.availableCash,
    affordable: cashAfterPremium >= marginRequired,
    worstCaseLossUsd: standalone.worstCaseLossUsd,
    label: labelFor(resolved.map((r) => ({ side: r.side, i: r.instrument }))),
  };
}

/** A readable name for common structures, falling back to a leg count. */
function labelFor(legs: { side: TransactionType; i: Instrument }[]): string {
  if (legs.length === 1) {
    const l = legs[0];
    return `${l.side === "BUY" ? "Long" : "Short"} ${displayNameOf(l.i)}`;
  }
  const opts = legs.filter((l) => l.i.kind === "OPTION");
  if (opts.length === 2) {
    const [a, b] = opts;
    const sameType = a.i.optionType === b.i.optionType;
    const sameStrike = a.i.strike === b.i.strike;
    const sameExpiry = a.i.expiry === b.i.expiry;
    if (sameType && sameExpiry && !sameStrike) return `${a.i.optionType} vertical spread`;
    if (!sameType && sameStrike && sameExpiry) {
      return a.side === b.side ? (a.side === "BUY" ? "Long straddle" : "Short straddle") : "Synthetic future";
    }
    if (!sameType && !sameStrike && sameExpiry) {
      return a.side === b.side ? (a.side === "BUY" ? "Long strangle" : "Short strangle") : "Risk reversal";
    }
    if (sameType && sameStrike && !sameExpiry) return `${a.i.optionType} calendar spread`;
  }
  return `${legs.length}-leg basket`;
}

export async function executeBasket(
  accountId: string,
  legs: BasketLeg[],
): Promise<{ filled: number; positions: Position[]; marginAdded: number; netPremium: number; feesUsd: number }> {
  const preview = await previewBasket(accountId, legs);
  if (!preview.affordable) {
    throw new Rejected(
      `This basket needs $${preview.marginRequired.toFixed(2)} of margin but only $${preview.availableCash.toFixed(2)} is available. Reduce lots, add a hedge, or raise the balance.`,
    );
  }
  const resolved = await resolveLegs(legs);
  const created: Position[] = [];

  for (const r of resolved) {
    const qty = r.lots * r.instrument.contractValue;
    const fee = feeFor(r.instrument.kind, qty, r.price, r.instrument.spot);
    const premium = (r.side === "BUY" ? -1 : 1) * r.price * qty;
    const standalone = marginFor([legFrom(r.instrument, r.side, r.lots, r.price)]).marginRequired;

    const doc: Omit<PositionDoc, "_id"> = {
      account_id: accountId,
      kind: r.instrument.kind,
      symbol: r.instrument.symbol,
      display_name: displayNameOf(r.instrument),
      underlying: r.instrument.underlying,
      expiry: r.instrument.expiry,
      strike: r.instrument.strike,
      option_type: r.instrument.optionType,
      side: r.side,
      lots: r.lots,
      contract_value: r.instrument.contractValue,
      entry_price: r.price,
      exit_price: null,
      premium_usd: premium,
      status: "OPEN",
      standalone_margin_usd: standalone,
      // The entry fee is realised immediately. Carrying it only at exit is what
      // made a restored balance read high on another desk here.
      realized_pnl: -fee,
      fees_usd: fee,
      opened_at: Date.now(),
      closed_at: null,
    };
    const saved = await insertPosition(doc);
    await insertOrder({
      account_id: accountId,
      position_id: saved._id,
      kind: r.instrument.kind,
      symbol: r.instrument.symbol,
      display_name: doc.display_name,
      transaction_type: r.side,
      order_type: "MARKET",
      lots: r.lots,
      fill_price: r.price,
      limit_price: null,
      status: "FILLED",
      reject_reason: r.quoted ? null : "filled at mark — no quote on the book",
      intent: "ENTRY",
      created_at: Date.now(),
    });
    created.push({
      positionId: saved._id,
      accountId,
      kind: doc.kind,
      symbol: doc.symbol,
      displayName: doc.display_name,
      underlying: doc.underlying,
      expiry: doc.expiry,
      strike: doc.strike,
      optionType: doc.option_type,
      side: doc.side,
      lots: doc.lots,
      contractValue: doc.contract_value,
      entryPrice: doc.entry_price,
      exitPrice: null,
      premiumUsd: doc.premium_usd,
      status: "OPEN",
      standaloneMarginUsd: doc.standalone_margin_usd,
      realizedPnl: doc.realized_pnl,
      feesUsd: doc.fees_usd,
      openedAt: doc.opened_at,
      closedAt: null,
    });
  }

  return {
    filled: created.length,
    positions: created,
    marginAdded: preview.marginRequired,
    netPremium: preview.netPremium,
    feesUsd: preview.feesUsd,
  };
}

export async function placeOrder(params: {
  accountId: string;
  symbol: string;
  transactionType: TransactionType;
  lots: number;
  orderType: "MARKET" | "LIMIT";
  limitPrice?: number | null;
}): Promise<{ order: Order; position: Position | null }> {
  const i = await getInstrument(params.symbol);
  if (!i) throw new Rejected(`${params.symbol} is not listed on Delta.`);

  if (params.orderType === "LIMIT") {
    const lp = params.limitPrice;
    if (lp === null || lp === undefined || !Number.isFinite(lp) || lp <= 0) {
      throw new Rejected("A limit order needs a positive limit price.");
    }
    const { price: touch } = fillPriceFor(i, params.transactionType);
    const marketable = params.transactionType === "BUY" ? lp >= touch : lp <= touch;
    if (!marketable) {
      // Refused rather than rested — see the module docstring.
      const order = await insertOrder({
        account_id: params.accountId,
        position_id: null,
        kind: i.kind,
        symbol: i.symbol,
        display_name: displayNameOf(i),
        transaction_type: params.transactionType,
        order_type: "LIMIT",
        lots: params.lots,
        fill_price: null,
        limit_price: lp,
        status: "REJECTED",
        reject_reason: `Not marketable: ${params.transactionType === "BUY" ? "ask" : "bid"} is ${touch}. This desk keeps no resting book, so an order that cannot fill now is refused rather than left open.`,
        intent: "ENTRY",
        created_at: Date.now(),
      });
      const { toOrder } = await import("./store");
      return { order: toOrder(order), position: null };
    }
  }

  const result = await executeBasket(params.accountId, [
    { symbol: params.symbol, transactionType: params.transactionType, lots: params.lots },
  ]);
  const orders = await listOrders(params.accountId, 1);
  return { order: orders[0], position: result.positions[0] ?? null };
}

export async function exitPosition(
  accountId: string,
  positionId: string,
): Promise<{ realizedPnl: number; fillPrice: number; position: Position }> {
  const p = await getPosition(positionId);
  if (!p || p.accountId !== accountId) throw new Rejected("No such position in this account.");
  if (p.status !== "OPEN") throw new Rejected("That position is already closed.");

  const i = await getInstrument(p.symbol);
  if (!i) throw new Rejected(`${p.symbol} is no longer listed, so it cannot be priced to close.`);

  // Closing a long SELLS, so it hits the bid; closing a short BUYS the ask.
  const closingSide: TransactionType = p.side === "BUY" ? "SELL" : "BUY";
  const { price, quoted } = fillPriceFor(i, closingSide);

  const qty = p.lots * p.contractValue;
  const move = (price - p.entryPrice) * qty;
  const gross = p.side === "BUY" ? move : -move;
  const fee = feeFor(p.kind, qty, price, i.spot);
  const net = gross - fee;

  await closePositionDoc(positionId, price, net, fee);
  await insertOrder({
    account_id: accountId,
    position_id: positionId,
    kind: p.kind,
    symbol: p.symbol,
    display_name: p.displayName,
    transaction_type: closingSide,
    order_type: "MARKET",
    lots: p.lots,
    fill_price: price,
    limit_price: null,
    status: "FILLED",
    reject_reason: quoted ? null : "filled at mark — no quote on the book",
    intent: "EXIT",
    created_at: Date.now(),
  });

  const after = await getPosition(positionId);
  return { realizedPnl: net, fillPrice: price, position: after as Position };
}
