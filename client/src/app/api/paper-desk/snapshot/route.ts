import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import {
  getPaperState,
  listOpenPositions,
  listPaperTrades,
  getStrategyHealthSummary,
} from "@/lib/paperDeskClient";

export const dynamic = "force-dynamic";

/**
 * Aggregated polling snapshot. The /paper-desk page polls THIS single endpoint
 * every 5 seconds instead of fanning out to 6 routes, so each device makes one
 * request per tick. Detail views (full trade pagination, OMS orders, equity
 * curve, full strategy table) use the dedicated routes on demand.
 */
export async function GET() {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;
  const accountKey = auth.ctx.userId;

  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }

  const [state, openPositions, recentTrades, healthSummary] = await Promise.all([
    getPaperState(accountKey),
    listOpenPositions(accountKey),
    listPaperTrades({ accountKey, limit: 20 }),
    getStrategyHealthSummary(accountKey),
  ]);

  return NextResponse.json({
    ok: true,
    server_time: new Date().toISOString(),
    state,
    open_positions: openPositions,
    recent_trades: recentTrades,
    health_summary: healthSummary,
  });
}
