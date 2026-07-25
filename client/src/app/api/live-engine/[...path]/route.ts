/**
 * Live Engine Proxy — REAL MONEY control plane.
 *
 * Forwards session-authenticated requests to the Live Engine control plane in
 * the main antigravity engine (cmd/antigravity/main.go, /api/live-engine/*).
 *
 * GET  → reads (state, account, positions, orders, roster, reconciliation, audit)
 * POST → mutations (arm, disarm, close-all)
 *
 * Mutations require a valid session AND inject the server-side X-API-Token that
 * the engine authorizer checks; the token never reaches the browser. The acting
 * principal (session userId) is forwarded as X-Actor for the audit trail.
 */
import { NextRequest, NextResponse } from "next/server";
import { cookies } from "next/headers";

async function verifySession(token: string): Promise<{ ok: boolean; userId?: string }> {
  const secret = process.env.AUTH_JWT_SECRET?.trim();
  if (!secret || !token) return { ok: false };
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return { ok: false };
    const [header, payload, sig] = parts as [string, string, string];
    const body = `${header}.${payload}`;
    const key = await crypto.subtle.importKey(
      "raw",
      new TextEncoder().encode(secret),
      { name: "HMAC", hash: "SHA-256" },
      false,
      ["sign"],
    );
    const expectedBuf = await crypto.subtle.sign("HMAC", key, new TextEncoder().encode(body));
    const expectedB64 = btoa(String.fromCharCode(...new Uint8Array(expectedBuf)))
      .replace(/\+/g, "-")
      .replace(/\//g, "_")
      .replace(/=/g, "");
    if (sig.length !== expectedB64.length) return { ok: false };
    let diff = 0;
    for (let i = 0; i < sig.length; i++) diff |= sig.charCodeAt(i) ^ expectedB64.charCodeAt(i);
    if (diff !== 0) return { ok: false };
    const p = JSON.parse(atob(payload.replace(/-/g, "+").replace(/_/g, "/")));
    if (typeof p.exp === "number" && p.exp < Math.floor(Date.now() / 1000)) return { ok: false };
    if (typeof p.userId !== "string") return { ok: false };
    return { ok: true, userId: p.userId };
  } catch {
    return { ok: false };
  }
}

const READ_ACTIONS = ["state", "account", "positions", "orders", "roster", "reconciliation", "audit"];
const WRITE_ACTIONS = ["arm", "disarm", "close-all"];

function upstreamBase(): string {
  return (
    process.env.LIVE_ENGINE_ENGINE_URL?.trim().replace(/\/+$/, "") ??
    process.env.INTERNAL_API_URL?.trim().replace(/\/+$/, "") ??
    process.env.ENGINE_URL?.trim().replace(/\/+$/, "") ??
    "http://127.0.0.1:8080"
  );
}

function engineToken(): string {
  return (process.env.LIVE_ENGINE_API_TOKEN?.trim() ?? process.env.INTERNAL_API_SECRET?.trim() ?? "");
}

type RouteCtx = { params: Promise<{ path: string[] }> };

export async function GET(req: NextRequest, ctx: RouteCtx): Promise<NextResponse> {
  const { path } = await ctx.params;
  const action = (path?.[0] ?? "").toLowerCase();
  if (!READ_ACTIONS.includes(action)) {
    return NextResponse.json({ ok: false, error: `unknown live-engine read: ${action}` }, { status: 404 });
  }
  const session = (await cookies()).get("raig_session")?.value ?? "";
  const s = await verifySession(session);
  if (!s.ok) return NextResponse.json({ ok: false, error: "Valid session required" }, { status: 401 });

  const target = `${upstreamBase()}/api/live-engine/${action}${req.nextUrl.search}`;
  try {
    const headers = new Headers();
    headers.set("Authorization", `Bearer ${session}`);
    const token = engineToken();
    if (token) headers.set("X-API-Token", token);
    if (s.userId) headers.set("X-Actor", s.userId);
    const upstream = await fetch(target, { method: "GET", headers, cache: "no-store", signal: AbortSignal.timeout(30_000) });
    const out = new Headers(upstream.headers);
    out.delete("transfer-encoding");
    return new NextResponse(upstream.body, { status: upstream.status, headers: out });
  } catch (e) {
    return NextResponse.json({ ok: false, error: e instanceof Error ? e.message : "proxy failed" }, { status: 502 });
  }
}

export async function POST(req: NextRequest, ctx: RouteCtx): Promise<NextResponse> {
  const { path } = await ctx.params;
  const action = (path?.[0] ?? "").toLowerCase();
  if (!WRITE_ACTIONS.includes(action)) {
    return NextResponse.json({ ok: false, error: `unknown live-engine action: ${action}` }, { status: 404 });
  }
  const session = (await cookies()).get("raig_session")?.value ?? "";
  const s = await verifySession(session);
  if (!s.ok) return NextResponse.json({ ok: false, error: "Valid session required" }, { status: 401 });

  const token = engineToken();
  if (!token) {
    return NextResponse.json(
      { ok: false, error: "Live Engine mutations are not configured on this deployment (no LIVE_ENGINE_API_TOKEN)" },
      { status: 503 },
    );
  }

  const rawBody = await req.text();
  const target = `${upstreamBase()}/api/live-engine/${action}`;
  try {
    const headers = new Headers();
    headers.set("Content-Type", "application/json");
    headers.set("Authorization", `Bearer ${session}`);
    headers.set("X-API-Token", token);
    if (s.userId) headers.set("X-Actor", s.userId);
    const upstream = await fetch(target, {
      method: "POST",
      headers,
      body: rawBody || "{}",
      cache: "no-store",
      signal: AbortSignal.timeout(30_000),
    });
    const out = new Headers(upstream.headers);
    out.delete("transfer-encoding");
    return new NextResponse(upstream.body, { status: upstream.status, headers: out });
  } catch (e) {
    return NextResponse.json({ ok: false, error: e instanceof Error ? e.message : "proxy failed" }, { status: 502 });
  }
}
