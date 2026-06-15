/**
 * FEAT 5 — Strategy Correlation Heatmap
 * GET ?account_key=... — Pearson correlation of daily PnL per strategy.
 */
import { NextResponse } from "next/server";
import { isMongoConfigured, getDb } from "@/lib/broker/mongoTradesClient";
import { MOCK_TRADES_COLLECTION } from "@/lib/trading/mockTradingMongo";
import { OWNER_ACCOUNT_KEY } from "@/lib/broker/ownerAuth";

export const dynamic = "force-dynamic";

function pearson(a: number[], b: number[]): number {
  const n = Math.min(a.length, b.length);
  if (n < 2) return 0;
  let sumA = 0, sumB = 0, sumAB = 0, sumA2 = 0, sumB2 = 0;
  for (let i = 0; i < n; i++) {
    sumA += a[i]; sumB += b[i];
    sumAB += a[i] * b[i];
    sumA2 += a[i] * a[i];
    sumB2 += b[i] * b[i];
  }
  const num = n * sumAB - sumA * sumB;
  const den = Math.sqrt((n * sumA2 - sumA * sumA) * (n * sumB2 - sumB * sumB));
  return den === 0 ? 0 : num / den;
}

export async function GET(req: Request) {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }

  const url = new URL(req.url);
  const accountKey = url.searchParams.get("account_key") ?? OWNER_ACCOUNT_KEY;

  try {
    const db = await getDb();
    const col = db.collection(MOCK_TRADES_COLLECTION);
    const trades = await col
      .find({ account_key: accountKey, status: "CLOSED" })
      .sort({ closed_at: 1 })
      .limit(50_000)
      .toArray();

    if (trades.length < 2) {
      return NextResponse.json({ ok: false, error: "Not enough closed trades" }, { status: 400 });
    }

    // Group by strategy, build daily PnL maps.
    const strategyDays = new Map<string, Map<string, number>>();
    const allDays = new Set<string>();

    for (const t of trades) {
      const strat = (t.strategy_name as string) || "unknown";
      const closedAt = t.closed_at as number;
      if (!closedAt) continue;
      const day = new Date(closedAt).toISOString().slice(0, 10);
      allDays.add(day);
      if (!strategyDays.has(strat)) strategyDays.set(strat, new Map());
      const dayMap = strategyDays.get(strat)!;
      dayMap.set(day, (dayMap.get(day) ?? 0) + ((t.realized_pnl as number) ?? 0));
    }

    const strategies = [...strategyDays.keys()].sort();
    const sortedDays = [...allDays].sort();
    const dayCount = sortedDays.length;

    // Build padded vectors (0 for missing days).
    const vectors: number[][] = strategies.map((s) => {
      const dayMap = strategyDays.get(s)!;
      return sortedDays.map((d) => dayMap.get(d) ?? 0);
    });

    // Build correlation matrix.
    const matrix: number[][] = strategies.map((_, i) =>
      strategies.map((_, j) => i === j ? 1 : pearson(vectors[i], vectors[j])),
    );

    return NextResponse.json({ ok: true, strategies, matrix, day_count: dayCount });
  } catch (err) {
    return NextResponse.json(
      { ok: false, error: err instanceof Error ? err.message : "unknown" },
      { status: 500 },
    );
  }
}
