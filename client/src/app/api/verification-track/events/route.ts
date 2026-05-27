/**
 * GET /api/verification-track/events
 * Query params: limit, type, strategy_id, since (ms)
 * Returns newest-first verification events (no secrets).
 */

import { NextResponse, type NextRequest } from "next/server";
import { listVerificationEvents } from "@/lib/verificationTrack/verificationTrackMongo";
import type { VerificationTrackEventType } from "@/lib/verificationTrack/types";

export const dynamic = "force-dynamic";
export const maxDuration = 10;

export async function GET(req: NextRequest) {
  const url = new URL(req.url);
  const limit = Math.min(200, Number(url.searchParams.get("limit") || 50));
  const type = url.searchParams.get("type") as VerificationTrackEventType | null;
  const strategyId = url.searchParams.get("strategy_id") ? Number(url.searchParams.get("strategy_id")) : undefined;
  const since = url.searchParams.get("since") ? Number(url.searchParams.get("since")) : undefined;

  const events = await listVerificationEvents({ limit, type: type ?? undefined, strategyId, sinceMs: since });

  return NextResponse.json({ ok: true, events });
}
