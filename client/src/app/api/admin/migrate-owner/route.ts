/**
 * POST /api/admin/migrate-owner
 *
 * One-time migration: renames all anon_* account_key values in MongoDB to
 * "owner_admin" so all trade history, balances, and scores unify under the
 * single owner account.
 *
 * Security: requires valid raig_session JWT (must be already logged in as owner).
 *
 * Request body:
 *   { dryRun?: boolean }   — when true, reports what WOULD be changed without writing
 *
 * Response:
 *   {
 *     ok: true,
 *     dryRun: boolean,
 *     collections: Array<{ name, matched, updated }>,
 *     totalMatched: number,
 *     totalUpdated: number,
 *     fromKeys: string[],
 *   }
 *
 * Idempotent: running it twice is safe — second run finds 0 documents to update.
 */

import { NextResponse } from "next/server";
import { getDb } from "@/lib/mongoTradesClient";
import { getAuthenticatedApiSession } from "@/lib/apiSessionAuth";
import { OWNER_ACCOUNT_KEY } from "@/lib/ownerAuth";

const COLLECTIONS_TO_MIGRATE = [
  "paper_trades",
  "paper_state",
  "paper_oms_orders",
  "equity_curve",
  "daily_pnl_history",
  "strategy_scores",
  "strategy_score_history",
  "strategy_signals",
  "mock_trades",
  "mock_account_snapshots",
  "mock_strategy_analytics",
  "mock_trade_logs",
  "mock_engine_config",
  "strategy_research",
  "desk_worker_events",
  "verification_track",
  "ai_app_tracker",
];

export async function POST(req: Request) {
  // Must be authenticated as owner
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;

  let dryRun = false;
  try {
    const body = (await req.json()) as { dryRun?: unknown };
    dryRun = body.dryRun === true;
  } catch {
    // body is optional
  }

  try {
    const db = await getDb();
    const existingCollections = await db
      .listCollections({}, { nameOnly: true })
      .toArray()
      .then((cols) => new Set(cols.map((c) => c.name)));

    const results: Array<{ name: string; matched: number; updated: number; fromKeys: string[] }> =
      [];
    let totalMatched = 0;
    let totalUpdated = 0;
    const allFromKeys = new Set<string>();

    for (const colName of COLLECTIONS_TO_MIGRATE) {
      if (!existingCollections.has(colName)) {
        results.push({ name: colName, matched: 0, updated: 0, fromKeys: [] });
        continue;
      }

      const col = db.collection(colName);

      // Find all distinct anon_ account_keys in this collection
      const anonKeys = (await col.distinct("account_key", {
        account_key: { $regex: "^anon_" },
      })) as string[];

      const matched = anonKeys.length > 0
        ? await col.countDocuments({ account_key: { $in: anonKeys } })
        : 0;

      anonKeys.forEach((k) => allFromKeys.add(k));

      let updated = 0;
      if (!dryRun && matched > 0) {
        // Check if a document with the target account_key already exists (unique-index collections).
        // If so, delete the anon duplicate instead of updating to avoid E11000.
        const ownerExists = await col.findOne({ account_key: OWNER_ACCOUNT_KEY });
        if (ownerExists) {
          // Singleton collection (e.g. paper_state) — drop the stale anon doc, owner doc wins.
          const del = await col.deleteMany({ account_key: { $in: anonKeys } });
          updated = del.deletedCount;
        } else {
          const result = await col.updateMany(
            { account_key: { $in: anonKeys } },
            { $set: { account_key: OWNER_ACCOUNT_KEY } },
          );
          updated = result.modifiedCount;
        }
      } else {
        updated = matched; // in dry-run, report what would be updated
      }

      totalMatched += matched;
      totalUpdated += updated;
      results.push({ name: colName, matched, updated, fromKeys: anonKeys });
    }

    return NextResponse.json({
      ok: true,
      dryRun,
      targetAccountKey: OWNER_ACCOUNT_KEY,
      fromKeys: [...allFromKeys],
      totalMatched,
      totalUpdated,
      collections: results,
      message: dryRun
        ? `Dry run: ${totalMatched} documents would be migrated across ${results.filter((r) => r.matched > 0).length} collections.`
        : `Migrated ${totalUpdated} documents across ${results.filter((r) => r.updated > 0).length} collections.`,
    });
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    return NextResponse.json({ ok: false, error: message }, { status: 500 });
  }
}
