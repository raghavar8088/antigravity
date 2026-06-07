import { cookies } from "next/headers";
import { NextResponse } from "next/server";
import { verifySession, SESSION_COOKIE } from "@/lib/jwtSession";

export const dynamic = "force-dynamic";

export async function GET() {
  // Session verification only requires AUTH_JWT_SECRET — not MONGODB_URI.
  // isMongoAuthConfigured() was incorrectly gating this endpoint on MongoDB.
  if (!process.env.AUTH_JWT_SECRET?.trim()) {
    return NextResponse.json(
      {
        ok: false,
        code: "AUTH_UNCONFIGURED",
        error: "Auth not configured — AUTH_JWT_SECRET is missing",
      },
      { status: 503 },
    );
  }

  const cookieStore = await cookies();
  const token = cookieStore.get(SESSION_COOKIE)?.value;
  if (!token) {
    return NextResponse.json({ ok: true, user: null });
  }

  let session: ReturnType<typeof verifySession>;
  try {
    session = verifySession(token);
  } catch {
    session = null;
  }

  if (!session) {
    return NextResponse.json({ ok: true, user: null });
  }

  return NextResponse.json({ ok: true, user: { id: session.userId, email: session.email } });
}
