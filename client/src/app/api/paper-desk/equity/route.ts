import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { getEquityCurve, getDailyPnLHistory } from "@/lib/paperDeskClient";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;
  const accountKey = auth.ctx.userId;

  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }

  const { searchParams } = new URL(req.url);
  const points = Math.min(parseInt(searchParams.get("points") ?? "288", 10) || 288, 2016);
  const days = Math.min(parseInt(searchParams.get("days") ?? "30", 10) || 30, 90);

  const [curve, dailyPnL] = await Promise.all([
    getEquityCurve(accountKey, points),
    getDailyPnLHistory(accountKey, days),
  ]);

  return NextResponse.json({ ok: true, curve, daily_pnl: dailyPnL });
}
