import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";
import { isMongoConfigured, getTradesCollection } from "@/lib/mongoTradesClient";
import { buildCorrelationMatrix, type TradeRow } from "@/lib/paperTradeAnalyticsApi";
import { mongoUnconfigured, mongoUnavailable } from "@/lib/paperDeskErrors";

export const dynamic = "force-dynamic";

export async function GET() {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;
  if (!isMongoConfigured()) return mongoUnconfigured();

  const cutoff = new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString();

  try {
    const col = await getTradesCollection();
    const docs = await col
      .find({ account_key: auth.ctx.userId, closed_at: { $gte: cutoff } })
      .project({ strategy_id: 1, net_pnl: 1, closed_at: 1, _id: 0 })
      .toArray();

    const trades: TradeRow[] = docs.map((d) => ({
      strategy_id: String(d.strategy_id ?? ""),
      net_pnl: Number(d.net_pnl ?? 0),
      closed_at: String(d.closed_at ?? ""),
    }));

    const { labels, matrix } = buildCorrelationMatrix(trades);
    return NextResponse.json({ ok: true, labels, matrix, trade_count: trades.length });
  } catch (err) {
    return mongoUnavailable(err instanceof Error ? err.message : "unknown");
  }
}
