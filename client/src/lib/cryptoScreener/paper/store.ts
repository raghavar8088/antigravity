/**
 * MongoDB persistence for the Crypto Screener paper desk.
 *
 * WHY A DATABASE AT ALL. Every other part of this module is stateless — it
 * reads Delta, computes, and caches in module scope. A paper desk cannot be:
 * a position opened by one request has to be visible to the next, and Vercel
 * gives no guarantee that the next request lands in the same container. Module
 * scope is a cache here, not a record.
 *
 * COLLECTION NAMES ARE PREFIXED `cs_`. The `loop_trades` database ALREADY has
 * `paper_positions`, `paper_trades` and `paper_state` collections belonging to
 * other desks in this app. Writing into those would silently interleave two
 * unrelated books and corrupt both leaderboards, which is the kind of failure
 * that looks like a strategy result rather than a bug.
 *
 * THE CONNECTION POOL IS SHARED, never re-opened. Atlas M0 caps at 500
 * connections and this app has already been taken down twice by exhausting it,
 * which is why `mongoTradesClient` keeps `maxPoolSize: 2` and a 10s idle reaper.
 * This module calls its `getDb()` and opens no client of its own.
 */

import type { Collection, Db } from "mongodb";
import { getDb, isMongoConfigured } from "@/lib/broker/mongoTradesClient";

export const CS_BOOKS = "cs_paper_books";
export const CS_POSITIONS = "cs_paper_positions";
export const CS_TRADES = "cs_paper_trades";
export const CS_STATE = "cs_paper_state";

export const STATE_ID = "crypto_screener_paper";

export type SignalFamily = "scalp" | "swing" | "breakout" | "pattern" | "momentum";

export const FAMILIES: SignalFamily[] = ["scalp", "swing", "breakout", "pattern", "momentum"];

export const FAMILY_LABELS: Record<SignalFamily, string> = {
  scalp: "Scalp",
  swing: "Swing",
  breakout: "Breakout",
  pattern: "Chart Pattern",
  momentum: "Momentum Rank",
};

/**
 * Maximum holding period per family, in hours.
 *
 * Without one, a losing position never closes and the win rate is computed only
 * over the trades that happened to resolve — which is how a desk reports 80%
 * accuracy while its open book bleeds.
 */
export const FAMILY_MAX_HOLD_HOURS: Record<SignalFamily, number> = {
  scalp: 6,
  swing: 120,
  breakout: 72,
  pattern: 120,
  momentum: 240,
};

export type PositionDoc = {
  position_id: string;
  symbol: string;
  family: SignalFamily;
  side: "long" | "short";
  status: "OPEN";

  entry: number;
  /** The price the signal named, before slippage. Kept so fill quality is auditable. */
  signal_price: number;
  stop: number;
  target: number;
  liquidation_price: number;

  contracts: number;
  quantity: number;
  notional_usd: number;
  leverage: number;
  margin_usd: number;
  risk_usd: number;
  maintenance_margin_pct: number;

  entry_fee_usd: number;
  entry_slippage_usd: number;
  /** Funding charged so far, signed from this position's own side. */
  funding_usd: number;
  /** Last instant funding was accrued to. Unix seconds. */
  funding_to: number;

  opened_at: number;
  /** Bars have been replayed up to here. Unix seconds. */
  checked_to: number;
  max_hold_hours: number;

  signal_reason: string | null;
  signal_chips: { label: string; tier: number }[];
  pattern: string | null;
  net_rr_at_entry: number | null;

  ts: number;
};

export type TradeDoc = Omit<PositionDoc, "status"> & {
  status: "CLOSED";
  exit: number;
  /** The level the exit was triggered at, before slippage. */
  exit_level: number;
  exit_reason: "TARGET" | "STOP" | "TIME" | "LIQUIDATION" | "MANUAL";
  closed_at: number;
  exit_fee_usd: number;
  exit_slippage_usd: number;
  gross_pnl_usd: number;
  costs_usd: number;
  net_pnl_usd: number;
  return_pct: number;
  r_multiple: number;
  hold_hours: number;
  /**
   * True when the stop and the target both fell inside one 15-minute bar and
   * the order had to be resolved (or assumed). See `engine.ts`.
   */
  same_bar_ambiguity: boolean;
  ambiguity_resolved_by: "1m-bars" | "assumed-stop-first" | null;
};

export type BookDoc = {
  symbol: string;
  starting_equity_usd: number;
  realised_pnl_usd: number;
  trades: number;
  wins: number;
  losses: number;
  opened_at: number;
  ts: number;
};

export type StateDoc = {
  _id: string;
  last_tick_at: number;
  last_tick_ms: number;
  locked_until: number;
  ticks: number;
  opened_total: number;
  closed_total: number;
  last_opened: number;
  last_closed: number;
  last_error: string | null;
};

export class PaperUnavailableError extends Error {}

export function paperConfigured(): boolean {
  return isMongoConfigured();
}

async function db(): Promise<Db> {
  if (!isMongoConfigured()) {
    throw new PaperUnavailableError(
      "MONGODB_URI is not set on this deployment, so the paper desk has nowhere to keep positions. " +
        "It is reported as unavailable rather than run in memory — an in-memory desk on serverless " +
        "would silently lose every position between requests and report the survivors as its record.",
    );
  }
  return getDb();
}

const indexed = new Set<string>();

