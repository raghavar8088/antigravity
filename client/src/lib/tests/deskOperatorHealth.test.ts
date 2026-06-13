import { describe, it, expect } from "vitest";
import {
  computeDeskOperatorHealth,
  type DeskOperatorHealthInput,
} from "../trading/deskOperatorHealth";

const base: DeskOperatorHealthInput = {
  workerEnabled: true,
  workerLastPollAt: Date.now() - 5_000,
  workerStale: false,
  balanceDriftUsd: 0,
  totalTrades: 60,
  openPositions: 0,
  closedTrades48h: 10,
  regime: "trendHigh",
  skipRegime: 0,
  skipAtrFees: 0,
  rotationSuspended: 0,
  rotationPending: 0,
  rotationActive: 3,
  rotationPromoted: 1,
  pauseEntries: false,
  drawdownLocked: false,
  probeDominant: false,
  scorecardState: "ON_TRACK",
};

describe("computeDeskOperatorHealth — NEEDS_REPAIR", () => {
  it("triggers when balance drift > $1", () => {
    const result = computeDeskOperatorHealth({ ...base, balanceDriftUsd: 5 });
    expect(result.status).toBe("NEEDS_REPAIR");
    expect(result.severity).toBe("danger");
    expect(result.blockers.some((b) => b.includes("drift"))).toBe(true);
  });

  it("triggers when probeDominant is true", () => {
    const result = computeDeskOperatorHealth({ ...base, probeDominant: true });
    expect(result.status).toBe("NEEDS_REPAIR");
    expect(result.blockers.some((b) => b.includes("Probe"))).toBe(true);
  });

  it("has highest priority over other issues", () => {
    const result = computeDeskOperatorHealth({
      ...base,
      balanceDriftUsd: 50,
      workerStale: true,
      pauseEntries: true,
    });
    expect(result.status).toBe("NEEDS_REPAIR");
  });

  it("next action mentions Repair state, not lowering thresholds", () => {
    const result = computeDeskOperatorHealth({ ...base, balanceDriftUsd: 2 });
    expect(result.nextAction.toLowerCase()).toContain("repair");
    expect(result.nextAction.toLowerCase()).not.toContain("lower");
    expect(result.nextAction.toLowerCase()).not.toContain("threshold");
  });
});

describe("computeDeskOperatorHealth — WORKER_STALE", () => {
  it("triggers when worker is enabled and stale", () => {
    const result = computeDeskOperatorHealth({
      ...base,
      workerEnabled: true,
      workerStale: true,
      workerLastPollAt: Date.now() - 90_000,
    });
    expect(result.status).toBe("WORKER_STALE");
    expect(result.severity).toBe("warning");
  });

  it("does NOT trigger when worker is disabled", () => {
    const result = computeDeskOperatorHealth({
      ...base,
      workerEnabled: false,
      workerStale: true,
    });
    expect(result.status).not.toBe("WORKER_STALE");
  });

  it("next action recommends pm2 restart", () => {
    const result = computeDeskOperatorHealth({
      ...base,
      workerEnabled: true,
      workerStale: true,
    });
    expect(result.nextAction).toContain("pm2");
  });
});

describe("computeDeskOperatorHealth — PAUSED", () => {
  it("triggers when pauseEntries is true", () => {
    const result = computeDeskOperatorHealth({ ...base, pauseEntries: true });
    expect(result.status).toBe("PAUSED");
    expect(result.severity).toBe("warning");
  });

  it("triggers when drawdownLocked is true", () => {
    const result = computeDeskOperatorHealth({ ...base, drawdownLocked: true });
    expect(result.status).toBe("PAUSED");
  });

  it("does not suggest lowering thresholds", () => {
    const result = computeDeskOperatorHealth({ ...base, pauseEntries: true });
    expect(result.nextAction.toLowerCase()).not.toContain("lower");
    expect(result.nextAction.toLowerCase()).not.toContain("threshold");
  });
});

