/**
 * GET  /api/ai-app-tracker/capture  — Vercel cron (every 15 min via vercel.json)
 * POST /api/ai-app-tracker/capture  — manual trigger from UI / operator scripts
 *
 * Captures a live AiAppTrackerSnapshot and stores a report in MongoDB.
 * Protected by CRON_SECRET when present.
 *
 * Response: { ok, reportId, severity, summary, recommendations }
 * No secrets in response — account_key_suffix is last 4 chars only.
 */

import { NextResponse, type NextRequest } from "next/server";
import { collectAppSnapshot } from "@/lib/aiAppTracker/collectAppSnapshot";
import { buildTrackerReport } from "@/lib/aiAppTracker/writeTrackerReport";
import { insertAiTrackerReport } from "@/lib/aiAppTracker/aiAppTrackerMongo";

export const dynamic = "force-dynamic";
export const maxDuration = 30;

function authorized(request: NextRequest): boolean {
  const secret = process.env.CRON_SECRET?.trim();
  if (!secret) return true;
  const authHeader = request.headers.get("authorization") ?? "";
  return authHeader === `Bearer ${secret}`;
}

async function runCapture(): Promise<NextResponse> {
  const accountKey = process.env.DESK_WORKER_ACCOUNT_KEY?.trim();
  const snapshot = await collectAppSnapshot({ accountKey });
  const report = buildTrackerReport(snapshot);
  await insertAiTrackerReport(report);

  return NextResponse.json({
    ok: true,
    reportId: report.report_id,
    severity: report.severity,
    summary: report.summary,
    recommendations: report.recommendations,
  });
}

/** Vercel cron fires GET — CRON_SECRET auth. */
export async function GET(request: NextRequest) {
  if (!authorized(request)) {
    return NextResponse.json({ ok: false, error: "Unauthorized" }, { status: 401 });
  }
  return runCapture();
}

/** Manual trigger from UI "Capture report now" button or operator scripts. */
export async function POST(request: NextRequest) {
  if (!authorized(request)) {
    return NextResponse.json({ ok: false, error: "Unauthorized" }, { status: 401 });
  }
  return runCapture();
}
