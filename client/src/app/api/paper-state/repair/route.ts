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
 * Implementation: delegates to runPaperStateRepair() so the same logic is
 * available to the env-gated self-healing executor.
 */

import { NextResponse } from "next/server";
import { isMongoConfigured } from "@/lib/mongoTradesClient";
import { runPaperStateRepair } from "@/lib/paperStateRepairCore";

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

  const result = await runPaperStateRepair({
    accountKey,
    initialBalance: typeof b.initialBalance === "number" ? b.initialBalance : undefined,
    reason: typeof b.reason === "string" ? b.reason : undefined,
    clearDisabled: b.clearDisabled === true,
  });

  return NextResponse.json({
    ok: true,
    repairId: result.repairId,
    clearedAt: result.clearedAt,
    balance: result.balance,
    preservedDisabledCount: result.preservedDisabledCount,
    workerWasFresh: result.workerWasFresh,
    message:
      "Paper state repaired — old trades retained but excluded from current session. Worker will pick this up within one tick.",
  });
}
