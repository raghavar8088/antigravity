/**
 * Crypto Options Selling Desk Proxy
 *
 * Forwards session-authenticated READ requests to the options_selling engine,
 * which runs inside the main antigravity process (cmd/antigravity/main.go),
 * on OPTIONS_SELLING_ENGINE_URL (default: same host as INTERNAL_API_URL /
 * ENGINE_URL, port 8080 locally).
 *
 * Paper-only, read-only over HTTP — this proxy exposes GET endpoints only.
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
  "/api/options-selling/positions",
  "/api/options-selling/trades",
  "/api/options-selling/strategies",
  "/api/options-selling/stats",
];

function isAllowed(pathname: string): boolean {
  const clean = pathname.replace(/\/+$/, "").toLowerCase();
  return ALLOWED_PATHS.some((p) => clean === p || clean.startsWith(p + "/"));
}

function upstreamBase(): string {
  return (
    process.env.OPTIONS_SELLING_ENGINE_URL?.trim().replace(/\/+$/, "") ??
    process.env.INTERNAL_API_URL?.trim().replace(/\/+$/, "") ??
    process.env.ENGINE_URL?.trim().replace(/\/+$/, "") ??
    "http://127.0.0.1:8080"
  );
}

type RouteCtx = { params: Promise<{ path: string[] }> };

export async function GET(req: NextRequest, ctx: RouteCtx): Promise<NextResponse> {
  const { path } = await ctx.params;
  const pathname = path?.length ? `/api/options-selling/${path.join("/")}` : "/api/options-selling";

  if (!isAllowed(pathname)) {
    return NextResponse.json(
      { ok: false, error: `${pathname} is not available via the options-selling proxy` },
      { status: 403 },
    );
  }

  const cookieStore = await cookies();
  const session = cookieStore.get("raig_session")?.value ?? "";
  if (!(await verifySessionToken(session))) {
    return NextResponse.json({ ok: false, error: "Valid session required" }, { status: 401 });
  }

  const target = `${upstreamBase()}${pathname}${req.nextUrl.search}`;
  try {
    const upstream = await fetch(target, {
      method: "GET",
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
