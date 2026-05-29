import { describe, expect, it } from "vitest";
import { MOCK_TRADE_TABLE_REQUIRED_HEADERS } from "@/components/MockTradingDashboard";

describe("MockTradingDashboard trade table", () => {
  it("includes fixed-dollar TP/SL columns", () => {
    expect([...MOCK_TRADE_TABLE_REQUIRED_HEADERS]).toEqual([
      "TP Price",
      "SL Price",
      "TP $",
      "SL $",
      "Risk/Reward",
      "Exit Reason",
    ]);
  });
});
