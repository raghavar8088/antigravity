/**
 * Trading Orders (OMS v3)
 *
 * GET /api/trading/orders?status=all|open|filled|rejected|cancelled&limit=100
 *
 * Proxies to Go engine: GET /api/paper-desk/oms/orders (or /api/oms/orders for live)
 *
 * Expected engine response shape:
 * {
 *   orders: Order[];   // see client/src/types/trading.ts
 *   total: number;
 * }
 */

import { type NextRequest, NextResponse } from "next/server";

const ENGINE_BASE = (process.env.INTERNAL_API_URL ?? process.env.ENGINE_URL ?? "http://127.0.0.1:8080").replace(/\/+$/, "");

export async function GET(req: NextRequest) {
  const { searchParams } = req.nextUrl;
  const qs = searchParams.toString();
  try {
    const res = await fetch(`${ENGINE_BASE}/api/oms/orders${qs ? "?" + qs : ""}`, {
      cache: "no-store",
      signal: AbortSignal.timeout(5_000),
      headers: { "X-Service-Name": "vercel-trading-orders" },
    });
    if (!res.ok) return NextResponse.json({ ok: false, error: `Engine ${res.status}` }, { status: res.status });
    return NextResponse.json(await res.json());
  } catch {
    return NextResponse.json({ ok: false, error: "engine_offline", orders: [], total: 0 }, { status: 200 });
  }
}
