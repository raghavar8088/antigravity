import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import { buildPlatformEvents } from "@/lib/trading/platformEvents";
import { mongoUnconfigured, mongoUnavailable } from "@/lib/trading/mockTradingApiErrors";
import { OWNER_ACCOUNT_KEY } from "@/lib/broker/ownerAuth";

export const dynamic = "force-dynamic";

export async function GET() {
  if (!isMongoConfigured()) return mongoUnconfigured();

  try {
    const events = await buildPlatformEvents(OWNER_ACCOUNT_KEY);
    return NextResponse.json({
      ok: true,
      account_key: OWNER_ACCOUNT_KEY,
      execution_authority: "mock-trading",
      event_count: events.length,
      events,
      server_time: new Date().toISOString(),
    });
  } catch (err) {
    return mongoUnavailable(err instanceof Error ? err.message : "unknown");
  }
}
