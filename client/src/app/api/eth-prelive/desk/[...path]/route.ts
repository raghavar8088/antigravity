/**
 * ETH Pre-Live Engine Proxy — $1,000 paper desk, 46-strategy ETH whitelist
 *
 * Forwards authenticated requests to the ETH pre-live instance (a second
 * cmd/pre_live process with its own whitelist/balance/account key) running on
 * ETH_PRE_LIVE_ENGINE_URL (default http://127.0.0.1:8093).
 *
 * Deliberately REDUCED allowlist vs the main pre-live proxy: no /api/live/*
 * paths. The Delta real-money mirror inside this instance stays unreachable
 * from the UI until Phase 4 explicitly adds a gated arm flow.
 *
 * Security: session cookie required for all requests (same as /api/pre-live).
 */
import { NextRequest, NextResponse } from "next/server";
import { cookies } from "next/headers";

async function verifySessionToken(token: string): Promise<boolean> {
  const secret = process.env.AUTH_JWT_SECRET?.trim();
  if (!secret || !token) return false;
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return false;
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
    if (sig.length !== expectedB64.length) return false;
    let diff = 0;
    for (let i = 0; i < sig.length; i++) diff |= sig.charCodeAt(i) ^ expectedB64.charCodeAt(i);
    if (diff !== 0) return false;
    const p = JSON.parse(atob(payload.replace(/-/g, "+").replace(/_/g, "/")));
    if (typeof p.exp === "number" && p.exp < Math.floor(Date.now() / 1000)) return false;
    return typeof p.userId === "string";
  } catch {
    return false;
  }
}

const ALLOWED_PATHS = [
  "/health",
  "/ready",
  "/api/positions",
  "/api/trades",
  "/api/stats",
  "/api/scalers/stats",
  "/api/admin/reset",
];

function isAllowed(pathname: string): boolean {
  const clean = pathname.replace(/\/+$/, "").toLowerCase();
  return ALLOWED_PATHS.some((p) => clean === p || clean.startsWith(p + "/") || clean.startsWith(p + "?"));
}

function upstreamBase(): string {
  return (
    process.env.ETH_PRE_LIVE_ENGINE_URL?.trim().replace(/\/+$/, "") ??
    "http://127.0.0.1:8093"
  );
}

type RouteCtx = { params: Promise<{ path: string[] }> };

async function handle(req: NextRequest, ctx: RouteCtx): Promise<NextResponse> {
  const { path } = await ctx.params;
  const pathname = path?.length ? `/${path.join("/")}` : "/";

  if (!isAllowed(pathname)) {
    return NextResponse.json(
      { ok: false, error: `${pathname} is not available via the ETH pre-live proxy` },
      { status: 403 },
    );
  }

  const cookieStore = await cookies();
  const token = cookieStore.get("raig_session")?.value ?? "";
  const sessionValid = await verifySessionToken(token);
  if (!sessionValid) {
    return NextResponse.json({ ok: false, error: "Valid session required" }, { status: 401 });
  }

  const target = `${upstreamBase()}${pathname}${req.nextUrl.search}`;
  try {
    const headers = new Headers();
    const ct = req.headers.get("content-type");
    if (ct) headers.set("content-type", ct);
    // The BTC desk's port is internet-exposed, so its API requires a shared
    // secret (PRE_LIVE_API_TOKEN on the engine side). The browser session was
    // already verified above — this header authenticates the PROXY to the desk.
    const apiToken = process.env.BTC_PRE_LIVE_API_TOKEN?.trim();
    if (apiToken) headers.set("Authorization", `Bearer ${apiToken}`);

    const init: RequestInit = {
      method: req.method,
      headers,
      cache: "no-store",
      signal: AbortSignal.timeout(30_000),
    };
    if (req.method !== "GET" && req.method !== "HEAD") {
      init.body = await req.arrayBuffer();
    }
    const upstream = await fetch(target, init);
    const out = new Headers(upstream.headers);
    out.delete("transfer-encoding");
    return new NextResponse(upstream.body, { status: upstream.status, headers: out });
  } catch (e) {
    const message = e instanceof Error ? e.message : "proxy failed";
    return NextResponse.json({ ok: false, error: message }, { status: 502 });
  }
}

export const GET = handle;
export const POST = handle;
export const PUT = handle;
export const PATCH = handle;
export const DELETE = handle;
