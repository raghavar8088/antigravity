import { NextResponse } from "next/server";
import {
  aggregateStrategyLeaderboard,
  leaderboardRowsFromDb,
  splitLeaderboardTopBottom,
} from "@/lib/paperTradesAnalytics";
import { paperTradeLeaderboardQuerySchema } from "@/lib/paperTradesTypes";
import { isMongoConfigured, getTradesCollection } from "@/lib/mongoTradesClient";

export const dynamic = "force-dynamic";

export async function GET(req: Request) {
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

  const account_key = parsed.data.account_key as string;
  if (!account_key?.trim()) {
    return NextResponse.json({ ok: false, error: "account_key is required" }, { status: 400 });
  }

  const { window_days, limit } = parsed.data;
  const cutoffMs = Date.now() - window_days * 24 * 60 * 60 * 1000;
  const cutoff = new Date(cutoffMs).toISOString();

  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "No MongoDB configured" }, { status: 503 });
  }

  try {
    const col = await getTradesCollection();
    const docs = await col
      .find({ account_key, closed_at: { $gte: cutoff } })
      .project({ strategy_id: 1, strategy_name: 1, net_pnl: 1, closed_at: 1, _id: 0 })
      .toArray();

    const rows = leaderboardRowsFromDb(
      docs as {
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
  } catch (err) {
    console.error("[paper-trades/leaderboard]", err);
    return NextResponse.json({ ok: false, error: err instanceof Error ? err.message : "Mongo read failed" }, { status: 500 });
  }
}
