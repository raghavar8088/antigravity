/**
 * POST /api/paper-state/repair
 *
 * Resets the paper account to a clean starting state while preserving
 * operator-managed settings. Emits a `paper_state_repaired` worker event
 * so the audit trail reflects the action.
 *
 * Body:
 *   accountKey: string         (required)
 *   initialBalance?: number    (default: 1000)
 *   reason?: string            (optional operator note, stored in event)
 *   clearDisabled?: boolean    (default: false — set true to also clear disabled strategies)
 *
 * What it resets:
 *   - balance → initialBalance
 *   - positions → []
 *   - cleared_at → Date.now()   ← worker detects this and also resets local state
 *   - pause_entries → false
 *   - last_trade_at → 0
 *
 * What it preserves (by default):
 *   - disabled_strategies  (operator blocklist survives repair unless clearDisabled=true)
 *   - worker_id / worker_last_poll_at / worker_owner  (active worker keeps its lease)
 *   - historical trade documents (not deleted — excluded from metrics by cleared_at)
 */

import { NextResponse } from "next/server";
import { isMongoConfigured, getAccountState, upsertAccountState, insertWorkerEvent } from "@/lib/mongoTradesClient";
import { isWorkerHeartbeatFresh } from "@/lib/paperDeskWorker/workerLease";
import { randomUUID } from "crypto";

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
  const reason = typeof b.reason === "string" ? b.reason.slice(0, 200).trim() : "operator repair";
  const clearDisabled = b.clearDisabled === true;

  // Read current state to preserve operator-managed fields
  const current = await getAccountState(accountKey);
  const workerWasFresh = isWorkerHeartbeatFresh(current?.worker_last_poll_at ?? null);
  const preservedDisabled = clearDisabled ? [] : (current?.disabled_strategies ?? []);

  const clearedAt = Date.now();
  const repairId = randomUUID().slice(0, 8);

  await upsertAccountState({
    account_key: accountKey,
    balance: initialBalance,
    positions: [],
    pause_entries: false,
    disabled_strategies: preservedDisabled,
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

  // Emit audit event (non-fatal if Mongo write fails)
  await insertWorkerEvent({
    account_key: accountKey,
    type: "paper_state_repaired",
    severity: "info",
    message: `Paper state repaired: balance reset to $${initialBalance}, positions cleared. Reason: ${reason}`,
    payload: {
      repairId,
      initialBalance,
      clearedAt,
      workerWasFresh,
      preservedDisabledCount: preservedDisabled.length,
      clearDisabled,
    },
  });

  return NextResponse.json({
    ok: true,
    repairId,
    clearedAt,
    balance: initialBalance,
    preservedDisabledCount: preservedDisabled.length,
    workerWasFresh,
    message: "Paper state repaired — old trades retained but excluded from current session. Worker will pick this up within one tick.",
  });
}
