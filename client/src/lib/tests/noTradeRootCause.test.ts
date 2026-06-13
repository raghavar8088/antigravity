import { describe, expect, it } from "vitest";
import { diagnoseNoTradeRootCause } from "../risk/noTradeRootCause";
import { buildFunnelSnapshot, emptyBlockerCounts } from "../trading/deskEntryFunnelSnapshot";
import type { SignalTraceSummary } from "../ai/strategySignalTrace";

function snap(overrides: Partial<Parameters<typeof buildFunnelSnapshot>[0]> = {}) {
  return buildFunnelSnapshot({
    tickAt: Date.now(),
    workerMode: "worker",
    workerFresh: true,
    symbol: "BTCUSD",
    markPrice: 100_000,
    bars: 200,
    activeStrategies: 20,
    evaluatedStrategies: 20,
    signalPassed: 0,
    confirmPassed: 0,
    candidateCount: 0,
    openAttempts: 0,
    opened: 0,
    blockerCounts: emptyBlockerCounts(),
    ...overrides,
  });
}

const summary = (topRejectedGate: string | null, fired = 0): SignalTraceSummary => ({
  totalEvaluated: 20,
  fired,
  candidates: 0,
  opened: 0,
  rejectedByGate: topRejectedGate ? { [topRejectedGate]: 20 } : {},
  topRejectedGate,
});

describe("diagnoseNoTradeRootCause", () => {
  it("detects worker stale", () => {
    const r = diagnoseNoTradeRootCause({ workerHealth: { stale: true, ageSeconds: 90 } });
    expect(r.rootCause).toBe("WORKER_STALE");
  });

  it("detects no data", () => {
    const c = emptyBlockerCounts();
    c.noData = 1;
    const r = diagnoseNoTradeRootCause({ funnel: snap({ bars: 0, blockerCounts: c }) });
    expect(r.rootCause).toBe("NO_DATA");
  });

  it("detects empty roster", () => {
    const r = diagnoseNoTradeRootCause({ funnel: snap({ activeStrategies: 0 }) });
    expect(r.rootCause).toBe("EMPTY_ROSTER");
  });

  it("detects invalid roster ids", () => {
    const c = emptyBlockerCounts();
    c.noStrategies = 2;
    const r = diagnoseNoTradeRootCause({
      funnel: snap({ activeStrategies: 0, blockerCounts: c }),
      signalTrace: {
        summary: summary("NO_STRATEGIES"),
        rows: [{
          traceId: "x",
          tickAt: Date.now(),
          mode: "worker",
          symbol: "BTCUSD",
          strategyId: 0,
          strategyName: "NO_STRATEGIES",
          status: "REJECTED",
          gate: "NO_STRATEGIES",
          reason: "unknown IDs: 9999",
          signalScore: 0,
          requiredThreshold: 26,
          confirmPassed: false,
          regime: "unknown",
          regimeAllowed: false,
        }],
      },
    });
    expect(r.rootCause).toBe("INVALID_ROSTER_IDS");
  });

  it("detects signal not firing", () => {
    const c = emptyBlockerCounts();
    c.signal = 20;
    const r = diagnoseNoTradeRootCause({ funnel: snap({ blockerCounts: c }), signalTrace: { summary: summary("SIGNAL"), rows: [] } });
    expect(r.rootCause).toBe("SIGNAL_NOT_FIRING");
    expect(r.safeFix.toLowerCase()).toContain("do not lower");
  });

  it("detects confirm blocking", () => {
    const c = emptyBlockerCounts();
    c.confirm = 5;
    const r = diagnoseNoTradeRootCause({ funnel: snap({ blockerCounts: c }), signalTrace: { summary: summary("CONFIRM", 5), rows: [] } });
    expect(r.rootCause).toBe("CONFIRM_BLOCKING");
  });

  it("detects regime blocking", () => {
    const c = emptyBlockerCounts();
    c.regime = 20;
    const r = diagnoseNoTradeRootCause({ funnel: snap({ blockerCounts: c }), signalTrace: { summary: summary("REGIME"), rows: [] } });
    expect(r.rootCause).toBe("REGIME_BLOCKING");
  });

  it("detects ATR/fee blocking", () => {
    const c = emptyBlockerCounts();
    c.atrFees = 3;
    const r = diagnoseNoTradeRootCause({ funnel: snap({ blockerCounts: c }), signalTrace: { summary: summary("ATR_FEES", 3), rows: [] } });
    expect(r.rootCause).toBe("ATR_FEE_BLOCKING");
  });

  it("detects dirty state before judging gates", () => {
    const r = diagnoseNoTradeRootCause({ funnel: snap(), paperState: { balanceDriftUsd: 20 } });
    expect(r.rootCause).toBe("STATE_DIRTY");
  });

  it("detects rotation blocking", () => {
    const c = emptyBlockerCounts();
    c.rotation = 1;
    const r = diagnoseNoTradeRootCause({ funnel: snap({ blockerCounts: c }), signalTrace: { summary: summary("ROTATION"), rows: [] } });
    expect(r.rootCause).toBe("ROTATION_BLOCKING");
  });
});
