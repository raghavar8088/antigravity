/**
 * POST /api/paper-state/repair
 *
 * Resets the paper account to a clean starting state while preserving
 * operator-managed settings (disabled strategies, worker lease fields).
 *
 * Body: { accountKey: string, initialBalance?: number }
 *
 * What it resets:
 *   - balance → initialBalance (default 1000)
 *   - positions → []
 *   - cleared_at → Date.now()   ← worker detects this and also resets local state
 *   - pause_entries → false
 *
 * What it preserves:
 *   - disabled_strategies  (operator blocklist survives repair)
 *   - worker_id / worker_last_poll_at / worker_owner  (preserve if worker is fresh)
 *   - historical trade documents (not deleted — excluded from metrics by cleared_at)
 */

import { NextResponse } from "next/server";
import { isMongoConfigured, getAccountState, upsertAccountState } from "@/lib/mongoTradesClient";

export const dynamic = "force-dynamic";

export async function POST(req: Request) {
  let body: unknown;
  try {
    body = await req.json();
  } catch {
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

  const initialBalance =
    typeof b.initialBalance === "number" && b.initialBalance > 0 ? b.initialBalance : 1000;

  // Read current state to preserve operator-managed fields
  const current = await getAccountState(accountKey);

  const clearedAt = Date.now();

  await upsertAccountState({
    account_key: accountKey,
    balance: initialBalance,
    positions: [],
    pause_entries: false,
    disabled_strategies: current?.disabled_strategies ?? [],
    last_trade_at: 0,
    day_start_balance: initialBalance,
    day_start_date: clearedAt,
    cleared_at: clearedAt,
    updated_at: new Date().toISOString(),
    // Preserve worker lease fields so an active VPS worker keeps its lease
    worker_id: current?.worker_id ?? null,
    worker_last_poll_at: current?.worker_last_poll_at ?? null,
    worker_owner: current?.worker_owner ?? null,
  });

  return NextResponse.json({
    ok: true,
    clearedAt,
    initialBalance,
    message: "Paper state repaired — old trades retained but excluded from current session.",
  });
}
