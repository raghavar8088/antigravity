import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";
import { isMongoConfigured, getTradesCollection } from "@/lib/mongoTradesClient";
import { aggregateByRegime, type TradeRow } from "@/lib/paperTradeAnalyticsApi";
import { mongoUnconfigured, mongoUnavailable } from "@/lib/paperDeskErrors";

export const dynamic = "force-dynamic";

export async function GET() {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;
  if (!isMongoConfigured()) return mongoUnconfigured();

  try {
    const col = await getTradesCollection();
    const docs = await col
      .find({ account_key: auth.ctx.userId })
      .project({ strategy_id: 1, net_pnl: 1, closed_at: 1, regime_at_entry: 1, payload: 1, _id: 0 })
      .limit(5000)
      .toArray();

    const trades: TradeRow[] = docs.map((d) => {
      const payload = d.payload as Record<string, unknown> | undefined;
      return {
        strategy_id: String(d.strategy_id ?? ""),
        net_pnl: Number(d.net_pnl ?? 0),
        closed_at: String(d.closed_at ?? ""),
        regime_at_entry: String(
          d.regime_at_entry ?? payload?.regime_at_entry ?? payload?.regime ?? "UNKNOWN",
        ),
      };
    });

    return NextResponse.json({ ok: true, regimes: aggregateByRegime(trades) });
  } catch (err) {
    return mongoUnavailable(err instanceof Error ? err.message : "unknown");
  }
}
