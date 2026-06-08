import { describe, expect, it } from "vitest";
import { normalizePaperPositionSide, unrealizedPnlForOpenPosition } from "./paperDeskClient";

describe("normalizePaperPositionSide", () => {
  it("maps buy/sell and long/short", () => {
    expect(normalizePaperPositionSide("BUY")).toBe("LONG");
    expect(normalizePaperPositionSide("SELL")).toBe("SHORT");
    expect(normalizePaperPositionSide("LONG")).toBe("LONG");
    expect(normalizePaperPositionSide("SHORT")).toBe("SHORT");
  });
});

describe("unrealizedPnlForOpenPosition", () => {
  const longPos = { side: "BUY", entry_price: 60_000, size: 0.1 };

  it("computes positive PnL for long when mark is above entry", () => {
    expect(unrealizedPnlForOpenPosition(longPos, 61_000)).toBeCloseTo(100, 6);
  });

  it("computes negative PnL for short when mark is above entry", () => {
    expect(
      unrealizedPnlForOpenPosition({ side: "SELL", entry_price: 60_000, size: 0.1 }, 61_000),
    ).toBeCloseTo(-100, 6);
  });

  it("returns 0 when mark is invalid", () => {
    expect(unrealizedPnlForOpenPosition(longPos, 0)).toBe(0);
  });
});
