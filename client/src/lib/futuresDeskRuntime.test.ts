import { describe, expect, it } from "vitest";

import {
  DESK_EXIT_LATE_EXIT_MIN_GAIN,
  resolveFuturesExitStep,
  type FuturesExitStepPosition,
} from "./futuresDeskRuntime";

/**
 * Build a position that satisfies PROFIT_LOCK's progress + return gates but has
 * tiny gross PnL — so the projected net (after exit slip + round-trip fees) is
 * negative. The guard should suppress PROFIT_LOCK and keep the trade open.
 */
function buildMicroGainLongPos(overrides: Partial<FuturesExitStepPosition> = {}): FuturesExitStepPosition {
  // entry 100_000, mark 100_030 → 0.03% price move
  // 25x leverage notional 5_000 / margin 200 → gross +$1.50 / returnPct +0.75% on margin
  // tpPrice 100_500 → tpPctAbs 0.50% → lockTh = max(0.22, 0.50 × 0.6) = 0.30%
  // progress = 0.75 / 0.50 = 1.50 (≥ 0.55 default profitLockMinProgress)
  // returnPct 0.75 ≥ lockTh 0.30 ✓  → would FIRE without guard
  // BUT: round-trip fees = 5_000 × 0.001 × 2 = $10; slip @ 5bps on $100_030 ≈ $0.50 swing
  // gross after slip ≈ $1.25 - $10 fees = -$8.75 net → guard MUST suppress
  return {
    side: "LONG",
    entryPrice: 100_000,
    markPrice: 100_030,
    lastPrice: 100_030,
    notional: 5_000,
    marginUsed: 200,
    fundingCosts: 0,
    lastFundingAppliedAt: Date.now(),
    unrealizedPnl: 1.5,
    unrealizedPnlPct: 0.03,
    returnPct: 0.75,
    peakReturnPct: 0.75,
    tpPrice: 100_500,
    slPrice: 99_750,
    adaptiveSl: 99_750,
    breakevenMoved: false,
    liquidationPrice: 96_000,
    openedAt: new Date(Date.now() - 60_000).toISOString(),
    holdMinutes: 30,
    ...overrides,
  };
}

describe("resolveFuturesExitStep — PROFIT_LOCK net-projection guard", () => {
  it("suppresses PROFIT_LOCK when projected net is negative after slip + fees", () => {
    const pos = buildMicroGainLongPos();
    const { close } = resolveFuturesExitStep(pos, 1, Date.now(), {
      profitLockMinNetUsd: 0.05,
      profitLockMinProgress: 0.55,
      takerFeePct: 0.001,
      exitSlippageBps: 5,
    });
    expect(close.shouldClose).toBe(false);
    expect(close.reason).toBeUndefined();
  });

  it("still fires PROFIT_LOCK when projected net clears the threshold", () => {
    // 0.3% move (below tpPrice 100_500 so TP doesn't fire first).
    // gross +$15 on $5_000 notional; fees $10; slip 5bps → net ~+$2.50 → ≥ $0.05 → LOCKS.
    const pos = buildMicroGainLongPos({
      markPrice: 100_300,
      unrealizedPnl: 15,
      returnPct: 7.5, // 0.3% × 25x
      peakReturnPct: 7.5,
    });
    const { close } = resolveFuturesExitStep(pos, 1, Date.now(), {
      profitLockMinNetUsd: 0.05,
      profitLockMinProgress: 0.55,
      takerFeePct: 0.001,
      exitSlippageBps: 5,
    });
    expect(close.shouldClose).toBe(true);
    expect(close.reason).toBe("PROFIT_LOCK");
  });

  it("suppresses PROFIT_LOCK when gross < minGrossMultipleOfFees × round-trip fees", () => {
    const pos = buildMicroGainLongPos({
      markPrice: 100_300,
      unrealizedPnl: 15,
      returnPct: 7.5,
      peakReturnPct: 7.5,
    });
    const { close } = resolveFuturesExitStep(pos, 1, Date.now(), {
      profitLockMinNetUsd: 0.05,
      profitLockMinProgress: 0.55,
      minGrossMultipleOfFees: 100,
      takerFeePct: 0.001,
      exitSlippageBps: 5,
    });
    expect(close.shouldClose).toBe(false);
  });

  it("default opts (no overrides) still apply the guard — production-safe default", () => {
    const pos = buildMicroGainLongPos();
    const { close } = resolveFuturesExitStep(pos, 1, Date.now());
    // Default profitLockMinNetUsd = 0.05; projected net is negative → no close.
    expect(close.shouldClose).toBe(false);
  });

  it("hard exit (SL hit) still fires regardless of profit lock guard", () => {
    const pos = buildMicroGainLongPos({
      markPrice: 99_700,    // below SL 99_750
      returnPct: -3,
      peakReturnPct: 0.75,
      openedAt: new Date(Date.now() - 10 * 60_000).toISOString(),
    });
    const { close } = resolveFuturesExitStep(pos, 1, Date.now());
    expect(close.shouldClose).toBe(true);
    expect(close.reason).toBe("SL");
  });

  it("suppresses SL during minAgeBeforeSlMs grace (paper discovery)", () => {
    const pos = buildMicroGainLongPos({
      markPrice: 99_700,
      returnPct: -3,
      peakReturnPct: -3,
      openedAt: new Date(Date.now() - 30_000).toISOString(),
    });
    const { close } = resolveFuturesExitStep(pos, 1, Date.now(), {
      minAgeBeforeSlMs: 5 * 60_000,
    });
    expect(close.shouldClose).toBe(false);
  });

  it("suppresses TRAIL during minAgeBeforeSlMs grace (paper discovery)", () => {
    const pos = buildMicroGainLongPos({
      markPrice: 99_750,
      returnPct: -0.5,
      peakReturnPct: 2,
      openedAt: new Date(Date.now() - 60_000).toISOString(),
    });
    const { close } = resolveFuturesExitStep(pos, 1, Date.now(), {
      minAgeBeforeSlMs: 5 * 60_000,
    });
    expect(close.shouldClose).toBe(false);
  });

  it("LATE_EXIT_MIN_GAIN floor still applies via lockTh", () => {
    // Verifies the lockTh = max(LATE_EXIT_MIN_GAIN, ...) constant is preserved.
    expect(DESK_EXIT_LATE_EXIT_MIN_GAIN).toBe(0.22);
  });

  it("paperTpBeforeSl books TP before SL would fire on small favorable move", () => {
    const pos = buildMicroGainLongPos({
      markPrice: 100_450,
      tpPrice: 100_450,
      slPrice: 99_000,
      adaptiveSl: 99_000,
      returnPct: 1.2,
      peakReturnPct: 1.2,
      openedAt: new Date(Date.now() - 10 * 60_000).toISOString(),
    });
    const { close } = resolveFuturesExitStep(pos, 1, Date.now(), {
      paperTpBeforeSl: true,
      minAgeBeforeSlMs: 0,
    });
    expect(close.shouldClose).toBe(true);
    expect(close.reason).toBe("TP");
  });
});
