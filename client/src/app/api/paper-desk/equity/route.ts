import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { getEquityCurve, getDailyPnLHistory } from "@/lib/paperDeskClient";
import { mongoUnconfigured, mongoUnavailable } from "@/lib/paperDeskErrors";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;
  const accountKey = auth.ctx.userId;

  if (!isMongoConfigured()) return mongoUnconfigured();

  const { searchParams } = new URL(req.url);
  const points = Math.min(parseInt(searchParams.get("points") ?? "288", 10) || 288, 2016);
  const days = Math.min(parseInt(searchParams.get("days") ?? "30", 10) || 30, 90);

  let curve, dailyPnL;
  try {
    [curve, dailyPnL] = await Promise.all([
      getEquityCurve(accountKey, points),
      getDailyPnLHistory(accountKey, days),
    ]);
  } catch (err) {
    return mongoUnavailable(err instanceof Error ? err.message : "unknown");
  }

  return NextResponse.json({ ok: true, account_key: accountKey, curve, daily_pnl: dailyPnL });
}
