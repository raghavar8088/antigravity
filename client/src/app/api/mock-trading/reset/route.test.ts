import { beforeEach, describe, expect, it, vi } from "vitest";
import { MOCK_RESET_CONFIRMATION } from "@/lib/mockTradingPersistenceTypes";

vi.mock("@/lib/mongoTradesClient", () => ({
  isMongoConfigured: vi.fn().mockReturnValue(true),
}));

vi.mock("@/lib/mockTradingMongo", () => ({
  resetMockTradingState: vi.fn().mockResolvedValue({
    tradesDeleted: 1,
    snapshotsDeleted: 2,
    analyticsDeleted: 3,
    logsDeleted: 4,
  }),
}));

import { DELETE } from "./route";
import * as mockMongo from "@/lib/mockTradingMongo";

function request(body: unknown) {
  return new Request("http://localhost/api/mock-trading/reset", {
    method: "DELETE",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

describe("DELETE /api/mock-trading/reset", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("requires explicit confirmation", async () => {
    const res = await DELETE(request({ accountKey: "mock_trading_default" }));
    expect(res.status).toBe(400);
    const body = await res.json() as { code: string };
    expect(body.code).toBe("CONFIRMATION_REQUIRED");
    expect(mockMongo.resetMockTradingState).not.toHaveBeenCalled();
  });

  it("resets mock state when confirmation is supplied", async () => {
    const res = await DELETE(request({
      accountKey: "mock_trading_default",
      confirmation: MOCK_RESET_CONFIRMATION,
    }));
    expect(res.status).toBe(200);
    const body = await res.json() as { ok: boolean; tradesDeleted: number };
    expect(body.ok).toBe(true);
    expect(body.tradesDeleted).toBe(1);
    expect(mockMongo.resetMockTradingState).toHaveBeenCalledWith("mock_trading_default");
  });
});
