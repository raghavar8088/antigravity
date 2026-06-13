/**
 * Risk Gates Pipeline Status
 *
 * GET /api/risk/gates
 *
 * Proxies to Go engine: GET /api/risk/gates
 *
 * Expected engine response shape:
 * {
 *   gates: Array<{
 *     id: string;                  // e.g. "risk_check" | "position_size" | "exposure_check" | "daily_limit" | "oms"
 *     label: string;               // Human-readable name
 *     state: "open" | "throttled" | "closed";
 *     lastCheckAt: string;         // ISO timestamp
 *     passCountToday: number;
 *     failCountToday: number;
 *   }>;
 *   pipelineHealthy: boolean;
 *   lastUpdated: string;
 * }
 */

import { NextResponse } from "next/server";

const ENGINE_BASE = (process.env.INTERNAL_API_URL ?? process.env.ENGINE_URL ?? "http://127.0.0.1:8080").replace(/\/+$/, "");

export async function GET() {
  try {
    const res = await fetch(`${ENGINE_BASE}/api/risk/gates`, {
      cache: "no-store",
      signal: AbortSignal.timeout(5_000),
      headers: { "X-Service-Name": "vercel-risk-gates" },
    });
    if (!res.ok) {
      return NextResponse.json({ ok: false, error: `Engine returned ${res.status}` }, { status: res.status });
    }
    return NextResponse.json(await res.json());
  } catch {
    return NextResponse.json({ ok: false, error: "engine_offline" }, { status: 503 });
  }
}
