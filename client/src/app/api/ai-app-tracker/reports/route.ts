/**
 * GET /api/ai-app-tracker/reports?limit=20
 *
 * Returns tracker reports newest-first. Max limit 100.
 * No secrets — account_key_suffix is last 4 chars only.
 */

import { NextResponse, type NextRequest } from "next/server";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { listAiTrackerReports } from "@/lib/aiAppTracker/aiAppTrackerMongo";
import { TRACKER_MODULE } from "@/lib/aiAppTracker/trackerConstants";

export const dynamic = "force-dynamic";

export async function GET(request: NextRequest) {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }

  const url = new URL(request.url);
  const limitParam = url.searchParams.get("limit");
  const limit = Math.min(Math.max(1, parseInt(limitParam ?? "20", 10) || 20), 100);

  const reports = await listAiTrackerReports({ limit, module: TRACKER_MODULE });
  return NextResponse.json({ ok: true, reports, count: reports.length });
}
