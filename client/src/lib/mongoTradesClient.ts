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

/** Return `true` only when MONGODB_URI is a non-empty `mongodb+srv://` or `mongodb://` URI. */
export function isMongoConfigured(): boolean {
  const uri = process.env.MONGODB_URI;
  if (typeof uri !== "string") return false;
  const trimmed = uri.trim();
  if (trimmed.length === 0) return false;
  return trimmed.startsWith("mongodb+srv://") || trimmed.startsWith("mongodb://");
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

// Track which collections already have indexes to avoid redundant createIndex calls.
const indexedCollections = new Set<string>();

async function ensureIndexes(entry: CachedClient, collectionName: string): Promise<void> {
  if (indexedCollections.has(collectionName)) return;
  const col = entry.db.collection<PaperTradeDbRow>(collectionName);
  await Promise.all([
    col.createIndex({ client_trade_id: 1 }, { unique: true, name: "uniq_client_trade_id" }),
    col.createIndex({ account_key: 1, closed_at: -1 }, { name: "by_account_closed" }),
    col.createIndex({ account_key: 1, strategy_id: 1, closed_at: -1 }, { name: "by_account_strat_closed" }),
    col.createIndex({ account_key: 1, module_key: 1, closed_at: -1 }, { name: "by_account_module_closed" }),
  ]);
  indexedCollections.add(collectionName);
}

/** Shared DB handle — lets other modules (e.g. mongoAuthClient) reuse the same connection pool. */
export async function getDb(): Promise<Db> {
  const entry = await connect();
  return entry.db;
}

/** Returns true when Atlas is reachable; used by the health endpoint. */
export async function pingMongo(): Promise<boolean> {
  try {
    const entry = await connect();
    await entry.db.command({ ping: 1 });
    return true;
  } catch {
    return false;
  }
}

export async function getTradesCollection(collectionName = TRADES_COLLECTION): Promise<Collection<PaperTradeDbRow>> {
  const entry = await connect();
  await ensureIndexes(entry, collectionName);
  return entry.db.collection<PaperTradeDbRow>(collectionName);
}

export type UpsertTradeResult =
  | { ok: true; upsertedCount: number; modifiedCount: number; matchedCount: number }
  | { ok: false; error: string };

/**
 * Upsert keyed by `client_trade_id`. `$set` writes the full row on every call;
 * `$setOnInsert` stamps `created_at` only when the document is first created.
 *
 * Re-sending the same `client_trade_id` overwrites the row (idempotent by key,
 * but values can be refreshed — useful for late-arriving fees/funding updates).
 *
 * Returns a structured result; never throws on driver/connection errors so the
 * route handler can map them to consistent error codes.
 */
export async function upsertTradeMongo(
  row: Omit<PaperTradeDbRow, "id" | "created_at">,
  collectionName = TRADES_COLLECTION,
): Promise<UpsertTradeResult> {
  try {
    const col = await getTradesCollection(collectionName);
    const now = new Date().toISOString();
    const result = await col.updateOne(
      { client_trade_id: row.client_trade_id },
      {
        $set: { ...row },
        $setOnInsert: { created_at: now },
      },
      { upsert: true },
    );
    return {
      ok: true,
      upsertedCount: result.upsertedCount ?? 0,
      modifiedCount: result.modifiedCount ?? 0,
      matchedCount: result.matchedCount ?? 0,
    };
  } catch (err) {
    return {
      ok: false,
      error: err instanceof Error ? err.message : "unknown mongo error",
    };
  }
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
export async function listTradesMongo(opts: ListTradesOpts & { collectionName?: string }): Promise<PaperTradeDbRow[]> {
  const col = await getTradesCollection(opts.collectionName ?? TRADES_COLLECTION);
  const filter: Record<string, unknown> = { account_key: opts.accountKey };
  if (opts.cursor) {
    filter.closed_at = { $lt: opts.cursor };
  }
  if (opts.moduleKey) {
    filter.module_key = opts.moduleKey;
  }
  return col.find(filter).sort({ closed_at: -1 }).limit(opts.limit).toArray();
}

// ── paper_state collection ────────────────────────────────────────────────────

const STATE_COLLECTION = "paper_state";
const RESEARCH_COLLECTION = "paper_research";

export type PaperStateDoc = {
  account_key: string;
  balance: number;
  positions: unknown[];
  pause_entries: boolean;
  disabled_strategies: number[];
  last_trade_at: number;
  day_start_balance: number;
  day_start_date: number;
  cleared_at: number;
  updated_at: string;
};

export type PaperResearchDoc = {
  account_key: string;
  namespace: string;
  winners: number[];
  retired_ids: number[];
  updated_at: string;
};

export async function getAccountState(accountKey: string): Promise<PaperStateDoc | null> {
  try {
    const entry = await connect();
    const col = entry.db.collection<PaperStateDoc>(STATE_COLLECTION);
    await col.createIndex({ account_key: 1 }, { unique: true, name: "uniq_account_key_state" }).catch(() => {});
    return await col.findOne({ account_key: accountKey }) ?? null;
  } catch {
    return null;
  }
}

export async function upsertAccountState(doc: PaperStateDoc): Promise<void> {
  try {
    const entry = await connect();
    const col = entry.db.collection<PaperStateDoc>(STATE_COLLECTION);
    await col.createIndex({ account_key: 1 }, { unique: true, name: "uniq_account_key_state" }).catch(() => {});
    await col.updateOne(
      { account_key: doc.account_key },
      { $set: { ...doc, updated_at: new Date().toISOString() } },
      { upsert: true },
    );
  } catch {
    // non-fatal — periodic save; next interval will retry
  }
}

export async function getResearchState(accountKey: string, namespace: string): Promise<PaperResearchDoc | null> {
  try {
    const entry = await connect();
    const col = entry.db.collection<PaperResearchDoc>(RESEARCH_COLLECTION);
    await col.createIndex({ account_key: 1, namespace: 1 }, { unique: true, name: "uniq_account_ns" }).catch(() => {});
    return await col.findOne({ account_key: accountKey, namespace }) ?? null;
  } catch {
    return null;
  }
}

export async function upsertResearchState(doc: PaperResearchDoc): Promise<void> {
  try {
    const entry = await connect();
    const col = entry.db.collection<PaperResearchDoc>(RESEARCH_COLLECTION);
    await col.createIndex({ account_key: 1, namespace: 1 }, { unique: true, name: "uniq_account_ns" }).catch(() => {});
    await col.updateOne(
      { account_key: doc.account_key, namespace: doc.namespace },
      { $set: { ...doc, updated_at: new Date().toISOString() } },
      { upsert: true },
    );
  } catch {
    // non-fatal
  }
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
