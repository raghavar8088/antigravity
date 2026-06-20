/**
 * GET /api/cron/policy-snapshot
 *
 * Daily policy snapshot (P2.4.4): captures the current desk state — enabled strats,
 * thresholds, caps, equity, recent regime distribution — and writes one row to the
 * `policy_snapshots` MongoDB collection. Provides an audit trail to answer "why did
 * this trade happen on day X?" after the fact.
 *
 * Vercel cron schedule: 0 0 * * * (midnight UTC, defined in vercel.json).
 * Protected by CRON_SECRET env var — must match Authorization: Bearer <secret>.
 *
 * Output: inserts one document into `policy_snapshots` with the day's state.
 */

import { NextResponse } from "next/server";
import { getDb, isMongoConfigured } from "@/lib/broker/mongoTradesClient";
import { CORE_BTC_FT_STRATEGY_IDS } from "@/lib/trading/btcFtRoster";
import {
  btcFtSignalThresholdFromEnv,
  deskVolSizedNotionalEnabledFromEnv,
  deskRiskPctOfEquityFromEnv,
  DESK_MAX_OPEN_PER_CLUSTER,
} from "@/lib/trading/futuresDeskPolicy";

export async function GET(req: Request): Promise<NextResponse> {
  // CRON_SECRET is mandatory — fail closed to prevent unauthenticated runs.
  const secret = process.env.CRON_SECRET?.trim();
  if (!secret) {
    return NextResponse.json(
      { ok: false, error: "CRON_SECRET env var is not configured — cron endpoint is locked" },
      { status: 503 },
    );
  }
  const auth = req.headers.get("authorization") ?? "";
  if (auth !== `Bearer ${secret}`) {
    return NextResponse.json({ ok: false, error: "Unauthorized" }, { status: 401 });
  }

  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MONGODB_URI not set" }, { status: 500 });
  }

  try {
    // Reuse the shared, pool-capped global client (mongoTradesClient.ts) instead
    // of opening a fresh ad-hoc MongoClient here. An ad-hoc client with no
    // maxPoolSize override defaults to the driver's 100-per-node pool — on a
    // 3-node Atlas M0 cluster that's up to 300 sockets from a single cron
    // invocation, a major contributor to the repeated 500/500 connection-cap
    // outages even though this client was being closed in `finally`.
    const db = await getDb();

    // Pull last 24h trade summary from MongoDB
    const cutoff = new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString();
    const recentTrades = await db
      .collection("paper_trades")
      .find({ closed_at: { $gte: cutoff } })
      .project({ strategy_id: 1, net_pnl: 1, exit_reason: 1, _id: 0 })
      .toArray();

    const tradeCount = recentTrades.length;
    const sumNet24h = recentTrades.reduce((s, t) => s + Number(t.net_pnl ?? 0), 0);
    const exitMix: Record<string, number> = {};
    for (const t of recentTrades) {
      const r = String(t.exit_reason ?? "UNKNOWN");
      exitMix[r] = (exitMix[r] ?? 0) + 1;
    }

    const snapshot = {
      type: "daily_policy",
      created_at: new Date().toISOString(),
      enabled_strategies: [...CORE_BTC_FT_STRATEGY_IDS],
      enabled_count: CORE_BTC_FT_STRATEGY_IDS.length,
      signal_threshold: btcFtSignalThresholdFromEnv(28),
      vol_sized_notional: deskVolSizedNotionalEnabledFromEnv(),
      risk_pct_of_equity: deskRiskPctOfEquityFromEnv(),
      max_open_per_cluster: DESK_MAX_OPEN_PER_CLUSTER,
      last_24h: {
        trade_count: tradeCount,
        sum_net_pnl: Math.round(sumNet24h * 100) / 100,
        expectancy: tradeCount > 0 ? Math.round((sumNet24h / tradeCount) * 10000) / 10000 : 0,
        exit_mix: exitMix,
      },
    };

    await db.collection("policy_snapshots").insertOne(snapshot);

    return NextResponse.json({ ok: true, snapshot });
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    return NextResponse.json({ ok: false, error: msg }, { status: 500 });
  }
}
