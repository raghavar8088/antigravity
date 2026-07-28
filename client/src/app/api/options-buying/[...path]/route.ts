/**
 * Crypto Options Buying Desk Proxy
 *
 * Forwards session-authenticated READ requests to the options (long-premium)
 * engine, which runs inside the main antigravity process (cmd/antigravity/main.go),
 * on OPTIONS_BUYING_ENGINE_URL (default: same host as OPTIONS_SELLING_ENGINE_URL /
 * INTERNAL_API_URL / ENGINE_URL, port 8080 locally).
 *
 * Paper-only, read-only over HTTP — this proxy exposes GET endpoints only.
 * Client-facing paths under /api/options-buying/* map to the engine's real
 * routes under /api/options/* (see engine/cmd/antigravity/main.go).
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
  "/api/options-buying/positions",
  "/api/options-buying/trades",
  "/api/options-buying/strategies",
  "/api/options-buying/stats",
];

/**
 * Paper-desk mutations reachable by POST. Kept as its own exact-match list so a
 * mutation can never be reached by a GET, and a read path can never be POSTed
 * to. This desk is paper-only — these reset simulated balances and statistics,
 * and there is no order-routing or real-money path behind them.
 */
const MUTATION_PATHS = [
  "/api/options-buying/reset",
  "/api/options-buying/clear-history",
];

function isAllowed(pathname: string): boolean {
  const clean = pathname.replace(/\/+$/, "").toLowerCase();
  return ALLOWED_PATHS.some((p) => clean === p || clean.startsWith(p + "/"));
}

function isMutation(pathname: string): boolean {
  const clean = pathname.replace(/\/+$/, "").toLowerCase();
  return MUTATION_PATHS.includes(clean);
}

function upstreamBase(): string {
  return (
    process.env.OPTIONS_BUYING_ENGINE_URL?.trim().replace(/\/+$/, "") ??
    process.env.OPTIONS_SELLING_ENGINE_URL?.trim().replace(/\/+$/, "") ??
    process.env.INTERNAL_API_URL?.trim().replace(/\/+$/, "") ??
    process.env.ENGINE_URL?.trim().replace(/\/+$/, "") ??
    "http://127.0.0.1:8080"
  );
}

type RouteCtx = { params: Promise<{ path: string[] }> };

export async function GET(req: NextRequest, ctx: RouteCtx): Promise<NextResponse> {
  const { path } = await ctx.params;
  const clientPathname = path?.length ? `/api/options-buying/${path.join("/")}` : "/api/options-buying";

  if (!isAllowed(clientPathname)) {
    return NextResponse.json(
      { ok: false, error: `${clientPathname} is not available via the options-buying proxy` },
      { status: 403 },
    );
  }

  const cookieStore = await cookies();
  const session = cookieStore.get("raig_session")?.value ?? "";
  if (!(await verifySessionToken(session))) {
    return NextResponse.json({ ok: false, error: "Valid session required" }, { status: 401 });
  }

  // The engine hosts the buying desk at /api/options/*, not /api/options-buying/*.
  const upstreamPathname = path?.length ? `/api/options/${path.join("/")}` : "/api/options";
  const target = `${upstreamBase()}${upstreamPathname}${req.nextUrl.search}`;
  try {
    const headers = new Headers();
    headers.set("X-Service-Name", "vercel-proxy");
    headers.set("X-Service-Timestamp", String(Date.now()));
    headers.set("Authorization", `Bearer ${session}`);
    const upstream = await fetch(target, {
      method: "GET",
      headers,
      cache: "no-store",
      signal: AbortSignal.timeout(30_000),
    });
    const out = new Headers(upstream.headers);
    out.delete("transfer-encoding");
    return new NextResponse(upstream.body, { status: upstream.status, headers: out });
  } catch (e) {
    const message = e instanceof Error ? e.message : "proxy failed";
    return NextResponse.json({ ok: false, error: message }, { status: 502 });
  }
}

export async function POST(req: NextRequest, ctx: RouteCtx): Promise<NextResponse> {
  const { path } = await ctx.params;
  const clientPathname = path?.length ? `/api/options-buying/${path.join("/")}` : "/api/options-buying";

  if (!isMutation(clientPathname)) {
    return NextResponse.json(
      { ok: false, error: `${clientPathname} is not a POST endpoint on the options-buying proxy` },
      { status: 403 },
    );
  }

  const cookieStore = await cookies();
  const session = cookieStore.get("raig_session")?.value ?? "";
  if (!(await verifySessionToken(session))) {
    return NextResponse.json({ ok: false, error: "Valid session required" }, { status: 401 });
  }

  // The engine hosts the buying desk at /api/options/*, not /api/options-buying/*.
  const upstreamPathname = `/api/options/${path.join("/")}`;
  const target = `${upstreamBase()}${upstreamPathname}`;
  try {
    const headers = new Headers();
    headers.set("X-Service-Name", "vercel-proxy");
    headers.set("X-Service-Timestamp", String(Date.now()));
    headers.set("Authorization", `Bearer ${session}`);
    headers.set("Content-Type", "application/json");
    const body = await req.text();
    const upstream = await fetch(target, {
      method: "POST",
      headers,
      body: body || "{}",
      cache: "no-store",
      signal: AbortSignal.timeout(30_000),
    });
    const out = new Headers(upstream.headers);
    out.delete("transfer-encoding");
    return new NextResponse(upstream.body, { status: upstream.status, headers: out });
  } catch (e) {
    const message = e instanceof Error ? e.message : "proxy failed";
    return NextResponse.json({ ok: false, error: message }, { status: 502 });
  }
}
