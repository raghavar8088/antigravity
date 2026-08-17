/**
 * Top Crypto Trading proxy — the full pattern catalogue on the ten highest
 * cumulative-turnover instruments.
 *
 * Forwards to a FOURTH scalp_prelive container (port 8097), same binary as the
 * scalp desk on 8094, the demo on 8095 and High Volume on 8096, with three
 * differences that are entirely configuration:
 *
 *   - an explicit ten-symbol universe, chosen on 400-day cumulative turnover
 *     rather than a 24h snapshot, so the basket is not whatever spiked
 *     yesterday;
 *   - SCALP_SYMBOL_BOOKS=true, giving one paper book per symbol instead of
 *     numbered candidate books;
 *   - SCALP_REWARD_RISK=6, the ratio requested for this module.
 *
 * A separate process for the same reason High Volume is one: concurrency
 * limits, pending-order queues and paper books are all per-process, so ten
 * symbols competing only with each other is a different experiment from ten
 * symbols competing with 210 others. Filtering an existing desk in the browser
 * would show the second and label it the first.
 *
 * SCALP_TOPCRYPTO_ENGINE_URL overrides the target (default: Lightsail :8097).
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

/**
 * Reads. Identical set to the top-crypto proxy, because the page is a clone of the
 * scalp desk page and asks for exactly the same endpoints.
 */
const ALLOWED_PATHS = [
  "/scalp/health",
  "/scalp/stats",
  "/scalp/leaderboard",
  "/scalp/trades",
  "/scalp/positions",
  // The perpetual arm's read-only views. This process holds no trading
  // credentials, so these answer `enabled: false` — kept in the list so the
  // cloned page's fetches resolve rather than 403, which reads to an operator
  // as a broken desk rather than an unarmed one.
  "/scalp/live/stats",
  "/scalp/live/trades",
  "/scalp/live/reconcile",
  "/scalp/live/paper",
];

/**
 * Mutations. Paper statistics only.
 *
 * The arm/disarm and per-stream switch paths that the top-crypto proxy carries are
 * deliberately ABSENT. Those exist there because that process holds the desk's
 * real Delta credentials; this one is launched without them, so an arm call
 * could only ever fail — and a control that renders, responds to a click and
 * reaches nothing is the exact failure mode this codebase has already paid for
 * twice. Better it 403 at the proxy with a reason than pretend.
 */
const MUTATION_PATHS = ["/scalp/reset", "/scalp/clear-trades"];

function isAllowed(pathname: string): boolean {
  const clean = pathname.replace(/\/+$/, "").toLowerCase();
  return ALLOWED_PATHS.some((p) => clean === p || clean.startsWith(p + "/"));
}

function isMutation(pathname: string): boolean {
  const clean = pathname.replace(/\/+$/, "").toLowerCase();
  return MUTATION_PATHS.includes(clean);
}

function upstreamBase(): string {
  // `||`, not `??`: an env var set to an empty string is a misconfiguration,
  // not a choice, and with `??` it survives as "" and collapses the fetch
  // target to a relative path.
  return process.env.SCALP_TOPCRYPTO_ENGINE_URL?.trim().replace(/\/+$/, "") || "http://13.233.8.80:8097";
}

function apiToken(): string {
  return (
    process.env.SCALP_HIGHVOL_API_TOKEN?.trim() ||
    process.env.SCALP_API_TOKEN?.trim() ||
    process.env.BTC_PRE_LIVE_API_TOKEN?.trim() ||
    ""
  );
}

type RouteCtx = { params: Promise<{ path: string[] }> };

export async function GET(req: NextRequest, ctx: RouteCtx): Promise<NextResponse> {
  const { path } = await ctx.params;
  const pathname = path?.length ? `/${path.join("/")}` : "/";

  if (!isAllowed(pathname)) {
    return NextResponse.json(
      { ok: false, error: `${pathname} is not available via the high-volume proxy` },
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
    const token = apiToken();
    if (token) headers.set("X-API-Token", token);

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
    // Report the reach failure as a failure. An empty payload here renders as a
    // flat, healthy-looking desk while the engine is down.
    const message = e instanceof Error ? e.message : "proxy failed";
    return NextResponse.json({ ok: false, error: message }, { status: 502 });
  }
}

export async function POST(req: NextRequest, ctx: RouteCtx): Promise<NextResponse> {
  const { path } = await ctx.params;
  const pathname = path?.length ? `/${path.join("/")}` : "/";

  if (!isMutation(pathname)) {
    return NextResponse.json(
      { ok: false, error: `${pathname} is not a POST endpoint on the high-volume proxy` },
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
    const token = apiToken();
    if (token) headers.set("X-API-Token", token);
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
