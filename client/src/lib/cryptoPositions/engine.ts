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
  atmStrikeFor,
  getSettledInstrument,
} from "./delta";
import {
  bookMaintenanceMargin,
  liquidationFor,
  marginFor,
  maxLeverageFor,
  type MarginLeg,
} from "./margin";
import {
  closePositionDoc,
  getAccount,
  getPosition,
  insertOrder,
  insertPosition,
  listOrders,
  listPositions,
  reduceLots,
  type PositionDoc,
} from "./store";
import type {
  BasketLeg,
  BasketPreview,
  ContractSpec,
  Instrument,
  LivePosition,
  Order,
  Position,
  PositionsSummary,
  RollLeg,
  RollResult,
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

/**
 * A contract's instrument, falling back to its SETTLEMENT once it has expired.
 *
 * Every path that needs a price goes through here. Delta drops an expired
 * contract from the ticker feed, and without this fallback the position could
 * not be valued, could not be closed, and stayed in the book forever.
 */
async function instrumentOrSettlement(symbol: string): Promise<Instrument | null> {
  return (await getInstrument(symbol)) ?? (await getSettledInstrument(symbol));
}

/** The price a side would actually pay, and whether a real quote backed it. */
export function fillPriceFor(i: Instrument, side: TransactionType): { price: number; quoted: boolean } {
  const q = side === "BUY" ? i.ask : i.bid;
  if (q !== null && q > 0) return { price: q, quoted: true };
  return { price: i.markPrice, quoted: false };
}

function legFrom(
  i: Instrument,
  side: TransactionType,
  lots: number,
  price: number,
  leverage?: number,
): MarginLeg {
  return {
    symbol: i.symbol,
    underlying: i.underlying,
    side,
    lots,
    contractValue: i.contractValue,
    price,
    instrument: i,
    // The venue's own default when the caller did not choose, clamped to what
    // the venue permits at this size. Asking for more than the contract allows
    // must not quietly succeed — it would understate margin and move the
    // liquidation price away from where Delta would put it.
    leverage: clampLeverage(i, lots * i.contractValue * price, leverage),
  };
}

/** A requested leverage, held inside 1x and what the venue allows at this size. */
export function clampLeverage(i: Instrument, notionalUsd: number, want?: number): number {
  const max = maxLeverageFor(i, notionalUsd);
  const asked = want && Number.isFinite(want) && want > 0 ? want : i.defaultLeverage;
  return Math.min(Math.max(1, asked), max);
}

/**
 * Open positions as margin legs, valued at their ENTRY price.
 *
 * Each leg carries its `positionId`. That is not decoration: a roll has to
 * price the book as it WOULD be once specific legs are replaced, and matching
 * them back by (symbol, side, lots) collides the moment two positions share all
 * three — which is the normal case for a book built up in several clips. The id
 * is the only thing that identifies a leg uniquely.
 */
async function openLegs(accountId: string): Promise<(MarginLeg & { positionId: string })[]> {
  const open = await listPositions(accountId, "OPEN");
  if (open.length === 0) return [];
  const marks = await marksFor(open.map((p) => p.symbol));
  const legs: (MarginLeg & { positionId: string })[] = [];
  for (const p of open) {
    const i = marks.get(p.symbol);
    // Not priced by the venue at all — excluded from the scan rather than
    // assumed flat, so the omission cannot silently lower the requirement.
    if (!i) continue;
    legs.push({ ...legFrom(i, p.side, p.lots, p.entryPrice, p.leverage), positionId: p.positionId });
  }
  return legs;
}

export async function livePositions(
  accountId: string,
  status?: "OPEN" | "CLOSED",
): Promise<LivePosition[]> {
  const rows = await listPositions(accountId, status);
  const marks = await marksFor(rows.map((p) => p.symbol));

  // Anything the live snapshot did not carry may have expired rather than
  // vanished. Resolve those to their settlement so they can be valued and
  // closed instead of being stuck.
  const settled = new Set<string>();
  for (const p of rows) {
    if (p.status !== "OPEN" || marks.has(p.symbol)) continue;
    const s = await getSettledInstrument(p.symbol);
    if (s) {
      marks.set(p.symbol, s);
      settled.add(p.symbol);
    }
  }

  // The at-the-money strike per (underlying, expiry), resolved once per group
  // rather than per row. Lets the UI disable a roll that would do nothing.
  const atm = new Map<string, number | null>();
  for (const p of rows) {
    if (p.status !== "OPEN" || p.optionType === null || !p.expiry) continue;
    const k = `${p.underlying}|${p.expiry}`;
    if (!atm.has(k)) atm.set(k, await atmStrikeFor(p.underlying, p.expiry));
  }

  return rows.map((p) => {
    const i = marks.get(p.symbol);
    const qty = p.lots * p.contractValue;
    if (p.status === "CLOSED" || !i) {
      return {
        ...p,
        markPrice: i?.markPrice ?? null,
        unrealizedPnl: 0,
        valueUsd: 0,
        notionalUsd: 0,
        accountMultiple: null,
        atmStrike: null,
        expired: false,
        settlementPrice: null,
        liquidation: null,
      };
    }
    const move = (i.markPrice - p.entryPrice) * qty;
    const unrealized = p.side === "BUY" ? move : -move;
    return {
      ...p,
      markPrice: i.markPrice,
      unrealizedPnl: unrealized,
      valueUsd: i.markPrice * qty * (p.side === "BUY" ? 1 : -1),
      // The UNDERLYING controlled, not the contracts' own value. On an option
      // those differ by orders of magnitude — see the field's own note.
      notionalUsd: Math.abs(qty * (i.spot ?? i.markPrice)),
      accountMultiple: null,
      atmStrike: p.optionType !== null && p.expiry ? (atm.get(`${p.underlying}|${p.expiry}`) ?? null) : null,
      expired: settled.has(p.symbol),
      settlementPrice: settled.has(p.symbol) ? i.markPrice : null,
      liquidation: liquidationFor(i, p.side, p.lots, p.entryPrice, p.leverage, i.markPrice),
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
  const maint = bookMaintenanceMargin(legs);
  const balance = account.initialCapital + realized;
  const wins = closed.filter((p) => p.realizedPnl > 0).length;
  // Underlying notional, which is what "exposure" has to mean. Summing the
  // contracts' market value instead reported a book at 9.7x the account as
  // 0.06x — the opposite of a warning.
  const exposure = open.reduce((s, p) => s + p.notionalUsd, 0);
  const premiumValue = open.reduce(
    (s, p) => s + Math.abs((p.markPrice ?? p.entryPrice) * p.lots * p.contractValue),
    0,
  );

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
    // Notional at the mark, not at entry: exposure is what the book is holding
    // now, and a position that has doubled is carrying twice the risk it opened
    // with even though its entry price has not changed.
    contractExposureUsd: exposure,
    premiumValueUsd: premiumValue,
    underlyingsOpen: new Set(open.map((p) => p.underlying)).size,
    maintenanceMarginUsd: maint,
    // Null when nothing is posted — see the field's own note. An account with
    // no maintenance floor is the SAFEST state, and 0% would read as the most
    // dangerous one.
    marginLevelPct: maint > 0 ? ((balance + unrealized) / maint) * 100 : null,
    accountLeverage: balance + unrealized > 0 ? exposure / (balance + unrealized) : null,
    liquidatablePositions: open.filter((p) => p.liquidation !== null).length,
  };
}

/**
 * What one contract of each underlying controls.
 *
 * Exists because "1 contract" means nothing on its own here: one BTC contract
 * is 0.001 BTC and the quote is per whole BTC, so a $2,000 call costs $2. The
 * commodity desk this borrows from learned the same lesson in rupees — a ZINC
 * lot is 5 tonnes — and the fix was to state the multiplier on screen rather
 * than let it be inferred from a position that came out the wrong size.
 */
export async function contractSpecs(): Promise<ContractSpec[]> {
  const s = await getSnapshot();
  const out: ContractSpec[] = [];
  for (const u of s.underlyings) {
    const opts = s.options.filter((o) => o.underlying === u.symbol);
    const perps = s.perpetuals.filter((p) => p.underlying === u.symbol);
    const cv = opts[0]?.contractValue ?? perps[0]?.contractValue ?? 1;
    // Margin parameters are per contract; a perpetual is the reference where
    // one exists, since that is what the leverage control mostly applies to.
    const ref = perps[0] ?? opts[0];
    out.push({
      underlying: u.symbol,
      contractValue: cv,
      priceUnit: `USD per 1 ${u.symbol}`,
      optionCount: opts.length,
      perpetualCount: perps.length,
      expiryCount: new Set(opts.map((o) => o.expiry).filter(Boolean)).size,
      spot: u.spot,
      tickSize: opts[0]?.tickSize ?? perps[0]?.tickSize ?? null,
      contractValueUsd: u.spot !== null ? u.spot * cv : null,
      maxLeverage: ref?.maxLeverage ?? 1,
      defaultLeverage: ref?.defaultLeverage ?? 1,
      initialMarginPct: ref?.initialMarginPct ?? 0,
      maintenanceMarginPct: ref?.maintenanceMarginPct ?? 0,
    });
  }
  return out;
}

/** Close several positions. Returns what actually closed and what did not. */
export async function closePositions(
  accountId: string,
  positionIds: string[],
): Promise<{ closed: number; realizedPnl: number; failed: { positionId: string; reason: string }[] }> {
  let closed = 0;
  let realized = 0;
  const failed: { positionId: string; reason: string }[] = [];
  for (const id of positionIds) {
    try {
      const r = await exitPosition(accountId, id);
      closed += 1;
      realized += r.realizedPnl;
    } catch (e) {
      failed.push({ positionId: id, reason: e instanceof Error ? e.message : String(e) });
    }
  }
  return { closed, realizedPnl: realized, failed };
}

/**
 * Reduce an open position by some of its contracts.
 *
 * A partial close is a real edit rather than a second position: it closes
 * `lots` of the original at the live price and leaves the remainder open at the
 * SAME entry price. Re-entering the remainder at the current mark instead would
 * silently restate the position's cost basis and wipe out its unrealised P&L,
 * which is the kind of quiet rewrite that makes a blotter untrustworthy.
 */
export async function reducePosition(
  accountId: string,
  positionId: string,
  lots: number,
): Promise<{ closedLots: number; remainingLots: number; realizedPnl: number; fillPrice: number }> {
  const p = await getPosition(positionId);
  if (!p || p.accountId !== accountId) throw new Rejected("No such position in this account.");
  if (p.status !== "OPEN") throw new Rejected("That position is already closed.");
  const want = Math.floor(lots);
  if (!Number.isFinite(want) || want <= 0) throw new Rejected("Lots to close must be a positive whole number.");
  if (want > p.lots) throw new Rejected(`That position holds ${p.lots} lot(s); cannot close ${want}.`);
  if (want === p.lots) {
    const r = await exitPosition(accountId, positionId);
    return { closedLots: want, remainingLots: 0, realizedPnl: r.realizedPnl, fillPrice: r.fillPrice };
  }

  const i = await instrumentOrSettlement(p.symbol);
  if (!i) throw new Rejected(`${p.symbol} has no price on Delta, so it cannot be reduced.`);
  const closingSide: TransactionType = p.side === "BUY" ? "SELL" : "BUY";
  const { price } = fillPriceFor(i, closingSide);

  const qty = want * p.contractValue;
  const move = (price - p.entryPrice) * qty;
  const gross = p.side === "BUY" ? move : -move;
  const fee = feeFor(p.kind, qty, price, i.spot);
  const net = gross - fee;

  await reduceLots(positionId, want, net, fee);
  await insertOrder({
    account_id: accountId,
    position_id: positionId,
    kind: p.kind,
    symbol: p.symbol,
    display_name: p.displayName,
    transaction_type: closingSide,
    order_type: "MARKET",
    lots: want,
    fill_price: price,
    limit_price: null,
    status: "FILLED",
    reject_reason: `partial close — ${p.lots - want} lot(s) left open at the original entry`,
    intent: "EXIT",
    created_at: Date.now(),
  });

  return { closedLots: want, remainingLots: p.lots - want, realizedPnl: net, fillPrice: price };
}

/**
 * Roll option legs to the money.
 *
 * Each leg is closed at the live price and re-opened at the listed strike
 * nearest its own spot — same side, same lots. Perpetuals have no strike and
 * are left alone.
 *
 * WHY THIS IS ONE OPERATION PER EXPIRY GROUP, and not a loop over the rows.
 * Rolling a straddle one leg at a time leaves a naked leg in between, and a
 * naked leg costs MORE margin than the pair did. On a tight book that
 * intermediate state can refuse the second roll and strand the position
 * half-rolled — worse than either the old position or the new one. So a group
 * is priced whole, checked against the account whole, and either moves whole or
 * is left exactly where it was.
 */
export async function rollToAtm(accountId: string, positionIds?: string[]): Promise<RollResult> {
  const open = await listPositions(accountId, "OPEN");
  const wanted = positionIds && positionIds.length > 0 ? new Set(positionIds) : null;
  const legs = open.filter(
    (p) => p.kind === "OPTION" && p.optionType !== null && p.expiry !== null && (!wanted || wanted.has(p.positionId)),
  );
  if (legs.length === 0) {
    return { rolled: [], realizedPnl: 0, marginDelta: 0, feesUsd: 0, failed: [], note: "No option legs to roll." };
  }

  const before = marginFor(await openLegs(accountId));
  const groups = new Map<string, typeof legs>();
  for (const l of legs) {
    const k = `${l.underlying}|${l.expiry}`;
    groups.set(k, [...(groups.get(k) ?? []), l]);
  }

  const rolled: RollLeg[] = [];
  const failed: RollResult["failed"] = [];
  let realized = 0;
  let feesTotal = 0;

  for (const [key, group] of groups) {
    const [underlying, expiry] = key.split("|");
    try {
      const chain = await getOptionChain(underlying, expiry as string);
      if (chain.atmStrike === null) {
        failed.push({ underlying, expiry: expiry as string, reason: "no at-the-money strike is listed" });
        continue;
      }

      // Resolve every replacement BEFORE touching anything.
      //
      // A leg ALREADY on the at-the-money strike is re-added too, at the
      // owner's direction. It closes and re-opens the same contract, which
      // realises the position's P&L and resets its entry to the current price —
      // a real operation, not a no-op, though it pays an exit and an entry fee
      // and crosses the spread twice for a strike that has not changed.
      const plan: { from: Position; to: Instrument }[] = [];
      for (const leg of group) {
        const row = chain.rows.find((r) => r.strike === chain.atmStrike);
        const target = leg.optionType === "CALL" ? row?.call : row?.put;
        if (!target) {
          throw new Rejected(`no ${leg.optionType} listed at the ${chain.atmStrike} strike`);
        }
        plan.push({ from: leg, to: target });
      }
      if (plan.length === 0) continue; // whole group already at the money

      // Price the book as it WOULD be, and refuse the group if it does not fit.
      const rolledIds = new Set(plan.map((x) => x.from.positionId));
      const survivors = (await openLegs(accountId)).filter((l) => !rolledIds.has(l.positionId));
      const incoming: MarginLeg[] = plan.map((x) => ({
        symbol: x.to.symbol,
        underlying: x.to.underlying,
        side: x.from.side,
        lots: x.from.lots,
        contractValue: x.to.contractValue,
        price: fillPriceFor(x.to, x.from.side).price,
        instrument: x.to,
        leverage: x.from.leverage,
      }));
      const after = marginFor([...survivors, ...incoming]);
      const s = await summary(accountId);
      if (after.marginRequired > s.balance) {
        failed.push({
          underlying,
          expiry: expiry as string,
          reason: `needs $${after.marginRequired.toFixed(2)} of margin against a $${s.balance.toFixed(2)} balance`,
        });
        continue;
      }

      // IT FITS. Close the WHOLE group first, then open the whole group as one
      // basket.
      //
      // Not leg by leg, which is what this used to do and which quietly broke
      // the promise made above. Closing one leg and immediately re-opening it
      // passes through a book holding the NEW strike alongside the OLD other
      // leg, and that mixed state is wider than either the original pair or the
      // intended one — the desk's own test pins that. On a tight account the
      // second open is then refused, and because the first leg is already gone
      // the position is left half-rolled: worse than either endpoint, and
      // exactly the outcome the group-at-a-time design exists to prevent.
      //
      // Closing everything first means the intermediate book is FLAT, which is
      // the cheapest state there is, so the re-open cannot be refused for
      // margin it just released. Opening as a single basket also prices the
      // legs against each other, so a straddle gets its offset instead of the
      // first leg being charged as a naked short.
      const exits = [];
      for (const { from } of plan) {
        const exit = await exitPosition(accountId, from.positionId);
        realized += exit.realizedPnl;
        exits.push({ from, exit });
      }

      const opened = await executeBasket(
        accountId,
        // Each replacement inherits the leverage its original was opened at —
        // a roll moves the strike, not the risk setting.
        plan.map(({ from, to }) => ({
          symbol: to.symbol,
          transactionType: from.side,
          lots: from.lots,
          leverage: from.leverage,
        })),
      );
      feesTotal += opened.feesUsd;

      exits.forEach(({ from, exit }, idx) => {
        const newPos = opened.positions[idx];
        rolled.push({
          positionId: from.positionId,
          from: from.displayName,
          to: newPos?.displayName ?? plan[idx].to.symbol,
          exitPrice: exit.fillPrice,
          entryPrice: newPos?.entryPrice ?? 0,
          realizedPnl: exit.realizedPnl,
        });
      });
    } catch (e) {
      failed.push({
        underlying,
        expiry: expiry as string,
        reason: e instanceof Error ? e.message : String(e),
      });
    }
  }

  const afterAll = marginFor(await openLegs(accountId));
  const unchanged = rolled.filter((r) => r.from === r.to).length;
  const note =
    rolled.length === 0
      ? failed.length > 0
        ? "Nothing was rolled."
        : "No option legs to roll."
      : `Re-added ${rolled.length} leg${rolled.length === 1 ? "" : "s"} at the money` +
        (unchanged > 0
          ? ` (${unchanged} onto the same strike — P&L realised and entry reset at the current price).`
          : ".");

  return {
    rolled,
    realizedPnl: realized,
    marginDelta: afterAll.marginRequired - before.marginRequired,
    feesUsd: feesTotal,
    failed,
    note,
  };
}

async function resolveLegs(
  legs: BasketLeg[],
): Promise<
  { instrument: Instrument; side: TransactionType; lots: number; price: number; quoted: boolean; leverage: number }[]
> {
  if (legs.length === 0) throw new Rejected("A basket needs at least one leg.");
  const out = [];
  for (const l of legs) {
    if (!Number.isFinite(l.lots) || l.lots <= 0) throw new Rejected(`${l.symbol}: lots must be a positive number.`);
    const i = await getInstrument(l.symbol);
    if (!i) throw new Rejected(`${l.symbol} is not listed on Delta.`);
    const { price, quoted } = fillPriceFor(i, l.transactionType);
    const lotsN = Math.floor(l.lots);
    out.push({
      instrument: i,
      side: l.transactionType,
      lots: lotsN,
      price,
      quoted,
      leverage: clampLeverage(i, lotsN * i.contractValue * price, l.leverage),
    });
  }
  return out;
}

export async function previewBasket(accountId: string, legs: BasketLeg[]): Promise<BasketPreview> {
  const resolved = await resolveLegs(legs);
  const existing = await openLegs(accountId);
  const newLegs = resolved.map((r) => legFrom(r.instrument, r.side, r.lots, r.price, r.leverage));

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
      leverage: r.leverage,
      maxLeverage: maxLeverageFor(r.instrument, r.lots * r.instrument.contractValue * r.price),
      liquidation: liquidationFor(r.instrument, r.side, r.lots, r.price, r.leverage, r.instrument.markPrice),
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
    const standalone = marginFor([legFrom(r.instrument, r.side, r.lots, r.price, r.leverage)]).marginRequired;

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
      leverage: r.leverage,
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
      leverage: doc.leverage,
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

  const i = await instrumentOrSettlement(p.symbol);
  if (!i) {
    throw new Rejected(
      `${p.symbol} is neither listed nor settled — Delta has no price for it, so it cannot be closed automatically. ` +
        `This is usually a contract that expired long enough ago that its candles are gone.`,
    );
  }

  // Closing a long SELLS, so it hits the bid; closing a short BUYS the ask.
  // A settled contract quotes both sides at its settlement value, so there is
  // no spread to cross and the "fill" is the settlement itself.
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
