/**
 * GET /api/desk-entry-funnel
 *
 * Returns the latest entry funnel snapshot stored in paper_state by the worker
 * or browser poll. Used by the "Why no BTC trades?" diagnostic panel and the
 * VPS monitoring dashboard.
 *
 * Query params:
 *   account_key  — optional override; defaults to DESK_WORKER_ACCOUNT_KEY env.
 *
 * Response:
 *   { ok: true, snapshot: EntryFunnelSnapshot, ageSeconds: number, healthy: boolean }
 *   { ok: false, error: string }
 *
 * healthy = true when ageSeconds < 30 (i.e. the snapshot is from a live worker).
 * No secrets (account_key env value) are reflected in the response.
 */

import { NextResponse, type NextRequest } from "next/server";
import { isMongoConfigured, getAccountState, getDb } from "@/lib/mongoTradesClient";
import type { EntryFunnelSnapshot } from "@/lib/deskEntryFunnelSnapshot";

export const dynamic = "force-dynamic";

const HEALTHY_AGE_SECONDS = 30;

export async function GET(request: NextRequest) {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }

  // Resolve account key: query param takes precedence, falls back to env.
  const url = new URL(request.url);
  const qpKey = url.searchParams.get("account_key")?.trim();
  const accountKey = qpKey || process.env.DESK_WORKER_ACCOUNT_KEY?.trim();

  if (!accountKey) {
    return NextResponse.json(
      { ok: false, error: "account_key required (query param or DESK_WORKER_ACCOUNT_KEY env)" },
      { status: 400 },
    );
  }

  const state = await getAccountState(accountKey);

  if (!state) {
    return NextResponse.json({ ok: false, error: "No paper_state found for this account" }, { status: 404 });
  }

  const snapshot = state.entry_funnel_snapshot as EntryFunnelSnapshot | null | undefined;

  if (!snapshot) {
    return NextResponse.json(
      { ok: false, error: "No funnel snapshot recorded yet — worker has not run a tick" },
      { status: 404 },
    );
  }

  const tickAt = typeof snapshot.tickAt === "number" ? snapshot.tickAt : 0;
  const ageSeconds = tickAt > 0 ? Math.floor((Date.now() - tickAt) / 1000) : null;
  const healthy = ageSeconds !== null && ageSeconds < HEALTHY_AGE_SECONDS;

  return NextResponse.json({
    ok: true,
    snapshot,
    ageSeconds,
    healthy,
  });
}

/**
 * POST /api/desk-entry-funnel
 *
 * Browser-side persistence: lightweight patch that writes only entry_funnel_snapshot
 * to paper_state without touching the rest of the document. Fire-and-forget from
 * the browser poll loop every tick.
 *
 * Body: { accountKey: string; snapshot: EntryFunnelSnapshot }
 */
export async function POST(request: NextRequest) {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }

  let body: unknown;
  try { body = await request.json(); } catch {
    return NextResponse.json({ ok: false, error: "Invalid JSON" }, { status: 400 });
  }

  const b = body as Record<string, unknown>;
  const accountKey = typeof b.accountKey === "string" ? b.accountKey.trim() : null;
  if (!accountKey) {
    return NextResponse.json({ ok: false, error: "accountKey required" }, { status: 400 });
  }

  const snapshot = b.snapshot;
  if (!snapshot || typeof snapshot !== "object") {
    return NextResponse.json({ ok: false, error: "snapshot required" }, { status: 400 });
  }

  try {
    const db = await getDb();
    await db.collection("paper_state").updateOne(
      { account_key: accountKey },
      { $set: { entry_funnel_snapshot: snapshot, updated_at: new Date().toISOString() } },
      { upsert: false }, // only update existing state; browser shouldn't create the doc
    );
  } catch (err) {
    return NextResponse.json(
      { ok: false, error: err instanceof Error ? err.message : "db error" },
      { status: 500 },
    );
  }

  return NextResponse.json({ ok: true });
}
