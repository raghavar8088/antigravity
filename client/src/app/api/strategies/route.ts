/**
 * Strategy Registry
 *
 * GET /api/strategies?status=ACTIVE|PAUSED|DISQUALIFIED&exchange=BINANCE|DELTA|COINBASE&tier=A|B|C
 *
 * Proxies to Go engine: GET /api/strategies/registry
 *
 * Expected engine response shape:
 * {
 *   strategies: StrategyRow[];   // see client/src/types/strategy.ts
 *   total: number;
 *   tierSummary: AlphaTierSummary[];
 * }
 */

import { type NextRequest, NextResponse } from "next/server";

const ENGINE_BASE = (process.env.INTERNAL_API_URL ?? process.env.ENGINE_URL ?? "http://127.0.0.1:8080").replace(/\/+$/, "");

export async function GET(req: NextRequest) {
  const qs = req.nextUrl.searchParams.toString();
  try {
    const res = await fetch(`${ENGINE_BASE}/api/strategies/registry${qs ? "?" + qs : ""}`, {
      cache: "no-store",
      signal: AbortSignal.timeout(10_000),
      headers: { "X-Service-Name": "vercel-strategies" },
    });
    if (!res.ok) return NextResponse.json({ ok: false, error: `Engine ${res.status}` }, { status: res.status });
    return NextResponse.json(await res.json());
  } catch {
    return NextResponse.json({ ok: false, error: "engine_offline", strategies: [], total: 0, tierSummary: [] }, { status: 200 });
  }
}
