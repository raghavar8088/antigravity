/**
 * POST /api/execution/request
 * Proxies authenticated execution intents to the Go engine institutional gateway.
 * Frontend must never call brokers directly.
 */

import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";

export const dynamic = "force-dynamic";

function engineBase(): string {
  return (
    process.env.INTERNAL_API_URL?.trim() ||
    process.env.ENGINE_URL?.trim() ||
    "http://127.0.0.1:8080"
  ).replace(/\/+$/, "");
}

export async function POST(req: Request) {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;

  let body: unknown;
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ ok: false, error: "Invalid JSON" }, { status: 400 });
  }

  const cookieHeader = req.headers.get("cookie") ?? "";

  try {
    const upstream = await fetch(`${engineBase()}/api/execution/request`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Cookie: cookieHeader,
        "X-Service-Name": "nextjs",
        ...(process.env.INTERNAL_API_SECRET
          ? { "X-Service-Auth": process.env.INTERNAL_API_SECRET }
          : {}),
      },
      body: JSON.stringify(body),
      cache: "no-store",
      signal: AbortSignal.timeout(60_000),
    });
    const payload = await upstream.json();
    return NextResponse.json(payload, { status: upstream.status });
  } catch (e) {
    const message = e instanceof Error ? e.message : "engine unreachable";
    return NextResponse.json({ ok: false, error: message }, { status: 502 });
  }
}
