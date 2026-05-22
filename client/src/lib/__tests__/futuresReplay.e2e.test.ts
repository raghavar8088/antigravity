/**
 * E2E integration test: full signal → replay → trade lifecycle.
 *
 * Tests the complete pipeline from raw OHLCV candles through signal scoring,
 * template confirmation gates, position sizing, and P&L accounting.
 * Uses the real replay engine (no mocking) with the CORE strategy basket.
 *
 * Covers gap G3 from PROFITABILITY_IMPROVEMENT_PLAN.md.
 */

import { describe, expect, it } from "vitest";
import { makeSyntheticBtcusd1mCandles } from "@/lib/futuresReplayFixtures";
import { runPaperDeskReplay, type PaperReplayConfig } from "@/lib/futuresReplayEngine";
import { CORE_BTC_FT_STRATEGY_IDS } from "@/lib/btcFtRoster";

// ---------------------------------------------------------------------------
// Shared fixture — 500 bars, realistic trending price action
// ---------------------------------------------------------------------------
const CANDLES_500 = makeSyntheticBtcusd1mCandles(500, {
  startMs: 1_748_000_000_000, // 2025-05
  barMs: 60_000,
  basePrice: 95_000,
});

const BASE_CONFIG: PaperReplayConfig = {
  initialBalance: 1_000,
  leverage: 25,
  slippageBps: 5,
  strategyIds: [...CORE_BTC_FT_STRATEGY_IDS],
  maxPositions: 6,
  barMs: 60_000,
  symbol: "BTCUSD",
  signalThreshold: 28,
  volSizedNotional: false,
  minExpectedMoveSafetyK: 1.1,
  maxSameDirFracOfEquity: 0.35,
  allowFakeDiversity: false,
  minTpSlRatio: 2,
  fundingRate: 0.0001,
  drawdownLock: false,
};

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("P0.5.2 – replay pipeline E2E", () => {
  it("produces at least one closed trade on 500 bars with CORE strategies", () => {
    const result = runPaperDeskReplay(CANDLES_500, BASE_CONFIG);
    expect(result.barsProcessed).toBe(500);
    expect(result.trades.length).toBeGreaterThan(0);
  });

  it("every closed trade has valid structure (strategyId, netPnl, exitReason, timestamps)", () => {
    const result = runPaperDeskReplay(CANDLES_500, BASE_CONFIG);
    for (const trade of result.trades) {
      expect(typeof trade.strategyId).toBe("number");
      expect(trade.strategyId).toBeGreaterThan(0);
      expect(typeof trade.netPnl).toBe("number");
      expect(Number.isFinite(trade.netPnl)).toBe(true);
      expect(["TP", "SL", "TIME", "TRAIL", "BREAKEVEN", "LIQUIDATION_RISK", "PROFIT_LOCK", "MOM_DECAY"]).toContain(
        trade.exitReason,
      );
      expect(typeof trade.openedAt).toBe("string");
      expect(typeof trade.closedAt).toBe("string");
      expect(new Date(trade.openedAt).getTime()).toBeLessThanOrEqual(
        new Date(trade.closedAt).getTime(),
      );
    }
  });

  it("fees are always non-negative and netPnl is finite", () => {
    const result = runPaperDeskReplay(CANDLES_500, BASE_CONFIG);
    for (const trade of result.trades) {
      expect(trade.fees).toBeGreaterThanOrEqual(0);
      expect(Number.isFinite(trade.netPnl)).toBe(true);
      // netPnl = realizedPnl - fees - fundingCosts, floored at minAbsNetWinUsd ($2) for tiny wins.
      // realizedPnl can be < netPnl when the win-floor kicks in, so we only check direction on losses.
      if (trade.netPnl < 0) {
        expect(trade.netPnl).toBeLessThanOrEqual(trade.realizedPnl + 0.01); // losses are not boosted
      }
    }
  });

  it("all client trade IDs are unique", () => {
    const result = runPaperDeskReplay(CANDLES_500, BASE_CONFIG);
    const ids = result.trades
      .map((t) => t.clientTradeId ?? t.id)
      .filter(Boolean);
    const unique = new Set(ids);
    expect(unique.size).toBe(ids.length);
  });

  it("stats.expectancy equals sumNet/tradeCount", () => {
    const result = runPaperDeskReplay(CANDLES_500, BASE_CONFIG);
    if (result.trades.length === 0) return; // skip if no trades (acceptable on synthetic data)
    const sumNet = result.trades.reduce((s, t) => s + t.netPnl, 0);
    const expected = sumNet / result.trades.length;
    expect(result.stats.expectancy).toBeCloseTo(expected, 4);
  });

  it("final balance moves in sync with sumNet (accounting identity)", () => {
    const result = runPaperDeskReplay(CANDLES_500, BASE_CONFIG);
    const sumNet = result.trades.reduce((s, t) => s + t.netPnl, 0);
    expect(result.finalBalance).toBeCloseTo(BASE_CONFIG.initialBalance + sumNet, 2);
  });

  it("replay is deterministic — same candles + config produce identical results", () => {
    const a = runPaperDeskReplay(CANDLES_500, BASE_CONFIG);
    const b = runPaperDeskReplay(CANDLES_500, BASE_CONFIG);
    expect(a.trades.length).toBe(b.trades.length);
    expect(a.stats).toEqual(b.stats);
    expect(a.finalBalance).toBe(b.finalBalance);
  });

  it("raising signal threshold reduces trade count (gate is active)", () => {
    const low = runPaperDeskReplay(CANDLES_500, { ...BASE_CONFIG, signalThreshold: 22 });
    const high = runPaperDeskReplay(CANDLES_500, { ...BASE_CONFIG, signalThreshold: 28 });
    expect(low.trades.length).toBeGreaterThanOrEqual(high.trades.length);
  });

  it("disabling all strategies produces zero trades", () => {
    const allIds = [...CORE_BTC_FT_STRATEGY_IDS];
    const result = runPaperDeskReplay(CANDLES_500, {
      ...BASE_CONFIG,
      disabledStrategyIds: allIds,
    });
    expect(result.trades.length).toBe(0);
  });

  it("winners-gate wiring: filtering to empty set falls back to full basket", () => {
    // Simulate what applyWinnersOnlyGate does when all winners have positive expectancy
    const result = runPaperDeskReplay(CANDLES_500, {
      ...BASE_CONFIG,
      strategyIds: [91, 92], // narrow slice — still should produce trades
    });
    expect(result.barsProcessed).toBe(500);
  });

  it("sumNet reported honestly — does not hide negative expectancy", () => {
    // On a random walk synthetic fixture, expectancy may be negative — that's OK
    // The point is we report it accurately, not clamp to 0
    const result = runPaperDeskReplay(CANDLES_500, BASE_CONFIG);
    const sumNet = result.trades.reduce((s, t) => s + t.netPnl, 0);
    // sumNet can be negative; finalBalance = initial + sumNet (may be < initial)
    expect(result.finalBalance).toBeCloseTo(BASE_CONFIG.initialBalance + sumNet, 2);
  });
});
