import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { listPositions } from "@/lib/paperDeskClient";
import { mongoUnconfigured, mongoUnavailable } from "@/lib/paperDeskErrors";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;
  const accountKey = auth.ctx.userId;

  if (!isMongoConfigured()) return mongoUnconfigured();

  const { searchParams } = new URL(req.url);
  const rawStatus = searchParams.get("status");
  const status =
    rawStatus === "OPEN" || rawStatus === "CLOSED" ? rawStatus : undefined;
  const limit = Math.min(parseInt(searchParams.get("limit") ?? "100", 10) || 100, 500);

  let positions;
  try {
    positions = await listPositions(accountKey, status, limit);
  } catch (err) {
    return mongoUnavailable(err instanceof Error ? err.message : "unknown");
  }

  return NextResponse.json({ ok: true, account_key: accountKey, positions, count: positions.length });
}
