/**
 * Backtest Run
 *
 * POST /api/backtest/run
 * Body: {
 *   strategyId: string;
 *   from: string;         // ISO date
 *   to: string;           // ISO date
 *   capitalUsdt?: number;
 *   capitalInr?: number;
 *   slippageModel: "none" | "fixed_bps" | "realistic";
 *   slippageBps?: number;
 *   commission: "none" | "fixed" | "percentage";
 *   commissionValue?: number;
 *   exchange: string;
 * }
 *
 * Proxies to Go engine: POST /api/backtest/run
 * Returns: { jobId: string; status: "queued" }
 *
 * Expected engine response shape:
 * {
 *   jobId: string;
 *   status: "queued" | "running" | "complete" | "failed";
 *   queuedAt: string;
 * }
 */

import { type NextRequest, NextResponse } from "next/server";
import { cookies } from "next/headers";

const ENGINE_BASE = (process.env.INTERNAL_API_URL ?? process.env.ENGINE_URL ?? "http://127.0.0.1:8080").replace(/\/+$/, "");

export async function POST(req: NextRequest) {
  const cookieStore = await cookies();
  if (!cookieStore.get("raig_session")?.value) {
    return NextResponse.json({ ok: false, error: "Unauthorized" }, { status: 401 });
  }
  try {
    const body = await req.json();
    const res = await fetch(`${ENGINE_BASE}/api/backtest/run`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Service-Name": "vercel-backtest" },
      body: JSON.stringify(body),
      signal: AbortSignal.timeout(30_000),
    });
    return NextResponse.json(await res.json(), { status: res.status });
  } catch (e) {
    return NextResponse.json({ ok: false, error: e instanceof Error ? e.message : "unknown" }, { status: 502 });
  }
}
