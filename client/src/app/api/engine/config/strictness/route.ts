/**
 * Master Strictness Dial — GET/POST /api/engine/config/strictness
 *
 * GET  proxies to the Go engine's GET /api/engine/config/strictness and
 *      returns the live StrictnessProfile[] (CurrentValue read fresh from
 *      the registry server-side) plus drift-aware dial position detection.
 * POST validates X-Engine-Admin-Secret (compared server-side, never exposed
 *      to the browser) and forwards {dialPosition} to the engine, which
 *      applies the whole batch via ApplyStrictnessDial and writes ONE audit
 *      log entry. This route performs no threshold math itself — the engine
 *      is the single source of truth, identical to /api/engine/config.
 */

import { NextResponse } from "next/server";
import type { StrictnessGetResponse, StrictnessSetResponse } from "@/lib/thresholdConfig/types";

export const dynamic = "force-dynamic";

const ENGINE_BASE = (process.env.INTERNAL_API_URL ?? process.env.ENGINE_URL ?? "http://127.0.0.1:8080").replace(
  /\/+$/,
  "",
);

export async function GET(): Promise<NextResponse> {
  try {
    const res = await fetch(`${ENGINE_BASE}/api/engine/config/strictness`, {
      cache: "no-store",
      signal: AbortSignal.timeout(10_000),
      headers: { "X-Service-Name": "vercel-threshold-config" },
    });
    if (!res.ok) {
      return NextResponse.json(
        { ok: false, error: `Engine returned ${res.status}`, currentDialPosition: null, profiles: [], affectedThresholdCount: 0 },
        { status: 502 },
      );
    }
    const data = (await res.json()) as StrictnessGetResponse;
    return NextResponse.json(data);
  } catch (err) {
    const message = err instanceof Error ? err.message : "engine_offline";
    return NextResponse.json(
      { ok: false, error: message, currentDialPosition: null, profiles: [], affectedThresholdCount: 0 },
      { status: 200 },
    );
  }
}

type SetRequestBody = { dialPosition: number };

export async function POST(req: Request): Promise<NextResponse> {
  // Browser authentication is enforced upstream by src/middleware.ts, which
  // requires a valid raig_session on every non-public path (this route
  // included). ENGINE_ADMIN_SECRET is a SERVER-TO-SERVER credential: it is read
  // from the environment here and forwarded to the Go engine as
  // X-Engine-Admin-Secret. It is never supplied by — nor exposed to — the
  // browser, so we must NOT require it on the inbound request (the browser
  // cannot send it). Same model as /api/killswitch/trigger.
  const expected = process.env.ENGINE_ADMIN_SECRET ?? "";
  if (!expected) {
    return NextResponse.json({ ok: false, error: "ENGINE_ADMIN_SECRET is not configured" }, { status: 503 });
  }

  let body: SetRequestBody;
  try {
    body = (await req.json()) as SetRequestBody;
  } catch {
    return NextResponse.json({ ok: false, error: "invalid JSON body" }, { status: 400 });
  }
  if (typeof body.dialPosition !== "number" || !Number.isFinite(body.dialPosition)) {
    return NextResponse.json({ ok: false, error: "dialPosition must be a finite number" }, { status: 400 });
  }

  try {
    const res = await fetch(`${ENGINE_BASE}/api/engine/config/strictness`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Engine-Admin-Secret": expected,
        "X-Service-Name": "vercel-threshold-config",
      },
      body: JSON.stringify({ dialPosition: body.dialPosition }),
      signal: AbortSignal.timeout(10_000),
    });
    const data = (await res.json()) as StrictnessSetResponse;
    return NextResponse.json(data, { status: res.status });
  } catch (err) {
    const message = err instanceof Error ? err.message : "engine_offline";
    return NextResponse.json({ ok: false, error: message }, { status: 502 });
  }
}
