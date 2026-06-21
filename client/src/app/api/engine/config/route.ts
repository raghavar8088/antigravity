/**
 * Trade Threshold Configuration — GET/POST /api/engine/config
 *
 * GET  proxies to the Go engine's GET /api/engine/config and returns the full
 *      ThresholdConfig[] (live registry values merged with static metadata —
 *      label, description, bounds, risk level, what-happens-if text).
 * POST validates the caller supplied X-Engine-Admin-Secret (compared against
 *      ENGINE_ADMIN_SECRET server-side — never exposed to the browser),
 *      forwards the {key, value} body to the engine's POST /api/engine/config,
 *      which performs the authoritative [min,max] validation, applies the
 *      change immediately via the ThresholdRegistry, and writes an audit log
 *      entry. This route does not duplicate that validation — the engine is
 *      the single source of truth for what is in-range.
 */

import { NextResponse } from "next/server";
import type { ThresholdConfigDTO, ThresholdConfigListResponse } from "@/lib/thresholdConfig/types";

export const dynamic = "force-dynamic";

const ENGINE_BASE = (process.env.INTERNAL_API_URL ?? process.env.ENGINE_URL ?? "http://127.0.0.1:8080").replace(
  /\/+$/,
  "",
);

export async function GET(): Promise<NextResponse> {
  try {
    const res = await fetch(`${ENGINE_BASE}/api/engine/config`, {
      cache: "no-store",
      signal: AbortSignal.timeout(10_000),
      headers: { "X-Service-Name": "vercel-threshold-config" },
    });
    if (!res.ok) {
      return NextResponse.json(
        { ok: false, error: `Engine returned ${res.status}`, thresholds: [] },
        { status: 502 },
      );
    }
    const data = (await res.json()) as ThresholdConfigListResponse;
    return NextResponse.json(data);
  } catch (err) {
    const message = err instanceof Error ? err.message : "engine_offline";
    return NextResponse.json({ ok: false, error: message, thresholds: [] }, { status: 200 });
  }
}

type SetRequestBody = {
  key: string;
  value: number;
  reason?: string;
};

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
  if (!body.key || typeof body.value !== "number" || !Number.isFinite(body.value)) {
    return NextResponse.json({ ok: false, error: "key and a finite numeric value are required" }, { status: 400 });
  }

  try {
    const res = await fetch(`${ENGINE_BASE}/api/engine/config`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Engine-Admin-Secret": expected,
        "X-Service-Name": "vercel-threshold-config",
      },
      body: JSON.stringify({ key: body.key, value: body.value, reason: body.reason }),
      signal: AbortSignal.timeout(10_000),
    });
    const data = (await res.json()) as {
      ok?: boolean;
      threshold?: ThresholdConfigDTO;
      requiresRestart?: boolean;
      error?: string;
      min?: number;
      max?: number;
    };
    return NextResponse.json(data, { status: res.status });
  } catch (err) {
    const message = err instanceof Error ? err.message : "engine_offline";
    return NextResponse.json({ ok: false, error: message }, { status: 502 });
  }
}
