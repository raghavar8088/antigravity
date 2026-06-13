/**
 * Strategy Detail
 *
 * GET /api/strategies/[id]
 *
 * Proxies to Go engine: GET /api/strategies/registry/:id
 *
 * Expected engine response shape: StrategyDetail (see client/src/types/strategy.ts)
 *
 * POST /api/strategies/[id]
 * Body: { action: "enable" | "pause" | "override_winners_only"; reason?: string }
 *
 * Proxies to engine: POST /api/strategies/registry/:id/action
 * Requires ENGINE_ADMIN_SECRET for mutating actions.
 */

import { type NextRequest, NextResponse } from "next/server";
import { cookies } from "next/headers";

const ENGINE_BASE = (process.env.INTERNAL_API_URL ?? process.env.ENGINE_URL ?? "http://127.0.0.1:8080").replace(/\/+$/, "");

type RouteCtx = { params: Promise<{ id: string }> };

export async function GET(_req: NextRequest, ctx: RouteCtx) {
  const { id } = await ctx.params;
  try {
    const res = await fetch(`${ENGINE_BASE}/api/strategies/registry/${id}`, {
      cache: "no-store",
      signal: AbortSignal.timeout(5_000),
      headers: { "X-Service-Name": "vercel-strategy-detail" },
    });
    if (!res.ok) return NextResponse.json({ ok: false, error: `Engine ${res.status}` }, { status: res.status });
    return NextResponse.json(await res.json());
  } catch {
    return NextResponse.json({ ok: false, error: "engine_offline" }, { status: 503 });
  }
}

export async function POST(req: NextRequest, ctx: RouteCtx) {
  const { id } = await ctx.params;
  const cookieStore = await cookies();
  const token = cookieStore.get("raig_session")?.value ?? "";
  if (!token) return NextResponse.json({ ok: false, error: "Unauthorized" }, { status: 401 });

  const adminSecret = process.env.ENGINE_ADMIN_SECRET;
  if (!adminSecret) return NextResponse.json({ ok: false, error: "ENGINE_ADMIN_SECRET not configured" }, { status: 503 });

  try {
    const body = await req.json();
    const res = await fetch(`${ENGINE_BASE}/api/strategies/registry/${id}/action`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Engine-Admin-Secret": adminSecret,
        "X-Service-Name": "vercel-strategy-action",
      },
      body: JSON.stringify(body),
      signal: AbortSignal.timeout(10_000),
    });
    return NextResponse.json(await res.json(), { status: res.status });
  } catch (e) {
    return NextResponse.json({ ok: false, error: e instanceof Error ? e.message : "unknown" }, { status: 502 });
  }
}
