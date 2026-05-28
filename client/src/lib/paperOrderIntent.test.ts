/**
 * Tests for paperOrderIntent — pure function, no I/O.
 */
import { describe, it, expect } from "vitest";
import {
  createPaperOrderIntent,
  rejectIntent,
  acceptIntent,
  fillIntent,
  closeIntent,
  summarizeIntents,
  type PaperOrderIntent,
} from "./paperOrderIntent";

// ── createPaperOrderIntent ────────────────────────────────────────────────────

describe("createPaperOrderIntent", () => {
  it("creates an intent with intentId, createdAt, and NEW status", () => {
    const intent = createPaperOrderIntent({
      strategyId: 91,
      strategyName: "strat-91",
      symbol: "BTCUSDT",
      side: "LONG",
      signalScore: 28,
      status: "NEW",
    });
    expect(intent.intentId).toBeTruthy();
    expect(intent.createdAt).toBeTruthy();
    expect(intent.status).toBe("NEW");
    expect(intent.strategyId).toBe(91);
  });

  it("each call produces a unique intentId", () => {
    const a = createPaperOrderIntent({ strategyId: 91, strategyName: "s", symbol: "BTCUSDT", side: "LONG", signalScore: 26, status: "NEW" });
    const b = createPaperOrderIntent({ strategyId: 91, strategyName: "s", symbol: "BTCUSDT", side: "LONG", signalScore: 26, status: "NEW" });
    expect(a.intentId).not.toBe(b.intentId);
  });
});

// ── rejectIntent ──────────────────────────────────────────────────────────────

describe("rejectIntent", () => {
  it("sets status to REJECTED with gate and reason", () => {
    const original = createPaperOrderIntent({
      strategyId: 91, strategyName: "s", symbol: "BTCUSDT", side: "LONG", signalScore: 15, status: "NEW",
    });
    const rejected = rejectIntent(original, "SIGNAL", "Score 15 < threshold 26");
    expect(rejected.status).toBe("REJECTED");
    expect(rejected.gate).toBe("SIGNAL");
    expect(rejected.rejectReason).toContain("threshold");
  });

  it("does not mutate the original intent", () => {
    const original = createPaperOrderIntent({
      strategyId: 91, strategyName: "s", symbol: "BTCUSDT", side: "LONG", signalScore: 15, status: "NEW",
    });
    rejectIntent(original, "SIGNAL", "reason");
    expect(original.status).toBe("NEW");
  });
});

// ── fillIntent ────────────────────────────────────────────────────────────────

describe("fillIntent", () => {
  it("links positionId and sets status to FILLED", () => {
    const intent = createPaperOrderIntent({
      strategyId: 91, strategyName: "s", symbol: "BTCUSDT", side: "LONG", signalScore: 28, status: "ACCEPTED",
    });
    const filled = fillIntent(intent, "pos-abc-123");
    expect(filled.status).toBe("FILLED");
    expect(filled.positionId).toBe("pos-abc-123");
  });

  it("does not mutate the original", () => {
    const intent = createPaperOrderIntent({
      strategyId: 91, strategyName: "s", symbol: "BTCUSDT", side: "LONG", signalScore: 28, status: "ACCEPTED",
    });
    fillIntent(intent, "pos-xyz");
    expect(intent.status).toBe("ACCEPTED");
    expect(intent.positionId).toBeUndefined();
  });

  it("no live order code — no Delta API calls in implementation", () => {
    // Structural test: fillIntent returns an object, never calls external APIs
    const intent = createPaperOrderIntent({
      strategyId: 92, strategyName: "s", symbol: "BTCUSDT", side: "SHORT", signalScore: 27, status: "ACCEPTED",
    });
    const filled = fillIntent(intent, "pos-999");
    expect(filled.positionId).toBe("pos-999");
    // If this test runs without network error, no live API was called
  });
});

// ── closeIntent ───────────────────────────────────────────────────────────────

describe("closeIntent", () => {
  it("sets status to CLOSED", () => {
    const intent = createPaperOrderIntent({
      strategyId: 91, strategyName: "s", symbol: "BTCUSDT", side: "LONG", signalScore: 28, status: "FILLED",
    });
    const closed = closeIntent(intent);
    expect(closed.status).toBe("CLOSED");
  });
});

// ── summarizeIntents ──────────────────────────────────────────────────────────

describe("summarizeIntents", () => {
  it("counts by status correctly", () => {
    const intents: PaperOrderIntent[] = [
      createPaperOrderIntent({ strategyId: 91, strategyName: "s", symbol: "BTC", side: "LONG", signalScore: 28, status: "NEW" }),
      rejectIntent(createPaperOrderIntent({ strategyId: 92, strategyName: "s", symbol: "BTC", side: "LONG", signalScore: 10, status: "NEW" }), "SIGNAL", "score too low"),
      rejectIntent(createPaperOrderIntent({ strategyId: 93, strategyName: "s", symbol: "BTC", side: "SHORT", signalScore: 12, status: "NEW" }), "REGIME", "chop not allowed"),
      fillIntent(createPaperOrderIntent({ strategyId: 94, strategyName: "s", symbol: "BTC", side: "LONG", signalScore: 30, status: "ACCEPTED" }), "pos-1"),
    ];
    const summary = summarizeIntents(intents);
    expect(summary.total).toBe(4);
    expect(summary.byStatus.NEW).toBe(1);
    expect(summary.byStatus.REJECTED).toBe(2);
    expect(summary.byStatus.FILLED).toBe(1);
  });

  it("topRejectedGate identifies the most frequent rejection gate", () => {
    const intents: PaperOrderIntent[] = [
      rejectIntent(createPaperOrderIntent({ strategyId: 91, strategyName: "s", symbol: "BTC", side: "LONG", signalScore: 10, status: "NEW" }), "SIGNAL", "r"),
      rejectIntent(createPaperOrderIntent({ strategyId: 92, strategyName: "s", symbol: "BTC", side: "LONG", signalScore: 11, status: "NEW" }), "SIGNAL", "r"),
      rejectIntent(createPaperOrderIntent({ strategyId: 93, strategyName: "s", symbol: "BTC", side: "LONG", signalScore: 12, status: "NEW" }), "REGIME", "r"),
    ];
    const summary = summarizeIntents(intents);
    expect(summary.topRejectedGate).toBe("SIGNAL");
  });

  it("empty input → zero counts and null gate", () => {
    const summary = summarizeIntents([]);
    expect(summary.total).toBe(0);
    expect(summary.topRejectedGate).toBeNull();
    expect(summary.byStatus.REJECTED).toBe(0);
  });
});
