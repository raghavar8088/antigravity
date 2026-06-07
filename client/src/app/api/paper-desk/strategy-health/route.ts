import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import {
  listStrategyScores,
  listStrategyHealth,
  getStrategyHealthSummary,
} from "@/lib/paperDeskClient";
import { mongoUnconfigured, mongoUnavailable } from "@/lib/paperDeskErrors";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;
  const accountKey = auth.ctx.userId;

  if (!isMongoConfigured()) return mongoUnconfigured();

  const { searchParams } = new URL(req.url);
  const status = searchParams.get("status") ?? undefined;
  const limit = Math.min(parseInt(searchParams.get("limit") ?? "200", 10) || 200, 600);

  let scores, health, summary;
  try {
    [scores, health, summary] = await Promise.all([
      listStrategyScores(accountKey, limit),
      listStrategyHealth(accountKey, status, limit),
      getStrategyHealthSummary(accountKey),
    ]);
  } catch (err) {
    return mongoUnavailable(err instanceof Error ? err.message : "unknown");
  }

  return NextResponse.json({ ok: true, account_key: accountKey, scores, health, summary });
}
