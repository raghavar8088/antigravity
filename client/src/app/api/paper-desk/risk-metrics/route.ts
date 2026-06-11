import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";
import { isMongoConfigured, getTradesCollection } from "@/lib/mongoTradesClient";
import { computePortfolioRiskMetrics, type TradeRow } from "@/lib/paperTradeAnalyticsApi";
import { PAPER_DESK_STARTING_BALANCE } from "@/lib/portfolioAccountingTypes";
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
      .project({ net_pnl: 1, closed_at: 1, strategy_id: 1, _id: 0 })
      .sort({ closed_at: 1 })
      .limit(5000)
      .toArray();

    const trades: TradeRow[] = docs.map((d) => ({
      strategy_id: String(d.strategy_id ?? ""),
      net_pnl: Number(d.net_pnl ?? 0),
      closed_at: String(d.closed_at ?? ""),
    }));

    const metrics = computePortfolioRiskMetrics(trades, PAPER_DESK_STARTING_BALANCE);
    return NextResponse.json({ ok: true, ...metrics });
  } catch (err) {
    return mongoUnavailable(err instanceof Error ? err.message : "unknown");
  }
}
