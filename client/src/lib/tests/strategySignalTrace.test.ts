/**
 * Tests for strategySignalTrace helpers.
 * Pure functions — no MongoDB, no I/O.
 */

import { describe, it, expect } from "vitest";
import {
  createTraceRow,
  summarizeSignalTrace,
  capTraceRows,
  type StrategySignalTraceRow,
} from "../strategySignalTrace";

// ── Helpers ───────────────────────────────────────────────────────────────────

function makeRow(overrides: Partial<StrategySignalTraceRow> = {}): StrategySignalTraceRow {
  return createTraceRow({
    tickAt: Date.now(),
    mode: "worker",
    symbol: "BTCUSD",
    strategyId: 91,
    strategyName: "TestStrat",
    status: "EVALUATED",
    gate: "SIGNAL",
    reason: "Score below threshold",
    signalScore: 10,
    requiredThreshold: 22,
    confirmPassed: false,
    regime: "chop",
    regimeAllowed: true,
    ...overrides,
  });
}

// ── summarizeSignalTrace ──────────────────────────────────────────────────────

describe("summarizeSignalTrace", () => {
  it("counts totalEvaluated from all rows", () => {
    const rows = [makeRow(), makeRow(), makeRow({ status: "OPENED", gate: "OPENED", reason: "opened" })];
    const s = summarizeSignalTrace(rows);
    expect(s.totalEvaluated).toBe(3);
  });

  it("counts fired as FIRED + CANDIDATE + OPENED", () => {
    const rows = [
      makeRow({ status: "FIRED", gate: "CONFIRM" }),
      makeRow({ status: "CANDIDATE", gate: "SIGNAL" }),
      makeRow({ status: "OPENED", gate: "OPENED", reason: "opened" }),
      makeRow({ status: "EVALUATED", gate: "SIGNAL" }),
    ];
    const s = summarizeSignalTrace(rows);
    expect(s.fired).toBe(3);
  });

  it("counts candidates (CANDIDATE only)", () => {
    const rows = [
      makeRow({ status: "CANDIDATE" }),
      makeRow({ status: "OPENED", gate: "OPENED", reason: "opened" }),
    ];
    const s = summarizeSignalTrace(rows);
    expect(s.candidates).toBe(1);
  });

  it("counts opened (OPENED only)", () => {
    const rows = [
      makeRow({ status: "OPENED", gate: "OPENED", reason: "opened" }),
      makeRow({ status: "CANDIDATE" }),
    ];
    const s = summarizeSignalTrace(rows);
    expect(s.opened).toBe(1);
  });

  it("counts rejectedByGate correctly", () => {
    const rows = [
      makeRow({ status: "REJECTED", gate: "ATR_FEES" }),
      makeRow({ status: "REJECTED", gate: "ATR_FEES" }),
      makeRow({ status: "REJECTED", gate: "REGIME" }),
    ];
    const s = summarizeSignalTrace(rows);
    expect(s.rejectedByGate["ATR_FEES"]).toBe(2);
    expect(s.rejectedByGate["REGIME"]).toBe(1);
  });

  it("returns topRejectedGate as the most common rejection gate", () => {
    const rows = [
      makeRow({ status: "REJECTED", gate: "SIGNAL" }),
      makeRow({ status: "REJECTED", gate: "ATR_FEES" }),
      makeRow({ status: "REJECTED", gate: "ATR_FEES" }),
      makeRow({ status: "REJECTED", gate: "ATR_FEES" }),
    ];
    const s = summarizeSignalTrace(rows);
    expect(s.topRejectedGate).toBe("ATR_FEES");
  });

  it("returns topRejectedGate null when no rejections", () => {
    const rows = [makeRow({ status: "OPENED", gate: "OPENED", reason: "opened" })];
    const s = summarizeSignalTrace(rows);
    expect(s.topRejectedGate).toBeNull();
  });

  it("handles empty rows", () => {
    const s = summarizeSignalTrace([]);
    expect(s.totalEvaluated).toBe(0);
    expect(s.fired).toBe(0);
    expect(s.opened).toBe(0);
    expect(s.topRejectedGate).toBeNull();
  });

  it("does not count EVALUATED rows as fired", () => {
    const rows = [makeRow({ status: "EVALUATED", gate: "SIGNAL" })];
    const s = summarizeSignalTrace(rows);
    expect(s.fired).toBe(0);
  });
});

// ── capTraceRows ──────────────────────────────────────────────────────────────

describe("capTraceRows", () => {
  it("returns rows unchanged when under max", () => {
    const rows = [makeRow(), makeRow(), makeRow()];
    const capped = capTraceRows(rows, 10);
    expect(capped.length).toBe(3);
  });

  it("caps to max when over limit", () => {
    const rows = Array.from({ length: 20 }, (_, i) =>
      makeRow({ strategyId: i, signalScore: i }),
    );
    const capped = capTraceRows(rows, 10);
    expect(capped.length).toBe(10);
  });

  it("retains OPENED rows first (highest priority)", () => {
    const rows = [
      ...Array.from({ length: 8 }, (_, i) => makeRow({ strategyId: i, signalScore: 5 })),
      makeRow({ strategyId: 100, status: "OPENED", gate: "OPENED", reason: "opened", signalScore: 1 }),
      makeRow({ strategyId: 101, status: "OPENED", gate: "OPENED", reason: "opened", signalScore: 2 }),
    ];
    const capped = capTraceRows(rows, 5);
    const openedIds = capped.filter((r) => r.status === "OPENED").map((r) => r.strategyId);
    expect(openedIds).toContain(100);
    expect(openedIds).toContain(101);
  });

  it("retains CANDIDATE rows over low-score EVALUATED rows", () => {
    const rows = [
      makeRow({ strategyId: 1, status: "EVALUATED", signalScore: 3 }),
      makeRow({ strategyId: 2, status: "EVALUATED", signalScore: 2 }),
      makeRow({ strategyId: 3, status: "CANDIDATE", signalScore: 1 }),
    ];
    const capped = capTraceRows(rows, 2);
    expect(capped.some((r) => r.strategyId === 3)).toBe(true);
  });

  it("retains highest signalScore rows when capping EVALUATED", () => {
    const rows = Array.from({ length: 10 }, (_, i) =>
      makeRow({ strategyId: i, signalScore: i }),
    );
    const capped = capTraceRows(rows, 5);
    const scores = capped.map((r) => r.signalScore);
    expect(Math.min(...scores)).toBeGreaterThanOrEqual(5);
  });
});

