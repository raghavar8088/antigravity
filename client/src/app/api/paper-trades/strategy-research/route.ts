import { NextResponse } from "next/server";
import { aggregateResearchStratStats, type ResearchDbRow } from "@/lib/paperTradesAnalytics";
import { computeVerdict } from "@/lib/btcFtResearch";
import { FUTURES_STRAT_DEFS } from "@/lib/futuresStrategies";
import { isMongoConfigured, getTradesCollection } from "@/lib/mongoTradesClient";

export const dynamic = "force-dynamic";

/**
 * GET /api/paper-trades/strategy-research
 *
 * Research tournament stats: per strategy → tradeCount, sumNet, expectancy,
 * winRate, feePctOfGross, avgHoldMin, lastTradeAt, verdict.
 *
 * Query params:
 *   account_key  — Mongo trade account key (user or anonymous account identifier)
 *   window_days  — lookback window (default 60)
 *   pool_ids     — comma-separated strategy IDs to filter (optional)
 */
export async function GET(req: Request) {
  const url = new URL(req.url);
  const account_key = url.searchParams.get("account_key") ?? "";
  const windowDaysRaw = Number(url.searchParams.get("window_days") ?? "60");
  const windowDays = Number.isFinite(windowDaysRaw) && windowDaysRaw > 0 ? Math.min(365, windowDaysRaw) : 60;
  const poolIdsRaw = url.searchParams.get("pool_ids") ?? "";
  const promotionsOnly = url.searchParams.get("promotions") === "1";

  if (!account_key?.trim()) {
    return NextResponse.json({ ok: false, error: "account_key is required" }, { status: 400 });
  }

  if (promotionsOnly) {
    return NextResponse.json({ ok: true, accountKey: account_key, promotions: [] });
  }

  const cutoff = new Date(Date.now() - windowDays * 24 * 60 * 60 * 1000).toISOString();

  const poolIds = poolIdsRaw
    .split(",")
    .map((s) => Number(s.trim()))
    .filter((n) => Number.isFinite(n) && n > 0);

  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "No MongoDB configured" }, { status: 503 });
  }

  try {
    const col = await getTradesCollection();
    const filter: Record<string, unknown> = {
      account_key,
      closed_at: { $gte: cutoff },
    };
    if (poolIds.length > 0) filter.strategy_id = { $in: poolIds };
    const docs = await col
      .find(filter)
      .project({
        strategy_id: 1,
        strategy_name: 1,
        net_pnl: 1,
        gross_pnl: 1,
        fees: 1,
        opened_at: 1,
        closed_at: 1,
        module_key: 1,
        template_family: 1,
        _id: 0,
      })
      .toArray();
    const rows = docs as ResearchDbRow[];

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

    const statsWithVerdict = statsWithPool
      .map((s) => ({
        ...s,
        verdict: computeVerdict(s),
      }))
      .sort((a, b) => b.sumNet - a.sumNet);

    return NextResponse.json({
      ok: true,
      accountKey: account_key,
      windowDays,
      poolFilter: poolIds.length > 0 ? poolIds : null,
      stats: statsWithVerdict,
    });
  } catch (err) {
    console.error("[paper-trades/strategy-research]", err);
    return NextResponse.json({ ok: false, error: err instanceof Error ? err.message : "Mongo read failed" }, { status: 500 });
  }
}
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

export async function POST(_req: Request) {
  return NextResponse.json(
    { ok: false, error: "Strategy promotion writes are not supported in Mongo-only mode" },
    { status: 501 },
  );
}
