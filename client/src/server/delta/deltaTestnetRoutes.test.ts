import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextResponse } from "next/server";

vi.mock("@/server/delta/testnetRouteGuards", () => ({
  guardTestnetApiRoute: vi.fn(),
  guardTestnetOpsPanelEnabled: vi.fn(() => null),
}));

vi.mock("@/server/delta/resolveTestnetProduct", () => ({
  resolveTestnetPerpProductId: vi.fn(async () => 27),
}));

vi.mock("@/server/delta/deltaTestnetAudit", () => ({
  appendDeltaAuditLog: vi.fn(async () => {}),
}));

vi.mock("@/server/delta/deltaTestnetRateLimit", () => ({
  checkTestnetPlaceOrderRateLimit: vi.fn(() => ({ allowed: true, remaining: 9 })),
  recordTestnetPlaceOrder: vi.fn(),
}));

const mockPlaceOrder = vi.fn();
const mockCancelOrder = vi.fn();
const mockGetPositions = vi.fn();
const mockGetOpenOrders = vi.fn();

vi.mock("@/server/delta/deltaClient", () => ({
  DeltaTestnetClient: {
    fromEnv: () => ({
      placeOrder: mockPlaceOrder,
      cancelOrder: mockCancelOrder,
      getPositions: mockGetPositions,
      getOpenOrders: mockGetOpenOrders,
    }),
  },
}));

import { guardTestnetApiRoute } from "@/server/delta/testnetRouteGuards";
import { POST as placeOrderPost } from "@/app/api/delta/testnet/place-order/route";
import { POST as cancelOrderPost } from "@/app/api/delta/testnet/cancel-order/route";
import { GET as positionsGet } from "@/app/api/delta/testnet/positions/route";

describe("testnet API routes (mocked)", () => {
  beforeEach(() => {
    vi.stubEnv("NEXT_PUBLIC_DESK_TESTNET_OPS", "1");
    vi.stubEnv("DELTA_TESTNET", "1");
    vi.mocked(guardTestnetApiRoute).mockResolvedValue({
      ok: true,
      ctx: { userId: "test-user-uuid" },
    });
    mockPlaceOrder.mockReset();
    mockCancelOrder.mockReset();
    mockGetPositions.mockReset();
    mockGetOpenOrders.mockReset();
  });

  it("place-order returns 410 even when session guard would fail — route retired", async () => {
    vi.mocked(guardTestnetApiRoute).mockResolvedValueOnce({
      ok: false,
      response: NextResponse.json({ ok: false, error: "Unauthorized" }, { status: 401 }),
    });

    const res = await placeOrderPost(
      new Request("http://localhost/api/delta/testnet/place-order", {
        method: "POST",
        body: JSON.stringify({
          symbol: "BTCUSD",
          side: "buy",
          size: 1,
          type: "market",
        }),
      }),
    );
    expect(res.status).toBe(410);
  });

  it("place-order returns 410 — route retired", async () => {
    const res = await placeOrderPost(
      new Request("http://localhost/api/delta/testnet/place-order", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          symbol: "BTCUSD",
          side: "buy",
          size: 1,
          type: "market",
        }),
      }),
    );
    expect(res.status).toBe(410);
  });

  it("cancel-order returns 410 — route retired", async () => {
    const res = await cancelOrderPost(
      new Request("http://localhost/api/delta/testnet/cancel-order", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ orderId: "99" }),
      }),
    );
    expect(res.status).toBe(410);
  });

  it("positions GET returns positions and open orders", async () => {
    mockGetPositions.mockResolvedValueOnce({
      ok: true,
      data: [{ symbol: "BTCUSD", productId: 27, size: 1, side: "LONG" }],
    });
    mockGetOpenOrders.mockResolvedValueOnce({
      ok: true,
      data: [{ orderId: "1", symbol: "BTCUSD", productId: 27, side: "buy", size: 1, limitPrice: null, state: "open", createdAt: "" }],
    });

    const res = await positionsGet();
    const json = (await res.json()) as { ok: boolean; positions?: unknown[]; openOrders?: unknown[] };
    expect(json.ok).toBe(true);
    expect(json.positions?.length).toBe(1);
    expect(json.openOrders?.length).toBe(1);
  });
});
