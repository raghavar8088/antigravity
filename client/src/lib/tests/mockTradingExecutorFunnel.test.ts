import { describe, expect, it } from "vitest";
import { funnelSnapshotFromTraceEval } from "@/lib/mockTradingExecutor/funnelFromTrace";
import { createTraceRow } from "@/lib/ai/strategySignalTrace";

describe("funnelSnapshotFromTraceEval", () => {
  it("builds dominant blocker from rejected trace rows", () => {
    const rows = [
      createTraceRow({
        traceId: "t1",
        tickAt: Date.now(),
        mode: "worker",
        symbol: "BTCUSD",
        strategyId: 1,
        strategyName: "Test",
        status: "REJECTED",
        gate: "REGIME",
        reason: "chop",
        signalScore: 0,
        requiredThreshold: 28,
        confirmPassed: false,
        regime: "chop",
        regimeAllowed: false,
      }),
    ];
    const snap = funnelSnapshotFromTraceEval({
      tickAt: Date.now(),
      symbol: "BTCUSD",
      markPrice: 100_000,
      bars: 40,
      activeStrategies: 5,
      evaluatedStrategies: 5,
      rows,
      candidateCount: 0,
      opened: 0,
      workerFresh: true,
    });
    expect(snap.dominantBlocker).toBe("regime");
    expect(snap.workerMode).toBe("worker");
  });
});
