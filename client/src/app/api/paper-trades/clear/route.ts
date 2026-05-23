import { NextResponse } from "next/server";
import { cookies } from "next/headers";
import { isMongoConfigured, getDb } from "@/lib/mongoTradesClient";
import { verifySession, SESSION_COOKIE } from "@/lib/jwtSession";

export const dynamic = "force-dynamic";

function anonAllowed(): boolean {
  return (
    process.env.ALLOW_PAPER_TRADES_ANON === "1" ||
    process.env.ALLOW_ANON_PAPER_TRADES === "1"
  );
}

/**
 * DELETE /api/paper-trades/clear
 * Body: { accountKey?: string, beforeMs?: number }
 *
 * Deletes all paper trades for the account that were closed before `beforeMs`
 * (defaults to Date.now() — clears everything). Also resets cleared_at in
 * paper_state to 0 so the client-side filter is no longer needed.
 */
export async function DELETE(req: Request) {
  let body: unknown;
  try { body = await req.json(); } catch { body = {}; }
  const b = (body ?? {}) as Record<string, unknown>;

  // Resolve account key (session cookie → body → anon)
  let accountKey: string | null = null;
  try {
    const cookieStore = await cookies();
    const token = cookieStore.get(SESSION_COOKIE)?.value;
    if (token && process.env.AUTH_JWT_SECRET) {
      const session = verifySession(token);
      if (session?.userId) accountKey = session.userId;
    }
  } catch { /* fall through */ }

  if (!accountKey) {
    const bodyKey = typeof b.accountKey === "string" ? b.accountKey.trim() : null;
    if (!bodyKey) {
      return NextResponse.json({ ok: false, error: "accountKey required" }, { status: 400 });
    }
    if (!anonAllowed()) {
      return NextResponse.json({ ok: false, error: "Anonymous deletes disabled" }, { status: 401 });
    }
    accountKey = bodyKey;
  }

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
