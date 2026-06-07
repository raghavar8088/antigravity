import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { listPaperOrders } from "@/lib/paperDeskClient";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;
  const accountKey = auth.ctx.userId;

  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }

  const { searchParams } = new URL(req.url);
  const orderId = searchParams.get("order_id") ?? undefined;
  const strategyId = searchParams.get("strategy_id") ?? undefined;
  const limit = Math.min(parseInt(searchParams.get("limit") ?? "100", 10) || 100, 500);

  const orders = await listPaperOrders({ accountKey, orderId, strategyId, limit });
  return NextResponse.json({ ok: true, orders, count: orders.length });
}
