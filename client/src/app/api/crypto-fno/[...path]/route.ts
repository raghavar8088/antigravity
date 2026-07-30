/**
 * Crypto F&O Paper Desk Proxy
 *
 * Forwards session-authenticated requests to the crypto F&O desk on the Go
 * engine. GET for reads (accounts, chain, positions); POST for the desk's
 * mutations (create/edit/reset/delete an account, preview, execute, close).
 *
 * Paper only — this desk holds no keys and places no broker orders. The balance
 * gate is enforced inside the engine's ExecuteBasket, not here: a proxy that
 * could bypass it would make the gate advisory.
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

const READ_PATHS = ["/accounts", "/chain", "/positions"];
const WRITE_PATHS = [
  "/accounts",
  "/accounts/edit",
  "/accounts/reset",
  "/accounts/delete",
  "/preview",
  "/execute",
  "/close",
];

function allowed(pathname: string, list: string[]): boolean {
  const clean = pathname.replace(/\/+$/, "").toLowerCase();
  return list.includes(clean);
}

function upstreamBase(): string {
  return (
    process.env.INTERNAL_API_URL?.trim().replace(/\/+$/, "") ??
    process.env.ENGINE_URL?.trim().replace(/\/+$/, "") ??
    "http://127.0.0.1:8080"
  );
}

type RouteCtx = { params: Promise<{ path: string[] }> };

async function requireSession(): Promise<string | null> {
  const cookieStore = await cookies();
  const session = cookieStore.get("raig_session")?.value ?? "";
  return (await verifySessionToken(session)) ? session : null;
}

export async function GET(req: NextRequest, ctx: RouteCtx): Promise<NextResponse> {
  const { path } = await ctx.params;
  const sub = path?.length ? `/${path.join("/")}` : "/";

  if (!allowed(sub, READ_PATHS)) {
    return NextResponse.json({ error: `${sub} is not a GET endpoint here` }, { status: 403 });
  }
  const session = await requireSession();
  if (!session) {
    return NextResponse.json({ error: "Valid session required" }, { status: 401 });
  }

  const target = `${upstreamBase()}/api/crypto-fno${sub}${req.nextUrl.search}`;
  try {
    const upstream = await fetch(target, {
      method: "GET",
      headers: { Authorization: `Bearer ${session}`, "X-Service-Name": "vercel-proxy" },
      cache: "no-store",
      signal: AbortSignal.timeout(30_000),
    });
    const out = new Headers(upstream.headers);
    out.delete("transfer-encoding");
    return new NextResponse(upstream.body, { status: upstream.status, headers: out });
  } catch (e) {
    return NextResponse.json(
      { error: e instanceof Error ? e.message : "proxy failed" },
      { status: 502 },
    );
  }
}

export async function POST(req: NextRequest, ctx: RouteCtx): Promise<NextResponse> {
  const { path } = await ctx.params;
  const sub = path?.length ? `/${path.join("/")}` : "/";

  if (!allowed(sub, WRITE_PATHS)) {
    return NextResponse.json({ error: `${sub} is not a POST endpoint here` }, { status: 403 });
  }
  const session = await requireSession();
  if (!session) {
    return NextResponse.json({ error: "Valid session required" }, { status: 401 });
  }

  const target = `${upstreamBase()}/api/crypto-fno${sub}`;
  try {
    const body = await req.text();
    const upstream = await fetch(target, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${session}`,
        "Content-Type": "application/json",
        "X-Service-Name": "vercel-proxy",
      },
      body: body || "{}",
      cache: "no-store",
      signal: AbortSignal.timeout(30_000),
    });
    const out = new Headers(upstream.headers);
    out.delete("transfer-encoding");
    // 422 (insufficient capital) is passed through unchanged — it is a business
    // outcome the UI renders, not a proxy failure to be masked as 500.
    return new NextResponse(upstream.body, { status: upstream.status, headers: out });
  } catch (e) {
    return NextResponse.json(
      { error: e instanceof Error ? e.message : "proxy failed" },
      { status: 502 },
    );
  }
}
