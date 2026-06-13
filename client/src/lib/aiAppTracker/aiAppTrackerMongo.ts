/**
 * MongoDB helpers for the ai_app_tracker_reports collection.
 * Server-side only — never import from browser code.
 *
 * Indexes:
 *   { module: 1, created_at: -1 }   — primary read pattern
 *   { report_id: 1 } unique          — idempotent inserts
 *   { created_at: 1 } TTL 30d        — auto-delete old reports
 */

import { getDb } from "@/lib/broker/mongoTradesClient";
import type { AiAppTrackerReport } from "./types";
import { TRACKER_COLLECTION, TRACKER_TTL_SECONDS } from "./trackerConstants";

let indexesEnsured = false;

async function ensureIndexes(): Promise<void> {
  if (indexesEnsured) return;
  try {
    const db = await getDb();
    const col = db.collection(TRACKER_COLLECTION);
    await Promise.all([
      col.createIndex(
        { module: 1, created_at: -1 },
        { name: "by_module_created_at" },
      ),
      col.createIndex(
        { report_id: 1 },
        { unique: true, name: "uniq_report_id" },
      ),
      col.createIndex(
        { created_at: 1 },
        { expireAfterSeconds: TRACKER_TTL_SECONDS, name: "ttl_tracker_30d" },
      ),
    ]);
    indexesEnsured = true;
  } catch {
    // non-fatal — indexes are advisory; next call will retry
  }
}

/** Insert a tracker report. Silently no-ops on error — monitoring must never crash the desk. */
export async function insertAiTrackerReport(report: AiAppTrackerReport): Promise<void> {
  try {
    const db = await getDb();
    await ensureIndexes();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    await (db.collection(TRACKER_COLLECTION) as any).insertOne({ ...report });
  } catch {
    // non-fatal
  }
}

/** List tracker reports newest-first. Strips MongoDB _id. */
export async function listAiTrackerReports(opts: {
  limit?: number;
  module?: "btc_future_trading";
}): Promise<AiAppTrackerReport[]> {
  try {
    const db = await getDb();
    await ensureIndexes();
    const col = db.collection<AiAppTrackerReport>(TRACKER_COLLECTION);
    const limit = Math.min(opts.limit ?? 20, 100);
    const filter: Record<string, unknown> = {};
    if (opts.module) filter["module"] = opts.module;
    return await col
      .find(filter)
      .sort({ created_at: -1 })
      .limit(limit)
      .toArray()
      .then((docs) => docs.map(({ _id: _ignored, ...rest }) => rest as AiAppTrackerReport));
  } catch {
    return [];
  }
}

/** Get the single most recent tracker report for a module. */
export async function getLatestAiTrackerReport(
  module: "btc_future_trading" = "btc_future_trading",
): Promise<AiAppTrackerReport | null> {
  try {
    const db = await getDb();
    await ensureIndexes();
    const col = db.collection<AiAppTrackerReport>(TRACKER_COLLECTION);
    const doc = await col
      .find({ module })
      .sort({ created_at: -1 })
      .limit(1)
      .next();
    if (!doc) return null;
    const { _id: _ignored, ...rest } = doc as typeof doc & { _id: unknown };
    return rest as AiAppTrackerReport;
  } catch {
    return null;
  }
}
