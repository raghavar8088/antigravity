/**
 * GET /api/strategy-attribution/[id]?window=7
 *
 * Per-strategy attribution dashboard (P2.5.3). Returns trade-level metrics for
 * a single strategy ID over a configurable rolling window. Ops uses this to
 * diagnose why a specific strategy is bleeding or thriving.
 *
 * Response shape:
 * {
 *   id: number,
 *   trades: number,
 *   expectancy: number,
 *   winRate: number,
 *   sumNet: number,
 *   feeDrag: number,        // sum(fees) / sum(realized_pnl)  — what % of gross goes to fees
 *   holdTimeP50: number,    // median holdtime in ms
 *   regimeMix: { chop, trendLow, trendHigh },
 *   exitMix: { TP, SL, TIME, TRAIL, MOM_DECAY, ... },
 *   recentTrades: [{ openedAt, closedAt, side, netPnl, exitReason }]
 * }
 */

import { NextResponse } from "next/server";
import { MongoClient } from "mongodb";

const MONGODB_URI = process.env.MONGODB_URI;
const MONGODB_DB = process.env.MONGODB_DB || "loop_trades";

function median(nums: number[]): number {
  if (nums.length === 0) return 0;
  const sorted = [...nums].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  return sorted.length % 2 === 0 ? (sorted[mid - 1] + sorted[mid]) / 2 : sorted[mid];
}

export async function GET(
  req: Request,
  { params }: { params: Promise<{ id: string }> },
): Promise<NextResponse> {
  const { id: rawId } = await params;
  const id = Number(rawId);
  if (!Number.isFinite(id) || id <= 0) {
    return NextResponse.json({ ok: false, error: "invalid id" }, { status: 400 });
  }

  const url = new URL(req.url);
  const windowDays = Math.min(90, Math.max(1, Number(url.searchParams.get("window") ?? "7")));

  if (!MONGODB_URI) {
    return NextResponse.json({ ok: false, error: "MONGODB_URI not set" }, { status: 500 });
  }

  const client = new MongoClient(MONGODB_URI, { serverSelectionTimeoutMS: 10_000 });
  try {
    await client.connect();
    const db = client.db(MONGODB_DB);
    const cutoff = new Date(Date.now() - windowDays * 24 * 60 * 60 * 1000).toISOString();
    const docs = await db
      .collection("paper_trades")
      .find({ strategy_id: id, closed_at: { $gte: cutoff } })
      .project({
        opened_at: 1,
        closed_at: 1,
        side: 1,
        net_pnl: 1,
        realized_pnl: 1,
        fees: 1,
        exit_reason: 1,
        regime: 1,
        _id: 0,
      })
      .toArray();

    const trades = docs.length;
    let sumNet = 0;
    let sumGross = 0;
    let sumFees = 0;
    let wins = 0;
    const holdTimes: number[] = [];
    const regimeMix: Record<string, number> = {};
    const exitMix: Record<string, number> = {};
    for (const t of docs) {
      const net = Number(t.net_pnl ?? 0);
      const gross = Number(t.realized_pnl ?? 0);
      const fees = Number(t.fees ?? 0);
      sumNet += net;
      sumGross += gross;
      sumFees += fees;
      if (net > 0) wins++;
      const openMs = new Date(String(t.opened_at)).getTime();
      const closeMs = new Date(String(t.closed_at)).getTime();
      if (Number.isFinite(openMs) && Number.isFinite(closeMs)) {
        holdTimes.push(closeMs - openMs);
      }
      const regime = String(t.regime ?? "unknown");
      regimeMix[regime] = (regimeMix[regime] ?? 0) + 1;
      const exit = String(t.exit_reason ?? "UNKNOWN");
      exitMix[exit] = (exitMix[exit] ?? 0) + 1;
    }

    return NextResponse.json({
      ok: true,
      id,
      windowDays,
      trades,
      expectancy: trades > 0 ? Math.round((sumNet / trades) * 10000) / 10000 : 0,
      winRate: trades > 0 ? Math.round((wins / trades) * 1000) / 1000 : 0,
      sumNet: Math.round(sumNet * 100) / 100,
      feeDrag: sumGross !== 0 ? Math.round((sumFees / Math.abs(sumGross)) * 1000) / 1000 : 0,
      holdTimeP50Ms: Math.round(median(holdTimes)),
      regimeMix,
      exitMix,
      recentTrades: docs.slice(-20).map((t) => ({
        openedAt: t.opened_at,
        closedAt: t.closed_at,
        side: t.side,
        netPnl: Number(t.net_pnl ?? 0),
        exitReason: t.exit_reason,
      })),
    });
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    return NextResponse.json({ ok: false, error: msg }, { status: 500 });
  } finally {
    await client.close();
  }
}
