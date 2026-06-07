import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";

export const dynamic = "force-dynamic";

/**
 * Proxies the Go engine's /api/paper-desk/diagnostics endpoint so the browser
 * can call it through Vercel (avoiding CORS).  Requires valid session.
 *
 * Also includes a local-side check: MONGODB_URI presence and AUTH config.
 */
export async function GET() {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;

  const engineBase =
    process.env.INTERNAL_API_URL?.trim().replace(/\/$/, "") ??
    "http://localhost:8080";

  // Local config checks surfaced alongside engine diagnostics.
  const localConfig = {
    mongodb_uri_set: typeof process.env.MONGODB_URI === "string" && process.env.MONGODB_URI.trim().startsWith("mongodb"),
    mongodb_db: process.env.MONGODB_DB?.trim() || process.env.MONGODB_DB_NAME?.trim() || "loop_trades (default)",
    auth_jwt_secret_set: typeof process.env.AUTH_JWT_SECRET === "string" && process.env.AUTH_JWT_SECRET.trim().length > 0,
    admin_username_set: typeof process.env.ADMIN_USERNAME === "string" && process.env.ADMIN_USERNAME.trim().length > 0,
    admin_password_hash_set: typeof process.env.ADMIN_PASSWORD_HASH === "string" && process.env.ADMIN_PASSWORD_HASH.trim().length > 0,
    internal_api_url: engineBase,
    checked_at: new Date().toISOString(),
  };

  // Forward to Go engine diagnostics.
  try {
    const res = await fetch(`${engineBase}/api/paper-desk/diagnostics`, {
      signal: AbortSignal.timeout(8_000),
    });
    const engineDiag = await res.json();
    return NextResponse.json({ ok: true, local_config: localConfig, engine: engineDiag });
  } catch (err) {
    return NextResponse.json({
      ok: false,
      local_config: localConfig,
      engine: null,
      engine_error: err instanceof Error ? err.message : "fetch failed",
    }, { status: 502 });
  }
}
