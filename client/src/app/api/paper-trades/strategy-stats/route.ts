import { NextResponse } from "next/server";
import { aggregateStrategyStats, strategyStatsRowsFromDb } from "@/lib/paperTradesAnalytics";
import { assertCloudAccountMatchesSession } from "@/lib/paperTradesAuth";
import { getAuthenticatedPaperApiUser } from "@/lib/paperTradesApiAuth";
import { paperTradeStrategyStatsQuerySchema } from "@/lib/paperTradesTypes";
import { createServiceSupabase } from "@/lib/supabase/server";
import { isMongoConfigured, getTradesCollection } from "@/lib/mongoTradesClient";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const auth = await getAuthenticatedPaperApiUser();
  if (!auth.ok) return auth.response;

  const url = new URL(req.url);
  const parsed = paperTradeStrategyStatsQuerySchema.safeParse({
    account_key: url.searchParams.get("account_key") ?? undefined,
    window_days: url.searchParams.get("window_days") ?? undefined,
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

  const { window_days } = parsed.data;
  const account_key = match.userId;
  const cutoff = new Date(Date.now() - window_days * 24 * 60 * 60 * 1000).toISOString();

  // PRIMARY: MongoDB
  if (isMongoConfigured()) {
    try {
      const col = await getTradesCollection();
      const docs = await col
        .find({ account_key, closed_at: { $gte: cutoff } })
        .project({ strategy_id: 1, net_pnl: 1, closed_at: 1, _id: 0 })
        .toArray();
      const rows = strategyStatsRowsFromDb(
        docs as { strategy_id: number; net_pnl: number; closed_at: string }[],
      );
      return NextResponse.json({
        ok: true,
        accountKey: account_key,
        windowDays: window_days,
        stats: aggregateStrategyStats(rows),
        source: "mongo",
      });
    } catch (err) {
      console.warn("[paper-trades/strategy-stats] mongo failed, falling back to supabase", err);
    }
  }

  // FALLBACK: Supabase
  const supabase = createServiceSupabase();
  if (!supabase) {
    return NextResponse.json({ ok: false, error: "No trade store configured" }, { status: 503 });
  }

  const { data, error } = await supabase
    .from("paper_trades")
    .select("strategy_id, net_pnl, closed_at")
    .eq("account_key", account_key)
    .gte("closed_at", cutoff);

  if (error) {
    console.error("[paper-trades/strategy-stats]", error);
    return NextResponse.json({ ok: false, error: error.message }, { status: 500 });
  }

  const rows = strategyStatsRowsFromDb(
    (data ?? []) as { strategy_id: number; net_pnl: number; closed_at: string }[],
  );
  return NextResponse.json({
    ok: true,
    accountKey: account_key,
    windowDays: window_days,
    stats: aggregateStrategyStats(rows),
    source: "supabase",
  });
}
