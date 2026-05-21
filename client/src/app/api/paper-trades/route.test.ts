import { describe, it, expect, vi, beforeEach } from "vitest";
import { randomUUID } from "crypto";

vi.mock("@/lib/mongoTradesClient", () => ({
  isMongoConfigured: vi.fn().mockReturnValue(true),
  upsertTradeMongo: vi.fn().mockResolvedValue(undefined),
  listTradesMongo: vi.fn().mockResolvedValue([]),
}));

import { POST } from "./route";
import * as mongoClient from "@/lib/mongoTradesClient";

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
    vi.mocked(mongoClient.isMongoConfigured).mockReturnValue(true);
    vi.mocked(mongoClient.upsertTradeMongo).mockResolvedValue(undefined as never);
  });

  it("returns 200 with ok:true for a valid closed trade", async () => {
    const res = await POST(makeRequest({ accountKey: "anon_test-key", trade: makeValidTrade() }));
    expect(res.status).toBe(200);
    const body = await res.json() as { ok: boolean; persistedTo: { mongo: boolean }; clientTradeId: string };
    expect(body.ok).toBe(true);
    expect(body.persistedTo.mongo).toBe(true);
    expect(typeof body.clientTradeId).toBe("string");
  });

  it("returns 503 when Mongo is not configured", async () => {
    vi.mocked(mongoClient.isMongoConfigured).mockReturnValue(false);
    const res = await POST(makeRequest({ accountKey: "anon_test-key", trade: makeValidTrade() }));
    expect(res.status).toBe(503);
    const body = await res.json() as { ok: boolean };
    expect(body.ok).toBe(false);
  });

  it("returns 400 when accountKey is missing", async () => {
    const res = await POST(makeRequest({ trade: makeValidTrade() }));
    expect(res.status).toBe(400);
  });

  it("returns 400 on invalid JSON body", async () => {
    const req = new Request("http://localhost/api/paper-trades", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "not-json{{",
    });
    const res = await POST(req);
    expect(res.status).toBe(400);
  });

  it("returns 400 when clientTradeId is not a valid UUID", async () => {
    const trade = { ...makeValidTrade(), clientTradeId: "not-a-uuid" };
    const res = await POST(makeRequest({ accountKey: "anon_key", trade }));
    expect(res.status).toBe(400);
    const body = await res.json() as { ok: boolean; error: string };
    expect(body.ok).toBe(false);
    expect(body.error).toBe("Validation failed");
  });

  it("returns 500 when Mongo upsert throws", async () => {
    vi.mocked(mongoClient.upsertTradeMongo).mockRejectedValue(new Error("connection refused"));
    const res = await POST(makeRequest({ accountKey: "anon_key", trade: makeValidTrade() }));
    expect(res.status).toBe(500);
    const body = await res.json() as { ok: boolean; detail: string };
    expect(body.ok).toBe(false);
    expect(body.detail).toContain("connection refused");
  });
});
