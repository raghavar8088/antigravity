import { NextResponse } from "next/server";

/**
 * Deprecated: email-only sign-in has been replaced by username+password auth.
 * All clients must use POST /api/auth/login.
 *
 * Returns 410 Gone so callers get a clear signal rather than silently failing.
 * Old sessions with UUID account_keys remain valid until they expire (24h), at
 * which point users must log in via /login with their configured credentials.
 */
export async function POST() {
  return NextResponse.json(
    {
      ok: false,
      error:
        "This endpoint has been retired. Use POST /api/auth/login with username and password.",
      login: "/login",
    },
    { status: 410 },
  );
}
