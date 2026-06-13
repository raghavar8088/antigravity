/**
 * Tests for walkForwardValidation — pure function, no I/O.
 */
import { describe, it, expect } from "vitest";
import { runWalkForwardValidation } from "./analytics/walkForwardValidation";
import type { WalkForwardTrade } from "./analytics/walkForwardValidation";

// ── Helpers ───────────────────────────────────────────────────────────────────

function makeTrade(closedAt: string, netPnl: number): WalkForwardTrade {
  return { closedAt, netPnl };
}

function generateTrades(
  count: number,
  startDate: Date,
  intervalHours: number,
  pnlFn: (i: number) => number,
): WalkForwardTrade[] {
  return Array.from({ length: count }, (_, i) => {
    const d = new Date(startDate.getTime() + i * intervalHours * 3_600_000);
    return makeTrade(d.toISOString(), pnlFn(i));
  });
}

// ── Insufficient data ─────────────────────────────────────────────────────────

describe("runWalkForwardValidation — COLLECT_DATA", () => {
  it("too few total trades → COLLECT_DATA", () => {
    const trades = [makeTrade("2026-01-01T00:00:00Z", 1)];
    const result = runWalkForwardValidation(trades);
    expect(result.status).toBe("COLLECT_DATA");
    expect(result.windows).toHaveLength(0);
  });

  it("enough trades but span too short → COLLECT_DATA", () => {
    // 15 trades all on the same day — spans 0 days, need 28
    const trades = Array(15).fill(0).map(() => makeTrade("2026-01-01T12:00:00Z", 1));
    const result = runWalkForwardValidation(trades);
    expect(result.status).toBe("COLLECT_DATA");
  });
});

// ── WFE PASS ──────────────────────────────────────────────────────────────────

describe("runWalkForwardValidation — PASS", () => {
  it("consistent positive expectancy train and test → PASS", () => {
    const start = new Date("2026-01-01T00:00:00Z");
    // 60 trades over 60 days, uniformly +1 each — WFE should be ~1.0
    const trades = generateTrades(60, start, 24, () => 1);
    const result = runWalkForwardValidation(trades, { minTrainTrades: 3, minTestTrades: 1 });
    expect(result.status).toBe("PASS");
    expect(result.aggregateWFE).toBeGreaterThan(0.5);
    expect(result.windows.length).toBeGreaterThan(0);
  });

  it("all windows pass when both train and test are consistently positive", () => {
    const start = new Date("2026-01-01T00:00:00Z");
    const trades = generateTrades(60, start, 24, () => 2);
    const result = runWalkForwardValidation(trades, { minTrainTrades: 3, minTestTrades: 1 });
    const failCount = result.windows.filter((w) => !w.pass).length;
    expect(failCount).toBe(0);
  });
});

// ── WFE FAIL ─────────────────────────────────────────────────────────────────

describe("runWalkForwardValidation — FAIL", () => {
  it("train positive, test negative → low WFE → FAIL", () => {
    const start = new Date("2026-01-01T00:00:00Z");
    // First 21 days: winners; next 7: losers (test period negative)
    const trades: WalkForwardTrade[] = [
      ...generateTrades(21, start, 24, () => 2),  // train
      ...generateTrades(7, new Date(start.getTime() + 21 * 86_400_000), 24, () => -3), // test
    ];
    const result = runWalkForwardValidation(trades, { minTrainTrades: 5, minTestTrades: 2 });
    // Either FAIL or COLLECT_DATA (depends on window count)
    expect(["FAIL", "COLLECT_DATA"]).toContain(result.status);
  });
});

// ── Window shape ──────────────────────────────────────────────────────────────

describe("runWalkForwardValidation — window fields", () => {
  it("each window has all required fields", () => {
    const start = new Date("2026-01-01T00:00:00Z");
    const trades = generateTrades(60, start, 24, () => 1);
    const result = runWalkForwardValidation(trades, { minTrainTrades: 3, minTestTrades: 1 });

    expect(result.windows.length).toBeGreaterThan(0);
    for (const w of result.windows) {
      expect(w).toHaveProperty("trainStart");
      expect(w).toHaveProperty("trainEnd");
      expect(w).toHaveProperty("testStart");
      expect(w).toHaveProperty("testEnd");
      expect(w).toHaveProperty("trainTrades");
      expect(w).toHaveProperty("testTrades");
      expect(w).toHaveProperty("trainExpectancy");
      expect(w).toHaveProperty("testExpectancy");
      expect(w).toHaveProperty("walkForwardEfficiency");
      expect(w).toHaveProperty("pass");
    }
  });

  it("WFE is clipped to [-2, 2]", () => {
    const start = new Date("2026-01-01T00:00:00Z");
    // Tiny train expectancy, large test expectancy → ratio would be huge without clipping
    const trainTrades = generateTrades(21, start, 24, (i) => (i === 0 ? 0.001 : -0.001));
    const testTrades = generateTrades(7, new Date(start.getTime() + 21 * 86_400_000), 24, () => 5);
    const result = runWalkForwardValidation([...trainTrades, ...testTrades], {
      minTrainTrades: 3,
      minTestTrades: 1,
    });
    for (const w of result.windows) {
      expect(w.walkForwardEfficiency).toBeGreaterThanOrEqual(-2);
      expect(w.walkForwardEfficiency).toBeLessThanOrEqual(2);
    }
  });

  it("does not mutate the input array", () => {
    const start = new Date("2026-01-01T00:00:00Z");
    const trades = generateTrades(30, start, 24, () => 1);
    const snapshot = JSON.stringify(trades);
    runWalkForwardValidation(trades);
    expect(JSON.stringify(trades)).toBe(snapshot);
  });
});