/**
 * Indexes, created once per container.
 *
 * The partial unique index on (symbol, family) for OPEN positions is the
 * idempotency guarantee: two concurrent ticks in two lambdas cannot both open
 * the same idea on the same contract. The lease in `state` makes that rare; the
 * index makes it impossible.
 */
async function ensureIndexes(d: Db): Promise<void> {
  if (indexed.has("cs")) return;
  await Promise.all([
    d.collection(CS_POSITIONS).createIndex(
      { symbol: 1, family: 1 },
      { unique: true, name: "uniq_open_symbol_family" },
    ),
    d.collection(CS_POSITIONS).createIndex({ opened_at: -1 }, { name: "by_opened" }),
    d.collection(CS_TRADES).createIndex({ closed_at: -1 }, { name: "by_closed" }),
    d.collection(CS_TRADES).createIndex({ symbol: 1, closed_at: -1 }, { name: "by_symbol_closed" }),
    d.collection(CS_TRADES).createIndex({ family: 1, closed_at: -1 }, { name: "by_family_closed" }),
    d.collection(CS_BOOKS).createIndex({ symbol: 1 }, { unique: true, name: "uniq_symbol" }),
  ]);
  indexed.add("cs");
}

export async function positions(): Promise<Collection<PositionDoc>> {
  const d = await db();
  await ensureIndexes(d);
  return d.collection<PositionDoc>(CS_POSITIONS);
}

export async function trades(): Promise<Collection<TradeDoc>> {
  const d = await db();
  await ensureIndexes(d);
  return d.collection<TradeDoc>(CS_TRADES);
}

export async function books(): Promise<Collection<BookDoc>> {
  const d = await db();
  await ensureIndexes(d);
  return d.collection<BookDoc>(CS_BOOKS);
}

export async function state(): Promise<Collection<StateDoc>> {
  const d = await db();
  return d.collection<StateDoc>(CS_STATE);
}

export async function readState(): Promise<StateDoc | null> {
  const col = await state();
  return col.findOne({ _id: STATE_ID });
}

/**
 * Take the tick lease, or return false because another container holds it.
 *
 * Every read endpoint on this desk can trigger a cycle, so on a busy page load
 * several lambdas race to manage the same positions at the same instant. This
 * is one atomic findOneAndUpdate: whoever flips `locked_until` into the future
 * owns the tick, and everyone else serves what is already stored.
 *
 * The lease EXPIRES rather than being released only on success. A container
 * that dies mid-tick — an OOM, a platform timeout — would otherwise hold the
 * lock forever and freeze the desk permanently, which is a far worse failure
 * than the double-tick the lock exists to prevent. The unique index on
 * (symbol, family) is what makes an expired-lease overlap safe.
 */
export async function acquireTickLease(leaseMs: number): Promise<boolean> {
  const col = await state();
  const now = Date.now();
  const res = await col.findOneAndUpdate(
    { _id: STATE_ID, $or: [{ locked_until: { $lte: now } }, { locked_until: { $exists: false } }] },
    {
      $set: { locked_until: now + leaseMs },
      $setOnInsert: {
        last_tick_at: 0,
        last_tick_ms: 0,
        ticks: 0,
        opened_total: 0,
        closed_total: 0,
        last_opened: 0,
        last_closed: 0,
        last_error: null,
      },
    },
    { upsert: true, returnDocument: "after" },
  );
  return res !== null;
}

export async function releaseTickLease(patch: Partial<StateDoc>): Promise<void> {
  const col = await state();
  await col.updateOne(
    { _id: STATE_ID },
    { $set: { ...patch, locked_until: 0 }, $inc: { ticks: 1 } },
  );
}

/** The book for one symbol, created at its starting equity the first time it is needed. */
export async function ensureBook(symbol: string, startingEquity: number): Promise<BookDoc> {
  const col = await books();
  const now = Date.now();
  const res = await col.findOneAndUpdate(
    { symbol },
    {
      $setOnInsert: {
        symbol,
        starting_equity_usd: startingEquity,
        realised_pnl_usd: 0,
        trades: 0,
        wins: 0,
        losses: 0,
        opened_at: now,
        ts: now,
      },
    },
    { upsert: true, returnDocument: "after" },
  );
  return res as BookDoc;
}

export async function applyTradeToBook(symbol: string, netPnlUsd: number): Promise<void> {
  const col = await books();
  await col.updateOne(
    { symbol },
    {
      $inc: {
        realised_pnl_usd: netPnlUsd,
        trades: 1,
        wins: netPnlUsd > 0 ? 1 : 0,
        losses: netPnlUsd <= 0 ? 1 : 0,
      },
      $set: { ts: Date.now() },
    },
  );
}

export async function resetAll(): Promise<{ books: number; positions: number; trades: number }> {
  const d = await db();
  const [b, p, t] = await Promise.all([
    d.collection(CS_BOOKS).deleteMany({}),
    d.collection(CS_POSITIONS).deleteMany({}),
    d.collection(CS_TRADES).deleteMany({}),
  ]);
  // Typed handle: an untyped collection defaults `_id` to ObjectId, and this
  // one is keyed by a string.
  await d.collection<StateDoc>(CS_STATE).deleteMany({ _id: STATE_ID });
  return { books: b.deletedCount, positions: p.deletedCount, trades: t.deletedCount };
}
