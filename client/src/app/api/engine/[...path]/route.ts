/**
 * Engine Proxy — Phase 15J+ Hardened
 *
 * Security model:
 *   - ALL requests require a valid session cookie (JWT verified)
 *   - Only allowlisted paths may be proxied
 *   - Admin/mutating paths additionally require X-Engine-Admin-Secret header
 *   - Dangerous paths (inject-candles, reset, seed) are blocked entirely from Vercel
 *   - The upstream host is env-fixed — no caller-controlled SSRF
 *   - Service identity header is added for engine-side audit logging
 *
 * Allowlist tiers:
 *   READ_PATHS    — session required, safe reads
 *   ADMIN_PATHS   — session + ENGINE_ADMIN_SECRET required
 *   BLOCKED_PATHS — always 403, must be called directly against engine
 */

import { NextRequest, NextResponse } from "next/server";
import { cookies } from "next/headers";

// ── JWT verification (Web Crypto — edge-safe) ─────────────────────────────────
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
    // Constant-time compare
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

// ── Path policy ────────────────────────────────────────────────────────────────

// Paths that may be read via Vercel with a valid session only.
const READ_PATH_PREFIXES = [
  // Paper desk (legacy path names that some UIs still use)
  "/api/paper-desk/state",
  "/api/paper-desk/positions",
  "/api/paper-desk/trades",
  "/api/paper-desk/diagnostics",
  "/api/paper-desk/metrics",
  "/api/paper-desk/strategy-stats",
  "/api/paper-desk/account-snapshot",
  // Direct engine REST endpoints (canonical paths served by the Go engine)
  "/api/positions",
  "/api/trades",
  "/api/stats",
  "/api/strategies",
  "/api/logs",
  "/api/regime",
  "/api/scalers/stats",
  "/api/ai/insights",
  "/api/health/mock-trading",
  "/api/security/status",
  "/api/security/audit",
  "/api/security/incidents",
  // Options
  "/api/options/positions",
  "/api/options/trades",
  "/api/options/stats",
  // Delta live (stats only — orders are admin-tier)
  "/api/delta-live/stats",
  // Kill switch status
  "/api/admin/ks/status",
  // Trade Threshold Configuration — history is read-only at the engine level
  // (no mutation endpoint at this path), safe to read with session only.
  "/api/engine/config/history",
  // Health / metrics
  "/health",
  "/ready",
  "/metrics",
];

// Paths that require session + ENGINE_ADMIN_SECRET to proxy via Vercel.
const ADMIN_PATH_PREFIXES = [
  "/api/admin/ks/block",
  "/api/admin/ks/release",
  "/api/admin/kill",
  "/api/admin/reset-stats",
  "/api/paper/oms",
  "/api/paper-desk/oms",
];

// Paths that are NEVER proxied via Vercel — must be called directly on engine.
const BLOCKED_PATH_PREFIXES = [
  "/api/admin/clear-history",
  "/api/admin/migrate",
  "/api/debug",
];

type PathTier = "read" | "admin" | "blocked" | "denied";

function classifyPath(pathname: string): PathTier {
  const clean = pathname.replace(/\/+$/, "").toLowerCase();
  for (const p of BLOCKED_PATH_PREFIXES) {
    if (clean === p || clean.startsWith(p + "/") || clean.startsWith(p + "?")) return "blocked";
  }
  for (const p of ADMIN_PATH_PREFIXES) {
    if (clean === p || clean.startsWith(p + "/") || clean.startsWith(p + "?")) return "admin";
  }
  for (const p of READ_PATH_PREFIXES) {
    if (clean === p || clean.startsWith(p + "/") || clean.startsWith(p + "?")) return "read";
  }
  return "denied";
}

// ── Upstream resolution ────────────────────────────────────────────────────────
function upstreamBase(): string {
  const raw =
    process.env.INTERNAL_API_URL?.trim() ||
    process.env.ENGINE_URL?.trim() ||
    "http://127.0.0.1:8080";
  return raw.replace(/\/+$/, "");
}

// ── Proxy core ─────────────────────────────────────────────────────────────────
async function proxyToEngine(
  req: NextRequest,
  segments: string[],
  forwardAdminSecret: boolean,
): Promise<Response> {
  const path = segments.length ? `/${segments.join("/")}` : "/";
  const target = `${upstreamBase()}${path}${req.nextUrl.search}`;

  const headers = new Headers();
  const ct = req.headers.get("content-type");
  if (ct) headers.set("content-type", ct);
  // Service identity for engine audit log
  headers.set("X-Service-Name", "vercel-proxy");
  headers.set("X-Service-Timestamp", String(Date.now()));
  if (forwardAdminSecret && process.env.ENGINE_ADMIN_SECRET) {
    headers.set("X-Engine-Admin-Secret", process.env.ENGINE_ADMIN_SECRET);
  }

  const init: RequestInit = {
    method: req.method,
    headers,
    cache: "no-store",
    signal: AbortSignal.timeout(60_000),
  };
  if (req.method !== "GET" && req.method !== "HEAD") {
    init.body = await req.arrayBuffer();
  }
  return fetch(target, init);
}

function deny(status: number, code: string, message: string): NextResponse {
  return NextResponse.json({ ok: false, code, error: message }, { status });
}

function passthrough(res: Response): NextResponse {
  const out = new Headers(res.headers);
  out.delete("transfer-encoding");
  return new NextResponse(res.body, { status: res.status, headers: out });
}

// ── Route handler ──────────────────────────────────────────────────────────────
type RouteCtx = { params: Promise<{ path: string[] }> };

async function handle(req: NextRequest, ctx: RouteCtx): Promise<NextResponse> {
  const { path } = await ctx.params;
  const pathname = path?.length ? `/${path.join("/")}` : "/";
  const tier = classifyPath(pathname);

  // 1. Always deny blocked paths from Vercel.
  if (tier === "blocked") {
    return deny(403, "BLOCKED", `${pathname} must be called directly on the engine`);
  }

  // 2. Paths not in any allowlist are denied — defence-in-depth against new engine
  //    routes inadvertently becoming public.
  if (tier === "denied") {
    return deny(403, "NOT_PROXIED", `${pathname} is not available via the Vercel proxy`);
  }

  // 3. Validate session for all proxied paths.
  const cookieStore = await cookies();
  const token = cookieStore.get("raig_session")?.value ?? "";
  const sessionValid = await verifySessionToken(token);
  if (!sessionValid) {
    return deny(401, "UNAUTHORIZED", "Valid session required");
  }

  // 4. Admin paths additionally require the caller to supply ENGINE_ADMIN_SECRET.
  //    The secret is compared server-side; it is not readable by the client.
  if (tier === "admin") {
    const supplied = req.headers.get("x-engine-admin-secret") ?? "";
    const expected = process.env.ENGINE_ADMIN_SECRET ?? "";
    if (!expected) {
      return deny(503, "UNCONFIGURED", "ENGINE_ADMIN_SECRET is not configured");
    }
    // Constant-time compare
    if (supplied.length !== expected.length) {
      return deny(403, "FORBIDDEN", "Admin secret required for this operation");
    }
    let diff = 0;
    for (let i = 0; i < supplied.length; i++) diff |= supplied.charCodeAt(i) ^ expected.charCodeAt(i);
    if (diff !== 0) {
      return deny(403, "FORBIDDEN", "Admin secret required for this operation");
    }
  }

  // 5. Proxy to engine.
  try {
    const upstream = await proxyToEngine(req, path ?? [], tier === "admin");
    return passthrough(upstream);
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
