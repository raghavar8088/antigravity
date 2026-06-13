import { beforeEach, describe, expect, it, vi } from "vitest";
import { DEFAULT_MOCK_TRADING_CONFIG, type MockTrade } from "@/lib/trading/mockTradingEngine";

vi.mock("@/lib/broker/mongoTradesClient", () => ({
  isMongoConfigured: vi.fn().mockReturnValue(true),
}));

vi.mock("@/lib/trading/mockTradingMongo", () => ({
  listMockTrades: vi.fn().mockResolvedValue({
    trades: [],
    total: 0,
    page: 1,
    limit: 100,
    totalPages: 1,
  }),
  upsertMockTrade: vi.fn().mockResolvedValue({
    upsertedCount: 1,
    modifiedCount: 0,
    matchedCount: 0,
  }),
  getMockTrade: vi.fn().mockResolvedValue(null),
  closeMockTradeInMongo: vi.fn().mockResolvedValue(null),
}));

import { GET, POST } from "./route";
import { GET as GET_ONE, PATCH } from "./[id]/route";
import { POST as CLOSE } from "./[id]/close/route";
import * as mongoClient from "@/lib/broker/mongoTradesClient";
import * as mockMongo from "@/lib/trading/mockTradingMongo";

const openTrade: MockTrade = {
  id: "mock-trace-1",
  traceId: "trace-1",
  strategyId: 91,
  strategyName: "Trend_Continuation_Long",
  symbol: "BTCUSD",
  side: "BUY",
  notional: 10_000,
  quantity: 0.166,
  leverage: 25,
  marginUsed: 400,
  signalPrice: 60_000,
  entryPrice: 60_030,
  takeProfitPrice: 60_600,
  stopLossPrice: 59_700,
  takeProfitUsd: 10,
  stopLossUsd: 5,
  riskRewardRatio: 2,
  signalScore: 28,
  requiredThreshold: 20,
  blockers: [{ gate: "REGIME", reason: "regime blocked" }],
  status: "OPEN",
  openedAt: 1_700_000_000_000,
  closedAt: null,
  currentPrice: 60_050,
  unrealizedPnl: 3.2,
  realizedPnl: 0,
  fees: 0,
  fundingCosts: 0,
  exitReason: null,
  exitPrice: null,
};

const closedTrade: MockTrade = {
  ...openTrade,
  status: "CLOSED",
  closedAt: 1_700_000_060_000,
  currentPrice: 61_000,
  unrealizedPnl: 0,
  realizedPnl: 130,
  fees: 10,
  exitReason: "TAKE_PROFIT",
  exitPrice: 60_970,
};

function request(url: string, body?: unknown, method = "POST") {
  return new Request(url, {
    method,
    headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
}

describe("Mock Trading trade API routes", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(mongoClient.isMongoConfigured).mockReturnValue(true);
    vi.mocked(mockMongo.listMockTrades).mockResolvedValue({
      trades: [openTrade],
      total: 1,
      page: 1,
      limit: 100,
      totalPages: 1,
    });
    vi.mocked(mockMongo.upsertMockTrade).mockResolvedValue({
      upsertedCount: 1,
      modifiedCount: 0,
      matchedCount: 0,
    });
    vi.mocked(mockMongo.getMockTrade).mockResolvedValue(openTrade);
    vi.mocked(mockMongo.closeMockTradeInMongo).mockResolvedValue(closedTrade);
  });

  it("POST creates a mock trade in Mongo", async () => {
    const res = await POST(request("http://localhost/api/mock-trading/trades", {
      accountKey: "owner",
      trade: openTrade,
      config: DEFAULT_MOCK_TRADING_CONFIG,
    }));
    expect(res.status).toBe(200);
    expect(mockMongo.upsertMockTrade).toHaveBeenCalled();
  });

  it("passes sanitized pagination, filters, and sorting to Mongo", async () => {
    const res = await GET(new Request(
      "http://localhost/api/mock-trading/trades?page=2&limit=25&status=OPEN&side=BUY&strategy_id=91&blocker_gate=REGIME&age_mode=between&age_min_minutes=5&age_max_minutes=30&sort=most_profitable",
    ));
    expect(res.status).toBe(200);
    expect(mockMongo.listMockTrades).toHaveBeenCalledWith(expect.objectContaining({
      page: 2,
      limit: 25,
      status: "OPEN",
      side: "BUY",
      strategy_id: 91,
      blocker_gate: "REGIME",
      age_mode: "between",
      age_min_minutes: 5,
      age_max_minutes: 30,
      sort: "most_profitable",
    }));
  });

  it("rejects invalid age filter numbers", async () => {
    const res = await GET(new Request(
      "http://localhost/api/mock-trading/trades?age_mode=less&age_max_minutes=not-a-number",
    ));
    expect(res.status).toBe(400);
    const body = await res.json() as { code: string };
    expect(body.code).toBe("VALIDATION_FAILED");
    expect(mockMongo.listMockTrades).not.toHaveBeenCalled();
  });

  it("reads one mock trade by id", async () => {
    const res = await GET_ONE(
      new Request("http://localhost/api/mock-trading/trades/mock-trace-1"),
      { params: Promise.resolve({ id: openTrade.id }) },
    );
    expect(res.status).toBe(200);
    const body = await res.json() as { trade: MockTrade };
    expect(body.trade.id).toBe(openTrade.id);
  });

  it("PATCH is deprecated — returns 410", async () => {
    const res = await PATCH();
    expect(res.status).toBe(410);
    const body = await res.json() as { code: string };
    expect(body.code).toBe("DEPRECATED");
  });

  it("CLOSE persists a closed mock trade", async () => {
    const res = await CLOSE(
      request(`http://localhost/api/mock-trading/trades/${openTrade.id}/close`, {
        accountKey: "owner",
        trade: closedTrade,
        config: DEFAULT_MOCK_TRADING_CONFIG,
      }),
      { params: Promise.resolve({ id: openTrade.id }) },
    );
    expect(res.status).toBe(200);
    expect(mockMongo.upsertMockTrade).toHaveBeenCalled();
  });

  it("returns 503 when MongoDB is not configured", async () => {
    vi.mocked(mongoClient.isMongoConfigured).mockReturnValue(false);
    const res = await GET(new Request("http://localhost/api/mock-trading/trades"));
    expect(res.status).toBe(503);
    const body = await res.json() as { code: string };
    expect(body.code).toBe("MONGO_NOT_CONFIGURED");
  });
});
