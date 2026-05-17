import { describe, expect, it } from "vitest";
import { paperEnsureThresholdDrop } from "./useBTCFuturesScalperEngine";

describe("paperEnsureThresholdDrop", () => {
  const mounted = 1_000_000;

  it("returns 0 when disabled", () => {
    expect(paperEnsureThresholdDrop(false, mounted + 20 * 60_000, mounted, 0)).toBe(0);
  });

  it("ramps down threshold over time for quiet strats", () => {
    expect(paperEnsureThresholdDrop(true, mounted + 3 * 60_000, mounted, 0)).toBe(4);
    expect(paperEnsureThresholdDrop(true, mounted + 10 * 60_000, mounted, 0)).toBe(8);
    expect(paperEnsureThresholdDrop(true, mounted + 20 * 60_000, mounted, 0)).toBe(10);
  });

  it("stops dropping once strat has 2+ trades", () => {
    expect(paperEnsureThresholdDrop(true, mounted + 20 * 60_000, mounted, 2)).toBe(0);
  });
});
