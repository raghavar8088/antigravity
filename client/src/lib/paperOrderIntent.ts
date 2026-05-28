/**
 * Paper Order Intent — minimal audit layer for the paper OMS.
 *
 * An intent is created when a strategy passes all signal gates and an open
 * attempt is made. It is rejected (with reason) or filled (with positionId).
 * Storing intents makes execution fully auditable without live orders.
 *
 * Hard invariants:
 *   - Paper only. No live Delta order placement, ever.
 *   - Intents are read-only once created (return new objects, no mutation).
 *   - No secrets in intent records.
 */

import { randomUUID } from "crypto";

// ─── Public types ─────────────────────────────────────────────────────────────

export type PaperIntentStatus =
  | "NEW"
  | "RISK_CHECKED"
  | "REJECTED"
  | "ACCEPTED"
  | "FILLED"
  | "CLOSED";

export interface PaperOrderIntent {
  intentId: string;
  createdAt: string;
  strategyId: number;
  strategyName: string;
  symbol: string;
  side: "LONG" | "SHORT";
  signalScore: number;
  status: PaperIntentStatus;
  rejectReason?: string;
  positionId?: string;
  /** The gate that caused a rejection (matches SignalTraceGate values). */
  gate?: string;
}

// ─── Factory functions ────────────────────────────────────────────────────────

export function createPaperOrderIntent(
  fields: Omit<PaperOrderIntent, "intentId" | "createdAt">,
): PaperOrderIntent {
  return {
    intentId: randomUUID(),
    createdAt: new Date().toISOString(),
    ...fields,
  };
}

/** Return a rejected copy of the intent — original is not mutated. */
export function rejectIntent(
  intent: PaperOrderIntent,
  gate: string,
  reason: string,
): PaperOrderIntent {
  return { ...intent, status: "REJECTED", gate, rejectReason: reason };
}

/** Return an accepted copy of the intent (pre-fill, risk checked). */
export function acceptIntent(intent: PaperOrderIntent): PaperOrderIntent {
  return { ...intent, status: "ACCEPTED" };
}

/** Return a filled copy of the intent, linking the opened position ID. */
export function fillIntent(
  intent: PaperOrderIntent,
  positionId: string,
): PaperOrderIntent {
  return { ...intent, status: "FILLED", positionId };
}

/** Return a closed copy when the linked position exits. */
export function closeIntent(intent: PaperOrderIntent): PaperOrderIntent {
  return { ...intent, status: "CLOSED" };
}

// ─── Summary helper ───────────────────────────────────────────────────────────

export interface PaperIntentSummary {
  total: number;
  byStatus: Record<PaperIntentStatus, number>;
  topRejectedGate: string | null;
}

export function summarizeIntents(intents: ReadonlyArray<PaperOrderIntent>): PaperIntentSummary {
  const byStatus = {
    NEW: 0,
    RISK_CHECKED: 0,
    REJECTED: 0,
    ACCEPTED: 0,
    FILLED: 0,
    CLOSED: 0,
  } satisfies Record<PaperIntentStatus, number>;

  const gateCounts = new Map<string, number>();

  for (const intent of intents) {
    byStatus[intent.status] = (byStatus[intent.status] ?? 0) + 1;
    if (intent.status === "REJECTED" && intent.gate) {
      gateCounts.set(intent.gate, (gateCounts.get(intent.gate) ?? 0) + 1);
    }
  }

  let topRejectedGate: string | null = null;
  let topCount = 0;
  for (const [gate, count] of gateCounts) {
    if (count > topCount) {
      topCount = count;
      topRejectedGate = gate;
    }
  }

  return { total: intents.length, byStatus, topRejectedGate };
}
