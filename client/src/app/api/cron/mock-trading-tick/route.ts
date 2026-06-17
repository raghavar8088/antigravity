/**
 * GET /api/cron/mock-trading-tick
 *
 * Headless mock-trading execution cycle. Protected by CRON_SECRET.
 * Vercel Hobby allows max 2 crons — schedule is every minute (PM2 worker
 * on Lightsail should run every 5s for production latency).
 */

import { NextResponse } from "next/server";
import { runMockTradingExecutorCycle } from "@/lib/mockTradingExecutor/runMockTradingExecutorCycle";
import { DEFAULT_MOCK_ACCOUNT_KEY } from "@/lib/trading/mockTradingPersistenceTypes";

export const dynamic = "force-dynamic";
export const maxDuration = 30;

function unauthorized() {
  return NextResponse.json({ ok: false, error: "Unauthorized" }, { status: 401 });
}

export async function GET(req: Request): Promise<NextResponse> {
  const secret = process.env.CRON_SECRET?.trim();
  if (!secret) {
    return NextResponse.json(
      { ok: false, error: "CRON_SECRET env var is not configured — cron endpoint is locked" },
      { status: 503 },
    );
  }
  const auth = req.headers.get("authorization") ?? "";
  if (auth !== `Bearer ${secret}`) return unauthorized();

  const url = new URL(req.url);
  const accountKey = url.searchParams.get("account_key")?.trim() || DEFAULT_MOCK_ACCOUNT_KEY;

  try {
    const result = await runMockTradingExecutorCycle({
      accountKey,
      mode: "cron",
    });
    return NextResponse.json({
      ok: result.ok,
      account_key: result.accountKey,
      opened: result.opened.length,
      closed: result.closed.length,
      updated_open: result.updatedOpen,
      candidate_count: result.diagnostics.candidateCount,
      dominant_blocker: result.diagnostics.dominantBlocker,
      regime: result.diagnostics.regime,
      mark_price: result.diagnostics.markPrice,
      duration_ms: result.diagnostics.durationMs,
      errors: result.diagnostics.errors,
      diagnostics: result.diagnostics,
    });
  } catch (err) {
    const message = err instanceof Error ? err.message : "executor cycle failed";
    return NextResponse.json({ ok: false, error: message }, { status: 500 });
  }
}
