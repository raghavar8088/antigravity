import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import {
  getPaperState,
  getClosedTradeStats,
  mergeClosedTradeStatsIntoState,
} from "@/lib/paperDeskClient";
import { mongoUnconfigured, mongoUnavailable } from "@/lib/paperDeskErrors";

export const dynamic = "force-dynamic";

export async function GET() {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;
  const accountKey = auth.ctx.userId;

  if (!isMongoConfigured()) return mongoUnconfigured();

  let state;
  try {
    const [paperState, closedStats] = await Promise.all([
      getPaperState(accountKey),
      getClosedTradeStats(accountKey),
    ]);
    state = mergeClosedTradeStatsIntoState(paperState, closedStats);

    if (!state && closedStats.total_trades > 0) {
      const INITIAL = 1_000_000;
      state = {
        account_key: accountKey,
        balance: INITIAL,
        equity: INITIAL + closedStats.realized_pnl,
        unrealized_pnl: 0,
        peak_equity: INITIAL,
        current_drawdown: 0,
        max_drawdown: 0,
        open_position_count: 0,
        total_exposure_btc: 0,
        long_exposure_btc: 0,
        short_exposure_btc: 0,
        session_start: new Date().toISOString(),
        snapped_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
        ...closedStats,
      };
    }
  } catch (err) {
    return mongoUnavailable(err instanceof Error ? err.message : "unknown");
  }

  return NextResponse.json({
    ok: true,
    account_key: accountKey,
    state,
    engine_writing: state !== null,
  });
}
