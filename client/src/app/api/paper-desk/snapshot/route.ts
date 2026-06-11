import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import {
  getPaperState,
  listOpenPositions,
  listPaperTrades,
  getStrategyHealthSummary,
  getClosedTradeStats,
  mergeClosedTradeStatsIntoState,
} from "@/lib/paperDeskClient";
import { buildPortfolioAccountingSnapshot, getPortfolioExtendedMetrics } from "@/lib/portfolioAccountingService";
import { mongoUnconfigured, mongoUnavailable } from "@/lib/paperDeskErrors";

export const dynamic = "force-dynamic";

/**
 * Aggregated polling snapshot. The /paper-desk page polls THIS single endpoint
 * every 5 seconds instead of fanning out to 6 routes, so each device makes one
 * request per tick. Detail views (full trade pagination, OMS orders, equity
 * curve, full strategy table) use the dedicated routes on demand.
 */
export async function GET() {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;
  const accountKey = auth.ctx.userId;

  if (!isMongoConfigured()) return mongoUnconfigured();

  let state, openPositions, recentTrades, healthSummary, closedStats;
  try {
    [state, openPositions, recentTrades, healthSummary, closedStats] = await Promise.all([
      getPaperState(accountKey),
      listOpenPositions(accountKey),
      listPaperTrades({ accountKey, limit: 20 }),
      getStrategyHealthSummary(accountKey),
      getClosedTradeStats(accountKey),
    ]);
  } catch (err) {
    return mongoUnavailable(err instanceof Error ? err.message : "unknown");
  }

  const accounting = buildPortfolioAccountingSnapshot({
    accountKey,
    state,
    closedStats,
    openPositions: openPositions ?? [],
  });
  const extended = await getPortfolioExtendedMetrics(accountKey);
  accounting.profit_factor = extended.profit_factor;
  accounting.sharpe = extended.sharpe;
  state = mergeClosedTradeStatsIntoState(state, closedStats, {
    unrealized_pnl: accounting.unrealized_pnl,
    exposure: accounting.exposure,
    drawdown: accounting.drawdown,
  });

  return NextResponse.json({
    ok: true,
    server_time: new Date().toISOString(),
    account_key: accountKey,
    state,
    open_positions: openPositions ?? [],
    recent_trades: recentTrades ?? [],
    health_summary: healthSummary,
    portfolio: accounting,
  });
}
