import { describe, it, expect, vi, beforeEach } from "vitest";
import { randomUUID } from "crypto";

vi.mock("@/lib/mongoTradesClient", () => ({
  isMongoConfigured: vi.fn().mockReturnValue(true),
  upsertTradeMongo: vi.fn().mockResolvedValue({
    ok: true,
    upsertedCount: 1,
    modifiedCount: 0,
    matchedCount: 0,
  }),
  listTradesMongo: vi.fn().mockResolvedValue([]),
  pingMongo: vi.fn().mockResolvedValue(true),
}));

// next/headers `cookies()` returns an async ReadonlyRequestCookies. Stub it
// so the route's auth path skips the JWT lookup and falls back to anon.
vi.mock("next/headers", () => ({
  cookies: vi.fn().mockResolvedValue({ get: () => undefined }),
}));

import { POST } from "./route";
import * as mongoClient from "@/lib/mongoTradesClient";

const OLD_ENV = { ...process.env };

function makeValidTrade() {
  const now = new Date().toISOString();
  return {
    clientTradeId: randomUUID(),
    id: `BTCUSDT-1-${Date.now()}-abcd`,
    symbol: "BTCUSDT",
    strategyId: 1,
    strategyName: "TestStrategy",
    side: "LONG",
    entryPrice: 50000,
    exitPrice: 51000,
    contracts: 2,
    notional: 100,
    marginUsed: 10,
    realizedPnl: 1.9,
    fees: 0.1,
    netPnl: 0.9,
    netPnlPct: 9,
    priceMovePct: 2,
    fundingCosts: 0,
    openedAt: now,
    closedAt: now,
    exitReason: "TP",
    liquidationPrice: 45000,
    liquidationDistancePct: 10,
  };
}

function makeRequest(body: unknown) {
  return new Request("http://localhost/api/paper-trades", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

describe("POST /api/paper-trades", () => {
  beforeEach(() => {
    process.env = { ...OLD_ENV, ALLOW_PAPER_TRADES_ANON: "1", ENGINE_EXECUTION_AUTHORITY: "0" };
    vi.mocked(mongoClient.isMongoConfigured).mockReturnValue(true);
    vi.mocked(mongoClient.upsertTradeMongo).mockResolvedValue({
      ok: true,
      upsertedCount: 1,
      modifiedCount: 0,
      matchedCount: 0,
    });
  });

  it("returns 410 DEPRECATED when Go engine is execution authority", async () => {
    process.env.ENGINE_EXECUTION_AUTHORITY = "1";
    const res = await POST(makeRequest({ accountKey: "anon_test-key", trade: makeValidTrade() }));
    expect(res.status).toBe(410);
    const body = await res.json() as { ok: boolean; code: string; error: string };
    expect(body.ok).toBe(false);
    expect(body.code).toBe("DEPRECATED");
    expect(body.error).toContain("Go engine is sole execution authority");
  });

  it("returns 200 with storage:'mongo' for a valid closed trade", async () => {
    const trade = makeValidTrade();
    const res = await POST(makeRequest({ accountKey: "anon_test-key", trade }));
    expect(res.status).toBe(200);
    const body = await res.json() as {
      ok: boolean;
      storage: string;
      clientTradeId: string;
      upsertedCount: number;
    };
    expect(body.ok).toBe(true);
    expect(body.storage).toBe("mongo");
    expect(body.clientTradeId).toBe(trade.clientTradeId);
    expect(body.upsertedCount).toBe(1);
  });

  it("returns 503 MONGO_NOT_CONFIGURED when Mongo is not configured", async () => {
    vi.mocked(mongoClient.isMongoConfigured).mockReturnValue(false);
    const res = await POST(makeRequest({ accountKey: "anon_test-key", trade: makeValidTrade() }));
    expect(res.status).toBe(503);
    const body = await res.json() as { ok: boolean; code: string };
    expect(body.ok).toBe(false);
    expect(body.code).toBe("MONGO_NOT_CONFIGURED");
  });

  it("returns 500 MONGO_WRITE_FAILED when upsert returns ok:false", async () => {
    vi.mocked(mongoClient.upsertTradeMongo).mockResolvedValue({
      ok: false,
      error: "connection refused",
    });
    const res = await POST(makeRequest({ accountKey: "anon_test-key", trade: makeValidTrade() }));
    expect(res.status).toBe(500);
    const body = await res.json() as { ok: boolean; code: string; message: string };
    expect(body.ok).toBe(false);
    expect(body.code).toBe("MONGO_WRITE_FAILED");
    expect(body.message).toContain("connection refused");
  });

  it("returns 401 AUTH_REQUIRED when no session and anon disabled", async () => {
    delete process.env.ALLOW_PAPER_TRADES_ANON;
    delete process.env.ALLOW_ANON_PAPER_TRADES;
    const res = await POST(makeRequest({ accountKey: "anon_key", trade: makeValidTrade() }));
    expect(res.status).toBe(401);
    const body = await res.json() as { ok: boolean; code: string };
    expect(body.ok).toBe(false);
    expect(body.code).toBe("AUTH_REQUIRED");
  });

  it("returns 401 AUTH_REQUIRED when no accountKey and no session", async () => {
    const res = await POST(makeRequest({ trade: makeValidTrade() }));
    expect(res.status).toBe(401);
    const body = await res.json() as { ok: boolean; code: string };
    expect(body.ok).toBe(false);
    expect(body.code).toBe("AUTH_REQUIRED");
  });

  it("returns 400 VALIDATION_FAILED when clientTradeId is not a valid UUID", async () => {
    const trade = { ...makeValidTrade(), clientTradeId: "not-a-uuid" };
    const res = await POST(makeRequest({ accountKey: "anon_key", trade }));
    expect(res.status).toBe(400);
    const body = await res.json() as { ok: boolean; code: string };
    expect(body.ok).toBe(false);
    expect(body.code).toBe("VALIDATION_FAILED");
  });

  it("returns 400 INVALID_JSON on malformed body", async () => {
    const req = new Request("http://localhost/api/paper-trades", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "not-json{{",
    });
    const res = await POST(req);
    expect(res.status).toBe(400);
    const body = await res.json() as { code: string };
    expect(body.code).toBe("INVALID_JSON");
  });
});
