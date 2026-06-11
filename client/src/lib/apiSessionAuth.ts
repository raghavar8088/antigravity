import { cookies } from "next/headers";
import { NextResponse } from "next/server";
import { verifySession, SESSION_COOKIE, isMongoAuthConfigured } from "@/lib/jwtSession";
import { OWNER_ACCOUNT_KEY } from "@/lib/ownerAuth";

export type ApiSessionContext = {
  userId: string;
};

export async function getAuthenticatedApiSession(): Promise<
  | { ok: true; ctx: ApiSessionContext }
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

  return { ok: true, ctx: { userId: OWNER_ACCOUNT_KEY } };
}
