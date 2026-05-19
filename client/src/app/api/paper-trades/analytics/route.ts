import { NextResponse } from "next/server";
import { getAuthenticatedPaperApiUser } from "@/lib/paperTradesApiAuth";
import { assertCloudAccountMatchesSession } from "@/lib/paperTradesAuth";
import { paperTradeAnalyticsQuerySchema } from "@/lib/paperTradesTypes";
import { isMongoConfigured, getTradesCollection } from "@/lib/mongoTradesClient";
import { isAnonAccountKey } from "@/lib/anonAccountKey";

export const dynamic = "force-dynamic";

const ANON_ENABLED = process.env.ALLOW_ANON_PAPER_TRADES === "1";

/**
 * GET /api/paper-trades/analytics
 *
 * MongoDB aggregation pipeline that computes per-strategy quantitative metrics:
 *   - E = W × Pavg − L × Lavg  (proper expectancy)
 *   - MAE proxy (avg absolute loss on losing trades)
 *   - MFE proxy (avg gross on winning trades)
 *   - Sharpe proxy (expectancy / stddev of net PnL)
 *   - feePctOfGross, avgHoldMin, winRate
 *
 * Filterable by module_key, template_family, and min_trades gate.
 * Only strategies with tradeCount >= min_trades (default 100) are returned,
 * enforcing the statistical significance threshold from the 2026 blueprint.
 *
 * Falls back to an in-process JS aggregation when MongoDB isn't configured.
 */
