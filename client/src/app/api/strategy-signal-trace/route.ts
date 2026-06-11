/**
 * GET /api/strategy-signal-trace
 *
 * Returns the latest per-strategy signal trace snapshot stored in paper_state.
 * Written by worker or browser each tick; overwritten each time (no history stored).
 *
 * Query params (all optional):
 *   account_key  — fallback to DESK_WORKER_ACCOUNT_KEY
 *   gate         — filter to rows matching this gate
 *   status       — filter to rows matching this status
 *   strategy_id  — filter to a specific strategy ID (number)
 *   limit        — max rows to return (default 200, max 500)
 */

import { NextResponse, type NextRequest } from "next/server";
import { OWNER_ACCOUNT_KEY } from "@/lib/ownerAuth";
import { getAccountState, isMongoConfigured } from "@/lib/mongoTradesClient";
import type { StrategySignalTraceRow, SignalTraceSummary } from "@/lib/strategySignalTrace";
import { summarizeSignalTrace } from "@/lib/strategySignalTrace";

export const dynamic = "force-dynamic";
export const maxDuration = 10;

type LatestTrace = {
  tickAt: number;
  mode: "browser" | "worker" | "cron";
  symbol: string;
  summary: SignalTraceSummary;
  rows: StrategySignalTraceRow[];
};

/** Browser writes its latest trace snapshot via POST. */
export async function POST(request: NextRequest) {
  let body: unknown;
  try { body = await request.json(); } catch {
    return NextResponse.json({ ok: false, error: "Invalid JSON" }, { status: 400 });
  }
  const b = body as Record<string, unknown>;
  const accountKey = typeof b.accountKey === "string" ? b.accountKey.trim() : null;
  if (!accountKey) {
    return NextResponse.json({ ok: false, error: "accountKey required" }, { status: 400 });
  }
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }

  const { upsertSignalTrace } = await import("@/lib/mongoTradesClient");
  await upsertSignalTrace(accountKey, b.trace as Record<string, unknown>);
  return NextResponse.json({ ok: true });
}

export async function GET(request: NextRequest) {
  const url = new URL(request.url);

  const accountKey =
    url.searchParams.get("account_key")?.trim() ||
    process.env.OWNER_ACCOUNT_KEY?.trim() ||
    process.env.DESK_WORKER_ACCOUNT_KEY?.trim() ||
    OWNER_ACCOUNT_KEY;

  if (!accountKey) {
    return NextResponse.json({ ok: false, error: "account_key required" }, { status: 400 });
  }
  if (!isMongoConfigured()) {
    return NextResponse.json({ ok: false, error: "MongoDB not configured" }, { status: 503 });
  }

  const state = await getAccountState(accountKey);
  const raw = state?.signal_trace_latest as LatestTrace | undefined | null;

  if (!raw || !raw.rows) {
    return NextResponse.json({
      ok: true,
      summary: null,
      rows: [],
      message: "No signal trace captured yet",
    });
  }

  const gateFilter = url.searchParams.get("gate")?.toUpperCase();
  const statusFilter = url.searchParams.get("status")?.toUpperCase();
  const stratIdFilter = url.searchParams.get("strategy_id")
    ? Number(url.searchParams.get("strategy_id"))
    : null;
  const limit = Math.min(500, Number(url.searchParams.get("limit") || 200));

  let rows: StrategySignalTraceRow[] = raw.rows;
  if (gateFilter) rows = rows.filter((r) => r.gate === gateFilter);
  if (statusFilter) rows = rows.filter((r) => r.status === statusFilter);
  if (stratIdFilter !== null && !Number.isNaN(stratIdFilter)) {
    rows = rows.filter((r) => r.strategyId === stratIdFilter);
  }
  rows = rows.slice(0, limit);

  const ageSeconds = raw.tickAt
    ? Math.floor((Date.now() - raw.tickAt) / 1000)
    : null;

  return NextResponse.json({
    ok: true,
    ageSeconds,
    mode: raw.mode,
    symbol: raw.symbol,
    summary: gateFilter || statusFilter || stratIdFilter !== null
      ? summarizeSignalTrace(rows)
      : raw.summary,
    rows,
  });
}
