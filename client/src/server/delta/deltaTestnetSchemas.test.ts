import { describe, expect, it } from "vitest";
import {
  testnetCancelOrderBodySchema,
  testnetPlaceOrderBodySchema,
} from "./deltaTestnetSchemas";

describe("testnetPlaceOrderBodySchema", () => {
  it("accepts market BTCUSD order", () => {
    const parsed = testnetPlaceOrderBodySchema.safeParse({
      symbol: "btcusd",
      side: "buy",
      size: 1,
      type: "market",
    });
    expect(parsed.success).toBe(true);
    if (parsed.success) {
      expect(parsed.data.symbol).toBe("BTCUSD");
    }
  });

  it("requires price for limit", () => {
    const parsed = testnetPlaceOrderBodySchema.safeParse({
      symbol: "BTCUSD",
      side: "sell",
      size: 2,
      type: "limit",
    });
    expect(parsed.success).toBe(false);
  });

  it("rejects price on market", () => {
    const parsed = testnetPlaceOrderBodySchema.safeParse({
      symbol: "BTCUSD",
      side: "buy",
      size: 1,
      type: "market",
      price: 90_000,
    });
    expect(parsed.success).toBe(false);
  });

  it("accepts limit with price", () => {
    const parsed = testnetPlaceOrderBodySchema.safeParse({
      symbol: "BTCUSD",
      side: "buy",
      size: 1,
      type: "limit",
      price: 50_000,
    });
    expect(parsed.success).toBe(true);
  });
});

describe("testnetCancelOrderBodySchema", () => {
  it("accepts string or numeric orderId", () => {
    expect(testnetCancelOrderBodySchema.safeParse({ orderId: "42" }).success).toBe(true);
    expect(testnetCancelOrderBodySchema.safeParse({ orderId: 42 }).success).toBe(true);
  });
});
