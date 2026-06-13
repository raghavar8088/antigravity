/**
 * Self-healing executor — env-gated automatic execution of safe healing actions.
 *
 * Hard invariants (PR-SELF-HEALING-DESK):
 *   - Only runs when DESK_SELF_HEAL_AUTO=1 (default OFF). All other modes
 *     produce execution results with status `skipped_disabled`.
 *   - Only acts on actions with `safeToAutomate: true`. Unsafe actions are
 *     left for operator review.
 *   - The supported-action allowlist is narrow on purpose. RESTART_WORKER,
 *     REDUCE_ROSTER, CHECK_DEPLOYMENT etc. require human / SSH involvement
 *     and are never executed here.
 *   - Failures are logged to the result list with `status: "failed"` and
 *     never thrown — the executor must not crash the tracker capture.
 *   - Every executed action emits a worker_event for the audit trail.
 *
 * Designed as a pure-ish function: takes injected dependencies so tests
 * don't need to mock module-level imports.
 */
import type {
  DeskHealingAction,
  DeskHealingActionType,
} from "./deskSelfHealing";

// ─── Public types ────────────────────────────────────────────────────────────

export type HealingExecutionStatus =
  | "executed"
  | "skipped_disabled"
  | "skipped_unsafe"
  | "skipped_unsupported"
  | "skipped_no_op"
  | "failed";

export interface HealingExecutionResult {
  actionType: DeskHealingActionType;
  title: string;
  status: HealingExecutionStatus;
  reason?: string;
  durationMs?: number;
  detail?: Record<string, unknown>;
}

export interface HealingExecutionDeps {
  accountKey: string;
  autoEnabled: boolean;
  /** Server-side paper-state repair. Must match runPaperStateRepair contract. */
  repairPaperState: (input: {
    initialBalance?: number;
    reason?: string;
  }) => Promise<{ repairId: string; balance: number; clearedAt: number }>;
  /** Audit-trail writer. Must not throw. */
  writeWorkerEvent: (event: {
    type: string;
    severity: "info" | "warning" | "error";
    message: string;
    payload?: Record<string, unknown>;
  }) => Promise<void>;
  /** Override `Date.now()` for deterministic tests. */
  now?: () => number;
}

// ─── Allowlist ───────────────────────────────────────────────────────────────

/**
 * Action types that the executor knows how to perform automatically.
 * Anything else maps to `skipped_unsupported` — surfaced to operator only.
 *
 * Intentional narrowness:
 *   - REPAIR_STATE: only meaningful path here (drift repair, 0 positions)
 *   - WAIT_FOR_MARKET / NO_ACTION: handled as `skipped_no_op` (recorded but
 *     no work performed — these are "do nothing" advice)
 */
const SUPPORTED_TYPES: ReadonlySet<DeskHealingActionType> = new Set([
  "REPAIR_STATE",
  "WAIT_FOR_MARKET",
  "NO_ACTION",
]);

// ─── Main entry ──────────────────────────────────────────────────────────────

export async function executeHealingActions(
  actions: ReadonlyArray<DeskHealingAction>,
  deps: HealingExecutionDeps,
): Promise<HealingExecutionResult[]> {
  const results: HealingExecutionResult[] = [];

  if (!deps.autoEnabled) {
    // Record one summary entry so the report shows "we considered N actions but auto is off"
    for (const action of actions) {
      results.push({
        actionType: action.type,
        title: action.title,
        status: "skipped_disabled",
        reason: "DESK_SELF_HEAL_AUTO is not enabled",
      });
    }
    return results;
  }

  for (const action of actions) {
    if (!SUPPORTED_TYPES.has(action.type)) {
      results.push({
        actionType: action.type,
        title: action.title,
        status: "skipped_unsupported",
        reason: "Action type cannot be executed automatically",
      });
      continue;
    }
    if (!action.safeToAutomate) {
      results.push({
        actionType: action.type,
        title: action.title,
        status: "skipped_unsafe",
        reason: "Action is marked safeToAutomate=false",
      });
      continue;
    }

    // No-op family
    if (action.type === "WAIT_FOR_MARKET" || action.type === "NO_ACTION") {
      results.push({
        actionType: action.type,
        title: action.title,
        status: "skipped_no_op",
        reason: "Action requires no execution",
      });
      continue;
    }

    // Executable branch
    const result = await runSingleAction(action, deps);
    results.push(result);
  }

  return results;
}

// ─── Single-action execution ─────────────────────────────────────────────────

async function runSingleAction(
  action: DeskHealingAction,
  deps: HealingExecutionDeps,
): Promise<HealingExecutionResult> {
  const now = deps.now ?? Date.now;
  const startedAt = now();

  try {
    switch (action.type) {
      case "REPAIR_STATE": {
        const reason = `auto-heal: ${action.title}`;
        const out = await deps.repairPaperState({ reason });

        // Emit audit event (non-fatal)
        try {
          await deps.writeWorkerEvent({
            type: "auto_heal_repair_state",
            severity: "info" as const,
            message: `Auto-healed paper_state: ${action.title}. Repair ID ${out.repairId}, balance reset to $${out.balance}.`,
            payload: {
              repairId: out.repairId,
              clearedAt: out.clearedAt,
              balance: out.balance,
              triggerActionType: action.type,
              triggerTitle: action.title,
              triggerReason: action.reason,
              autoEnabled: true,
            },
          });
        } catch {
          // event write failure is non-fatal
        }

        return {
          actionType: action.type,
          title: action.title,
          status: "executed",
          durationMs: now() - startedAt,
          detail: {
            repairId: out.repairId,
            balance: out.balance,
            clearedAt: out.clearedAt,
          },
        };
      }
      default:
        // Should never reach here — type guarded by SUPPORTED_TYPES + no-op branch.
        return {
          actionType: action.type,
          title: action.title,
          status: "skipped_unsupported",
          reason: "Internal: no execution branch defined",
          durationMs: now() - startedAt,
        };
    }
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);

    try {
      await deps.writeWorkerEvent({
        type: "auto_heal_failed",
        severity: "warning",
        message: `Auto-heal failed for ${action.type}: ${msg}`,
        payload: {
          triggerActionType: action.type,
          triggerTitle: action.title,
          error: msg,
        },
      });
    } catch {
      // never throw
    }

    return {
      actionType: action.type,
      title: action.title,
      status: "failed",
      reason: msg,
      durationMs: now() - startedAt,
    };
  }
}
