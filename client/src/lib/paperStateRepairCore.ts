/**
 * Shared core for the paper-state repair operation.
 *
 * Used by:
 *   - POST /api/paper-state/repair (operator-initiated)
 *   - deskSelfHealingExecutor.ts   (env-gated automation)
 *
 * Pure server-side helper — uses mongoTradesClient directly. Always emits a
 * `paper_state_repaired` worker_event for the audit trail; the caller decides
 * how to surface success/failure to its consumer.
 *
 * Behavior intentionally matches the existing route handler, so existing
 * callers see no observable change.
 */
import { randomUUID } from "crypto";
import {
  getAccountState,
  upsertAccountState,
  insertWorkerEvent,
} from "@/lib/mongoTradesClient";
import { isWorkerHeartbeatFresh } from "@/lib/paperDeskWorker/workerLease";

export interface RunRepairInput {
  accountKey: string;
  initialBalance?: number;
  reason?: string;
  clearDisabled?: boolean;
}

export interface RunRepairResult {
  repairId: string;
  clearedAt: number;
  balance: number;
  preservedDisabledCount: number;
  workerWasFresh: boolean;
}

/**
 * Run the paper-state repair logic. Throws on invalid input; safe to call
 * with `try/catch` from automation paths.
 */
export async function runPaperStateRepair(
  input: RunRepairInput,
): Promise<RunRepairResult> {
  const accountKey = input.accountKey?.trim();
  if (!accountKey) {
    throw new Error("accountKey required");
  }

  const initialBalance =
    typeof input.initialBalance === "number" && input.initialBalance > 0
      ? input.initialBalance
      : 1000;
  const reason =
    typeof input.reason === "string" && input.reason.trim() !== ""
      ? input.reason.slice(0, 200).trim()
      : "operator repair";
  const clearDisabled = input.clearDisabled === true;

  const current = await getAccountState(accountKey);
  const workerWasFresh = isWorkerHeartbeatFresh(
    current?.worker_last_poll_at ?? null,
  );
  const preservedDisabled = clearDisabled
    ? []
    : (current?.disabled_strategies ?? []);

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
    worker_id: current?.worker_id ?? null,
    worker_last_poll_at: current?.worker_last_poll_at ?? null,
    worker_owner: current?.worker_owner ?? null,
  });

  // Audit event (non-fatal if Mongo write fails)
  try {
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
  } catch {
    // never block repair on event write
  }

  return {
    repairId,
    clearedAt,
    balance: initialBalance,
    preservedDisabledCount: preservedDisabled.length,
    workerWasFresh,
  };
}
