import { describe, expect, it, beforeEach } from "vitest";
import {
  checkTestnetPlaceOrderRateLimit,
  recordTestnetPlaceOrder,
  resetTestnetPlaceOrderRateLimitForTests,
  TESTNET_PLACE_ORDER_LIMIT_PER_HOUR,
} from "./deltaTestnetRateLimit";

describe("testnet place-order rate limit", () => {
  beforeEach(() => {
    resetTestnetPlaceOrderRateLimitForTests();
  });

  it("allows up to limit per hour", () => {
    const userId = "user-a";
    const now = Date.now();
    for (let i = 0; i < TESTNET_PLACE_ORDER_LIMIT_PER_HOUR; i++) {
      const check = checkTestnetPlaceOrderRateLimit(userId, now + i);
      expect(check.allowed).toBe(true);
      recordTestnetPlaceOrder(userId, now + i);
    }
    const blocked = checkTestnetPlaceOrderRateLimit(userId, now + 100);
    expect(blocked.allowed).toBe(false);
    if (!blocked.allowed) {
      expect(blocked.retryAfterSec).toBeGreaterThan(0);
    }
  });
});
