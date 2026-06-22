import { beforeEach, describe, expect, it, vi } from "vitest";
import { MOCK_RESET_CONFIRMATION } from "@/lib/trading/mockTradingPersistenceTypes";
import { OWNER_ACCOUNT_KEY } from "@/lib/broker/ownerAccountKey";

const ADMIN_SECRET = "test-admin-secret";

vi.mock("@/lib/broker/mongoTradesClient", () => ({
  isMongoConfigured: vi.fn().mockReturnValue(true),
}));

vi.mock("@/lib/trading/mockTradingMongo", () => ({
  resetMockTradingState: vi.fn().mockResolvedValue({
    tradesDeleted: 1,
    snapshotsDeleted: 2,
    analyticsDeleted: 3,
    logsDeleted: 4,
  }),
}));

const getAuthenticatedApiSession = vi.fn();
vi.mock("@/lib/broker/getAuthenticatedApiSession", () => ({
  getAuthenticatedApiSession: () => getAuthenticatedApiSession(),
}));

import { DELETE, POST } from "./route";
import * as mockMongo from "@/lib/trading/mockTradingMongo";

function request(body: unknown, headers: Record<string, string> = { "x-engine-admin-secret": ADMIN_SECRET }) {
  return new Request("http://localhost/api/mock-trading/reset", {
    method: "DELETE",
    headers: { "Content-Type": "application/json", ...headers },
    body: JSON.stringify(body),
  });
}

describe("DELETE /api/mock-trading/reset", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    process.env.ENGINE_ADMIN_SECRET = ADMIN_SECRET;
  });

  it("rejects callers without the admin secret", async () => {
    const res = await DELETE(request({ confirmation: MOCK_RESET_CONFIRMATION }, {}));
    expect(res.status).toBe(403);
    expect(mockMongo.resetMockTradingState).not.toHaveBeenCalled();
  });

  it("requires explicit confirmation", async () => {
    const res = await DELETE(request({ accountKey: OWNER_ACCOUNT_KEY }));
    expect(res.status).toBe(400);
    const body = await res.json() as { code: string };
    expect(body.code).toBe("CONFIRMATION_REQUIRED");
    expect(mockMongo.resetMockTradingState).not.toHaveBeenCalled();
  });

  it("resets mock state when confirmation is supplied", async () => {
    const res = await DELETE(request({
      accountKey: OWNER_ACCOUNT_KEY,
      confirmation: MOCK_RESET_CONFIRMATION,
    }));
    expect(res.status).toBe(200);
    const body = await res.json() as { ok: boolean; tradesDeleted: number };
    expect(body.ok).toBe(true);
    expect(body.tradesDeleted).toBe(1);
    expect(mockMongo.resetMockTradingState).toHaveBeenCalledWith(OWNER_ACCOUNT_KEY);
  });
});

describe("POST /api/mock-trading/reset (session-authenticated)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("rejects unauthenticated callers", async () => {
    getAuthenticatedApiSession.mockResolvedValue({
      ok: false,
      response: Response.json({ ok: false, error: "Authentication required" }, { status: 401 }),
    });
    const res = await POST();
    expect(res.status).toBe(401);
    expect(mockMongo.resetMockTradingState).not.toHaveBeenCalled();
  });

  it("resets the owner account for an authenticated owner", async () => {
    getAuthenticatedApiSession.mockResolvedValue({ ok: true, ctx: { userId: OWNER_ACCOUNT_KEY } });
    const res = await POST();
    expect(res.status).toBe(200);
    const body = await res.json() as { ok: boolean; tradesDeleted: number };
    expect(body.ok).toBe(true);
    expect(body.tradesDeleted).toBe(1);
    expect(mockMongo.resetMockTradingState).toHaveBeenCalledWith(OWNER_ACCOUNT_KEY);
  });
});