describe("computeDeskOperatorHealth — MARKET_WAIT", () => {
  it("triggers when chop + high regime skips + no open positions", () => {
    const result = computeDeskOperatorHealth({
      ...base,
      regime: "chop",
      skipRegime: 10,
      openPositions: 0,
    });
    expect(result.status).toBe("MARKET_WAIT");
    expect(result.severity).toBe("info");
  });

  it("does NOT trigger with open positions", () => {
    const result = computeDeskOperatorHealth({
      ...base,
      regime: "chop",
      skipRegime: 10,
      openPositions: 2,
    });
    expect(result.status).not.toBe("MARKET_WAIT");
  });

  it("does NOT trigger in trend regime", () => {
    const result = computeDeskOperatorHealth({
      ...base,
      regime: "trendHigh",
      skipRegime: 10,
      openPositions: 0,
    });
    expect(result.status).not.toBe("MARKET_WAIT");
  });

  it("next action tells operator not to lower threshold", () => {
    const result = computeDeskOperatorHealth({
      ...base,
      regime: "chop",
      skipRegime: 10,
      openPositions: 0,
    });
    expect(result.nextAction.toUpperCase()).toContain("NOT");
    expect(result.nextAction.toUpperCase()).toContain("THRESHOLD");
  });
});

describe("computeDeskOperatorHealth — COLLECT_DATA", () => {
  it("triggers when totalTrades < 50", () => {
    const result = computeDeskOperatorHealth({ ...base, totalTrades: 12 });
    expect(result.status).toBe("COLLECT_DATA");
    expect(result.severity).toBe("info");
    expect(result.headline).toContain("12/50");
  });

  it("is overridden by NEEDS_REPAIR at < 50 trades", () => {
    const result = computeDeskOperatorHealth({
      ...base,
      totalTrades: 5,
      balanceDriftUsd: 10,
    });
    expect(result.status).toBe("NEEDS_REPAIR");
  });

  it("shows rotation pending count if any", () => {
    const result = computeDeskOperatorHealth({
      ...base,
      totalTrades: 20,
      rotationPending: 5,
    });
    expect(result.status).toBe("COLLECT_DATA");
    expect(result.blockers.some((b) => b.includes("5"))).toBe(true);
  });
});

describe("computeDeskOperatorHealth — READY", () => {
  it("triggers when scorecard ON_TRACK, worker fresh, balance clean", () => {
    const result = computeDeskOperatorHealth({
      ...base,
      scorecardState: "ON_TRACK",
      workerStale: false,
      balanceDriftUsd: 0,
    });
    expect(result.status).toBe("READY");
    expect(result.severity).toBe("success");
    expect(result.blockers).toHaveLength(0);
  });

  it("does NOT trigger when scorecard is REVIEW", () => {
    const result = computeDeskOperatorHealth({
      ...base,
      scorecardState: "REVIEW",
    });
    expect(result.status).not.toBe("READY");
  });

  it("does NOT trigger when balance drifts even slightly", () => {
    const result = computeDeskOperatorHealth({
      ...base,
      scorecardState: "ON_TRACK",
      balanceDriftUsd: 1.5,
    });
    // balanceDrift > 1 → NEEDS_REPAIR, never READY
    expect(result.status).toBe("NEEDS_REPAIR");
  });
});

describe("computeDeskOperatorHealth — never recommends lowering threshold", () => {
  const statuses = [
    "NEEDS_REPAIR",
    "WORKER_STALE",
    "PAUSED",
    "MARKET_WAIT",
    "COLLECT_DATA",
    "BLOCKED_STATE",
  ] as const;

  it.each([
    [{ ...base, balanceDriftUsd: 5 }],
    [{ ...base, workerEnabled: true, workerStale: true }],
    [{ ...base, pauseEntries: true }],
    // MARKET_WAIT explicitly says "Do NOT lower" which is correct guidance — test that
    // the recommendation is prohibitive, not permissive.
    [{ ...base, regime: "chop", skipRegime: 10, openPositions: 0 }],
    [{ ...base, totalTrades: 5 }],
    [{ ...base, scorecardState: "REVIEW" as const, totalTrades: 60 }],
  ])("nextAction never positively recommends lowering threshold for input %#", (input) => {
    const result = computeDeskOperatorHealth(input);
    // The action may say "Do NOT lower" (correct) — it must not say "lower threshold" as an imperative
    const text = result.nextAction.toLowerCase();
    const prohibited = /^lower threshold|please lower|try lowering|consider lowering|reduce the threshold/;
    expect(text).not.toMatch(prohibited);
    // Specifically for MARKET_WAIT: must contain a "not" or "do not" near threshold reference
    if (result.status === "MARKET_WAIT") {
      expect(text).toMatch(/not.*threshold|threshold.*not/);
    }
  });
});
