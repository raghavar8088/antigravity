import { describe, expect, it } from "vitest";
import {
  buildExecutorDiagnosis,
  diagnoseNoTradeFromExecutorState,
} from "@/lib/mockTradingExecutor/diagnoseExecutor";
import { executorStateHealth } from "@/lib/mockTradingExecutor/persistExecutorState";
import type { MockExecutorStateDoc } from "@/lib/mockTradingExecutor/types";

function state(partial: Partial<MockExecutorStateDoc>): MockExecutorStateDoc {
  return {
    account_key: "mock_trading_main",
    last_tick_at: Date.now() - 5_000,
    last_duration_ms: 120,
    last_mode: "worker",
    last_opened: 0,
    last_closed: 0,
    last_candidate_count: 0,
    last_dominant_blocker: "signal",
    last_error: null,
    entry_funnel_snapshot: null,
    updated_at: new Date().toISOString(),
    ...partial,
  };
}

describe("diagnoseNoTradeFromExecutorState", () => {
  it("detects executor not running", () => {
    const r = diagnoseNoTradeFromExecutorState(null);
    expect(r.reason).toBe("EXECUTOR_NOT_RUNNING");
  });

  it("detects stale executor", () => {
    const r = diagnoseNoTradeFromExecutorState(state({ last_tick_at: Date.now() - 120_000 }));
    expect(r.reason).toBe("EXECUTOR_STALE");
  });

  it("maps regime blocker", () => {
    const r = diagnoseNoTradeFromExecutorState(state({ last_dominant_blocker: "regime" }));
    expect(r.reason).toBe("REGIME_BLOCKING");
  });

  it("maps atrFees blocker", () => {
    const r = diagnoseNoTradeFromExecutorState(state({ last_dominant_blocker: "atrFees" }));
    expect(r.reason).toBe("ATR_FEES_BLOCKING");
  });

  it("maps signal blocker", () => {
    const r = diagnoseNoTradeFromExecutorState(state({ last_dominant_blocker: "signal" }));
    expect(r.reason).toBe("SIGNAL_NOT_FIRING");
  });
});

describe("buildExecutorDiagnosis", () => {
  it("REGIME includes wait-for-trend guidance and MARKET_WAIT status", () => {
    const r = buildExecutorDiagnosis(state({ last_dominant_blocker: "regime" }));
    expect(r.reason).toBe("REGIME_BLOCKING");
    expect(r.status).toBe("MARKET_WAIT");
    expect(r.isHealthy).toBe(true);
    expect(r.nextAction.toLowerCase()).toMatch(/trend|regime|ranging|setup/);
  });

  it("ATR_FEES is healthy protective behavior", () => {
    const r = buildExecutorDiagnosis(state({ last_dominant_blocker: "atrFees" }));
    expect(r.reason).toBe("ATR_FEES_BLOCKING");
    expect(r.isHealthy).toBe(true);
    expect(r.status).toBe("MARKET_WAIT");
  });

  it("SIGNAL_SCORE maps to SIGNAL_NOT_FIRING", () => {
    const r = buildExecutorDiagnosis(state({ last_dominant_blocker: "signal" }));
    expect(r.reason).toBe("SIGNAL_NOT_FIRING");
    expect(r.headline).toBe("SIGNAL_SCORE_BLOCKING");
    expect(r.isHealthy).toBe(true);
  });

  it("WORKER_STALE is not healthy", () => {
    const r = buildExecutorDiagnosis(state({ last_tick_at: Date.now() - 120_000 }));
    expect(r.reason).toBe("EXECUTOR_STALE");
    expect(r.isHealthy).toBe(false);
    expect(r.status).toBe("EXECUTOR_FAULT");
  });

  it("uses funnel snapshot recommendation when present", () => {
    const r = buildExecutorDiagnosis(
      state({
        last_dominant_blocker: "regime",
        entry_funnel_snapshot: {
          tickAt: Date.now(),
          workerMode: "worker",
          workerFresh: true,
          symbol: "BTCUSD",
          markPrice: 100_000,
          bars: 50,
          activeStrategies: 10,
          evaluatedStrategies: 10,
          signalPassed: 0,
          confirmPassed: 0,
          candidateCount: 0,
          openAttempts: 0,
          opened: 0,
          blockerCounts: {
            noData: 0,
            noStrategies: 0,
            signal: 0,
            confirm: 0,
            regime: 8,
            atrFees: 0,
            rotation: 0,
            suspended: 0,
            spread: 0,
            session: 0,
            category: 0,
            sameSide: 0,
            margin: 0,
            maxOpen: 0,
            cooldown: 0,
          },
          dominantBlocker: "regime",
          recommendation: "Wait for trend regime — custom funnel copy.",
        },
      }),
    );
    expect(r.nextAction).toBe("Wait for trend regime — custom funnel copy.");
  });
});

describe("executorStateHealth", () => {
  it("is healthy when last cycle < 30s", () => {
    const flags = executorStateHealth(state({ last_tick_at: Date.now() - 5_000 }));
    expect(flags.healthy).toBe(true);
    expect(flags.stale).toBe(false);
  });

  it("is stale when last cycle > 30s", () => {
    const flags = executorStateHealth(state({ last_tick_at: Date.now() - 60_000 }));
    expect(flags.healthy).toBe(false);
    expect(flags.stale).toBe(true);
  });
});
