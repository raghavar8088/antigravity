/**
 * GET /api/engine/config/history?key=X&limit=N
 *
 * Proxies to the Go engine's GET /api/engine/config/history — the audit
 * trail of every threshold edit made through the Trade Threshold
 * Configuration module (collection config_changes). Omitting key returns
 * changes across all thresholds, newest first, capped at 50 by the engine.
 */

import { NextResponse } from "next/server";
import type { ConfigChangeHistoryResponse } from "@/lib/thresholdConfig/types";

export const dynamic = "force-dynamic";

const ENGINE_BASE = (process.env.INTERNAL_API_URL ?? process.env.ENGINE_URL ?? "http://127.0.0.1:8080").replace(
  /\/+$/,
  "",
);

export async function GET(req: Request): Promise<NextResponse> {
  const url = new URL(req.url);
  const key = url.searchParams.get("key");
  const limit = url.searchParams.get("limit");

  const params = new URLSearchParams();
  if (key) params.set("key", key);
  if (limit) params.set("limit", limit);
  const qs = params.toString();

  try {
    const res = await fetch(`${ENGINE_BASE}/api/engine/config/history${qs ? `?${qs}` : ""}`, {
      cache: "no-store",
      signal: AbortSignal.timeout(10_000),
      headers: { "X-Service-Name": "vercel-threshold-config" },
    });
    if (!res.ok) {
      return NextResponse.json({ ok: false, error: `Engine returned ${res.status}`, history: [] }, { status: 502 });
    }
    const data = (await res.json()) as ConfigChangeHistoryResponse;
    return NextResponse.json(data);
  } catch (err) {
    const message = err instanceof Error ? err.message : "engine_offline";
    return NextResponse.json({ ok: false, error: message, history: [] }, { status: 200 });
  }
}
