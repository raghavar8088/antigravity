import { NextResponse, type NextRequest } from "next/server";

// ─────────────────────────────────────────────────────────────────────────────
// Institutional Security Middleware — Phase 15J
//
// Responsibilities:
//   1. Security response headers (CSP, HSTS, X-Frame, etc.)
//   2. Edge-level rate limiting (in-memory per-pod; Redis tier handles distributed)
//   3. Auth guard — redirect unauthenticated browsers away from protected pages
// ─────────────────────────────────────────────────────────────────────────────

// ── Rate limiting (edge, per-pod) ────────────────────────────────────────────
// Key: "<ip>:<route-class>", Value: { count, resetAt }
// Cleared per cold-start; distributed enforcement is handled at engine level.
const rlMap = new Map<string, { count: number; resetAt: number }>();

const RL_POLICIES: Record<string, { max: number; windowMs: number }> = {
  auth:   { max: 10,  windowMs: 60_000 },   // 10 req/min on sign-in
  admin:  { max: 20,  windowMs: 60_000 },   // 20 req/min on admin ops
  trade:  { max: 100, windowMs: 60_000 },   // 100 req/min on trading ops
  api:    { max: 200, windowMs: 60_000 },   // 200 req/min general API
  global: { max: 500, windowMs: 60_000 },   // 500 req/min everything else
};

function routeClass(pathname: string): string {
  if (pathname.startsWith("/api/auth")) return "auth";
  if (pathname.startsWith("/api/admin") || pathname.startsWith("/api/security")) return "admin";
  if (
    pathname.startsWith("/api/paper-") ||
    pathname.startsWith("/api/delta-live/order") ||
    pathname.startsWith("/api/mock-trading") ||
    pathname.startsWith("/api/angel")
  ) return "trade";
  if (pathname.startsWith("/api/")) return "api";
  return "global";
}

function checkRateLimit(ip: string, pathname: string): boolean {
  const cls = routeClass(pathname);
  const policy = RL_POLICIES[cls];
  const key = `${ip}:${cls}`;
  const now = Date.now();

  const entry = rlMap.get(key);
  if (!entry || now > entry.resetAt) {
    rlMap.set(key, { count: 1, resetAt: now + policy.windowMs });
    return true; // allowed
  }
  entry.count++;
  return entry.count <= policy.max;
}

// Clean stale entries periodically (simple GC on every 1000th request).
let gcCounter = 0;
function maybeGC() {
  if (++gcCounter % 1000 !== 0) return;
  const now = Date.now();
  for (const [k, v] of rlMap) {
    if (now > v.resetAt) rlMap.delete(k);
  }
}

// ── Security headers ──────────────────────────────────────────────────────────
function applySecurityHeaders(res: NextResponse): void {
  const isDev = process.env.NODE_ENV !== "production";

  // Content-Security-Policy — tightened for trading dashboard
  const cspDirectives = [
    "default-src 'self'",
    "script-src 'self' 'unsafe-inline'",   // Next.js requires inline scripts
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data: blob:",
    "connect-src 'self' wss: https:",      // allow WS and HTTPS for market data
    "font-src 'self' data:",
    "frame-ancestors 'none'",
    "form-action 'self'",
    "base-uri 'self'",
    "object-src 'none'",
  ].join("; ");

  res.headers.set("Content-Security-Policy", cspDirectives);
  res.headers.set("X-Frame-Options", "DENY");
  res.headers.set("X-Content-Type-Options", "nosniff");
  res.headers.set("X-XSS-Protection", "1; mode=block");
  res.headers.set("Referrer-Policy", "strict-origin-when-cross-origin");
  res.headers.set("Permissions-Policy", "camera=(), microphone=(), geolocation=()");

  if (!isDev) {
    res.headers.set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload");
  }

  // Remove fingerprinting headers
  res.headers.delete("X-Powered-By");
  res.headers.delete("Server");
}

// ── Protected page paths (browser nav guard) ──────────────────────────────────
// /login is the only public route. Every other page requires a valid raig_session.
const PUBLIC_PATHS = ["/login", "/api/auth/login", "/api/auth/session", "/api/health", "/mock-trading", "/api/mock-trading", "/api/strategy-signal-trace", "/paper-desk", "/paperdesk", "/api/paper-desk"];

function isProtectedPage(pathname: string): boolean {
  // Static assets, Next.js internals, and explicitly public paths are allowed.
  return !PUBLIC_PATHS.some((p) => pathname === p || pathname.startsWith(p + "/"));
}

function hasSessionCookie(req: NextRequest): boolean {
  return !!req.cookies.get("raig_session")?.value;
}

// ── Main middleware ───────────────────────────────────────────────────────────
export function middleware(request: NextRequest): NextResponse {
  const { pathname } = request.nextUrl;
  maybeGC();

  // Extract client IP — Vercel sets x-forwarded-for
  const ip =
    request.headers.get("x-forwarded-for")?.split(",")[0]?.trim() ??
    request.headers.get("x-real-ip") ??
    "unknown";

  // ── Rate limiting ────────────────────────────────────────────────────────
  if (pathname.startsWith("/api/") && !checkRateLimit(ip, pathname)) {
    const res = NextResponse.json(
      { error: "rate limit exceeded", code: 429 },
      { status: 429 }
    );
    res.headers.set("Retry-After", "60");
    applySecurityHeaders(res);
    return res;
  }

  // ── Page auth guard ──────────────────────────────────────────────────────
  if (isProtectedPage(pathname) && !hasSessionCookie(request)) {
    const url = request.nextUrl.clone();
    url.pathname = "/login";
    url.searchParams.set("next", pathname);
    return NextResponse.redirect(url);
  }

  // ── Pass through with security headers ───────────────────────────────────
  const res = NextResponse.next();
  applySecurityHeaders(res);
  return res;
}

export const config = {
  matcher: [
    "/((?!_next/static|_next/image|favicon.ico|.*\\.(?:svg|png|jpg|jpeg|gif|webp)$).*)",
  ],
};
