/**
 * Paper OMS — order lifecycle state machine.
 *
 * Models the full paper execution lifecycle:
 *   NEW → RISK_CHECKED → ACCEPTED → SIMULATED_FILL → POSITION_OPENED → POSITION_CLOSED
 *   NEW → RISK_CHECKED → REJECTED  (gate/reason recorded)
 *   NEW → CANCELLED
 *
 * Hard invariants:
 *   - Paper only. No live Delta order placement, ever.
 *   - All helpers are pure — return new objects, never mutate.
 *   - No secrets stored in orders.
 */

import { randomUUID } from "crypto";

// ─── Status ────────────────────────────────────────────────────────────────────

export type PaperOmsOrderStatus =
  | "NEW"
  | "RISK_CHECKED"
  | "REJECTED"
  | "ACCEPTED"
  | "SIMULATED_FILL"
  | "POSITION_OPENED"
  | "POSITION_CLOSED"
  | "CANCELLED";

// ─── Order document ────────────────────────────────────────────────────────────

export interface PaperOmsOrder {
  orderId: string;
  /** Links to PaperOrderIntent.intentId — shared correlation ID. */
  intentId: string;
  createdAt: string;
  updatedAt: string;
  symbol: string;
  strategyId: number;
  strategyName: string;
  side: "LONG" | "SHORT";
  status: PaperOmsOrderStatus;
  signalScore: number;
  requiredThreshold: number;
  notional: number;
  contracts?: number;
  entryPrice?: number;
  fillPrice?: number;
  positionId?: string;
  tradeId?: string;
  rejectReason?: string;
  gate?: string;
}

// ─── Factory ───────────────────────────────────────────────────────────────────

export function createPaperOmsOrder(
  fields: Omit<PaperOmsOrder, "orderId" | "intentId" | "createdAt" | "updatedAt" | "status">,
): PaperOmsOrder {
  const ts = new Date().toISOString();
  return {
    orderId: randomUUID(),
    intentId: randomUUID(),
    createdAt: ts,
    updatedAt: ts,
    status: "NEW",
    ...fields,
  };
}

// ─── Pure state transitions ────────────────────────────────────────────────────

/** Signal passed threshold — risk check in progress. */
export function markRiskChecked(order: PaperOmsOrder): PaperOmsOrder {
  return { ...order, status: "RISK_CHECKED", updatedAt: new Date().toISOString() };
}

/** Gate rejected the order — no open. */
export function markRejected(
  order: PaperOmsOrder,
  gate: string,
  reason: string,
): PaperOmsOrder {
  return {
    ...order,
    status: "REJECTED",
    gate,
    rejectReason: reason,
    updatedAt: new Date().toISOString(),
  };
}

/** All risk gates passed — queued for paper execution. */
export function markAccepted(order: PaperOmsOrder): PaperOmsOrder {
  return { ...order, status: "ACCEPTED", updatedAt: new Date().toISOString() };
}

/** Simulated fill at market price. */
export function markSimulatedFill(
  order: PaperOmsOrder,
  fillPrice: number,
  contracts: number,
): PaperOmsOrder {
  return {
    ...order,
    status: "SIMULATED_FILL",
    fillPrice,
    contracts,
    updatedAt: new Date().toISOString(),
  };
}

/** Paper position created and linked. */
export function markPositionOpened(
  order: PaperOmsOrder,
  positionId: string,
  entryPrice: number,
): PaperOmsOrder {
  return {
    ...order,
    status: "POSITION_OPENED",
    positionId,
    entryPrice,
    updatedAt: new Date().toISOString(),
  };
}

/** Position closed — linked trade ID recorded. */
export function markPositionClosed(
  order: PaperOmsOrder,
  tradeId: string,
): PaperOmsOrder {
  return {
    ...order,
    status: "POSITION_CLOSED",
    tradeId,
    updatedAt: new Date().toISOString(),
  };
}

/** Cancelled before execution (e.g. worker restart before fill). */
export function markCancelled(order: PaperOmsOrder, reason: string): PaperOmsOrder {
  return {
    ...order,
    status: "CANCELLED",
    rejectReason: reason,
    updatedAt: new Date().toISOString(),
  };
}

// ─── Summary helper ────────────────────────────────────────────────────────────

export interface PaperOmsSummary {
  total: number;
  countsByStatus: Record<PaperOmsOrderStatus, number>;
  topRejectGate: string | null;
  fillRate: number;
  rejectRate: number;
}

const ALL_STATUSES: PaperOmsOrderStatus[] = [
  "NEW",
  "RISK_CHECKED",
  "REJECTED",
  "ACCEPTED",
  "SIMULATED_FILL",
  "POSITION_OPENED",
  "POSITION_CLOSED",
  "CANCELLED",
];

export function summarizeOmsOrders(orders: ReadonlyArray<PaperOmsOrder>): PaperOmsSummary {
  const countsByStatus = Object.fromEntries(
    ALL_STATUSES.map((s) => [s, 0]),
  ) as Record<PaperOmsOrderStatus, number>;

  const gateCounts = new Map<string, number>();

  for (const order of orders) {
    countsByStatus[order.status] = (countsByStatus[order.status] ?? 0) + 1;
    if (order.status === "REJECTED" && order.gate) {
      gateCounts.set(order.gate, (gateCounts.get(order.gate) ?? 0) + 1);
    }
  }

  let topRejectGate: string | null = null;
  let topCount = 0;
  for (const [gate, count] of gateCounts) {
    if (count > topCount) {
      topCount = count;
      topRejectGate = gate;
    }
  }

  const total = orders.length;
  const filled =
    countsByStatus.SIMULATED_FILL +
    countsByStatus.POSITION_OPENED +
    countsByStatus.POSITION_CLOSED;
  const rejected = countsByStatus.REJECTED;

  return {
    total,
    countsByStatus,
    topRejectGate,
    fillRate: total > 0 ? filled / total : 0,
    rejectRate: total > 0 ? rejected / total : 0,
  };
}
