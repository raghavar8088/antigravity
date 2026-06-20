import crypto from "node:crypto";
import { NextResponse, type NextRequest } from "next/server";
import {
  getAdminCredentials,
  verifyPassword,
  OWNER_ACCOUNT_KEY,
} from "@/lib/broker/ownerAuth";
import { signSession, SESSION_COOKIE } from "@/lib/broker/jwtSession";

// ── In-process rate limit: 5 attempts per 15 minutes per IP ──────────────────
// Per-pod; distributed enforcement is handled by middleware Redis tier.
const loginAttempts = new Map<string, { count: number; resetAt: number }>();
const RL_MAX = 5;
const RL_WINDOW_MS = 15 * 60 * 1000; // 15 minutes

function checkLoginRateLimit(ip: string): { allowed: boolean; retryAfterSecs: number } {
  const now = Date.now();
  const entry = loginAttempts.get(ip);
  if (!entry || now > entry.resetAt) {
    loginAttempts.set(ip, { count: 1, resetAt: now + RL_WINDOW_MS });
    return { allowed: true, retryAfterSecs: 0 };
  }
  entry.count++;
  if (entry.count > RL_MAX) {
    return { allowed: false, retryAfterSecs: Math.ceil((entry.resetAt - now) / 1000) };
  }
  return { allowed: true, retryAfterSecs: 0 };
}

function clearRateLimit(ip: string): void {
  loginAttempts.delete(ip);
}

// ── Handler ───────────────────────────────────────────────────────────────────

export async function POST(req: NextRequest) {
  // Verify both the credential store AND the JWT secret are ready before
  // accepting any request — prevents a half-configured state where credentials
  // are present but signSession() would throw because AUTH_JWT_SECRET is missing.
  const creds = getAdminCredentials();
  if (!creds) {
    return NextResponse.json(
      {
        ok: false,
        code: "AUTH_UNCONFIGURED",
        error: "Auth not configured — set ADMIN_USERNAME + ADMIN_PASSWORD_HASH",
        hint: "Run: node client/scripts/generate-password-hash.mjs <password> and add the three auth env vars to .env.local and Vercel.",
      },
      { status: 503 },
    );
  }

  if (!process.env.AUTH_JWT_SECRET?.trim()) {
    return NextResponse.json(
      {
        ok: false,
        code: "AUTH_UNCONFIGURED",
        error: "Auth not configured — AUTH_JWT_SECRET is missing",
        hint: "Generate: node -e \"console.log(require('crypto').randomBytes(32).toString('hex'))\" and set AUTH_JWT_SECRET in .env.local and Vercel.",
      },
      { status: 503 },
    );
  }

  const ip =
    req.headers.get("x-forwarded-for")?.split(",")[0]?.trim() ??
    req.headers.get("x-real-ip") ??
    "unknown";

  const rl = checkLoginRateLimit(ip);
  if (!rl.allowed) {
    return NextResponse.json(
      { ok: false, error: "Too many login attempts — try again later" },
      {
        status: 429,
        headers: { "Retry-After": String(rl.retryAfterSecs) },
      },
    );
  }

  let body: { username?: unknown; password?: unknown };
  try {
    body = (await req.json()) as { username?: unknown; password?: unknown };
  } catch {
    return NextResponse.json({ ok: false, error: "Invalid request body" }, { status: 400 });
  }

  const username = typeof body.username === "string" ? body.username.trim() : "";
  const password = typeof body.password === "string" ? body.password : "";

  if (!username || !password) {
    return NextResponse.json(
      { ok: false, error: "Username and password are required" },
      { status: 400 },
    );
  }

  // Constant-time username comparison to prevent timing leaks
  const usernameMatch = crypto_safeEqual(username, creds.username);
  const passwordMatch = await verifyPassword(password, creds.passwordHash);

  if (!usernameMatch || !passwordMatch) {
    return NextResponse.json({ ok: false, error: "Invalid credentials" }, { status: 401 });
  }

  // Successful login — clear rate limit counter and issue session
  clearRateLimit(ip);

  // The only configured login is the operator/admin account — grant ADMIN role
  // so admin-only engine endpoints (e.g. POST /api/admin/reset) are reachable
  // from the dashboard. Previously this always defaulted to "TRADER" (signSession's
  // default), making admin-gated engine routes unreachable from any UI session.
  const token = signSession(OWNER_ACCOUNT_KEY, creds.username, "ADMIN");
  const res = NextResponse.json({
    ok: true,
    user: { id: OWNER_ACCOUNT_KEY, username: creds.username },
  });
  res.cookies.set(SESSION_COOKIE, token, {
    httpOnly: true,
    sameSite: "lax",
    path: "/",
    maxAge: 24 * 60 * 60, // 24 hours — matches jwtSession EXPIRY_SECS
    secure: process.env.NODE_ENV === "production",
  });
  return res;
}

// Timing-safe string comparison — pads shorter buffer to prevent length leak
function crypto_safeEqual(a: string, b: string): boolean {
  const aBuf = Buffer.from(a, "utf8");
  const bBuf = Buffer.from(b, "utf8");
  const len = Math.max(aBuf.length, bBuf.length);
  const aPad = Buffer.alloc(len);
  const bPad = Buffer.alloc(len);
  aBuf.copy(aPad);
  bBuf.copy(bPad);
  const equal = crypto.timingSafeEqual(aPad, bPad);
  // Must also check original lengths to avoid "abc" == "abcXX" passing
  return equal && aBuf.length === bBuf.length;
}
