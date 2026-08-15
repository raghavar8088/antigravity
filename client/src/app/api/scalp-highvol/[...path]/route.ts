/**
 * High Volume Crypto Trading proxy — the scalp engine on the majors only.
 *
 * Forwards to a THIRD scalp_prelive container (port 8096), running the same
 * binary and the same strategies as the Crypto Scalp Desk on 8094, but with an
 * explicit `-symbols` list instead of `-symbols auto`.
 *
 * A separate process rather than a filter on the existing desk. The scalp desk
 * discovers its universe from Delta above a turnover floor and currently runs
 * ~220 perpetuals; filtering its output down to fourteen in the browser would
 * show the same streams competing against 200-odd others for the same fill
 * slots and call the result "high volume". It would not be. Concurrency limits,
 * pending-order queues and the paper books are all per-process, so the only way
 * to learn what these strategies do when they are only allowed to trade liquid
 * majors is to run them in a process that only has liquid majors.
 *
 * The scalp desk is untouched by this file, deliberately: it is the control
 * arm. Two processes, identical code, different universes — that difference is
 * the whole experiment, and it stops meaning anything the moment either side is
 * tuned to make the comparison look better.
 *
 * SCALP_HIGHVOL_ENGINE_URL overrides the target (default: Lightsail :8096).
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
 * Reads. Identical set to the scalp proxy, because the page is a clone of the
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
 * The arm/disarm and per-stream switch paths that the scalp proxy carries are
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
  return process.env.SCALP_HIGHVOL_ENGINE_URL?.trim().replace(/\/+$/, "") || "http://13.233.8.80:8096";
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
