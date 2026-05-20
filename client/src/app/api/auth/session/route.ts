import { cookies } from "next/headers";
import { NextResponse } from "next/server";
import { verifySession, SESSION_COOKIE, isMongoAuthConfigured } from "@/lib/jwtSession";

export const dynamic = "force-dynamic";

export async function GET() {
  if (!isMongoAuthConfigured()) {
    return NextResponse.json({ ok: false, error: "Auth not configured" }, { status: 503 });
  }

  const cookieStore = await cookies();
  const token = cookieStore.get(SESSION_COOKIE)?.value;
  if (!token) {
    return NextResponse.json({ ok: true, user: null });
  }

  const session = verifySession(token);
  if (!session) {
    return NextResponse.json({ ok: true, user: null });
  }

  return NextResponse.json({ ok: true, user: { id: session.userId, email: session.email } });
}
