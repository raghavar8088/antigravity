import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { PaperTradeDbRow } from "@/lib/paperTradesTypes";

afterEach(() => {
  vi.unstubAllEnvs();
  vi.resetModules();
});

describe("isMongoConfigured", () => {
  it("returns false when MONGODB_URI is unset or empty", async () => {
    vi.stubEnv("MONGODB_URI", "");
    const { isMongoConfigured } = await import("./mongoTradesClient");
    expect(isMongoConfigured()).toBe(false);
  });

  it("returns true when MONGODB_URI is set", async () => {
    vi.stubEnv("MONGODB_URI", "mongodb+srv://user:pass@cluster.test.mongodb.net/?retryWrites=true");
    const { isMongoConfigured } = await import("./mongoTradesClient");
    expect(isMongoConfigured()).toBe(true);
  });

  it("treats whitespace-only URI as unconfigured", async () => {
    vi.stubEnv("MONGODB_URI", "   ");
    const { isMongoConfigured } = await import("./mongoTradesClient");
    expect(isMongoConfigured()).toBe(false);
  });
});

// --------------------------------------------------------------------------
// Mocked driver — verify our helpers use the right mongodb API surface.
// --------------------------------------------------------------------------

type MockCollection = {
  createIndex: ReturnType<typeof vi.fn>;
  updateOne: ReturnType<typeof vi.fn>;
  find: ReturnType<typeof vi.fn>;
};

function buildMockCollection(returnRows: PaperTradeDbRow[] = []): MockCollection {
  const cursor = {
    sort: vi.fn().mockReturnThis(),
    limit: vi.fn().mockReturnThis(),
    toArray: vi.fn().mockResolvedValue(returnRows),
  };
  return {
    createIndex: vi.fn().mockResolvedValue("ok"),
    updateOne: vi.fn().mockResolvedValue({ upsertedCount: 1 }),
    find: vi.fn().mockReturnValue(cursor),
  };
}

function sampleRow(overrides: Partial<PaperTradeDbRow> = {}): Omit<PaperTradeDbRow, "id" | "created_at"> {
  return {
    account_key: "user-1",
    client_trade_id: "00000000-0000-0000-0000-000000000001",
    opened_at: "2026-05-17T10:00:00.000Z",
    closed_at: "2026-05-17T10:30:00.000Z",
    symbol: "BTCUSDT",
    strategy_id: 91,
    strategy_name: "TestStrat",
    side: "LONG",
    entry_price: 100_000,
    exit_price: 100_500,
    contracts: 1,
    notional: 100_000,
    margin_used: 4_000,
    gross_pnl: 500,
    fees: 5,
    funding_costs: 0,
    net_pnl: 495,
    exit_reason: "TP",
    payload: { foo: "bar" },
    ...overrides,
  };
}

