/**
 * GET /api/ai-app-tracker/latest
 *
 * Returns the most recent AI tracker report for the btc_future_trading module.
 * No secrets in response — account_key_suffix is the last 4 chars only.
 */

import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { getLatestAiTrackerReport } from "@/lib/aiAppTrackerMongo";
import { TRACKER_MODULE } from "@/lib/aiAppTracker/trackerConstants";

export const dynamic = "force-dynamic";

export async function GET() {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }

  const report = await getLatestAiTrackerReport(TRACKER_MODULE);

  if (!report) {
    return NextResponse.json(
      { ok: false, error: "No tracker reports found. POST /api/ai-app-tracker/capture to create one." },
      { status: 404 },
    );
  }

  return NextResponse.json({ ok: true, report });
}
