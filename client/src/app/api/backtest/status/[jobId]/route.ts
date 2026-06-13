/**
 * Backtest Job Status
 *
 * GET /api/backtest/status/:jobId
 *
 * Proxies to Go engine: GET /api/backtest/status/:jobId
 *
 * Expected engine response shape:
 * {
 *   jobId: string;
 *   status: "queued" | "running" | "complete" | "failed";
 *   progressPct: number;         // 0-100
 *   barsProcessed: number;
 *   totalBars: number;
 *   startedAt: string | null;
 *   completedAt: string | null;
 *   error: string | null;
 *   result?: BacktestResult;     // present when status === "complete"
 * }
 *
 * BacktestResult shape:
 * {
 *   totalReturnPct: number;
 *   annualizedReturnPct: number;
 *   sharpeRatio: number;
 *   sortinoRatio: number;
 *   maxDrawdownPct: number;
 *   maxDrawdownDurationDays: number;
 *   winRatePct: number;
 *   profitFactor: number;
 *   avgTradeDurationMs: number;
 *   totalTrades: number;
 *   equityCurve: Array<{ date: string; value: number }>;
 *   monthlyReturns: Array<{ year: number; month: number; returnPct: number }>;
 *   trades: BacktestTrade[];
 *   benchmarkEquityCurve?: Array<{ date: string; value: number }>;
 * }
 */

import { type NextRequest, NextResponse } from "next/server";

const ENGINE_BASE = (process.env.INTERNAL_API_URL ?? process.env.ENGINE_URL ?? "http://127.0.0.1:8080").replace(/\/+$/, "");

type RouteCtx = { params: Promise<{ jobId: string }> };

export async function GET(_req: NextRequest, ctx: RouteCtx) {
  const { jobId } = await ctx.params;
  try {
    const res = await fetch(`${ENGINE_BASE}/api/backtest/status/${jobId}`, {
      cache: "no-store",
      signal: AbortSignal.timeout(10_000),
      headers: { "X-Service-Name": "vercel-backtest-status" },
    });
    if (!res.ok) return NextResponse.json({ ok: false, error: `Engine ${res.status}` }, { status: res.status });
    return NextResponse.json(await res.json());
  } catch {
    return NextResponse.json({ ok: false, error: "engine_offline" }, { status: 503 });
  }
}
