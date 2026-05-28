/**
 * Tests for promotionGate — pure function, no I/O.
 */
import { describe, it, expect } from "vitest";
import { canPromoteStrategy } from "./promotionGate";

// ─── Helpers ─────────────────────────────────────────────────────────────────

function makeInput(overrides: Partial<Parameters<typeof canPromoteStrategy>[0]> = {}) {
  return {
    paperTrades: 25,
    paperExpectancy: 2.5,
    replayTrades: 25,
    replayExpectancy: 1.8,
    walkForwardPass: true,
    feePctOfAbsGross: 30,
    ...overrides,
  };
}

// ─── Passing case ─────────────────────────────────────────────────────────────

describe("canPromoteStrategy — all criteria pass", () => {
  it("returns pass=true when all 6 criteria are met", () => {
    const result = canPromoteStrategy(makeInput());
    expect(result.pass).toBe(true);
    expect(result.criteria.every((c) => c.pass)).toBe(true);
    expect(result.reason).toContain("All 6 promotion criteria met");
  });
});

// ─── Individual failures ──────────────────────────────────────────────────────

describe("canPromoteStrategy — individual failures", () => {
  it("fails when paperTrades < 20", () => {
    const result = canPromoteStrategy(makeInput({ paperTrades: 19 }));
    expect(result.pass).toBe(false);
    expect(result.reason).toContain("paperTrades");
    expect(result.criteria.find((c) => c.name === "paperTrades ≥ 20")?.pass).toBe(false);
  });

  it("fails when paperExpectancy ≤ 0", () => {
    const result = canPromoteStrategy(makeInput({ paperExpectancy: 0 }));
    expect(result.pass).toBe(false);
    expect(result.reason).toContain("paperExpectancy");
  });

  it("fails when replayTrades < 20", () => {
    const result = canPromoteStrategy(makeInput({ replayTrades: 5 }));
    expect(result.pass).toBe(false);
    expect(result.reason).toContain("replayTrades");
  });

  it("fails when replayExpectancy ≤ 0", () => {
    const result = canPromoteStrategy(makeInput({ replayExpectancy: -1 }));
    expect(result.pass).toBe(false);
    expect(result.reason).toContain("replayExpectancy");
  });

  it("fails when walkForwardPass is false", () => {
    const result = canPromoteStrategy(makeInput({ walkForwardPass: false }));
    expect(result.pass).toBe(false);
    expect(result.reason).toContain("walkForwardPass");
    const wfCrit = result.criteria.find((c) => c.name === "walkForwardPass");
    expect(wfCrit?.pass).toBe(false);
    expect(wfCrit?.detail).toContain("FAIL");
  });

  it("fails when feePctOfAbsGross > 50", () => {
    const result = canPromoteStrategy(makeInput({ feePctOfAbsGross: 51 }));
    expect(result.pass).toBe(false);
    expect(result.reason).toContain("feePctOfAbsGross");
  });
});

// ─── Multi-failure ────────────────────────────────────────────────────────────

describe("canPromoteStrategy — multiple failures", () => {
  it("reports all failing criteria in reason", () => {
    const result = canPromoteStrategy(makeInput({
      paperTrades: 5,
      paperExpectancy: -1,
      walkForwardPass: false,
    }));
    expect(result.pass).toBe(false);
    const failingNames = result.criteria.filter((c) => !c.pass).map((c) => c.name);
    expect(failingNames.length).toBeGreaterThanOrEqual(3);
    for (const name of failingNames) {
      expect(result.reason).toContain(name);
    }
  });
});

// ─── Criteria shape ───────────────────────────────────────────────────────────

describe("canPromoteStrategy — criteria array", () => {
  it("always returns exactly 6 criteria", () => {
    const result = canPromoteStrategy(makeInput());
    expect(result.criteria).toHaveLength(6);
  });

  it("each criterion has name, pass, detail", () => {
    const result = canPromoteStrategy(makeInput());
    for (const c of result.criteria) {
      expect(typeof c.name).toBe("string");
      expect(typeof c.pass).toBe("boolean");
      expect(typeof c.detail).toBe("string");
    }
  });
});
