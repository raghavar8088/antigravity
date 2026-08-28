/**
 * Persistence for the Crypto Positions desk.
 *
 * Three collections, one shared Mongo connection. This module opens no client
 * of its own — Atlas M0 caps connections at 500 and every desk that pooled
 * separately was one of the reasons the app used to exhaust them.
 *
 * RESET ARCHIVES, IT DOES NOT DELETE. `resetAccount` copies the account's
 * positions and orders into `cp_archive` with a balance snapshot before
 * clearing them. A reset that hard-deletes has destroyed real paper history on
 * this project before, and it was not recoverable: the trades were gone, and
 * the balance could not be reconstructed from the remaining rows because a
 * closed trade's net P&L excludes the entry fee. The snapshot is what makes the
 * old balance answerable afterwards.
 */

import type { Collection, Db } from "mongodb";
import { getDb, isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import type { Account, Order, Position } from "./types";
import { DEFAULT_ACCOUNT_CAPITAL_USD } from "./types";

export const CP_ACCOUNTS = "cp_accounts";
export const CP_POSITIONS = "cp_positions";
export const CP_ORDERS = "cp_orders";
export const CP_ARCHIVE = "cp_archive";

export type AccountDoc = {
  _id: string;
  name: string;
  initial_capital: number;
  created_at: number;
};

export type PositionDoc = {
  _id: string;
  account_id: string;
  kind: Position["kind"];
  symbol: string;
  display_name: string;
  underlying: string;
  expiry: string | null;
  strike: number | null;
  option_type: Position["optionType"];
  side: Position["side"];
  lots: number;
  contract_value: number;
  entry_price: number;
  exit_price: number | null;
  premium_usd: number;
  status: Position["status"];
  standalone_margin_usd: number;
  realized_pnl: number;
  fees_usd: number;
  opened_at: number;
  closed_at: number | null;
};

export type OrderDoc = {
  _id: string;
  account_id: string;
  position_id: string | null;
  kind: Order["kind"];
  symbol: string;
  display_name: string;
  transaction_type: Order["transactionType"];
  order_type: Order["orderType"];
  lots: number;
  fill_price: number | null;
  limit_price: number | null;
  status: Order["status"];
  reject_reason: string | null;
  intent: Order["intent"];
  created_at: number;
};

export class NotConfigured extends Error {
  constructor() {
    super("MongoDB is not configured, so this desk has nowhere to keep its book.");
    this.name = "NotConfigured";
  }
}

async function db(): Promise<Db> {
  if (!isMongoConfigured()) throw new NotConfigured();
  return getDb();
}

const indexed = new Set<string>();

async function ensureIndexes(d: Db): Promise<void> {
  if (indexed.has("cp")) return;
  await Promise.all([
    d.collection(CP_POSITIONS).createIndex({ account_id: 1, status: 1 }, { name: "by_account_status" }),
    d.collection(CP_POSITIONS).createIndex({ account_id: 1, opened_at: -1 }, { name: "by_account_opened" }),
    d.collection(CP_ORDERS).createIndex({ account_id: 1, created_at: -1 }, { name: "by_account_created" }),
    d.collection(CP_ACCOUNTS).createIndex({ created_at: 1 }, { name: "by_created" }),
  ]);
  indexed.add("cp");
}

export async function accountsCol(): Promise<Collection<AccountDoc>> {
  const d = await db();
  await ensureIndexes(d);
  return d.collection<AccountDoc>(CP_ACCOUNTS);
}

export async function positionsCol(): Promise<Collection<PositionDoc>> {
  const d = await db();
  await ensureIndexes(d);
  return d.collection<PositionDoc>(CP_POSITIONS);
}

export async function ordersCol(): Promise<Collection<OrderDoc>> {
  const d = await db();
  await ensureIndexes(d);
  return d.collection<OrderDoc>(CP_ORDERS);
}

function newId(prefix: string): string {
  return `${prefix}_${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`;
}

export function toAccount(d: AccountDoc): Account {
  return {
    accountId: d._id,
    name: d.name,
    initialCapital: d.initial_capital,
    createdAt: d.created_at,
  };
}

export function toPosition(d: PositionDoc): Position {
  return {
    positionId: d._id,
    accountId: d.account_id,
    kind: d.kind,
    symbol: d.symbol,
    displayName: d.display_name,
    underlying: d.underlying,
    expiry: d.expiry,
    strike: d.strike,
    optionType: d.option_type,
    side: d.side,
    lots: d.lots,
    contractValue: d.contract_value,
    entryPrice: d.entry_price,
    exitPrice: d.exit_price,
    premiumUsd: d.premium_usd,
    status: d.status,
    standaloneMarginUsd: d.standalone_margin_usd,
    realizedPnl: d.realized_pnl,
    feesUsd: d.fees_usd,
    openedAt: d.opened_at,
    closedAt: d.closed_at,
  };
}

export function toOrder(d: OrderDoc): Order {
  return {
    orderId: d._id,
    accountId: d.account_id,
    positionId: d.position_id,
    kind: d.kind,
    symbol: d.symbol,
    displayName: d.display_name,
    transactionType: d.transaction_type,
    orderType: d.order_type,
    lots: d.lots,
    fillPrice: d.fill_price,
    limitPrice: d.limit_price,
    status: d.status,
    rejectReason: d.reject_reason,
    intent: d.intent,
    createdAt: d.created_at,
  };
}

/**
 * Every account, creating a default one the first time the desk is opened.
 *
 * A desk with no account cannot be used and its buttons all fail; seeding one
 * makes the page work on first load rather than presenting an empty state that
 * looks like a fault.
 */
export async function listAccounts(): Promise<Account[]> {
  const col = await accountsCol();
  const rows = await col.find({}).sort({ created_at: 1 }).toArray();
  if (rows.length === 0) {
    const seeded = await createAccount("Main", DEFAULT_ACCOUNT_CAPITAL_USD);
    return [seeded];
  }
  return rows.map(toAccount);
}

export async function getAccount(accountId: string): Promise<Account | null> {
  const col = await accountsCol();
  const d = await col.findOne({ _id: accountId });
  return d ? toAccount(d) : null;
}

export async function createAccount(name: string, initialCapital?: number): Promise<Account> {
  const col = await accountsCol();
  const doc: AccountDoc = {
    _id: newId("cpa"),
    name: name.trim() || "Account",
    initial_capital:
      initialCapital !== undefined && initialCapital > 0 ? initialCapital : DEFAULT_ACCOUNT_CAPITAL_USD,
    created_at: Date.now(),
  };
  await col.insertOne(doc);
  return toAccount(doc);
}

export async function editAccount(
  accountId: string,
  changes: { name?: string; initialCapital?: number },
): Promise<Account | null> {
  const col = await accountsCol();
  const set: Partial<AccountDoc> = {};
  if (changes.name !== undefined && changes.name.trim()) set.name = changes.name.trim();
  if (changes.initialCapital !== undefined && changes.initialCapital > 0) {
    set.initial_capital = changes.initialCapital;
  }
  if (Object.keys(set).length === 0) return getAccount(accountId);
  await col.updateOne({ _id: accountId }, { $set: set });
  return getAccount(accountId);
}

export async function deleteAccount(accountId: string): Promise<void> {
  const [acc, pos, ord] = await Promise.all([accountsCol(), positionsCol(), ordersCol()]);
  await archive(accountId, "delete");
  await Promise.all([
    pos.deleteMany({ account_id: accountId }),
    ord.deleteMany({ account_id: accountId }),
    acc.deleteOne({ _id: accountId }),
  ]);
}

export async function insertPosition(doc: Omit<PositionDoc, "_id">): Promise<PositionDoc> {
  const col = await positionsCol();
  const full: PositionDoc = { ...doc, _id: newId("cpp") };
  await col.insertOne(full);
  return full;
}

export async function insertOrder(doc: Omit<OrderDoc, "_id">): Promise<OrderDoc> {
  const col = await ordersCol();
  const full: OrderDoc = { ...doc, _id: newId("cpo") };
  await col.insertOne(full);
  return full;
}

export async function listPositions(accountId: string, status?: "OPEN" | "CLOSED"): Promise<Position[]> {
  const col = await positionsCol();
  const q: Record<string, unknown> = { account_id: accountId };
  if (status) q.status = status;
  const rows = await col.find(q).sort({ opened_at: -1 }).limit(500).toArray();
  return rows.map(toPosition);
}

export async function getPosition(positionId: string): Promise<Position | null> {
  const col = await positionsCol();
  const d = await col.findOne({ _id: positionId });
  return d ? toPosition(d) : null;
}

export async function closePositionDoc(
  positionId: string,
  exitPrice: number,
  realizedPnl: number,
  feesUsd: number,
): Promise<void> {
  const col = await positionsCol();
  await col.updateOne(
    { _id: positionId, status: "OPEN" },
    {
      $set: { status: "CLOSED", exit_price: exitPrice, closed_at: Date.now() },
      $inc: { realized_pnl: realizedPnl, fees_usd: feesUsd },
    },
  );
}

export async function listOrders(accountId: string, limit = 200): Promise<Order[]> {
  const col = await ordersCol();
  const rows = await col.find({ account_id: accountId }).sort({ created_at: -1 }).limit(limit).toArray();
  return rows.map(toOrder);
}

/** Snapshot the account's book into cp_archive. See the module docstring. */
async function archive(accountId: string, reason: string): Promise<number> {
  const d = await db();
  const [pos, ord] = await Promise.all([positionsCol(), ordersCol()]);
  const [positions, orders, account] = await Promise.all([
    pos.find({ account_id: accountId }).toArray(),
    ord.find({ account_id: accountId }).toArray(),
    getAccount(accountId),
  ]);
  const realized = positions.reduce((s, p) => s + (p.realized_pnl ?? 0), 0);
  const fees = positions.reduce((s, p) => s + (p.fees_usd ?? 0), 0);
  // Typed handle: an untyped collection defaults `_id` to ObjectId, and this
  // archive keys its rows by the same string ids the live collections use.
  await d.collection<{ _id: string; [k: string]: unknown }>(CP_ARCHIVE).insertOne({
    _id: newId("cpx"),
    account_id: accountId,
    account_name: account?.name ?? null,
    reason,
    archived_at: Date.now(),
    // The snapshot exists because realized P&L alone cannot rebuild a balance:
    // it excludes the entry fee, so a restore computed from the rows lands high.
    initial_capital: account?.initialCapital ?? null,
    realized_pnl: realized,
    fees_usd: fees,
    balance_at_reset: (account?.initialCapital ?? 0) + realized,
    positions,
    orders,
  });
  return positions.length + orders.length;
}

export async function resetAccount(
  accountId: string,
): Promise<{ positionsCleared: number; ordersCleared: number; archived: number }> {
  const [pos, ord] = await Promise.all([positionsCol(), ordersCol()]);
  const archived = await archive(accountId, "reset");
  const [p, o] = await Promise.all([
    pos.deleteMany({ account_id: accountId }),
    ord.deleteMany({ account_id: accountId }),
  ]);
  return { positionsCleared: p.deletedCount ?? 0, ordersCleared: o.deletedCount ?? 0, archived };
}
