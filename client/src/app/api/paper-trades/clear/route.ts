import { NextResponse } from "next/server";
import { isMongoConfigured, getDb } from "@/lib/mongoTradesClient";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";

export const dynamic = "force-dynamic";

// Anonymous access to destructive operations is disabled regardless of env flags.
// ALLOW_PAPER_TRADES_ANON / ALLOW_ANON_PAPER_TRADES must not enable DELETE.

/**
 * DELETE /api/paper-trades/clear
 * Body: { accountKey?: string, beforeMs?: number }
 *
 * Deletes all paper trades for the account that were closed before `beforeMs`
 * (defaults to Date.now() — clears everything). Also resets cleared_at in
 * paper_state to 0 so the client-side filter is no longer needed.
 */
export async function DELETE(req: Request) {
  // Always require an authenticated session — no anonymous destructive operations.
  const session = await getAuthenticatedApiSession();
  if (!session.ok) return session.response;
  const accountKey = session.ctx.userId;

  let body: unknown;
  try { body = await req.json(); } catch { body = {}; }
  const b = (body ?? {}) as Record<string, unknown>;

  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }

  const beforeMs = typeof b.beforeMs === "number" ? b.beforeMs : Date.now();
  const beforeIso = new Date(beforeMs).toISOString();

  try {
    const db = await getDb();
    const result = await db.collection("paper_trades").deleteMany({
      account_key: accountKey,
      closed_at: { $lte: beforeIso },
    });
    // Also reset cleared_at in paper_state so no client-side filter is needed
    await db.collection("paper_state").updateOne(
      { account_key: accountKey },
      { $set: { cleared_at: 0, updated_at: new Date().toISOString() } },
    );
    return NextResponse.json({ ok: true, deletedCount: result.deletedCount, accountKey });
  } catch (err) {
    return NextResponse.json(
      { ok: false, error: err instanceof Error ? err.message : "delete failed" },
      { status: 500 },
    );
  }
}