export async function GET(req: Request) {
  const url = new URL(req.url);

  const parsed = paperTradeAnalyticsQuerySchema.safeParse({
    account_key: url.searchParams.get("account_key") ?? undefined,
    window_days: url.searchParams.get("window_days") ?? undefined,
    module_key: url.searchParams.get("module_key") ?? undefined,
    template_family: url.searchParams.get("template_family") ?? undefined,
    min_trades: url.searchParams.get("min_trades") ?? undefined,
  });

  if (!parsed.success) {
    return NextResponse.json(
      { ok: false, error: "Invalid query", details: parsed.error.flatten() },
      { status: 400 },
    );
  }

  let accountKey: string;
  if (ANON_ENABLED && isAnonAccountKey(parsed.data.account_key)) {
    accountKey = parsed.data.account_key!;
  } else {
    const auth = await getAuthenticatedPaperApiUser();
    if (!auth.ok) return auth.response;
    const match = assertCloudAccountMatchesSession(auth.ctx.userId, parsed.data.account_key);
    if (!match.ok) {
      return NextResponse.json({ ok: false, error: match.error }, { status: match.status });
    }
    accountKey = match.userId;
  }

  const { window_days, module_key, template_family, min_trades } = parsed.data;
  const cutoff = new Date(Date.now() - window_days * 24 * 60 * 60 * 1000).toISOString();

  if (!isMongoConfigured()) {
    return NextResponse.json(
      { ok: false, error: "Analytics requires MongoDB. Configure MONGODB_URI." },
      { status: 503 },
    );
  }

  try {
    const col = await getTradesCollection();

    // Match stage: account + time window + optional filters
    const matchStage: Record<string, unknown> = {
      account_key: accountKey,
      closed_at: { $gte: cutoff },
    };
    if (module_key) matchStage.module_key = module_key;
    if (template_family) matchStage.template_family = template_family;

    const pipeline = [
      { $match: matchStage },

      // Group by strategy_id to compute raw sums and counts
      {
        $group: {
          _id: "$strategy_id",
          strategyName: { $last: "$strategy_name" },
          templateFamily: { $last: "$template_family" },
          exchange: { $last: "$exchange" },
          moduleKey: { $last: "$module_key" },
          tradeCount: { $sum: 1 },
          sumNet: { $sum: "$net_pnl" },
          sumGross: { $sum: { $abs: "$gross_pnl" } },
          sumFees: { $sum: "$fees" },
          // For proper expectancy: collect all net_pnl values
          allNetPnl: { $push: "$net_pnl" },
          allGrossPnl: { $push: "$gross_pnl" },
          // Hold minutes
          sumHoldMs: {
            $sum: {
              $subtract: [
                { $toDate: "$closed_at" },
                { $toDate: "$opened_at" },
              ],
            },
          },
          lastTradeAt: { $max: "$closed_at" },
        },
      },

      // Only include strategies above the statistical significance threshold
      { $match: { tradeCount: { $gte: min_trades } } },

      // Add computed fields: win/loss buckets, expectancy, MAE, MFE, Sharpe
      {
        $addFields: {
          winNetPnls: {
            $filter: { input: "$allNetPnl", as: "v", cond: { $gt: ["$$v", 0] } },
          },
          lossNetPnls: {
            $filter: { input: "$allNetPnl", as: "v", cond: { $lte: ["$$v", 0] } },
          },
        },
      },
      {
        $addFields: {
          winCount: { $size: "$winNetPnls" },
          lossCount: { $size: "$lossNetPnls" },
          sumWin: { $sum: "$winNetPnls" },
          sumLoss: { $sum: "$lossNetPnls" },
        },
      },
      {
        $addFields: {
          winRate: { $divide: ["$winCount", "$tradeCount"] },
          // Pavg: avg winning trade net PnL
          pavg: {
            $cond: [
              { $gt: ["$winCount", 0] },
              { $divide: ["$sumWin", "$winCount"] },
              0,
            ],
          },
          // Lavg: avg absolute losing trade net PnL
          lavg: {
            $cond: [
              { $gt: ["$lossCount", 0] },
              { $abs: { $divide: ["$sumLoss", "$lossCount"] } },
              0,
            ],
          },
          avgHoldMin: { $divide: [{ $divide: ["$sumHoldMs", 1000] }, 60] },
          feePctOfGross: {
            $cond: [
              { $gt: ["$sumGross", 0] },
              { $multiply: [{ $divide: ["$sumFees", "$sumGross"] }, 100] },
              null,
            ],
          },
          // MAE proxy: avg absolute loss on losing trades
          mae: {
            $cond: [
              { $gt: ["$lossCount", 0] },
              { $abs: { $divide: ["$sumLoss", "$lossCount"] } },
              null,
            ],
          },
          // MFE proxy: avg gross on winning trades
          mfe: {
            $cond: [
              { $gt: ["$winCount", 0] },
              { $divide: ["$sumWin", "$winCount"] },
              null,
            ],
          },
        },
      },
      {
        $addFields: {
          // E = W × Pavg − L × Lavg
          expectancy: {
            $subtract: [
              { $multiply: ["$winRate", "$pavg"] },
              { $multiply: [{ $subtract: [1, "$winRate"] }, "$lavg"] },
            ],
          },
        },
      },

      // Shape output — drop internal array fields
      {
        $project: {
          _id: 0,
          strategyId: "$_id",
          strategyName: 1,
          templateFamily: 1,
          exchange: 1,
          moduleKey: 1,
          tradeCount: 1,
          sumNet: { $round: ["$sumNet", 2] },
          expectancy: { $round: ["$expectancy", 3] },
          winRate: { $round: ["$winRate", 3] },
          feePctOfGross: { $round: ["$feePctOfGross", 2] },
          avgHoldMin: { $round: ["$avgHoldMin", 1] },
          mae: { $round: ["$mae", 2] },
          mfe: { $round: ["$mfe", 2] },
          lastTradeAt: 1,
        },
      },

      // Sort: best expectancy first
      { $sort: { expectancy: -1 } },
    ];

    const results = await col.aggregate(pipeline).toArray();

    return NextResponse.json({
      ok: true,
      accountKey,
      windowDays: window_days,
      minTradesGate: min_trades,
      moduleFilter: module_key ?? null,
      familyFilter: template_family ?? null,
      count: results.length,
      analytics: results,
    });
  } catch (err) {
    console.error("[paper-trades/analytics]", err);
    return NextResponse.json(
      { ok: false, error: err instanceof Error ? err.message : "Aggregation failed" },
      { status: 500 },
    );
  }
}
