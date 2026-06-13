import { describe, it, expect, afterEach } from "vitest";
import {
  profitModeExitConfig,
  profitModeFromEnv,
  profitModeSessionGateEnabled,
  profitModeAllocationByEdgeEnabled,
} from "../trading/futuresProfitMode";
import { computeDeskRollingPnLScorecard } from "../portfolio/futuresDeskPnLTracker";
import { resolveFuturesExitStep, type FuturesExitStepPosition } from "../trading/futuresDeskRuntime";

describe("profitModeExitConfig", () => {
  it("returns strict exits when enabled", () => {
    const cfg = profitModeExitConfig({ enabled: true } as never);
    expect(cfg).not.toBeNull();
    expect(cfg!.profitLockMinProgress).toBe(0.55);
    expect(cfg!.disablePaperQuickTp).toBe(true);
    expect(cfg!.minGrossMultipleOfFees).toBe(2);
  });

  it("returns null when disabled", () => {
    expect(profitModeExitConfig({ enabled: false } as never)).toBeNull();
  });
});

describe("profit mode v2 toggles", () => {
  const prev = process.env.NEXT_PUBLIC_DESK_PROFIT_MODE;

  afterEach(() => {
    if (prev === undefined) delete process.env.NEXT_PUBLIC_DESK_PROFIT_MODE;
    else process.env.NEXT_PUBLIC_DESK_PROFIT_MODE = prev;
  });

  it("auto-enables session gate and allocation", () => {
    process.env.NEXT_PUBLIC_DESK_PROFIT_MODE = "1";
    const cfg = profitModeFromEnv();
    expect(profitModeSessionGateEnabled(cfg)).toBe(true);
    expect(profitModeAllocationByEdgeEnabled(cfg)).toBe(true);
  });
});

describe("computeDeskRollingPnLScorecard", () => {
  const now = Date.now();
  const ago = (h: number) => new Date(now - h * 3_600_000).toISOString();

  const mk = (net: number, h = 1) => ({
    closedAt: ago(h),
    netPnl: net,
    grossPnl: net + 2,
    fees: 2,
    strategyName: "MTF_Trend",
  });

  it("flags ON_TRACK when last 50 pass targets", () => {
    const trades = Array.from({ length: 55 }, (_, i) => mk(5 + (i % 3), 1 + i * 0.5));
    const card = computeDeskRollingPnLScorecard(trades, now);
    expect(card.last50.tradeCount).toBe(50);
    expect(card.paperReadyHint).toBe("ON_TRACK");
    expect(card.passesExpectancyTarget50).toBe(true);
  });
});

describe("PROFIT_LOCK minGrossMultipleOfFees", () => {
  const pos: FuturesExitStepPosition = {
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
  };

  it("suppresses lock when gross < 2× fees", () => {
    const { close } = resolveFuturesExitStep(pos, 1, Date.now(), {
      profitLockMinProgress: 0.55,
      profitLockMinNetUsd: 0.05,
      minGrossMultipleOfFees: 2,
      takerFeePct: 0.001,
      exitSlippageBps: 5,
    });
    expect(close.shouldClose).toBe(false);
  });
});
