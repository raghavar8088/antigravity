import { describe, expect, it } from "vitest";
import { DEFAULT_MOCK_ACCOUNT_KEY, type MockTradeListQuery } from "@/lib/mockTradingPersistenceTypes";
import { mockTradeMongoFilterForQuery } from "@/lib/mockTradingMongo";

function query(overrides: Partial<MockTradeListQuery>): MockTradeListQuery {
  return {
    account_key: DEFAULT_MOCK_ACCOUNT_KEY,
    page: 1,
    limit: 100,
    sort: "newest",
    ...overrides,
  };
}

describe("mockTradeMongoFilterForQuery", () => {
  it("adds a less-than age expression against openedAt/current time", () => {
    const filter = mockTradeMongoFilterForQuery(query({
      age_mode: "less",
      age_max_minutes: 15,
    }), 1_700_000_000_000);

    expect(filter).toMatchObject({
      account_key: DEFAULT_MOCK_ACCOUNT_KEY,
      $and: [
        {
          $expr: {
            $lt: [expect.any(Object), 900_000],
          },
        },
      ],
    });
  });

  it("adds a more-than age expression with existing filters", () => {
    const filter = mockTradeMongoFilterForQuery(query({
      status: "CLOSED",
      profitability: "profit",
      age_mode: "more",
      age_min_minutes: 30,
    }), 1_700_000_000_000);

    expect(filter).toMatchObject({
      account_key: DEFAULT_MOCK_ACCOUNT_KEY,
      status: "CLOSED",
      pnl_value: { $gt: 0 },
      $and: [
        {
          $expr: {
            $gt: [expect.any(Object), 1_800_000],
          },
        },
      ],
    });
  });
});
