/**
 * GET /api/cron/rank-strategies
 *
 * Nightly cron (P1.4.5): reads last 7d closed trades from MongoDB, computes
 * per-strategy expectancy + win-rate, and writes an updated rankings JSON file.
 *
 * Promote criteria: expectancy ≥ $0.05 AND trades ≥ 12  → added to winners list
 * Demote  criteria: expectancy < −$0.02 AND trades ≥ 8   → removed from winners list
 *
 * Vercel cron schedule: 0 0 * * * (midnight UTC, defined in vercel.json).
 * Protected by CRON_SECRET env var — must match Authorization: Bearer <secret>.
 *
 * Output: overwrites fixtures/replay/btc_ft_strategy_rankings.json with fresh
 * MongoDB-derived rankings so the hook's ranked-winners gate uses live data.
 */

import { NextResponse } from "next/server";
import fs from "fs";
import path from "path";
import { MongoClient } from "mongodb";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { evaluateAndUpdateGrades } from "@/lib/strategyAuthority/strategyAuthorityMongo";

const MONGODB_URI = process.env.MONGODB_URI;
const MONGODB_DB = process.env.MONGODB_DB || "loop_trades";
const WINDOW_DAYS = 7;
const PROMOTE_EXPECTANCY = 0.05;
const PROMOTE_MIN_TRADES = 12;
const DEMOTE_EXPECTANCY = -0.02;
const DEMOTE_MIN_TRADES = 8;

export async function GET(req: Request): Promise<NextResponse> {
  // CRON_SECRET is mandatory — fail closed to prevent unauthenticated runs.
  // Vercel injects Authorization: Bearer <CRON_SECRET> automatically when the env var is set.
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

  if (!MONGODB_URI) {
    return NextResponse.json({ ok: false, error: "MONGODB_URI not set" }, { status: 500 });
  }

  const client = new MongoClient(MONGODB_URI, { serverSelectionTimeoutMS: 10_000 });
  try {
    await client.connect();
    const db = client.db(MONGODB_DB);
    const cutoff = new Date(Date.now() - WINDOW_DAYS * 24 * 60 * 60 * 1000).toISOString();
    const docs = await db
      .collection("paper_trades")
      .find({ closed_at: { $gte: cutoff } })
      .project({ strategy_id: 1, net_pnl: 1, _id: 0 })
      .toArray();

    // Group by strategy_id
    const byId = new Map<number, number[]>();
    for (const doc of docs) {
      const id = Number(doc.strategy_id);
      const pnl = Number(doc.net_pnl);
      if (!Number.isFinite(id) || id <= 0 || !Number.isFinite(pnl)) continue;
      if (!byId.has(id)) byId.set(id, []);
      byId.get(id)!.push(pnl);
    }

    type RankingRow = {
      id: number;
      trades: number;
      expectancy: number;
      winRate: number;
      sumNet: number;
      promoted: boolean;
      demoted: boolean;
    };

    const rankings: RankingRow[] = [];
    for (const [id, pnls] of byId.entries()) {
      const trades = pnls.length;
      const sumNet = pnls.reduce((s, p) => s + p, 0);
      const expectancy = sumNet / trades;
      const wins = pnls.filter((p) => p > 0).length;
      const winRate = wins / trades;
      const promoted = expectancy >= PROMOTE_EXPECTANCY && trades >= PROMOTE_MIN_TRADES;
      const demoted = expectancy < DEMOTE_EXPECTANCY && trades >= DEMOTE_MIN_TRADES;
      rankings.push({ id, trades, expectancy, winRate, sumNet, promoted, demoted });
    }
    rankings.sort((a, b) => b.expectancy - a.expectancy);

    // Write updated rankings file (same format consumed by /api/strategy-rankings)
    const outDir = path.resolve(process.cwd(), "fixtures/replay");
    fs.mkdirSync(outDir, { recursive: true });
    const outPath = path.join(outDir, "btc_ft_strategy_rankings.json");
    fs.writeFileSync(
      outPath,
      JSON.stringify(
        {
          generatedAt: new Date().toISOString(),
          source: "mongodb-live",
          windowDays: WINDOW_DAYS,
          rankings: rankings.map(({ id, trades, expectancy, winRate }) => ({
            id,
            trades,
            expectancy: Math.round(expectancy * 10000) / 10000,
            winRate: Math.round(winRate * 1000) / 1000,
          })),
        },
        null,
        2,
      ),
    );

    // Write policy snapshot to MongoDB for audit trail
    await db.collection("policy_snapshots").insertOne({
      type: "strategy_rank",
      created_at: new Date().toISOString(),
      window_days: WINDOW_DAYS,
      total_strategies: rankings.length,
      promoted: rankings.filter((r) => r.promoted).map((r) => r.id),
      demoted: rankings.filter((r) => r.demoted).map((r) => r.id),
      rankings: rankings.map(({ id, trades, expectancy, winRate }) => ({
        id,
        trades,
        expectancy,
        winRate,
      })),
    });

    // Piggyback ISPAP grade evaluation — no extra cron slot needed (already at 2/2)
    let isapResult: { promoted: number; demoted: number; retired: number; evaluated: number } | null = null;
    if (isMongoConfigured()) {
      try {
        isapResult = await evaluateAndUpdateGrades();
      } catch {
        // Non-fatal — ranking output still valid if ISPAP evaluation fails
      }
    }

    return NextResponse.json({
      ok: true,
      rankedCount: rankings.length,
      promoted: rankings.filter((r) => r.promoted).map((r) => r.id),
      demoted: rankings.filter((r) => r.demoted).map((r) => r.id),
      windowDays: WINDOW_DAYS,
      ispap: isapResult ?? { skipped: "MONGO_NOT_CONFIGURED" },
    });
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    return NextResponse.json({ ok: false, error: msg }, { status: 500 });
  } finally {
    await client.close();
  }
}
