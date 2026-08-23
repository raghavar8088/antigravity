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
/** One row per archived generation: what the desk looked like when it was reset. */
export const PT_ARCHIVE = "pt_archive";

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
/**
 * Reset a venue's desk by ARCHIVING it, never by deleting it.
 *
 * WHY THIS IS NOT A DELETE. It used to be, and the confirmation text even said
 * "the trade log is the only record of what the account did, and it is not
 * recoverable" — which was true, and was the bug rather than the disclaimer.
 * A desk was wiped by an operator who believed they were clearing their own
 * test rows, and the account's real history went with it. Nothing could bring
 * it back: this database has no point-in-time restore.
 *
 * So a reset now stamps every order, position and trade with the generation it
 * belonged to and hides it. The account starts fresh, the reader sees an empty
 * desk, and the previous life is still on disk and restorable. Storage is
 * cheap; a trade log is not reproducible.
 *
 * Every read filters `archived: { $ne: true }` — see `LIVE` below — so an
 * archived generation is invisible to the engine, the leaderboard and the P&L
 * until it is explicitly restored.
 */
export async function resetVenue(venue: VenueId, startingBalance: number): Promise<{
  orders: number;
  positions: number;
  trades: number;
  generation: number;
}> {
  const d = await db();
  const id = accountIdFor(venue);
  const acct = await d.collection<Account>(PT_ACCOUNTS).findOne({ accountId: id });
  // The generation being closed is the reset count BEFORE this reset, so
  // generation 0 is the account's first life.
  const generation = acct?.resetCount ?? 0;
  const at = Date.now();
  const stamp = { $set: { archived: true, archivedAt: at, generation } };

  // THE BALANCE IS SNAPSHOT, NOT RECONSTRUCTED. The first version of restore
  // recomputed it as startingBalance + the sum of every trade's net P&L, and
  // came out 90 cents high on a single round trip: `netPnlUsd` excludes the
  // ENTRY fee, which was debited when the position opened. Any formula here has
  // to track every future change to fee and carry accounting, and will silently
  // drift the first time one of them moves. Recording the number is exact by
  // construction and cannot go stale.
  await d.collection(PT_ARCHIVE).insertOne({
    accountId: id,
    generation,
    archivedAt: at,
    balance: acct?.balance ?? startingBalance,
    startingBalance: acct?.startingBalance ?? startingBalance,
  });

  const [o, p, t] = await Promise.all([
    d.collection(PT_ORDERS).updateMany({ accountId: id, archived: { $ne: true } }, stamp),
    d.collection(PT_POSITIONS).updateMany({ accountId: id, archived: { $ne: true } }, stamp),
    d.collection(PT_TRADES).updateMany({ accountId: id, archived: { $ne: true } }, stamp),
  ]);
  await d.collection<Account>(PT_ACCOUNTS).updateOne(
    { accountId: id },
    { $set: { balance: startingBalance, startingBalance }, $inc: { resetCount: 1 } },
  );
  return {
    orders: o.modifiedCount,
    positions: p.modifiedCount,
    trades: t.modifiedCount,
    generation,
  };
}

/** What every live read filters on. An archived row belongs to a past account. */
export const LIVE = { archived: { $ne: true } } as const;

export type ArchivedGeneration = {
  generation: number;
  archivedAt: number | null;
  trades: number;
  orders: number;
  positions: number;
  netPnlUsd: number;
  /** The desk's balance at the moment it was archived. */
  balance: number;
};

/** The past lives of this desk, newest first. */
export async function listArchive(venue: VenueId): Promise<ArchivedGeneration[]> {
  const d = await db();
  const id = accountIdFor(venue);
  // Driven off the archive metadata rather than off the trades, so a generation
  // that was reset without ever closing a trade still shows up — otherwise a
  // reader sees "no archive" and concludes the reset deleted something.
  const metas = await d
    .collection<{ generation: number; archivedAt: number; balance: number; startingBalance: number }>(PT_ARCHIVE)
    .find({ accountId: id })
    .sort({ generation: -1 })
    .toArray();

  const out: ArchivedGeneration[] = [];
  for (const m of metas) {
    const q = { accountId: id, archived: true, generation: m.generation };
    const [trades, orders, positions] = await Promise.all([
      d.collection<TradeRecord>(PT_TRADES).find(q).toArray(),
      d.collection(PT_ORDERS).countDocuments(q),
      d.collection(PT_POSITIONS).countDocuments(q),
    ]);
    out.push({
      generation: m.generation,
      archivedAt: m.archivedAt ?? null,
      trades: trades.length,
      orders,
      positions,
      // The account's own arithmetic, not a sum over trades: this is what the
      // balance actually was, fees and carry included.
      netPnlUsd: Math.round((m.balance - m.startingBalance) * 100) / 100,
      balance: m.balance,
    });
  }
  return out;
}

/** Closed trades from one archived generation, for reading rather than resuming. */
export async function readArchive(venue: VenueId, generation: number, limit = 300): Promise<TradeRecord[]> {
  const d = await db();
  return d
    .collection<TradeRecord>(PT_TRADES)
    .find({ accountId: accountIdFor(venue), archived: true, generation })
    .sort({ closedAt: -1 })
    .limit(limit)
    .toArray();
}

/**
 * Bring an archived generation back to life.
 *
 * Refuses when the desk is not empty, rather than merging two accounts' books
 * into one. Restoring into a live desk would interleave trades from two
 * different starting balances and make every statistic over them meaningless.
 */
export async function restoreArchive(
  venue: VenueId,
  generation: number,
): Promise<{ orders: number; positions: number; trades: number }> {
  const d = await db();
  const id = accountIdFor(venue);
  const liveTrades = await d.collection(PT_TRADES).countDocuments({ accountId: id, ...LIVE });
  const livePositions = await d.collection(PT_POSITIONS).countDocuments({ accountId: id, ...LIVE });
  if (liveTrades > 0 || livePositions > 0) {
    throw new Error(
      "This desk already has live trades or positions. Restoring on top of them would interleave " +
        "two accounts started from different balances, which makes every statistic over the " +
        "combined book meaningless. Reset the desk first — that archives what is there now, so " +
        "nothing is lost either way.",
    );
  }

  const unset = { $unset: { archived: "", archivedAt: "", generation: "" } };
  const q = { accountId: id, archived: true, generation };
  const [o, p, t] = await Promise.all([
    d.collection(PT_ORDERS).updateMany(q, unset),
    d.collection(PT_POSITIONS).updateMany(q, unset),
    d.collection(PT_TRADES).updateMany(q, unset),
  ]);

  // The balance comes back from the snapshot taken at reset, exactly as it was.
  const meta = await d
    .collection<{ balance: number; startingBalance: number }>(PT_ARCHIVE)
    .findOne({ accountId: id, generation });
  if (meta) {
    await d
      .collection<Account>(PT_ACCOUNTS)
      .updateOne(
        { accountId: id },
        { $set: { balance: meta.balance, startingBalance: meta.startingBalance } },
      );
    await d.collection(PT_ARCHIVE).deleteOne({ accountId: id, generation });
  }

  return { orders: o.modifiedCount, positions: p.modifiedCount, trades: t.modifiedCount };
}
