/**
 * MongoDB persistence for both paper-trading terminals.
 *
 * COLLECTIONS ARE PREFIXED `pt_`. The `loop_trades` database already holds
 * `paper_positions`, `paper_trades` and `paper_state` for other desks, and
 * `cs_paper_*` for the Crypto Screener's automated desk. Writing into any of
 * those would interleave unrelated books and corrupt both records — a failure
 * that reads as a strategy result rather than a bug.
 *
 * ONE ACCOUNT PER VENUE, for now. Both terminals are single-user desks inside
 * an already-authenticated app, so the account id is derived from the venue
 * rather than from a session. The schema carries `accountId` on every document
 * so multi-account is a query change rather than a migration.
 *
 * THE CONNECTION POOL IS SHARED, never re-opened — Atlas M0 caps at 500
 * connections and this app has been taken down twice by exhausting it.
 */

import type { Collection, Db } from "mongodb";
import { getDb, isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import type { Account, Order, Position, TradeRecord, VenueId } from "./types";

export const PT_ACCOUNTS = "pt_accounts";
export const PT_ORDERS = "pt_orders";
export const PT_POSITIONS = "pt_positions";
export const PT_TRADES = "pt_trades";
export const PT_STATE = "pt_state";

export class PaperTradingUnavailable extends Error {}

export function accountIdFor(venue: VenueId): string {
  return `${venue}-paper-main`;
}

export function ptConfigured(): boolean {
  return isMongoConfigured();
}

async function db(): Promise<Db> {
  if (!isMongoConfigured()) {
    throw new PaperTradingUnavailable(
      "MONGODB_URI is not set on this deployment, so the paper terminal has nowhere to keep orders " +
        "and positions. It is reported as unavailable rather than run from memory — on serverless, " +
        "an in-memory book loses every position between requests and then reports the survivors as " +
        "its record, which is worse than having no terminal at all.",
    );
  }
  return getDb();
}

const indexed = new Set<string>();

async function ensureIndexes(d: Db): Promise<void> {
  if (indexed.has("pt")) return;
  await Promise.all([
    d.collection(PT_ACCOUNTS).createIndex({ accountId: 1 }, { unique: true, name: "uniq_account" }),
    d.collection(PT_ORDERS).createIndex({ orderId: 1 }, { unique: true, name: "uniq_order" }),
    d.collection(PT_ORDERS).createIndex({ accountId: 1, status: 1, createdAt: -1 }, { name: "by_account_status" }),
    d.collection(PT_POSITIONS).createIndex({ positionId: 1 }, { unique: true, name: "uniq_position" }),
    d.collection(PT_POSITIONS).createIndex({ accountId: 1, symbol: 1 }, { name: "by_account_symbol" }),
    d.collection(PT_TRADES).createIndex({ accountId: 1, closedAt: -1 }, { name: "by_account_closed" }),
  ]);
  indexed.add("pt");
}

export async function accounts(): Promise<Collection<Account>> {
  const d = await db();
  await ensureIndexes(d);
  return d.collection<Account>(PT_ACCOUNTS);
}

export async function orders(): Promise<Collection<Order>> {
  const d = await db();
  await ensureIndexes(d);
  return d.collection<Order>(PT_ORDERS);
}

export async function positionsCol(): Promise<Collection<Position>> {
  const d = await db();
  await ensureIndexes(d);
  return d.collection<Position>(PT_POSITIONS);
}

export async function tradesCol(): Promise<Collection<TradeRecord>> {
  const d = await db();
  await ensureIndexes(d);
  return d.collection<TradeRecord>(PT_TRADES);
}

type StateDoc = {
  _id: string;
  lockedUntil: number;
  lastTickAt: number;
  lastTickMs: number;
  ticks: number;
  lastError: string | null;
};

async function state(): Promise<Collection<StateDoc>> {
  const d = await db();
  return d.collection<StateDoc>(PT_STATE);
}

export async function readState(venue: VenueId): Promise<StateDoc | null> {
  const col = await state();
  return col.findOne({ _id: `pt:${venue}` });
}

/**
 * Take the tick lease, or return false because another request holds it.
 *
 * Both terminals tick on read, so a page load fires several requests that all
 * want to advance the same book. The lease makes one of them do it. It EXPIRES
 * rather than being released only on success: a container that dies mid-tick
 * would otherwise hold the lock forever and freeze the desk, which is a worse
 * failure than the double-tick it prevents.
 */
export async function acquireLease(venue: VenueId, leaseMs: number): Promise<boolean> {
  const col = await state();
  const now = Date.now();
  const id = `pt:${venue}`;
  const res = await col.findOneAndUpdate(
    { _id: id, $or: [{ lockedUntil: { $lte: now } }, { lockedUntil: { $exists: false } }] },
    {
      $set: { lockedUntil: now + leaseMs },
      $setOnInsert: { lastTickAt: 0, lastTickMs: 0, ticks: 0, lastError: null },
    },
    { upsert: true, returnDocument: "after" },
  );
  return res !== null;
}

export async function releaseLease(venue: VenueId, patch: Partial<StateDoc>): Promise<void> {
  const col = await state();
  await col.updateOne({ _id: `pt:${venue}` }, { $set: { ...patch, lockedUntil: 0 }, $inc: { ticks: 1 } });
}

/** The venue's account, created at its default settings the first time it is needed. */
export async function ensureAccount(
  venue: VenueId,
  defaults: Pick<Account, "startingBalance" | "leverage" | "marginMode" | "accountType" | "stopOutLevelPct" | "marginCallLevelPct">,
): Promise<Account> {
  const col = await accounts();
  const id = accountIdFor(venue);
  const now = Date.now();
  const res = await col.findOneAndUpdate(
    { accountId: id },
    {
      $setOnInsert: {
        accountId: id,
        venue,
        currency: "USD",
        balance: defaults.startingBalance,
        startingBalance: defaults.startingBalance,
        leverage: defaults.leverage,
        marginMode: defaults.marginMode,
        accountType: defaults.accountType,
        stopOutLevelPct: defaults.stopOutLevelPct,
        marginCallLevelPct: defaults.marginCallLevelPct,
        createdAt: now,
        resetCount: 0,
      },
    },
    { upsert: true, returnDocument: "after" },
  );
  return res as Account;
}

export async function creditAccount(accountId: string, deltaUsd: number): Promise<void> {
  const col = await accounts();
  await col.updateOne({ accountId }, { $inc: { balance: deltaUsd } });
}

export async function updateAccount(accountId: string, patch: Partial<Account>): Promise<void> {
  const col = await accounts();
  await col.updateOne({ accountId }, { $set: patch });
}

/**
 * Wipe one venue's terminal back to a fresh account.
 *
 * `resetCount` survives so a reader can tell an account that has never traded
 * from one that was rebased this morning — both otherwise show the same
 * untouched balance and the same 0.00% return.
 */
export async function resetVenue(venue: VenueId, startingBalance: number): Promise<{
  orders: number;
  positions: number;
  trades: number;
}> {
  const d = await db();
  const id = accountIdFor(venue);
  const [o, p, t] = await Promise.all([
    d.collection(PT_ORDERS).deleteMany({ accountId: id }),
    d.collection(PT_POSITIONS).deleteMany({ accountId: id }),
    d.collection(PT_TRADES).deleteMany({ accountId: id }),
  ]);
  await d.collection<Account>(PT_ACCOUNTS).updateOne(
    { accountId: id },
    { $set: { balance: startingBalance, startingBalance }, $inc: { resetCount: 1 } },
  );
  return { orders: o.deletedCount, positions: p.deletedCount, trades: t.deletedCount };
}
