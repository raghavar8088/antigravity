import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import { getAllEvents, getEventsByType } from "@/lib/strategyAuthority/strategyAuthorityMongo";
import type { StrategyEventDoc } from "@/lib/strategyAuthority/types";

export const dynamic = "force-dynamic";

export async function GET(req: Request): Promise<NextResponse> {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, code: "MONGO_NOT_CONFIGURED" }, { status: 503 });
  }
  try {
    const url = new URL(req.url);
    const limit = Math.min(parseInt(url.searchParams.get("limit") ?? "500"), 500);
    const type = url.searchParams.get("type") as StrategyEventDoc["event_type"] | null;

    const events = type
      ? await getEventsByType(type, limit)
      : await getAllEvents(limit);

    return NextResponse.json({ ok: true, events, count: events.length });
  } catch (err) {
    return NextResponse.json(
      { ok: false, code: "EVENTS_ERROR", error: err instanceof Error ? err.message : "unknown" },
      { status: 500 }
    );
  }
}
