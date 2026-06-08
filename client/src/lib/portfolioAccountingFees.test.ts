import { describe, expect, it } from "vitest";
import { canonicalNetPnl, canonicalTradeFees, resolveTradeFeeLegs } from "@/lib/portfolioAccountingFees";

describe("resolveTradeFeeLegs", () => {
  it("prefers explicit entry/exit fee fields", () => {
    const legs = resolveTradeFeeLegs({
      entry_price: 100_000,
      exit_price: 101_000,
      quantity: 0.01,
      entry_fee: 5,
      exit_fee: 5.05,
    });
    expect(legs.entry_fee).toBe(5);
    expect(legs.exit_fee).toBe(5.05);
  });

  it("splits legacy fees evenly", () => {
    const legs = resolveTradeFeeLegs({
      entry_price: 100_000,
      exit_price: 101_000,
      quantity: 0.01,
      fees: 10,
    });
    expect(legs.total_fee).toBe(10);
    expect(legs.entry_fee).toBe(5);
    expect(legs.exit_fee).toBe(5);
  });

  it("computes canonical fees when no legacy data", () => {
    const legs = resolveTradeFeeLegs({
      entry_price: 100_000,
      exit_price: 101_000,
      quantity: 0.01,
    });
    expect(legs.total_fee).toBeCloseTo(canonicalTradeFees(100_000, 101_000, 0.01).total_fee, 6);
    expect(canonicalNetPnl(100, legs)).toBeCloseTo(100 - legs.total_fee, 6);
  });
});
