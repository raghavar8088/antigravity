/**
 * GET /api/verification-track/summary?window_minutes=60
 * Aggregates counts, top blockers, opened/closed, latest root cause.
 */

import { NextResponse, type NextRequest } from "next/server";
import { listVerificationEvents } from "@/lib/verificationTrack/verificationTrackMongo";

export const dynamic = "force-dynamic";

export async function GET(req: NextRequest) {
  const url = new URL(req.url);
  const windowMin = Math.min(1440, Number(url.searchParams.get("window_minutes") || 60));
  const sinceMs = Date.now() - windowMin * 60_000;

  const events = await listVerificationEvents({ limit: 500, sinceMs });

  const countsByType: Record<string, number> = {};
  const blockers: Record<string, number> = {};
  let opened = 0;
  let closed = 0;
  let latestRootCause: string | null = null;
  let latestWorkerHealth: string | null = null;

  for (const e of events) {
    countsByType[e.type] = (countsByType[e.type] ?? 0) + 1;
    if (e.type === "POSITION_OPENED") opened++;
    if (e.type === "POSITION_CLOSED") closed++;
    if (e.type === "NO_TRADE_ROOT_CAUSE" && !latestRootCause) latestRootCause = e.summary;
    if (e.type === "WORKER_HEALTH" && !latestWorkerHealth) latestWorkerHealth = e.summary;
    if (e.dominant_blocker) blockers[e.dominant_blocker] = (blockers[e.dominant_blocker] ?? 0) + 1;
  }

  const topBlockers = Object.entries(blockers).sort((a, b) => b[1] - a[1]).slice(0, 5).map(([b, c]) => ({ blocker: b, count: c }));

  const recommendations: string[] = [];
  if (latestRootCause?.includes("WORKER_STALE")) recommendations.push("Restart pm2 worker");
  if (topBlockers[0]?.blocker === "signal") recommendations.push("Wait for stronger market setup (correct behavior)");

  return NextResponse.json({
    ok: true,
    windowMinutes: windowMin,
    countsByType,
    topBlockers,
    opened,
    closed,
    latestRootCause,
    latestWorkerHealth,
    recommendations,
  });
}
