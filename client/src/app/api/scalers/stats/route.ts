/**
 * GET /api/scalers/stats
 *
 * Proxies to Go engine GET /api/scalers/stats — per-strategy scalper telemetry,
 * regime, CVD, and recent signal snapshots for the Trade Engine UI.
 */

import { NextResponse } from "next/server";

const ENGINE_BASE = (process.env.INTERNAL_API_URL ?? process.env.ENGINE_URL ?? "http://127.0.0.1:8080").replace(/\/+$/, "");

const EMPTY_STATS = {
  strategies: [] as unknown[],
  regime: "UNKNOWN",
  cvd: 0,
  eval_count: 0,
  raw_signals_last_cycle: 0,
  approved_signals_last_cycle: 0,
  rejected_signals_last_cycle: 0,
  recent_signals: [] as unknown[],
};

export async function GET() {
  try {
    const res = await fetch(`${ENGINE_BASE}/api/scalers/stats`, {
      cache: "no-store",
      signal: AbortSignal.timeout(5_000),
      headers: { "X-Service-Name": "vercel-scalers-stats" },
    });
    if (!res.ok) {
      return NextResponse.json(
        { ok: false, error: `Engine returned ${res.status}`, ...EMPTY_STATS },
        { status: 502 },
      );
    }
    const data = await res.json();
    return NextResponse.json({ ok: true, ...data });
  } catch {
    return NextResponse.json(
      { ok: false, error: "engine_offline", ...EMPTY_STATS },
      { status: 200 },
    );
  }
}
