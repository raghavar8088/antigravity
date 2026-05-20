import { NextResponse, type NextRequest } from "next/server";
import { findOrCreateAuthUser } from "@/lib/mongoAuthClient";
import { signSession, SESSION_COOKIE, isMongoAuthConfigured } from "@/lib/jwtSession";

export async function POST(req: NextRequest) {
  if (!isMongoAuthConfigured()) {
    return NextResponse.json({ ok: false, error: "Auth not configured" }, { status: 503 });
  }

  let email: string | undefined;
  try {
    const body = (await req.json()) as { email?: unknown };
    email = typeof body.email === "string" ? body.email.trim() : undefined;
  } catch {
    return NextResponse.json({ ok: false, error: "Invalid request body" }, { status: 400 });
  }

  if (!email) {
    return NextResponse.json({ ok: false, error: "Email required" }, { status: 400 });
  }

  try {
    const { userId, email: normalized } = await findOrCreateAuthUser(email);
    const token = signSession(userId, normalized);
    const res = NextResponse.json({ ok: true, user: { id: userId, email: normalized } });
    res.cookies.set(SESSION_COOKIE, token, {
      httpOnly: true,
      sameSite: "lax",
      path: "/",
      maxAge: 30 * 24 * 60 * 60,
      secure: process.env.NODE_ENV === "production",
    });
    return res;
  } catch (err) {
    const message = err instanceof Error ? err.message : "Sign-in failed";
    return NextResponse.json({ ok: false, error: message }, { status: 500 });
  }
}
