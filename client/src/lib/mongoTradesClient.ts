/**
 * MongoDB client for paper trades storage (primary persistence).
 *
 * Architecture:
 *   - MongoDB Atlas (M0 free cluster) = PRIMARY trade store
 *   - Supabase paper_trades table     = PARALLEL safety mirror (best-effort)
 *
 * Connection pooling pattern is Node.js-best-practice for serverless (Vercel) +
 * long-running (EC2) environments: a single shared MongoClient is cached on the
 * module scope and reused across hot reloads, so we don't exhaust the Atlas
 * connection pool on lambda warm starts.
 *
 * Schema: documents in the `paper_trades` collection mirror the Supabase row
 * shape (PaperTradeDbRow) 1:1 so existing analytics/mapper code keeps working
 * when reading from MongoDB.
 *
 * Indexes (created lazily on first use):
 *   - { client_trade_id: 1 } unique  → idempotent upserts (same as Supabase)
 *   - { account_key: 1, closed_at: -1 }  → primary read pattern (per-user list)
 */

import { MongoClient, type Collection, type Db } from "mongodb";
import type { PaperTradeDbRow } from "@/lib/paperTradesTypes";

const TRADES_COLLECTION = "paper_trades";
const DEFAULT_DB_NAME = "loop_trades";

type CachedClient = {
  client: MongoClient;
  db: Db;
  indexesEnsured: boolean;
};

// Module-scope cache (survives hot reloads in dev; per-instance in serverless).
let cached: CachedClient | null = null;
let connectPromise: Promise<CachedClient> | null = null;

/** Return `true` only when MONGODB_URI is configured. Used to gate dual-write logic. */
export function isMongoConfigured(): boolean {
  const uri = process.env.MONGODB_URI;
  return typeof uri === "string" && uri.trim().length > 0;
}

async function connect(): Promise<CachedClient> {
  if (cached) return cached;
  if (connectPromise) return connectPromise;

  const uri = process.env.MONGODB_URI;
  if (!uri || uri.trim().length === 0) {
    throw new Error("MONGODB_URI not set");
  }
  const dbName = process.env.MONGODB_DB?.trim() || DEFAULT_DB_NAME;

  connectPromise = (async () => {
    const client = new MongoClient(uri, {
      // Atlas M0 caps at 500 conns project-wide; keep our pool small.
      maxPoolSize: 10,
      minPoolSize: 0,
      // Fail fast in serverless when the cluster is unreachable.
      serverSelectionTimeoutMS: 5_000,
      connectTimeoutMS: 5_000,
    });
    await client.connect();
    const db = client.db(dbName);
    const entry: CachedClient = { client, db, indexesEnsured: false };
    cached = entry;
    return entry;
  })().catch((err) => {
    connectPromise = null; // allow retry on next call
    throw err;
  });

  return connectPromise;
}

async function ensureIndexes(entry: CachedClient): Promise<void> {
  if (entry.indexesEnsured) return;
  const col = entry.db.collection<PaperTradeDbRow>(TRADES_COLLECTION);
  await Promise.all([
    col.createIndex({ client_trade_id: 1 }, { unique: true, name: "uniq_client_trade_id" }),
    col.createIndex({ account_key: 1, closed_at: -1 }, { name: "by_account_closed" }),
    col.createIndex({ account_key: 1, strategy_id: 1, closed_at: -1 }, { name: "by_account_strat_closed" }),
    // Module-scoped reads (per-tab leaderboards, exports filtered by module).
    col.createIndex({ account_key: 1, module_key: 1, closed_at: -1 }, { name: "by_account_module_closed" }),
  ]);
  entry.indexesEnsured = true;
}

export async function getTradesCollection(): Promise<Collection<PaperTradeDbRow>> {
  const entry = await connect();
  await ensureIndexes(entry);
  return entry.db.collection<PaperTradeDbRow>(TRADES_COLLECTION);
}

/**
 * Idempotent upsert on `client_trade_id`. Mirrors the Supabase
 * `.upsert({}, { onConflict: "client_trade_id", ignoreDuplicates: true })` semantics
 * — re-sending the same trade is a no-op.
 */
export async function upsertTradeMongo(row: Omit<PaperTradeDbRow, "id" | "created_at">): Promise<void> {
  const col = await getTradesCollection();
  const now = new Date().toISOString();
  await col.updateOne(
    { client_trade_id: row.client_trade_id },
    {
      $setOnInsert: {
        created_at: now,
        ...row,
      },
    },
    { upsert: true },
  );
}

export type ListTradesOpts = {
  accountKey: string;
  limit: number;
  /** ISO timestamp; rows with `closed_at < cursor` are returned (cursor pagination). */
  cursor?: string;
  /** Optional module filter (e.g. show only btc_option_buying trades). */
  moduleKey?: string;
};

/** Mirrors the Supabase GET path: per-account list ordered by closed_at desc. */
export async function listTradesMongo(opts: ListTradesOpts): Promise<PaperTradeDbRow[]> {
  const col = await getTradesCollection();
  const filter: Record<string, unknown> = { account_key: opts.accountKey };
  if (opts.cursor) {
    filter.closed_at = { $lt: opts.cursor };
  }
  if (opts.moduleKey) {
    filter.module_key = opts.moduleKey;
  }
  return col.find(filter).sort({ closed_at: -1 }).limit(opts.limit).toArray();
}

/**
 * Test-only: close cached client (used by Vitest cleanup so the suite doesn't hang
 * on open sockets). Safe no-op when nothing is connected.
 */
export async function _closeMongoForTests(): Promise<void> {
  if (cached) {
    await cached.client.close().catch(() => undefined);
    cached = null;
    connectPromise = null;
  }
}
