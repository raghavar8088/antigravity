import { beforeEach, describe, expect, it, vi } from "vitest";
import { MOCK_CLEAR_CLOSED_CONFIRMATION } from "@/lib/trading/mockTradingPersistenceTypes";

vi.mock("@/lib/broker/mongoTradesClient", () => ({
  isMongoConfigured: vi.fn().mockReturnValue(true),
}));

vi.mock("@/lib/trading/mockTradingMongo", () => ({
  deleteClosedMockTrades: vi.fn().mockResolvedValue({ tradesDeleted: 12, paperTradesDeleted: 35 }),
}));

import { DELETE } from "./route";
import * as mockMongo from "@/lib/trading/mockTradingMongo";

function request(body: unknown) {
  return new Request("http://localhost/api/mock-trading/trades/closed", {
    method: "DELETE",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

describe("DELETE /api/mock-trading/trades/closed", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("requires explicit confirmation", async () => {
    const res = await DELETE(request({ accountKey: "mock_trading_main" }));
    expect(res.status).toBe(400);
    const body = await res.json() as { code: string };
    expect(body.code).toBe("CONFIRMATION_REQUIRED");
    expect(mockMongo.deleteClosedMockTrades).not.toHaveBeenCalled();
  });

  it("clears closed trades when confirmation is supplied", async () => {
    const res = await DELETE(request({
      accountKey: "mock_trading_main",
      confirmation: MOCK_CLEAR_CLOSED_CONFIRMATION,
    }));
    expect(res.status).toBe(200);
    const body = await res.json() as { ok: boolean; tradesDeleted: number; paperTradesDeleted: number };
    expect(body.ok).toBe(true);
    expect(body.tradesDeleted).toBe(12);
    expect(body.paperTradesDeleted).toBe(35);
    expect(mockMongo.deleteClosedMockTrades).toHaveBeenCalledWith("mock_trading_main");
  });
});
