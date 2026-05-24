import { describe, expect, it } from "vitest";
import { paperEnsureThresholdDrop, paperQuietEntryBoost } from "./useBTCFuturesScalperEngine";

describe("paperEnsureThresholdDrop", () => {
  const mounted = 1_000_000;

  it("returns 0 when disabled", () => {
    expect(paperEnsureThresholdDrop(false, mounted + 20 * 60_000, mounted, 0)).toBe(0);
  });

  it("ramps down threshold over time for quiet strats", () => {
    expect(paperEnsureThresholdDrop(true, mounted + 3 * 60_000, mounted, 0)).toBe(6);
    expect(paperEnsureThresholdDrop(true, mounted + 6 * 60_000, mounted, 0)).toBe(10);
    expect(paperEnsureThresholdDrop(true, mounted + 11 * 60_000, mounted, 0)).toBe(12);
  });

  it("stops dropping once strat has 5+ trades", () => {
    expect(paperEnsureThresholdDrop(true, mounted + 20 * 60_000, mounted, 5)).toBe(0);
  });

  it("halves drop for strats that already have samples", () => {
    expect(paperEnsureThresholdDrop(true, mounted + 11 * 60_000, mounted, 2)).toBe(6);
  });
});

describe("paperQuietEntryBoost", () => {
  const now = 1_000_000;

  it("returns 0 when positions are open", () => {
    expect(paperQuietEntryBoost(true, now, now - 600_000, 1)).toBe(0);
  });

  it("ramps boost when desk is idle after last close", () => {
    expect(paperQuietEntryBoost(true, now, now - 3 * 60_000, 0)).toBe(3);
    expect(paperQuietEntryBoost(true, now, now - 6 * 60_000, 0)).toBe(4);
    expect(paperQuietEntryBoost(true, now, now - 11 * 60_000, 0)).toBe(6);
  });
});
