import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";
import { isMongoConfigured, getDb } from "@/lib/mongoTradesClient";
import { getPaperState } from "@/lib/paperDeskClient";
import { mongoUnconfigured, mongoUnavailable } from "@/lib/paperDeskErrors";

export const dynamic = "force-dynamic";

async function computeStatsFromTrades(accountKey: string) {
  try {
    const db = await getDb();
    const trades = await db
      .collection("paper_trades")
      .find({ account_key: accountKey })
      .project({ net_pnl: 1, exit_at: 1, fees: 1 })
      .toArray();

    const total = trades.length;
    const wins = trades.filter((t) => (t.net_pnl ?? 0) > 0).length;
    const losses = trades.filter((t) => (t.net_pnl ?? 0) <= 0).length;
    const realized_pnl = trades.reduce((s, t) => s + (t.net_pnl ?? 0), 0);
    const total_fees = trades.reduce((s, t) => s + (t.fees ?? 0), 0);
    const win_rate = total > 0 ? wins / total : 0;

    return { total_trades: total, winning_trades: wins, losing_trades: losses, realized_pnl, total_fees, win_rate };
  } catch {
    return null;
  }
}

export async function GET() {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;
  const accountKey = auth.ctx.userId;

  if (!isMongoConfigured()) return mongoUnconfigured();

  let state;
  try {
    state = await getPaperState(accountKey);
  } catch (err) {
    return mongoUnavailable(err instanceof Error ? err.message : "unknown");
  }

  // If the Go engine hasn't written a proper paper_state yet (missing trade stats),
  // compute them from paper_trades so the dashboard doesn't show "undefined".
  if (state && (state.total_trades == null || state.total_trades === undefined)) {
    const computed = await computeStatsFromTrades(accountKey);
    if (computed) {
      state = {
        ...state,
        equity: state.equity ?? state.balance ?? 0,
        ...computed,
      };
    }
  }

  // Also handle case where state is null but trades exist (cron worker schema in paper_state)
  if (!state) {
    const computed = await computeStatsFromTrades(accountKey);
    if (computed && computed.total_trades > 0) {
      const INITIAL = 1_000_000;
      state = {
        account_key: accountKey,
        balance: INITIAL,
        equity: INITIAL + computed.realized_pnl,
        unrealized_pnl: 0,
        realized_pnl: computed.realized_pnl,
        peak_equity: INITIAL,
        current_drawdown: 0,
        max_drawdown: 0,
        open_position_count: 0,
        total_exposure_btc: 0,
        long_exposure_btc: 0,
        short_exposure_btc: 0,
        total_fees: computed.total_fees,
        session_start: new Date().toISOString(),
        snapped_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
        ...computed,
      };
    }
  }

  return NextResponse.json({
    ok: true,
    account_key: accountKey,
    state,
    engine_writing: state !== null,
  });
}