describe("upsertTradeMongo", () => {
  let mockCol: MockCollection;

  beforeEach(() => {
    mockCol = buildMockCollection();
    vi.doMock("mongodb", () => ({
      MongoClient: vi.fn().mockImplementation(() => ({
        connect: vi.fn().mockResolvedValue(undefined),
        db: vi.fn().mockReturnValue({
          collection: vi.fn().mockReturnValue(mockCol),
        }),
        close: vi.fn().mockResolvedValue(undefined),
      })),
    }));
    vi.stubEnv("MONGODB_URI", "mongodb+srv://user:pass@cluster.test.mongodb.net/?retryWrites=true");
    vi.stubEnv("MONGODB_DB", "test_db");
  });

  it("upserts by client_trade_id with $set for row fields and $setOnInsert for created_at", async () => {
    const { upsertTradeMongo, _closeMongoForTests } = await import("./mongoTradesClient");
    const row = sampleRow();
    const result = await upsertTradeMongo(row);

    expect(result.ok).toBe(true);
    expect(mockCol.updateOne).toHaveBeenCalledOnce();
    const [filter, update, opts] = mockCol.updateOne.mock.calls[0]!;
    expect(filter).toEqual({ client_trade_id: row.client_trade_id });
    expect(opts).toEqual({ upsert: true });
    const u = update as { $set: Record<string, unknown>; $setOnInsert: Record<string, unknown> };
    expect(u.$set).toMatchObject(row);
    expect(u.$setOnInsert).toHaveProperty("created_at");
    // created_at must only be in $setOnInsert so updates don't overwrite the original insert timestamp.
    expect(u.$set).not.toHaveProperty("created_at");

    await _closeMongoForTests();
  });

  it("creates the unique + per-account + per-module indexes on first access", async () => {
    const { upsertTradeMongo, _closeMongoForTests } = await import("./mongoTradesClient");
    await upsertTradeMongo(sampleRow());

    expect(mockCol.createIndex).toHaveBeenCalledTimes(4);
    const indexCalls = mockCol.createIndex.mock.calls.map((c) => c[0]);
    expect(indexCalls).toContainEqual({ client_trade_id: 1 });
    expect(indexCalls).toContainEqual({ account_key: 1, closed_at: -1 });
    expect(indexCalls).toContainEqual({ account_key: 1, module_key: 1, closed_at: -1 });

    await _closeMongoForTests();
  });

  it("reuses the cached client across multiple writes (no reconnect)", async () => {
    const { upsertTradeMongo, _closeMongoForTests } = await import("./mongoTradesClient");
    await upsertTradeMongo(sampleRow());
    await upsertTradeMongo(sampleRow({ client_trade_id: "00000000-0000-0000-0000-000000000002" }));

    // createIndex should only have run once (per first call) — second call uses cached indexesEnsured
    expect(mockCol.createIndex).toHaveBeenCalledTimes(4);
    expect(mockCol.updateOne).toHaveBeenCalledTimes(2);

    await _closeMongoForTests();
  });

  it("returns { ok:false, error } when MONGODB_URI is missing", async () => {
    vi.stubEnv("MONGODB_URI", "");
    const { upsertTradeMongo } = await import("./mongoTradesClient");
    const result = await upsertTradeMongo(sampleRow());
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error).toMatch(/MONGODB_URI/);
    }
  });
});

describe("listTradesMongo", () => {
  let mockCol: MockCollection;
  const ROW: PaperTradeDbRow = {
    id: "00000000-0000-0000-0000-aaaaaaaaaaaa",
    created_at: "2026-05-17T10:30:00.000Z",
    ...sampleRow(),
  };

  beforeEach(() => {
    mockCol = buildMockCollection([ROW]);
    vi.doMock("mongodb", () => ({
      MongoClient: vi.fn().mockImplementation(() => ({
        connect: vi.fn().mockResolvedValue(undefined),
        db: vi.fn().mockReturnValue({
          collection: vi.fn().mockReturnValue(mockCol),
        }),
        close: vi.fn().mockResolvedValue(undefined),
      })),
    }));
    vi.stubEnv("MONGODB_URI", "mongodb+srv://user:pass@cluster.test.mongodb.net/?retryWrites=true");
  });

  it("filters by accountKey, sorts closed_at desc, applies limit", async () => {
    const { listTradesMongo, _closeMongoForTests } = await import("./mongoTradesClient");
    const rows = await listTradesMongo({ accountKey: "user-1", limit: 50 });

    expect(rows).toEqual([ROW]);
    expect(mockCol.find).toHaveBeenCalledWith({ account_key: "user-1" });
    const cursor = mockCol.find.mock.results[0]!.value as ReturnType<typeof buildMockCollection>["find"] extends (...args: unknown[]) => infer R ? R : never;
    // sort + limit + toArray
    expect((cursor as { sort: ReturnType<typeof vi.fn> }).sort).toHaveBeenCalledWith({ closed_at: -1 });
    expect((cursor as { limit: ReturnType<typeof vi.fn> }).limit).toHaveBeenCalledWith(50);

    await _closeMongoForTests();
  });

  it("applies cursor as a closed_at < cursor filter", async () => {
    const { listTradesMongo, _closeMongoForTests } = await import("./mongoTradesClient");
    await listTradesMongo({ accountKey: "user-2", limit: 10, cursor: "2026-05-17T09:00:00.000Z" });

    expect(mockCol.find).toHaveBeenCalledWith({
      account_key: "user-2",
      closed_at: { $lt: "2026-05-17T09:00:00.000Z" },
    });

    await _closeMongoForTests();
  });
});
