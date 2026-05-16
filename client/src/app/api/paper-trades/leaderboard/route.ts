import { NextResponse } from "next/server";
import {
  aggregateStrategyLeaderboard,
  leaderboardRowsFromDb,
  splitLeaderboardTopBottom,
} from "@/lib/paperTradesAnalytics";
import { assertCloudAccountMatchesSession } from "@/lib/paperTradesAuth";
import { getAuthenticatedPaperApiUser } from "@/lib/paperTradesApiAuth";
import { paperTradeLeaderboardQuerySchema } from "@/lib/paperTradesTypes";
import { createServiceSupabase } from "@/lib/supabase/server";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const auth = await getAuthenticatedPaperApiUser();
  if (!auth.ok) return auth.response;

  const url = new URL(req.url);
  const parsed = paperTradeLeaderboardQuerySchema.safeParse({
    account_key: url.searchParams.get("account_key") ?? undefined,
    window_days: url.searchParams.get("window_days") ?? undefined,
    limit: url.searchParams.get("limit") ?? undefined,
  });

  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, error: "Invalid query", details: parsed.error.flatten() },
      { status: 400 },
    );
  }

  const match = assertCloudAccountMatchesSession(auth.ctx.userId, parsed.data.account_key);
  if (!match.ok) {
    return NextResponse.json({ ok: false, error: match.error }, { status: match.status });
  }

  const supabase = createServiceSupabase();
  if (!supabase) {
    return NextResponse.json({ ok: false, error: "Supabase client unavailable" }, { status: 503 });
  }

  const { window_days, limit } = parsed.data;
  const account_key = match.userId;
  const cutoffMs = Date.now() - window_days * 24 * 60 * 60 * 1000;
  const cutoff = new Date(cutoffMs).toISOString();

  const { data, error } = await supabase
    .from("paper_trades")
    .select("strategy_id, strategy_name, net_pnl, closed_at")
    .eq("account_key", account_key)
    .gte("closed_at", cutoff);

  if (error) {
    console.error("[paper-trades/leaderboard]", error);
    return NextResponse.json({ ok: false, error: error.message }, { status: 500 });
  }

  const rows = leaderboardRowsFromDb(
    (data ?? []) as {
      strategy_id: number;
      strategy_name: string;
      net_pnl: number;
      closed_at: string;
    }[],
  );
  const aggregated = aggregateStrategyLeaderboard(rows);
  const { top, bottom } = splitLeaderboardTopBottom(aggregated, limit);

  return NextResponse.json({
    ok: true,
    accountKey: account_key,
    windowDays: window_days,
    top,
    bottom,
  });
}
