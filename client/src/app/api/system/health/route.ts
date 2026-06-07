/**
 * /api/system/health — Complete system readiness endpoint.
 *
 * Returns a full diagnostic report covering:
 *   • environment variable validation (auth + MongoDB + engine URL)
 *   • live MongoDB connectivity check (ping)
 *   • account_key consistency across frontend and engine
 *   • Go engine reachability and persistence status
 *
 * Authentication: requires valid raig_session cookie.
 * Safe to poll — read-only, no side effects.
 *
 * HTTP 200 — system fully operational
 * HTTP 206 — system partially operational (warnings exist)
 * HTTP 503 — system not operational (critical failures)
 */

import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";
import { runEnvChecks } from "@/lib/envCheck";
import { pingMongo, isMongoConfigured } from "@/lib/mongoTradesClient";
import { OWNER_ACCOUNT_KEY } from "@/lib/ownerAuth";

export const dynamic = "force-dynamic";

export async function GET() {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;

  const envReport = runEnvChecks();

  // ── MongoDB live ping ──────────────────────────────────────────────────────
  let mongoPingOk = false;
  let mongoPingMs = -1;
  let mongoPingError: string | null = null;

  if (isMongoConfigured()) {
    const t0 = Date.now();
    try {
      mongoPingOk = await pingMongo();
      mongoPingMs = Date.now() - t0;
    } catch (err) {
      mongoPingError = err instanceof Error ? err.message : "unknown";
      mongoPingMs = Date.now() - t0;
    }
  } else {
    mongoPingError = "MONGODB_URI not configured";
  }

  // ── Go engine reachability ─────────────────────────────────────────────────
  const engineBase =
    process.env.INTERNAL_API_URL?.trim().replace(/\/$/, "") ??
    "http://localhost:8080";

  let engineOk = false;
  let engineDiag: unknown = null;
  let engineError: string | null = null;
  let enginePingMs = -1;

  try {
    const t0 = Date.now();
    const res = await fetch(`${engineBase}/api/paper-desk/diagnostics`, {
      signal: AbortSignal.timeout(5_000),
    });
    enginePingMs = Date.now() - t0;
    if (res.ok) {
      engineDiag = await res.json();
      engineOk = true;
    } else {
      engineError = `HTTP ${res.status}`;
    }
  } catch (err) {
    engineError = err instanceof Error ? err.message : "fetch failed";
  }

  // ── account_key consistency ────────────────────────────────────────────────
  const frontendAccountKey = OWNER_ACCOUNT_KEY; // "mock_trading_default" hardcoded
  const sessionAccountKey = auth.ctx.userId;     // from JWT, should match
  const engineAccountKey =
    (engineDiag as { account_key?: string } | null)?.account_key ?? null;

  const accountKeyConsistent =
    frontendAccountKey === sessionAccountKey &&
    (engineAccountKey === null || engineAccountKey === frontendAccountKey);

  // ── Overall health ─────────────────────────────────────────────────────────
  const fails = envReport.checks.filter((c) => c.status === "FAIL");
  const warnings = envReport.checks.filter((c) => c.status === "WARNING");
  const systemOk =
    fails.length === 0 && mongoPingOk && accountKeyConsistent;
  const systemPartial =
    !systemOk && fails.length === 0 && (warnings.length > 0 || !engineOk);

  const report = {
    ok: systemOk,
    partial: systemPartial,
    checked_at: new Date().toISOString(),

    auth: {
      ready: envReport.ready_for_login,
      jwt_secret_set: envReport.checks.find((c) => c.key === "AUTH_JWT_SECRET")?.status === "PASS",
      admin_username_set: envReport.checks.find((c) => c.key === "ADMIN_USERNAME")?.status === "PASS",
      admin_password_hash_set: envReport.checks.find((c) => c.key === "ADMIN_PASSWORD_HASH")?.status === "PASS",
      session_account_key: sessionAccountKey,
    },

    mongodb: {
      configured: isMongoConfigured(),
      ping_ok: mongoPingOk,
      ping_ms: mongoPingMs,
      error: mongoPingError,
      db_name: process.env.MONGODB_DB_NAME?.trim() || process.env.MONGODB_DB?.trim() || "loop_trades",
    },

    account_key: {
      frontend_constant: frontendAccountKey,
      session_value: sessionAccountKey,
      engine_value: engineAccountKey,
      consistent: accountKeyConsistent,
      issue: accountKeyConsistent
        ? null
        : `Mismatch: frontend="${frontendAccountKey}" session="${sessionAccountKey}" engine="${engineAccountKey}". Paper Desk will show empty data.`,
    },

    engine: {
      url: engineBase,
      reachable: engineOk,
      ping_ms: enginePingMs,
      error: engineError,
      diagnostics: engineDiag,
    },

    env_checks: envReport.checks,
    blockers: [
      ...fails.map((c) => `ENV FAIL: ${c.key} — ${c.message}`),
      ...(!mongoPingOk && isMongoConfigured() ? [`MongoDB ping failed: ${mongoPingError}`] : []),
      ...(!accountKeyConsistent ? [`account_key mismatch: ${frontendAccountKey} vs ${sessionAccountKey}`] : []),
    ],
    warnings: [
      ...warnings.map((c) => `ENV WARN: ${c.key} — ${c.message}`),
      ...(!engineOk ? [`Go engine unreachable at ${engineBase}: ${engineError}`] : []),
    ],
  };

  const status = systemOk ? 200 : systemPartial ? 206 : 503;
  return NextResponse.json(report, { status });
}
