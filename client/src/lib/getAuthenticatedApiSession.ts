/**
 * Server-side session guard for API route handlers.
 *
 * Usage in every protected route handler:
 *   const auth = await getAuthenticatedApiSession();
 *   if (!auth.ok) return auth.response;
 *   const accountKey = auth.ctx.userId;   // always "owner_admin"
 *
 * The account identity is NEVER read from request body, query params, or URL.
 * It is derived solely from the signed httpOnly JWT cookie.
 */

import { cookies } from "next/headers";
import { NextResponse } from "next/server";
import { verifySession, SESSION_COOKIE } from "@/lib/jwtSession";
import type { SessionPayload } from "@/lib/jwtSession";

type AuthOk = { ok: true; ctx: SessionPayload };
type AuthFail = { ok: false; response: NextResponse };

export async function getAuthenticatedApiSession(): Promise<AuthOk | AuthFail> {
  const cookieStore = await cookies();
  const token = cookieStore.get(SESSION_COOKIE)?.value;

  if (!token) {
    return {
      ok: false,
      response: NextResponse.json(
        { ok: false, code: "UNAUTHORIZED", error: "Authentication required" },
        { status: 401 },
      ),
    };
  }

  const session = verifySession(token);
  if (!session) {
    return {
      ok: false,
      response: NextResponse.json(
        { ok: false, code: "UNAUTHORIZED", error: "Invalid or expired session" },
        { status: 401 },
      ),
    };
  }

  return { ok: true, ctx: session };
}
