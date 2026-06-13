/**
 * MongoDB persistence for Verification Track events.
 *
 * Collection: verification_track_events
 * - Append-only diagnostic log (never mutates trading state).
 * - TTL index on created_at (30 days).
 * - Writes are best-effort; trading must continue if Mongo is unreachable.
 * - Only account_key_suffix is stored (never full key).
 */

import { getDb, isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import type { VerificationTrackEvent } from "./types";

const COLLECTION = "verification_track_events";
const TTL_DAYS = 30;

let indexesEnsured = false;

async function ensureVerificationIndexes(): Promise<void> {
  if (indexesEnsured || !isMongoConfigured()) return;
  try {
    const db = await getDb();
    const col = db.collection<VerificationTrackEvent>(COLLECTION);

    await Promise.all([
      col.createIndex({ module: 1, created_at: -1 }, { name: "by_module_time" }),
      col.createIndex({ type: 1, created_at: -1 }, { name: "by_type_time" }),
      col.createIndex({ strategy_id: 1, created_at: -1 }, { name: "by_strategy_time", sparse: true }),
      col.createIndex({ event_id: 1 }, { unique: true, name: "uniq_event_id" }),
      col.createIndex({ created_at: 1 }, { expireAfterSeconds: TTL_DAYS * 86400, name: "ttl_created_at" }),
    ]);
    indexesEnsured = true;
  } catch {
    // Non-fatal — indexes are best-effort.
  }
}

/** Insert a single verification event (non-fatal). */
export async function insertVerificationEvent(event: VerificationTrackEvent): Promise<void> {
  if (!isMongoConfigured()) return;
  try {
    await ensureVerificationIndexes();
    const db = await getDb();
    const col = db.collection<VerificationTrackEvent>(COLLECTION);
    await col.insertOne(event);
  } catch {
    // Swallow errors — verification must never break trading.
  }
}

/** Batch insert (non-fatal). */
export async function insertVerificationEvents(events: VerificationTrackEvent[]): Promise<void> {
  if (!isMongoConfigured() || events.length === 0) return;
  try {
    await ensureVerificationIndexes();
    const db = await getDb();
    const col = db.collection<VerificationTrackEvent>(COLLECTION);
    await col.insertMany(events, { ordered: false });
  } catch {
    // Non-fatal.
  }
}

/** List newest-first events with optional filters. */
export async function listVerificationEvents(opts: {
  limit?: number;
  type?: VerificationTrackEvent["type"];
  strategyId?: number;
  sinceMs?: number;
}): Promise<VerificationTrackEvent[]> {
  if (!isMongoConfigured()) return [];
  try {
    const db = await getDb();
    const col = db.collection<VerificationTrackEvent>(COLLECTION);
    const query: Record<string, unknown> = {};
    if (opts.type) query.type = opts.type;
    if (opts.strategyId) query.strategy_id = opts.strategyId;
    if (opts.sinceMs) query.created_at = { $gte: new Date(opts.sinceMs).toISOString() };

    return await col
      .find(query)
      .sort({ created_at: -1 })
      .limit(Math.min(opts.limit ?? 100, 500))
      .toArray();
  } catch {
    return [];
  }
}

/** Convenience: latest N events. */
export async function getLatestVerificationEvents(limit = 50): Promise<VerificationTrackEvent[]> {
  return listVerificationEvents({ limit });
}
