/**
 * Kill Switch Status
 *
 * GET /api/killswitch/status
 *
 * Proxies to Go engine: GET /api/admin/ks/status
 *
 * Expected engine response shape:
 * {
 *   active: boolean;           // true = kill switch triggered, all strategies halted
 *   triggeredAt: string | null; // ISO timestamp of when it was triggered
 *   triggeredBy: string | null; // "operator" | "risk_gate" | "manual"
 *   activeStrategies: number;   // count of currently running strategies
 *   openOrders: number;         // count of orders currently in OMS
 *   blockedAt: string | null;   // alias for triggeredAt (engine may use either)
 * }
 */

import { NextResponse } from "next/server";

const ENGINE_BASE = (process.env.INTERNAL_API_URL ?? process.env.ENGINE_URL ?? "http://127.0.0.1:8080").replace(/\/+$/, "");

export async function GET() {
  try {
    const res = await fetch(`${ENGINE_BASE}/api/admin/ks/status`, {
      cache: "no-store",
      signal: AbortSignal.timeout(5_000),
      headers: { "X-Service-Name": "vercel-ks-status" },
    });
    if (!res.ok) {
      return NextResponse.json({ ok: false, error: "Engine returned " + res.status }, { status: res.status });
    }
    const data = await res.json();
    return NextResponse.json(data);
  } catch {
    // Return a safe "unknown" state so UI can show a warning rather than crash
    return NextResponse.json(
      { active: false, triggeredAt: null, triggeredBy: null, activeStrategies: 0, openOrders: 0, error: "engine_offline" },
      { status: 200 },
    );
  }
}