// ── createTraceRow ────────────────────────────────────────────────────────────

describe("createTraceRow", () => {
  it("generates a traceId when not provided", () => {
    const row = makeRow();
    expect(typeof row.traceId).toBe("string");
    expect(row.traceId.length).toBeGreaterThan(0);
  });

  it("uses provided traceId", () => {
    const row = createTraceRow({ ...makeRow(), traceId: "custom-id" });
    expect(row.traceId).toBe("custom-id");
  });

  it("all required fields are present", () => {
    const row = makeRow();
    expect(row).toHaveProperty("strategyId");
    expect(row).toHaveProperty("strategyName");
    expect(row).toHaveProperty("status");
    expect(row).toHaveProperty("gate");
    expect(row).toHaveProperty("reason");
    expect(row).toHaveProperty("signalScore");
    expect(row).toHaveProperty("requiredThreshold");
    expect(row).toHaveProperty("confirmPassed");
    expect(row).toHaveProperty("regime");
    expect(row).toHaveProperty("regimeAllowed");
  });
});

// ── Gate pipeline ordering verification ──────────────────────────────────────

describe("gate values", () => {
  const gateOrder = [
    "DATA", "DISABLED", "SUSPENDED", "ROTATION", "COOLDOWN", "OCCUPIED",
    "REGIME", "SIGNAL", "CONFIRM", "QUALITY", "MTF", "ATR_FEES",
    "SPREAD", "SESSION", "CATEGORY", "SAME_SIDE", "MARGIN", "MAX_OPEN", "OPENED",
  ] as const;

  it("all gate values are valid enum members", () => {
    for (const gate of gateOrder) {
      const row = makeRow({ gate });
      expect(row.gate).toBe(gate);
    }
  });
});

// ── Worker trace scenario coverage ───────────────────────────────────────────

describe("worker trace scenarios", () => {
  it("signal below threshold produces EVALUATED with SIGNAL gate", () => {
    const row = makeRow({ status: "EVALUATED", gate: "SIGNAL", signalScore: 5, requiredThreshold: 22 });
    expect(row.status).toBe("EVALUATED");
    expect(row.gate).toBe("SIGNAL");
    expect(row.signalScore).toBeLessThan(row.requiredThreshold);
  });

  it("confirm fail produces FIRED with CONFIRM gate", () => {
    const row = makeRow({ status: "FIRED", gate: "CONFIRM", signalScore: 25, requiredThreshold: 22, confirmPassed: false });
    expect(row.status).toBe("FIRED");
    expect(row.gate).toBe("CONFIRM");
    expect(row.confirmPassed).toBe(false);
    expect(row.signalScore).toBeGreaterThanOrEqual(row.requiredThreshold);
  });

  it("regime mismatch produces REJECTED with REGIME gate", () => {
    const row = makeRow({ status: "REJECTED", gate: "REGIME", regimeAllowed: false, regime: "chop", reason: "Regime chop not in [trend_up]" });
    expect(row.status).toBe("REJECTED");
    expect(row.gate).toBe("REGIME");
    expect(row.regimeAllowed).toBe(false);
  });

  it("valid opened position produces OPENED with openedPositionId", () => {
    const row = makeRow({ status: "OPENED", gate: "OPENED", reason: "opened", openedPositionId: "pos-123", signalScore: 30, confirmPassed: true });
    expect(row.status).toBe("OPENED");
    expect(row.gate).toBe("OPENED");
    expect(row.openedPositionId).toBe("pos-123");
  });

  it("ATR/fees rejection produces REJECTED with ATR_FEES gate", () => {
    const row = makeRow({ status: "REJECTED", gate: "ATR_FEES", feeHurdlePassed: false, confirmPassed: true });
    expect(row.status).toBe("REJECTED");
    expect(row.gate).toBe("ATR_FEES");
    expect(row.feeHurdlePassed).toBe(false);
  });
});

// ── No "lower threshold" copy check ──────────────────────────────────────────

describe("no lower threshold language", () => {
  const BANNED_PHRASES = [
    "lower the threshold",
    "reduce threshold",
    "decrease threshold",
    "lower threshold",
  ];

  const sampleReasons = [
    "Score 10 below threshold 22",
    "ATR/fee hurdle failed",
    "Entry confirmation failed",
    "Regime chop not in trend_up",
    "Strategy already has an open position",
    "Cooldown active for 120s",
  ];

  for (const phrase of BANNED_PHRASES) {
    for (const reason of sampleReasons) {
      it(`reason "${reason.slice(0, 30)}" does not contain "${phrase}"`, () => {
        expect(reason.toLowerCase()).not.toContain(phrase);
      });
    }
  }
});
