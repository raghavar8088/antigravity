/**
 * FEAT 8 — Unit tests for futuresPaperMath.ts
 * Run with: cd client && npx vitest run src/lib/trading/__tests__/futuresPaperMath.test.ts
 */
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  paperLiquidationPrice,
  paperApplyEntrySlippage,
  paperApplyExitSlippage,
  paperComputeAccruedFunding,
} from "../futuresPaperMath";

describe("paperLiquidationPrice", () => {
  it("LONG at 25x: liq price is below entry", () => {
    // initialMarginRate = 1/25 = 0.04, maintenanceMarginRate = 0.005
    // liqPrice = entry * (1 - 0.04 + 0.005) = entry * 0.965
    const liq = paperLiquidationPrice("LONG", 100_000, 25);
    expect(liq).toBeCloseTo(100_000 * 0.965, 2);
    expect(liq).toBeLessThan(100_000);
  });

  it("SHORT at 25x: liq price is above entry", () => {
    // liqPrice = entry * (1 + 0.04 - 0.005) = entry * 1.035
    const liq = paperLiquidationPrice("SHORT", 100_000, 25);
    expect(liq).toBeCloseTo(100_000 * 1.035, 2);
    expect(liq).toBeGreaterThan(100_000);
  });

  it("returns 0 for invalid entry", () => {
    expect(paperLiquidationPrice("LONG", 0, 10)).toBe(0);
    expect(paperLiquidationPrice("LONG", -1, 10)).toBe(0);
  });
});

describe("paperApplyEntrySlippage", () => {
  it("LONG entry adds slippage (price goes up)", () => {
    const result = paperApplyEntrySlippage("LONG", 100_000, 10);
    // 10 bps = 0.001, price * 1.001
    expect(result).toBeCloseTo(100_100, 1);
    expect(result).toBeGreaterThan(100_000);
  });

  it("SHORT entry subtracts slippage (price goes down)", () => {
    const result = paperApplyEntrySlippage("SHORT", 100_000, 10);
    expect(result).toBeCloseTo(99_900, 1);
    expect(result).toBeLessThan(100_000);
  });

  it("dynamic scaling with atrPct increases slippage", () => {
    // atrPct = 0.005 → dynamic = baseBps * (1 + 0.005/0.005) = baseBps * 2
    const base = paperApplyEntrySlippage("LONG", 100_000, 10);
    const dynamic = paperApplyEntrySlippage("LONG", 100_000, 10, 0.005);
    expect(dynamic).toBeGreaterThan(base);
  });

  it("dynamic scaling is capped at 5x base", () => {
    // atrPct = 0.1 → would be baseBps * 21 but capped at baseBps * 5
    const capped = paperApplyEntrySlippage("LONG", 100_000, 10, 0.1);
    const maxAllowed = paperApplyEntrySlippage("LONG", 100_000, 50); // 5x10 bps
    expect(capped).toBeCloseTo(maxAllowed, 2);
  });
});

describe("paperApplyExitSlippage", () => {
  it("LONG exit receives less (adverse)", () => {
    const result = paperApplyExitSlippage("LONG", 100_000, 10);
    expect(result).toBeCloseTo(99_900, 1);
    expect(result).toBeLessThan(100_000);
  });

  it("SHORT exit covers higher (adverse)", () => {
    const result = paperApplyExitSlippage("SHORT", 100_000, 10);
    expect(result).toBeCloseTo(100_100, 1);
    expect(result).toBeGreaterThan(100_000);
  });

  it("zero slippageBps returns price unchanged", () => {
    expect(paperApplyExitSlippage("LONG", 100_000, 0)).toBe(100_000);
    expect(paperApplyExitSlippage("SHORT", 100_000, 0)).toBe(100_000);
  });
});

describe("paperComputeAccruedFunding", () => {
  const FUNDING_INTERVAL_MS = 8 * 60 * 60 * 1000; // 8h

  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns 0 for 0 elapsed windows", () => {
    const openedAt = Date.now();
    // Set current time to 7h later — less than one 8h window.
    vi.setSystemTime(openedAt + 7 * 60 * 60 * 1000);
    const accrued = paperComputeAccruedFunding(openedAt, 100_000, 0.01);
    expect(accrued).toBe(0);
  });

  it("computes correct amount for 3 windows", () => {
    const openedAt = Date.now();
    // Set current time to 25h later → floor(25/8) = 3 windows.
    vi.setSystemTime(openedAt + 25 * 60 * 60 * 1000);
    const accrued = paperComputeAccruedFunding(openedAt, 100_000, 0.01);
    // 3 * (0.01/100) * 100_000 = 3 * 10 = 30
    expect(accrued).toBeCloseTo(30, 4);
  });

  it("returns 0 for invalid inputs", () => {
    expect(paperComputeAccruedFunding(NaN, 100_000, 0.01)).toBe(0);
    expect(paperComputeAccruedFunding(Date.now(), 0, 0.01)).toBe(0);
    expect(paperComputeAccruedFunding(Date.now(), 100_000, NaN)).toBe(0);
  });
});
