import { NextResponse } from "next/server";
import { aggregateResearchStratStats, type ResearchDbRow } from "@/lib/paperTradesAnalytics";
import { computeVerdict } from "@/lib/btcFtResearch";
import { FUTURES_STRAT_DEFS } from "@/lib/futuresStrategies";
import { assertCloudAccountMatchesSession } from "@/lib/paperTradesAuth";
import { getAuthenticatedPaperApiUser } from "@/lib/paperTradesApiAuth";
import { createServiceSupabase } from "@/lib/supabase/server";

export const dynamic = "force-dynamic";

/**
 * GET /api/paper-trades/strategy-research
 *
 * Research tournament stats: per strategy → tradeCount, sumNet, expectancy,
 * winRate, feePctOfGross, avgHoldMin, lastTradeAt, verdict.
 *
 * Query params:
 *   account_key  — Supabase user id
 *   window_days  — lookback window (default 60)
 *   pool_ids     — comma-separated strategy IDs to filter (optional)
 */
export async function GET(req: Request) {
  const auth = await getAuthenticatedPaperApiUser();
  if (!auth.ok) return auth.response;

  const url = new URL(req.url);
  const accountKeyParam = url.searchParams.get("account_key") ?? undefined;
  const windowDaysRaw = Number(url.searchParams.get("window_days") ?? "60");
  const windowDays = Number.isFinite(windowDaysRaw) && windowDaysRaw > 0 ? Math.min(365, windowDaysRaw) : 60;
  const poolIdsRaw = url.searchParams.get("pool_ids") ?? "";

  const match = assertCloudAccountMatchesSession(auth.ctx.userId, accountKeyParam);
  if (!match.ok) {
    return NextResponse.json({ ok: false, error: match.error }, { status: match.status });
  }

  const supabase = createServiceSupabase();
  if (!supabase) {
    return NextResponse.json({ ok: false, error: "Supabase client unavailable" }, { status: 503 });
  }

  const account_key = match.userId;
  const cutoff = new Date(Date.now() - windowDays * 24 * 60 * 60 * 1000).toISOString();

  let query = supabase
    .from("paper_trades")
    .select("strategy_id, strategy_name, net_pnl, gross_pnl, fees, opened_at, closed_at")
    .eq("account_key", account_key)
    .gte("closed_at", cutoff);

  // Filter by pool IDs if provided
  const poolIds = poolIdsRaw
    .split(",")
    .map((s) => Number(s.trim()))
    .filter((n) => Number.isFinite(n) && n > 0);
  if (poolIds.length > 0) {
    query = query.in("strategy_id", poolIds);
  }

  const { data, error } = await query;

  if (error) {
    console.error("[paper-trades/strategy-research]", error);
    return NextResponse.json({ ok: false, error: error.message }, { status: 500 });
  }

  const rows = (data ?? []) as ResearchDbRow[];
  const stats = aggregateResearchStratStats(rows);
  const byId = new Map(stats.map((s) => [s.strategyId, s]));
  const nameById = new Map(FUTURES_STRAT_DEFS.map((s) => [s.id, s.name]));
  const statsWithPool =
    poolIds.length > 0
      ? poolIds.map((id) => {
          const existing = byId.get(id);
          if (existing) return existing;
          return {
            strategyId: id,
            strategyName: nameById.get(id) ?? `Strat ${id}`,
            tradeCount: 0,
            sumNet: 0,
            expectancy: 0,
            winRate: 0,
            feePctOfGross: null,
            avgHoldMin: null,
            lastTradeAt: null,
          };
        })
      : stats;

  // Attach verdict
  const statsWithVerdict = statsWithPool.map((s) => ({
    ...s,
    verdict: computeVerdict(s),
  })).sort((a, b) => b.sumNet - a.sumNet);

  return NextResponse.json({
    ok: true,
    accountKey: account_key,
    windowDays,
    poolFilter: poolIds.length > 0 ? poolIds : null,
    stats: statsWithVerdict,
  });
}

export async function POST(req: Request) {
  const auth = await getAuthenticatedPaperApiUser();
  if (!auth.ok) return auth.response;

  let body: unknown;
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ ok: false, error: "Invalid JSON" }, { status: 400 });
  }

  const parsed = body as { strategyId?: unknown; status?: unknown; note?: unknown };
  const strategyId = Number(parsed.strategyId);
  const status = parsed.status === "winner" || parsed.status === "retired" ? parsed.status : null;
  if (!Number.isFinite(strategyId) || strategyId <= 0 || !status) {
    return NextResponse.json({ ok: false, error: "Invalid strategyId/status" }, { status: 400 });
  }

  const supabase = createServiceSupabase();
  if (!supabase) {
    return NextResponse.json({ ok: false, error: "Supabase client unavailable" }, { status: 503 });
  }

  const { error } = await supabase.from("strategy_promotions").upsert(
    {
      user_id: auth.ctx.userId,
      strategy_id: Math.floor(strategyId),
      status,
      promoted_at: new Date().toISOString(),
      note: typeof parsed.note === "string" ? parsed.note.slice(0, 500) : null,
    },
    { onConflict: "user_id,strategy_id" },
  );

  if (error) {
    console.error("[paper-trades/strategy-research] promotion", error);
    return NextResponse.json({ ok: false, error: error.message }, { status: 500 });
  }

  return NextResponse.json({ ok: true, strategyId: Math.floor(strategyId), status });
}
