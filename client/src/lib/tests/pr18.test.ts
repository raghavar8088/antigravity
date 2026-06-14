import { describe, it, expect } from "vitest";
import {
  profitModeAllowsStrategyInChop,
  type ProfitModeConfig,
} from "../trading/futuresProfitMode";
import { deriveScorecardAction } from "../analytics/futuresScorecardActions";
import { computeDeskRollingPnLScorecard } from "../portfolio/futuresDeskPnLTracker";
import { suggestWinnersFromScorecard } from "../trading/futuresWinnersRefresh";
import type { StrategyDiagnosticRow } from "../trading/futuresStrategyDiagnostics";

const pmOn = (): ProfitModeConfig => ({
  enabled: true,
  minQualityScore: 70,
  minMtfConfluence: 65,
  chopThresholdBoost: 10,
  minExpectedMoveK: 3,
  maxSameSideChop: 1,
  maxSameSideTrend: 2,
  maxOpenPositions: 6,
  dailyStratCap: 4,
  requireHighQualityInChop: true,
  blockCounterTrend: true,
  onlyPromotedOrActive: true,
});

const pmOff = (): ProfitModeConfig => ({ ...pmOn(), enabled: false });

describe("profitModeAllowsStrategyInChop", () => {
  it("blocks mtf template in chop", () => {
    expect(
      profitModeAllowsStrategyInChop(
        "MTF_TREND_ALIGN",
        "MTF_Trend_Align_Long",
        ["chop", "trendHigh"],
        "chop",
        pmOn(),
      ),
    ).toBe(false);
  });

  it("allows vwap template with regimes=['chop'] only", () => {
    expect(
      profitModeAllowsStrategyInChop(
        "BTCFT_VWAP_V0_LONG",
        "BTCFT_VWAP_V0_LONG_203",
        ["chop"],
        "chop",
        pmOn(),
      ),
    ).toBe(true);
  });

  it("allows vwap keyword family in chop", () => {
    expect(
      profitModeAllowsStrategyInChop(
        "BTCFT_VWAP_V0",
        "BTCFT_VWAP_V0_LONG_203",
        undefined,
        "chop",
        pmOn(),
      ),
    ).toBe(true);
  });

  it("inactive when profit mode off", () => {
    expect(
      profitModeAllowsStrategyInChop("MTF_BREAKOUT", "MTF_Breakout_Long", [], "chop", pmOff()),
    ).toBe(true);
  });
});

describe("deriveScorecardAction", () => {
  const now = Date.now();
  const ago = (h: number) => new Date(now - h * 3_600_000).toISOString();

  const mkTrades = (net: number, n: number) =>
    Array.from({ length: n }, (_, i) => ({
      closedAt: ago(1 + i * 0.1),
      netPnl: net,
      grossPnl: net + 3,
      fees: 2,
      strategyName: "MTF_Trend",
    }));

  it("ACT when feePct > 60 on last50", () => {
    const trades = [
      ...mkTrades(-8, 45),
      ...Array.from({ length: 10 }, () => ({
        closedAt: ago(0.5),
        netPnl: 2,
        grossPnl: 3,
        fees: 20,
        strategyName: "Scalp",
      })),
    ];
    const scorecard = computeDeskRollingPnLScorecard(trades, now);
    const action = deriveScorecardAction(scorecard, null, null, 28);
    expect(action.severity).toBe("ACT");
    expect(["RAISE_THRESHOLD", "TIGHTEN_EXITS"]).toContain(action.action);
  });

  it("OK when all targets pass", () => {
    const trades = Array.from({ length: 55 }, (_, i) => ({
      closedAt: ago(20 + i * 0.5),
      netPnl: 8,
      grossPnl: 12,
      fees: 2,
      strategyName: "VWAP_Range",
    }));
    const scorecard = computeDeskRollingPnLScorecard(trades, now);
    const action = deriveScorecardAction(scorecard, null, null, 28);
    expect(action.severity).toBe("OK");
    expect(action.action).toBe("NO_CHANGE");
  });
});

describe("suggestWinnersFromScorecard", () => {
  const row = (
    id: number,
    avg: number,
    trades: number,
    fee: number,
  ): StrategyDiagnosticRow => ({
    strategyId: id,
    strategyName: `Strat_${id}`,
    templateFamily: "TEST",
    totalTrades: trades,
    wins: avg > 0 ? trades : 0,
    losses: avg <= 0 ? trades : 0,
    winRate: avg > 0 ? 0.6 : 0.2,
    totalNetPnl: avg * trades,
    avgNetPnl: avg,
    avgWin: 10,
    avgLoss: -5,
    profitFactor: avg > 0 ? 1.5 : 0.5,
    totalFees: 10,
    feePctOfAbsGross: fee,
    avgHoldMinutes: 10,
    exitReasonCounts: {},
    slCount: 0,
    tpCount: trades,
    timeCount: 0,
    trailCount: 0,
    profitLockCount: 0,
    worstTrade: -10,
    bestTrade: 20,
    lastTradeAt: new Date().toISOString(),
    isProbe: false,
  });

  it("promotes positive expectancy only", () => {
    const suggestion = suggestWinnersFromScorecard([
      row(1, 12, 10, 0.2),
      row(2, -15, 10, 0.8),
      row(3, 5, 10, 0.3),
    ]);
    expect(suggestion.promote).toContain(1);
    expect(suggestion.promote).toContain(3);
    expect(suggestion.promote).not.toContain(2);
    expect(suggestion.demote).toContain(2);
  });
});
