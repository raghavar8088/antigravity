/**
 * MongoDB helpers for the ai_app_tracker_reports collection.
 * Server-side only — never import from browser code.
 */

import { getDb } from "@/lib/mongoTradesClient";
import type { AiTrackerReport } from "@/lib/aiAppTracker/types";
import { TRACKER_COLLECTION, TRACKER_TTL_SECONDS } from "@/lib/aiAppTracker/trackerConstants";

const indexedTracker = { done: false };

async function ensureTrackerIndexes(): Promise<void> {
  if (indexedTracker.done) return;
  try {
    const db = await getDb();
    const col = db.collection(TRACKER_COLLECTION);
    await Promise.all([
      col.createIndex(
        { created_at: -1 },
        { name: "by_created_at_desc" },
      ),
      col.createIndex(
        { module: 1, created_at: -1 },
        { name: "by_module_created_at" },
      ),
      // TTL: auto-delete reports older than 30 days
      col.createIndex(
        { created_at: 1 },
        { expireAfterSeconds: TRACKER_TTL_SECONDS, name: "ttl_tracker_reports_30d" },
      ),
    ]);
    indexedTracker.done = true;
  } catch {
    // non-fatal — indexes are advisory
  }
}

/** Insert a tracker report. Silently no-ops on error so the caller stays unblocked. */
export async function insertAiTrackerReport(report: AiTrackerReport): Promise<void> {
  try {
    const db = await getDb();
    await ensureTrackerIndexes();
    const col = db.collection<AiTrackerReport>(TRACKER_COLLECTION);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    await (col as any).insertOne({ ...report });
  } catch {
    // non-fatal — monitoring must never crash the desk
  }
}

/** List tracker reports newest-first. Strips MongoDB _id field. */
export async function listAiTrackerReports(opts: {
  limit?: number;
  module?: string;
}): Promise<AiTrackerReport[]> {
  try {
    const db = await getDb();
    await ensureTrackerIndexes();
    const col = db.collection<AiTrackerReport>(TRACKER_COLLECTION);
    const limit = Math.min(opts.limit ?? 20, 100);
    const filter: Record<string, unknown> = {};
    if (opts.module) filter["module"] = opts.module;
    return await col
      .find(filter)
      .sort({ created_at: -1 })
      .limit(limit)
      .toArray()
      .then((docs) => docs.map(({ _id: _ignored, ...rest }) => rest as AiTrackerReport));
  } catch {
    return [];
  }
}

/** Get the single most recent tracker report for a module. */
export async function getLatestAiTrackerReport(module: string): Promise<AiTrackerReport | null> {
  try {
    const db = await getDb();
    await ensureTrackerIndexes();
    const col = db.collection<AiTrackerReport>(TRACKER_COLLECTION);
    const doc = await col
      .find({ module: module as "btc_future_trading" })
      .sort({ created_at: -1 })
      .limit(1)
      .next();
    if (!doc) return null;
    const { _id: _ignored, ...rest } = doc as typeof doc & { _id: unknown };
    return rest as AiTrackerReport;
  } catch {
    return null;
  }
}
