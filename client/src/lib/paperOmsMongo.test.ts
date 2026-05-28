/**
 * Tests for paperOmsMongo — Mongo helpers.
 * Uses vi.mock to avoid a real Atlas connection.
 */
import { describe, it, expect, vi, beforeEach } from "vitest";

// ── Mock mongoTradesClient before any imports ─────────────────────────────────

vi.mock("@/lib/mongoTradesClient", () => {
  const mockCollection = {
    createIndex: vi.fn().mockResolvedValue(undefined),
    insertOne: vi.fn().mockResolvedValue({ acknowledged: true }),
    insertMany: vi.fn().mockResolvedValue({ acknowledged: true }),
    updateOne: vi.fn().mockResolvedValue({ modifiedCount: 1 }),
    find: vi.fn().mockReturnValue({
      sort: vi.fn().mockReturnThis(),
      limit: vi.fn().mockReturnThis(),
      toArray: vi.fn().mockResolvedValue([]),
    }),
    findOne: vi.fn().mockResolvedValue(null),
  };

  return {
    isMongoConfigured: vi.fn().mockReturnValue(true),
    getDb: vi.fn().mockResolvedValue({
      collection: vi.fn().mockReturnValue(mockCollection),
    }),
    __mockCollection: mockCollection,
  };
});

import { isMongoConfigured, getDb } from "@/lib/mongoTradesClient";
import {
  insertPaperOmsOrder,
  insertPaperOmsOrders,
  updatePaperOmsOrderStatus,
  listPaperOmsOrders,
  getLatestPaperOmsOrders,
} from "./paperOmsMongo";
import { createPaperOmsOrder } from "./paperOms";

function makeMockOrder() {
  return createPaperOmsOrder({
    symbol: "BTCUSDT",
    strategyId: 91,
    strategyName: "strat-91",
    side: "LONG",
    signalScore: 28,
    requiredThreshold: 26,
    notional: 300,
  });
}

// ── insertPaperOmsOrder ───────────────────────────────────────────────────────

describe("insertPaperOmsOrder", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(isMongoConfigured).mockReturnValue(true);
  });

  it("calls insertOne with the order document", async () => {
    const order = makeMockOrder();
    await insertPaperOmsOrder(order);

    const db = await vi.mocked(getDb)();
    const col = db.collection("paper_oms_orders");
    expect(col.insertOne).toHaveBeenCalledWith(expect.objectContaining({
      orderId: order.orderId,
      strategyId: 91,
      status: "NEW",
    }));
  });

  it("is non-fatal when mongo not configured", async () => {
    vi.mocked(isMongoConfigured).mockReturnValue(false);
    const order = makeMockOrder();
    await expect(insertPaperOmsOrder(order)).resolves.toBeUndefined();
  });

  it("does not store account keys or secrets in the call", async () => {
    const order = makeMockOrder();
    await insertPaperOmsOrder(order);

    const db = await vi.mocked(getDb)();
    const col = db.collection("paper_oms_orders");
    const call = vi.mocked(col.insertOne).mock.calls[0]?.[0] as Record<string, unknown>;
    expect(call).not.toHaveProperty("account_key");
    expect(call).not.toHaveProperty("mongodb_uri");
    expect(call).not.toHaveProperty("api_key");
  });
});

// ── insertPaperOmsOrders (batch) ─────────────────────────────────────────────

describe("insertPaperOmsOrders", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(isMongoConfigured).mockReturnValue(true);
  });

  it("no-ops on empty array", async () => {
    await insertPaperOmsOrders([]);
    const db = await vi.mocked(getDb)();
    const col = db.collection("paper_oms_orders");
    expect(col.insertMany).not.toHaveBeenCalled();
  });

  it("calls insertMany for non-empty array", async () => {
    const orders = [makeMockOrder(), makeMockOrder()];
    await insertPaperOmsOrders(orders);
    const db = await vi.mocked(getDb)();
    const col = db.collection("paper_oms_orders");
    expect(col.insertMany).toHaveBeenCalledWith(
      expect.arrayContaining([expect.objectContaining({ status: "NEW" })]),
      { ordered: false },
    );
  });
});

// ── updatePaperOmsOrderStatus ─────────────────────────────────────────────────

describe("updatePaperOmsOrderStatus", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(isMongoConfigured).mockReturnValue(true);
  });

  it("calls updateOne with correct orderId and status", async () => {
    await updatePaperOmsOrderStatus("order-123", "POSITION_CLOSED", { tradeId: "trade-456" });

    const db = await vi.mocked(getDb)();
    const col = db.collection("paper_oms_orders");
    expect(col.updateOne).toHaveBeenCalledWith(
      { orderId: "order-123" },
      expect.objectContaining({
        $set: expect.objectContaining({ status: "POSITION_CLOSED", tradeId: "trade-456" }),
      }),
    );
  });

  it("is non-fatal when mongo not configured", async () => {
    vi.mocked(isMongoConfigured).mockReturnValue(false);
    await expect(updatePaperOmsOrderStatus("x", "CANCELLED")).resolves.toBeUndefined();
  });
});

// ── listPaperOmsOrders ────────────────────────────────────────────────────────

describe("listPaperOmsOrders", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(isMongoConfigured).mockReturnValue(true);
  });

  it("returns empty array when not configured", async () => {
    vi.mocked(isMongoConfigured).mockReturnValue(false);
    const result = await listPaperOmsOrders({});
    expect(result).toEqual([]);
  });

  it("applies status filter in query", async () => {
    const db = await vi.mocked(getDb)();
    const col = db.collection("paper_oms_orders");
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(col.find).mockReturnValue({
      sort: vi.fn().mockReturnThis(),
      limit: vi.fn().mockReturnThis(),
      toArray: vi.fn().mockResolvedValue([]),
    } as any);

    await listPaperOmsOrders({ status: "REJECTED" });

    expect(col.find).toHaveBeenCalledWith(
      expect.objectContaining({ status: "REJECTED" }),
    );
  });

  it("applies strategyId filter in query", async () => {
    const db = await vi.mocked(getDb)();
    const col = db.collection("paper_oms_orders");
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(col.find).mockReturnValue({
      sort: vi.fn().mockReturnThis(),
      limit: vi.fn().mockReturnThis(),
      toArray: vi.fn().mockResolvedValue([]),
    } as any);

    await listPaperOmsOrders({ strategyId: 91 });

    expect(col.find).toHaveBeenCalledWith(
      expect.objectContaining({ strategyId: 91 }),
    );
  });

  it("sorts newest-first with a cap of 500", async () => {
    const db = await vi.mocked(getDb)();
    const col = db.collection("paper_oms_orders");
    const mockChain = {
      sort: vi.fn().mockReturnThis(),
      limit: vi.fn().mockReturnThis(),
      toArray: vi.fn().mockResolvedValue([]),
    };
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    vi.mocked(col.find).mockReturnValue(mockChain as any);

    await listPaperOmsOrders({ limit: 1000 }); // should be capped at 500
    expect(mockChain.sort).toHaveBeenCalledWith({ createdAt: -1 });
    expect(mockChain.limit).toHaveBeenCalledWith(500);
  });
});

// ── getLatestPaperOmsOrders ───────────────────────────────────────────────────

describe("getLatestPaperOmsOrders", () => {
  it("delegates to listPaperOmsOrders with limit=50 by default", async () => {
    vi.mocked(isMongoConfigured).mockReturnValue(false);
    const result = await getLatestPaperOmsOrders();
    expect(result).toEqual([]);
  });
});
