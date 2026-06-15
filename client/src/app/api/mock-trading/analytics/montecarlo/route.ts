/**
 * FEAT 4 — Monte Carlo Simulation for closed mock trades.
 * POST { iterations: number, accountKey?: string }
 */
import { NextResponse } from "next/server";
import { isMongoConfigured, getDb } from "@/lib/broker/mongoTradesClient";
import { MOCK_TRADES_COLLECTION } from "@/lib/trading/mockTradingMongo";
import { OWNER_ACCOUNT_KEY } from "@/lib/broker/ownerAuth";

export const dynamic = "force-dynamic";

interface MonteCarloBody {
  iterations?: number;
  accountKey?: string;
}

function computeDrawdown(equity: number[]): number {
  let peak = equity[0] ?? 0;
  let maxDD = 0;
  for (const e of equity) {
    if (e > peak) peak = e;
    const dd = peak > 0 ? (peak - e) / peak : 0;
    if (dd > maxDD) maxDD = dd;
  }
  return maxDD;
}

function percentile(sorted: number[], pct: number): number {
  const idx = Math.floor((pct / 100) * sorted.length);
  return sorted[Math.min(idx, sorted.length - 1)] ?? 0;
}

export async function POST(req: Request) {
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }

  let body: MonteCarloBody = {};
  try { body = (await req.json()) as MonteCarloBody; } catch { /* use defaults */ }

  const iterations = Math.min(Math.max(body.iterations ?? 500, 10), 5000);
  const accountKey = body.accountKey ?? OWNER_ACCOUNT_KEY;

  try {
    const db = await getDb();
    const col = db.collection(MOCK_TRADES_COLLECTION);
    const trades = await col
      .find({ account_key: accountKey, status: "CLOSED" })
      .sort({ closed_at: 1 })
      .limit(10_000)
      .toArray();

    if (trades.length < 5) {
      return NextResponse.json({ ok: false, error: "Not enough closed trades for Monte Carlo (need 5+)" }, { status: 400 });
    }

    const startingEquity = 1_000_000;
    // Extract PnL returns as fractions of notional.
    const returns: number[] = trades.map((t) => {
      const notional = (t.notional as number) || 1;
      const pnl = (t.realized_pnl as number) || 0;
      return pnl / notional;
    });

    const finalEquities: number[] = [];
    const maxDrawdowns: number[] = [];
    let ruinCount = 0;

    for (let i = 0; i < iterations; i++) {
      // Shuffle returns.
      const shuffled = [...returns].sort(() => Math.random() - 0.5);
      let equity = startingEquity;
      const path: number[] = [equity];
      for (const r of shuffled) {
        equity = equity * (1 + r);
        if (equity < 0) equity = 0;
        path.push(equity);
      }
      const finalEq = equity;
      finalEquities.push(finalEq);
      maxDrawdowns.push(computeDrawdown(path));
      if (finalEq < startingEquity * 0.5) ruinCount++;
    }

    finalEquities.sort((a, b) => a - b);
    maxDrawdowns.sort((a, b) => a - b);

    return NextResponse.json({
      ok: true,
      iterations,
      trade_count: trades.length,
      starting_equity: startingEquity,
      final_equity: {
        p5: percentile(finalEquities, 5),
        p50: percentile(finalEquities, 50),
        p95: percentile(finalEquities, 95),
      },
      max_drawdown: {
        p5: percentile(maxDrawdowns, 5),
        p50: percentile(maxDrawdowns, 50),
        p95: percentile(maxDrawdowns, 95),
      },
      ruin_probability: ruinCount / iterations,
    });
  } catch (err) {
    return NextResponse.json(
      { ok: false, error: err instanceof Error ? err.message : "unknown" },
      { status: 500 },
    );
  }
}
