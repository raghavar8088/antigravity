import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { buildPlatformEvents } from "@/lib/platformEvents";
import { mongoUnconfigured, mongoUnavailable } from "@/lib/paperDeskErrors";

export const dynamic = "force-dynamic";

export async function GET() {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;
  if (!isMongoConfigured()) return mongoUnconfigured();

  try {
    const events = await buildPlatformEvents(auth.ctx.userId);
    return NextResponse.json({
      ok: true,
      account_key: auth.ctx.userId,
      event_count: events.length,
      events,
      server_time: new Date().toISOString(),
    });
  } catch (err) {
    return mongoUnavailable(err instanceof Error ? err.message : "unknown");
  }
}
