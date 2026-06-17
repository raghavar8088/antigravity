import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { recommendExecutorHealing } from "@/lib/mockTradingExecutor/healingEngine";
import type { MockExecutorStateDoc } from "@/lib/mockTradingExecutor/types";

function state(partial: Partial<MockExecutorStateDoc>): MockExecutorStateDoc {
  return {
    account_key: "mock_trading_main",
    last_tick_at: Date.now() - 5_000,
    last_duration_ms: 100,
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

describe("recommendExecutorHealing", () => {
  it("alerts when executor never ran", () => {
    const r = recommendExecutorHealing(null);
    expect(r.healed).toBe(false);
    expect(r.isHealthy).toBe(false);
    expect(r.action).toContain("PM2");
  });

  it("treats regime blocking as expected market wait", () => {
    const r = recommendExecutorHealing(state({ last_dominant_blocker: "regime" }));
    expect(r.healed).toBe(true);
    expect(r.isHealthy).toBe(true);
    expect(r.status).toBe("MARKET_WAIT");
    expect(r.reason).toBe("REGIME_BLOCKING");
    expect(r.nextAction.toLowerCase()).toMatch(/trend|regime|setup/);
  });

  it("alerts on stale executor", () => {
    const r = recommendExecutorHealing(state({ last_tick_at: Date.now() - 120_000 }));
    expect(r.healed).toBe(false);
    expect(r.status).toBe("EXECUTOR_FAULT");
    expect(r.action).toContain("restart");
  });
});

describe("executor-status API schema", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => ({
        ok: true,
        json: async () => ({
          ok: true,
          account_key: "mock_trading_main",
          executor: {
            healthy: true,
            stale: false,
            ageSeconds: 3,
            lastTickAt: Date.now(),
            lastMode: "worker",
            dominantBlocker: "regime",
            candidateCount: 0,
            openedLastCycle: 0,
            closedLastCycle: 0,
            errors: [],
          },
          no_trade_diagnosis: {
            reason: "REGIME_BLOCKING",
            headline: "REGIME_BLOCKING_ENTRIES",
            explanation: "Market regime is ranging.",
            nextAction: "Wait for trend regime.",
            isHealthy: true,
            status: "MARKET_WAIT",
            dominantBlocker: "regime",
            evidence: [],
            safeFix: "Wait.",
          },
          healing: {
            healed: true,
            action: "Wait for trend regime.",
            reason: "REGIME_BLOCKING",
            safeToAutomate: true,
            nextAction: "Wait for trend regime.",
            isHealthy: true,
            status: "MARKET_WAIT",
          },
          recent_trades: [],
        }),
      })),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("returns executor, diagnosis, and healing fields", async () => {
    const res = await fetch("/api/mock-trading/executor-status?account_key=mock_trading_main");
    const json = await res.json();
    expect(json.ok).toBe(true);
    expect(json.executor).toMatchObject({ healthy: true, dominantBlocker: "regime" });
    expect(json.no_trade_diagnosis).toMatchObject({
      reason: "REGIME_BLOCKING",
      nextAction: expect.any(String),
      isHealthy: true,
      status: "MARKET_WAIT",
    });
    expect(json.healing).toMatchObject({ isHealthy: true, status: "MARKET_WAIT" });
  });
});
