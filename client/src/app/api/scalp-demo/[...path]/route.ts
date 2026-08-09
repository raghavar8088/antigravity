/**
 * Live DEMO Engine Proxy — demo.delta.exchange, NOT real money.
 *
 * Forwards to the demo scalp engine (a second scalp_prelive container running
 * against Delta's demo venue, cdn-ind.testnet.deltaex.org, on port 8095).
 *
 * This is a deliberate second process rather than a mode flag on the live one.
 * The two hold DIFFERENT API credentials against DIFFERENT venues, and a flag
 * that silently selects between them is one misread environment variable away
 * from sending a demo order to the real wallet, or worse, reporting demo fills
 * as real ones. Separate ports make the venue impossible to confuse.
 *
 * SCALP_DEMO_ENGINE_URL overrides the target (default: Lightsail :8095).
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
  "/scalp/health",
  "/scalp/stats",
  "/scalp/leaderboard",
  "/scalp/trades",
  // Open paper positions. Each row is flagged `live`, so the desk page can
  // show the streams that can actually reach the venue rather than all ~330.
  "/scalp/positions",
  // Read-only views of the desk's REAL-MONEY perpetual arm. The Live Engine
  // page needs these: both it and the scalp bridge spend from one Delta wallet,
  // so a leaderboard that showed only one of them would report half the real
  // exposure while looking complete.
  //
  // Reads, plus arm/disarm below. close-all stays off the proxy: flattening
  // positions is not needed from here, because the bridge's custody loop keeps
  // managing TP/SL/TTL on open positions even while disarmed — disarming stops
  // NEW orders without abandoning the ones already funded.
  "/scalp/live/stats",
  "/scalp/live/trades",
  "/scalp/live/reconcile",
  // The Live Engine Paper Desk: the promoted strategies on $100 of paper money
  // against real Delta prices and real taker fees. Read-only here — its reset
  // stays behind the desk token, because clearing the record that decides what
  // gets real capital should not be one click from a browser session.
  "/scalp/live/paper",
];

/**
 * Desk-management mutations reachable by POST. Exact-match, so a mutation can
 * never be hit by a GET and a read path can never be POSTed to. Paper only —
 * these clear simulated statistics; the upstream binary has no order routing.
 */
const MUTATION_PATHS = [
  "/scalp/reset",
  "/scalp/clear-trades",
  // Arming the PERPETUAL desk.
  //
  // These were held back so real money stayed behind the desk's own token. The
  // cost of that turned out to be worse than the risk it avoided: the Live
  // Engine page showed the perp desk's positions, trades and leaderboard but
  // had no control for it, while the one visible toggle armed the OPTIONS
  // engine. An operator turned that toggle on and reasonably believed the
  // scalp strategies were live. They were not, for 105 minutes.
  //
  // A control that exists nowhere is not a safety measure — it is a page that
  // misreports its own state. Both still require a valid session cookie, and
  // the proxy injects the desk token server-side.
  "/scalp/live/arm",
  "/scalp/live/disarm",
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
    process.env.SCALP_DEMO_ENGINE_URL?.trim().replace(/\/+$/, "") ??
    "http://13.233.8.80:8095"
  );
}

type RouteCtx = { params: Promise<{ path: string[] }> };

export async function GET(req: NextRequest, ctx: RouteCtx): Promise<NextResponse> {
  const { path } = await ctx.params;
  const pathname = path?.length ? `/${path.join("/")}` : "/";

  if (!isAllowed(pathname)) {
    return NextResponse.json(
      { ok: false, error: `${pathname} is not available via the scalp proxy` },
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
    const headers = new Headers();
    const apiToken =
      process.env.SCALP_DEMO_API_TOKEN?.trim() ?? process.env.BTC_PRE_LIVE_API_TOKEN?.trim();
    if (apiToken) headers.set("X-API-Token", apiToken);

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
  const pathname = path?.length ? `/${path.join("/")}` : "/";

  if (!isMutation(pathname)) {
    return NextResponse.json(
      { ok: false, error: `${pathname} is not a POST endpoint on the scalp proxy` },
      { status: 403 },
    );
  }

  const cookieStore = await cookies();
  const session = cookieStore.get("raig_session")?.value ?? "";
  if (!(await verifySessionToken(session))) {
    return NextResponse.json({ ok: false, error: "Valid session required" }, { status: 401 });
  }

  const target = `${upstreamBase()}${pathname}`;
  try {
    const headers = new Headers();
    const apiToken =
      process.env.SCALP_DEMO_API_TOKEN?.trim() ?? process.env.BTC_PRE_LIVE_API_TOKEN?.trim();
    if (apiToken) headers.set("X-API-Token", apiToken);
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
