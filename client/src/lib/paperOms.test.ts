/**
 * Tests for paperOms — pure OMS state machine, no I/O.
 */
import { describe, it, expect } from "vitest";
import {
  createPaperOmsOrder,
  markRiskChecked,
  markRejected,
  markAccepted,
  markSimulatedFill,
  markPositionOpened,
  markPositionClosed,
  markCancelled,
  summarizeOmsOrders,
  type PaperOmsOrder,
} from "./paperOms";

// ── Helpers ───────────────────────────────────────────────────────────────────

function makeOrder(overrides: Partial<Omit<PaperOmsOrder, "orderId" | "intentId" | "createdAt" | "updatedAt" | "status">> = {}): PaperOmsOrder {
  return createPaperOmsOrder({
    symbol: "BTCUSDT",
    strategyId: 91,
    strategyName: "strat-91",
    side: "LONG",
    signalScore: 28,
    requiredThreshold: 26,
    notional: 300,
    ...overrides,
  });
}

// ── createPaperOmsOrder ────────────────────────────────────────────────────────

describe("createPaperOmsOrder", () => {
  it("creates with status NEW and unique orderId/intentId", () => {
    const o = makeOrder();
    expect(o.status).toBe("NEW");
    expect(o.orderId).toBeTruthy();
    expect(o.intentId).toBeTruthy();
    expect(o.createdAt).toBeTruthy();
    expect(o.updatedAt).toBeTruthy();
  });

  it("each call produces unique orderId", () => {
    const a = makeOrder();
    const b = makeOrder();
    expect(a.orderId).not.toBe(b.orderId);
    expect(a.intentId).not.toBe(b.intentId);
  });

  it("fields are set correctly", () => {
    const o = makeOrder({ strategyId: 95, notional: 500 });
    expect(o.strategyId).toBe(95);
    expect(o.notional).toBe(500);
    expect(o.signalScore).toBe(28);
  });
});

// ── Full lifecycle ─────────────────────────────────────────────────────────────

describe("OMS lifecycle — NEW → POSITION_CLOSED", () => {
  it("advances through all states in sequence", () => {
    const order = makeOrder();
    expect(order.status).toBe("NEW");

    const rc = markRiskChecked(order);
    expect(rc.status).toBe("RISK_CHECKED");

    const acc = markAccepted(rc);
    expect(acc.status).toBe("ACCEPTED");

    const fill = markSimulatedFill(acc, 65000, 5);
    expect(fill.status).toBe("SIMULATED_FILL");
    expect(fill.fillPrice).toBe(65000);
    expect(fill.contracts).toBe(5);

    const opened = markPositionOpened(fill, "pos-abc-123", 65000);
    expect(opened.status).toBe("POSITION_OPENED");
    expect(opened.positionId).toBe("pos-abc-123");
    expect(opened.entryPrice).toBe(65000);

    const closed = markPositionClosed(opened, "trade-xyz-456");
    expect(closed.status).toBe("POSITION_CLOSED");
    expect(closed.tradeId).toBe("trade-xyz-456");
    expect(closed.positionId).toBe("pos-abc-123"); // preserved
  });
});

// ── markRejected ──────────────────────────────────────────────────────────────

describe("markRejected", () => {
  it("sets status REJECTED with gate and reason", () => {
    const order = makeOrder();
    const rejected = markRejected(order, "ATR_FEES", "ATR hurdle failed");
    expect(rejected.status).toBe("REJECTED");
    expect(rejected.gate).toBe("ATR_FEES");
    expect(rejected.rejectReason).toBe("ATR hurdle failed");
  });

  it("does not mutate the original", () => {
    const order = makeOrder();
    markRejected(order, "CONFIRM", "confirmation failed");
    expect(order.status).toBe("NEW");
    expect(order.gate).toBeUndefined();
    expect(order.rejectReason).toBeUndefined();
  });

  it("preserves all original fields", () => {
    const order = makeOrder({ strategyId: 92, notional: 600 });
    const rejected = markRejected(order, "MARGIN", "insufficient balance");
    expect(rejected.strategyId).toBe(92);
    expect(rejected.notional).toBe(600);
    expect(rejected.orderId).toBe(order.orderId);
  });
});

// ── markCancelled ─────────────────────────────────────────────────────────────

describe("markCancelled", () => {
  it("sets status CANCELLED with reason", () => {
    const order = makeOrder();
    const cancelled = markCancelled(order, "worker restart");
    expect(cancelled.status).toBe("CANCELLED");
    expect(cancelled.rejectReason).toBe("worker restart");
  });
});

// ── markPositionClosed ────────────────────────────────────────────────────────

describe("markPositionClosed", () => {
  it("links tradeId", () => {
    const order = makeOrder();
    const closed = markPositionClosed(order, "trade-999");
    expect(closed.status).toBe("POSITION_CLOSED");
    expect(closed.tradeId).toBe("trade-999");
  });

  it("does not mutate the original", () => {
    const order = makeOrder();
    markPositionClosed(order, "trade-999");
    expect(order.tradeId).toBeUndefined();
    expect(order.status).toBe("NEW");
  });
});

// ── updatedAt advances ────────────────────────────────────────────────────────

describe("updatedAt", () => {
  it("each transition updates updatedAt", async () => {
    const order = makeOrder();
    const before = order.updatedAt;
    // Short delay to ensure timestamp difference
    await new Promise<void>((r) => setTimeout(r, 2));
    const rc = markRiskChecked(order);
    expect(rc.updatedAt).not.toBe(before);
  });
});

// ── summarizeOmsOrders ────────────────────────────────────────────────────────

describe("summarizeOmsOrders", () => {
  it("counts by status correctly", () => {
    const orders: PaperOmsOrder[] = [
      makeOrder(),
      markRejected(makeOrder(), "CONFIRM", "r1"),
      markRejected(makeOrder(), "ATR_FEES", "r2"),
      markPositionOpened(markSimulatedFill(markAccepted(markRiskChecked(makeOrder())), 60000, 5), "pos-1", 60000),
    ];
    const s = summarizeOmsOrders(orders);
    expect(s.total).toBe(4);
    expect(s.countsByStatus.NEW).toBe(1);
    expect(s.countsByStatus.REJECTED).toBe(2);
    expect(s.countsByStatus.POSITION_OPENED).toBe(1);
  });

  it("topRejectGate is the most-frequent gate", () => {
    const orders: PaperOmsOrder[] = [
      markRejected(makeOrder(), "ATR_FEES", "r"),
      markRejected(makeOrder(), "ATR_FEES", "r"),
      markRejected(makeOrder(), "CONFIRM", "r"),
    ];
    const s = summarizeOmsOrders(orders);
    expect(s.topRejectGate).toBe("ATR_FEES");
  });

  it("empty input → zero totals and null gate", () => {
    const s = summarizeOmsOrders([]);
    expect(s.total).toBe(0);
    expect(s.topRejectGate).toBeNull();
    expect(s.fillRate).toBe(0);
    expect(s.rejectRate).toBe(0);
  });

  it("fillRate and rejectRate computed correctly", () => {
    const filled = markPositionOpened(makeOrder(), "pos-x", 60000);
    const rejected = markRejected(makeOrder(), "MARGIN", "insufficient");
    const s = summarizeOmsOrders([filled, rejected]);
    expect(s.fillRate).toBeCloseTo(0.5);
    expect(s.rejectRate).toBeCloseTo(0.5);
  });
});
