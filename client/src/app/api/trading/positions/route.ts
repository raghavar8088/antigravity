/**
 * Trading Positions
 *
 * GET /api/trading/positions
 *
 * Proxies to Go engine: GET /api/positions
 *
 * Expected engine response shape:
 * {
 *   positions: Position[];   // see client/src/types/trading.ts
 *   totalUnrealizedPnl: number;
 *   totalExposure: number;
 * }
 */

import { NextResponse } from "next/server";

const ENGINE_BASE = (process.env.INTERNAL_API_URL ?? process.env.ENGINE_URL ?? "http://127.0.0.1:8080").replace(/\/+$/, "");

export async function GET() {
  try {
    const res = await fetch(`${ENGINE_BASE}/api/positions`, {
      cache: "no-store",
      signal: AbortSignal.timeout(5_000),
      headers: { "X-Service-Name": "vercel-trading-positions" },
    });
    if (!res.ok) return NextResponse.json({ ok: false, error: `Engine ${res.status}` }, { status: res.status });
    return NextResponse.json(await res.json());
  } catch {
    return NextResponse.json({ ok: false, error: "engine_offline", positions: [], totalUnrealizedPnl: 0, totalExposure: 0 }, { status: 200 });
  }
}

export async function DELETE(req: Request) {
  /**
   * Force close a position
   * DELETE /api/trading/positions
   * Body: { positionId: string; symbol: string; quantity: number }
   *
   * Proxies to engine: POST /api/positions/force-close
   */
  try {
    const body = await req.json();
    const ENGINE_ADMIN_SECRET = process.env.ENGINE_ADMIN_SECRET;
    if (!ENGINE_ADMIN_SECRET) {
      return Response.json({ ok: false, error: "ENGINE_ADMIN_SECRET not configured" }, { status: 503 });
    }
    const res = await fetch(`${ENGINE_BASE}/api/positions/force-close`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Engine-Admin-Secret": ENGINE_ADMIN_SECRET,
        "X-Service-Name": "vercel-force-close",
      },
      body: JSON.stringify(body),
      signal: AbortSignal.timeout(10_000),
    });
    return Response.json(await res.json(), { status: res.status });
  } catch (e) {
    const msg = e instanceof Error ? e.message : "unknown error";
    return Response.json({ ok: false, error: msg }, { status: 502 });
  }
}
