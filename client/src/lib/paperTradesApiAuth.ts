import { cookies } from "next/headers";
import { NextResponse } from "next/server";
import { requireAuthenticatedPaperUser } from "@/lib/paperTradesAuth";
import { verifySession, SESSION_COOKIE, isMongoAuthConfigured } from "@/lib/jwtSession";

export type AuthenticatedPaperApiContext = {
  userId: string;
};

export async function getAuthenticatedPaperApiUser(): Promise<
  | { ok: true; ctx: AuthenticatedPaperApiContext }
  | { ok: false; response: NextResponse }
> {
  if (!isMongoAuthConfigured()) {
    return {
      ok: false,
      response: NextResponse.json(
        { ok: false, error: "Auth not configured (MONGODB_URI + AUTH_JWT_SECRET)" },
        { status: 503 },
      ),
    };
  }

  const cookieStore = await cookies();
  const token = cookieStore.get(SESSION_COOKIE)?.value;
  const session = token ? verifySession(token) : null;

  if (!session?.userId) {
    return {
      ok: false,
      response: NextResponse.json({ ok: false, error: "Unauthorized" }, { status: 401 }),
    };
  }

  const guard = requireAuthenticatedPaperUser(session.userId);
  if (!guard.ok) {
    return {
      ok: false,
      response: NextResponse.json({ ok: false, error: guard.error }, { status: guard.status }),
    };
  }

  return { ok: true, ctx: { userId: guard.userId } };
}
