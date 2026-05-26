/**
 * GET /api/ai-app-tracker/latest
 *
 * Returns the most recent AI tracker report for the btc_future_trading module,
 * or { ok: true, report: null } when no reports exist yet.
 * No secrets — account_key_suffix is last 4 chars only.
 */

import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { getLatestAiTrackerReport } from "@/lib/aiAppTracker/aiAppTrackerMongo";
import { TRACKER_MODULE } from "@/lib/aiAppTracker/trackerConstants";

export const dynamic = "force-dynamic";

export async function GET() {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }

  const report = await getLatestAiTrackerReport(TRACKER_MODULE);
  return NextResponse.json({ ok: true, report: report ?? null });
}
