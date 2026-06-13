/**
 * GET /api/desk-worker-events?limit=50
 *
 * Returns recent worker audit events from `desk_worker_events` collection,
 * newest first. Used by the UI "Worker Events" panel.
 *
 * Auth: reads DESK_WORKER_ACCOUNT_KEY from env (same key as the VPS worker).
 * No secrets are returned in the event payload.
 */

import { NextResponse } from "next/server";
import { isMongoConfigured, listWorkerEvents } from "@/lib/broker/mongoTradesClient";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const url = new URL(req.url);
  const limitParam = url.searchParams.get("limit");
  const limit = Math.min(200, Math.max(1, parseInt(limitParam ?? "50", 10) || 50));

  const accountKey = process.env.DESK_WORKER_ACCOUNT_KEY?.trim();
  if (!accountKey) {
    return NextResponse.json({ ok: false, error: "DESK_WORKER_ACCOUNT_KEY not set" }, { status: 503 });
  }

  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }

  const events = await listWorkerEvents(accountKey, limit);

  // Strip account_key from response — no secrets in output
  const safe = events.map(({ account_key: _hidden, ...rest }) => rest);

  return NextResponse.json({ ok: true, events: safe, count: safe.length });
}
