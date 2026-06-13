/**
 * Risk Positions
 *
 * GET /api/risk/positions
 *
 * Proxies to Go engine: GET /api/risk/positions
 *
 * Expected engine response shape:
 * {
 *   positions: Array<{
 *     symbol: string;
 *     exchange: "NSE" | "BSE" | "BINANCE" | "DELTA";
 *     direction: "LONG" | "SHORT";
 *     size: number;
 *     entryPrice: number;
 *     currentPrice: number;
 *     unrealizedPnl: number;
 *     portfolioPct: number;       // % of total portfolio this position represents
 *     correlationBucket: string;  // e.g. "BTC_MOMENTUM", "NIFTY_TREND"
 *     strategyId: string;
 *   }>;
 *   concentrationWarnings: Array<{
 *     symbol: string;
 *     portfolioPct: number;
 *     reason: string;
 *   }>;
 *   correlatedGroupsWarning: boolean;  // true if 5+ correlated positions open
 * }
 */

import { NextResponse } from "next/server";

const ENGINE_BASE = (process.env.INTERNAL_API_URL ?? process.env.ENGINE_URL ?? "http://127.0.0.1:8080").replace(/\/+$/, "");

export async function GET() {
  try {
    const res = await fetch(`${ENGINE_BASE}/api/risk/positions`, {
      cache: "no-store",
      signal: AbortSignal.timeout(5_000),
      headers: { "X-Service-Name": "vercel-risk-positions" },
    });
    if (!res.ok) {
      return NextResponse.json({ ok: false, error: `Engine returned ${res.status}` }, { status: res.status });
    }
    return NextResponse.json(await res.json());
  } catch {
    return NextResponse.json({ ok: false, error: "engine_offline" }, { status: 503 });
  }
}
