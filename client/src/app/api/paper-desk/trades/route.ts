import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { listPaperTrades, countPaperTrades } from "@/lib/paperDeskClient";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;
  const accountKey = auth.ctx.userId;

  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }

  const { searchParams } = new URL(req.url);
  const limit = Math.min(parseInt(searchParams.get("limit") ?? "50", 10) || 50, 200);
  const offset = Math.max(parseInt(searchParams.get("offset") ?? "0", 10) || 0, 0);
  const cursor = searchParams.get("cursor") ?? undefined;
  const strategyId = searchParams.get("strategy_id") ?? undefined;
  const symbol = searchParams.get("symbol") ?? undefined;
  const side = searchParams.get("side") ?? undefined;
  const sortBy = (searchParams.get("sort_by") ?? "closed_at") as "closed_at" | "net_pnl";
  const sortDir = (searchParams.get("sort_dir") ?? "desc") as "asc" | "desc";

  const [trades, total] = await Promise.all([
    listPaperTrades({ accountKey, limit, offset, cursor, strategyId, symbol, side, sortBy, sortDir }),
    countPaperTrades(accountKey),
  ]);

  return NextResponse.json({
    ok: true,
    trades,
    pagination: { limit, offset, total, has_more: offset + trades.length < total },
  });
}
