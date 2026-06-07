/**
 * /api/system/production-validation — automated production alignment checks.
 *
 * Returns PASS / WARNING / FAIL per subsystem with exact diagnostics.
 * Auth: requires valid raig_session cookie (same as /api/system/health).
 */

import { NextResponse } from "next/server";
import { getAuthenticatedApiSession } from "@/lib/getAuthenticatedApiSession";
import { runProductionValidation } from "@/lib/productionValidation";

export const dynamic = "force-dynamic";

export async function GET() {
  const auth = await getAuthenticatedApiSession();
  if (!auth.ok) return auth.response;

  const report = await runProductionValidation(auth.ctx.userId);
  const status = report.overall === "FAIL" ? 503 : report.overall === "WARNING" ? 206 : 200;
  return NextResponse.json(report, { status });
}
