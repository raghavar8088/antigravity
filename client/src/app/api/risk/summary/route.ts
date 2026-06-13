/**
 * Risk Summary
 *
 * GET /api/risk/summary
 *
 * Proxies to Go engine: GET /api/risk/summary
 *
 * Expected engine response shape:
 * {
 *   dailyPnl: number;              // Current session P&L in base currency
 *   dailyPnlLimit: number;         // Max daily loss allowed (negative number)
 *   dailyPnlUsedPct: number;       // 0-100: how much of the daily limit is consumed
 *   drawdownPct: number;           // Current drawdown % from session high
 *   drawdownLimitPct: number;      // Max drawdown % before auto-halt
 *   drawdownUsedPct: number;       // 0-100: drawdown utilization
 *   exposureUsdt: number;          // Total capital currently deployed (USDT)
 *   totalCapitalUsdt: number;      // Total account capital
 *   exposurePct: number;           // 0-100: % of capital deployed
 *   cashPct: number;               // % held as cash
 *   nseExposurePct: number;        // % in NSE equity
 *   cryptoLongPct: number;         // % in crypto long
 *   cryptoShortPct: number;        // % in crypto short
 *   activeStrategies: number;
 *   openPositions: number;
 *   openOrders: number;
 *   lastUpdated: string;           // ISO timestamp
 * }
 */

import { NextResponse } from "next/server";

const ENGINE_BASE = (process.env.INTERNAL_API_URL ?? process.env.ENGINE_URL ?? "http://127.0.0.1:8080").replace(/\/+$/, "");

export async function GET() {
  try {
    const res = await fetch(`${ENGINE_BASE}/api/risk/summary`, {
      cache: "no-store",
      signal: AbortSignal.timeout(5_000),
      headers: { "X-Service-Name": "vercel-risk-summary" },
    });
    if (!res.ok) {
      return NextResponse.json({ ok: false, error: `Engine returned ${res.status}` }, { status: res.status });
    }
    return NextResponse.json(await res.json());
  } catch {
    return NextResponse.json({ ok: false, error: "engine_offline" }, { status: 503 });
  }
}
