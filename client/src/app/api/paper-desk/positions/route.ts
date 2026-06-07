import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { listPositions } from "@/lib/paperDeskClient";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;
  const accountKey = auth.ctx.userId;

  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }

  const { searchParams } = new URL(req.url);
  const rawStatus = searchParams.get("status");
  const status =
    rawStatus === "OPEN" || rawStatus === "CLOSED" ? rawStatus : undefined;
  const limit = Math.min(parseInt(searchParams.get("limit") ?? "100", 10) || 100, 500);

  const positions = await listPositions(accountKey, status, limit);
  return NextResponse.json({ ok: true, positions, count: positions.length });
}
