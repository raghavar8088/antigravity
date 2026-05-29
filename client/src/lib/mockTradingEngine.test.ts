/**
 * Smoke tests for the Mock Trading engine.
 *
 * Each test proves a requirement from the Mock Trading spec:
 *   - Signals blocked by REGIME / SIGNAL / ATR_FEES still create mock trades.
 *   - Many open mock trades can coexist (no max-open cap).
 *   - PnL updates when the live BTC price changes.
 *   - TP / SL / max-hold exits behave correctly.
 *   - Analytics and filters work as documented.
 */

import { describe, expect, it } from "vitest";
import { createTraceRow, type StrategySignalTraceRow } from "@/lib/strategySignalTrace";
import {
  applyPriceTickToTrade,
  buildMockTradeFromTrace,
  closeMockTrade,
  computeAnalytics,
  computeMockPnl,
  DEFAULT_MOCK_EXIT,
  filterMockTrades,
  isStrategySignalRaised,
  MOCK_IGNORED_GATES,
  type MockExitConfig,
  type MockTrade,
} from "@/lib/mockTradingEngine";

const T0 = 1_700_000_000_000;

const baseConfig: MockExitConfig = {
  takeProfitPct: 1.5,
  stopLossPct: 0.6,
  maxHoldMinutes: 45,
  notionalUsd: 100,
};

function traceRow(
  overrides: Partial<StrategySignalTraceRow> = {},
): StrategySignalTraceRow {
  return createTraceRow({
    tickAt: T0,
    mode: "browser",
    symbol: "BTCUSD",
    strategyId: 91,
    strategyName: "Trend_Continuation_Long",
    side: "LONG",
    status: "REJECTED",
    gate: "REGIME",
    reason: "regime chop blocked",
    signalScore: 28,
    requiredThreshold: 20,
    confirmPassed: true,
    regime: "chop",
    regimeAllowed: false,
    ...overrides,
  });
}

function buildOpenTrade(overrides: Partial<MockTrade> = {}): MockTrade {
  const trade = buildMockTradeFromTrace({
    row: traceRow(),
    currentPrice: 60_000,
    config: baseConfig,
    now: T0,
  });
  if (!trade) throw new Error("expected a mock trade");
  return { ...trade, ...overrides };
}

// ── Required: blocked-by-REGIME signals still create mock trades ─────────────
describe("REGIME_BLOCKING signals still create mock trades", () => {
  it("REGIME rejection produces a mock trade with the blocker recorded", () => {
    const row = traceRow({ gate: "REGIME", reason: "regime chop blocked" });
    const trade = buildMockTradeFromTrace({
      row,
      currentPrice: 60_000,
      config: baseConfig,
      now: T0,
    });
    expect(trade).not.toBeNull();
    expect(trade?.status).toBe("OPEN");
    expect(trade?.blockers.some((b) => b.gate === "REGIME")).toBe(true);
  });
});

// ── Required: blocked-by-SIGNAL still create mock trades ─────────────────────
describe("SIGNAL gate rejections still create mock trades", () => {
  it("a SIGNAL rejection (score < threshold) still produces a mock trade", () => {
    const row = traceRow({
      gate: "SIGNAL",
      reason: "Score below threshold",
      status: "REJECTED",
      signalScore: 12,
      requiredThreshold: 20,
    });
    const trade = buildMockTradeFromTrace({
      row,
      currentPrice: 60_000,
      config: baseConfig,
      now: T0,
    });
    expect(trade).not.toBeNull();
    expect(trade?.blockers.some((b) => b.gate === "SIGNAL")).toBe(true);
    expect(trade?.signalScore).toBe(12);
    expect(trade?.requiredThreshold).toBe(20);
  });
});

// ── Required: blocked-by-ATR_FEES still create mock trades ───────────────────
describe("ATR_FEES rejections still create mock trades", () => {
  it("an ATR_FEES rejection still produces a mock trade with blocker recorded", () => {
    const row = traceRow({
      gate: "ATR_FEES",
      reason: "ATR insufficient vs fee hurdle",
      atrPct: 0.0002,
      feeHurdlePassed: false,
    });
    const trade = buildMockTradeFromTrace({
      row,
      currentPrice: 60_000,
      config: baseConfig,
      now: T0,
    });
    expect(trade).not.toBeNull();
    expect(trade?.blockers.some((b) => b.gate === "ATR_FEES")).toBe(true);
  });
});

// ── Ignored-gate inventory ───────────────────────────────────────────────────
describe("MOCK_IGNORED_GATES inventory", () => {
  it("includes every gate the spec says must be ignored", () => {
    const required = [
      "REGIME",
      "SIGNAL",
      "ATR_FEES",
      "CONFIRM",
      "MTF",
      "COOLDOWN",
      "MAX_OPEN",
      "SAME_SIDE",
      "CATEGORY",
      "QUALITY",
    ];
    for (const gate of required) {
      expect(MOCK_IGNORED_GATES).toContain(gate);
    }
  });
});

// ── Signal raising detection ─────────────────────────────────────────────────
describe("isStrategySignalRaised", () => {
  it("returns false when no side is produced", () => {
    expect(isStrategySignalRaised(traceRow({ side: undefined }))).toBe(false);
  });
  it("returns false when score is zero", () => {
    expect(isStrategySignalRaised(traceRow({ signalScore: 0 }))).toBe(false);
  });
  it("returns true for a FIRED row", () => {
    expect(isStrategySignalRaised(traceRow({ status: "FIRED", gate: "CONFIRM" }))).toBe(true);
  });
  it("returns true for a REJECTED row with side+score", () => {
    expect(isStrategySignalRaised(traceRow({ status: "REJECTED", gate: "REGIME" }))).toBe(true);
  });
});

// ── Required: many open trades can coexist (no max-open cap) ─────────────────
describe("many mock trades can remain open simultaneously", () => {
  it("opens 50 trades from 50 distinct trace rows", () => {
    const trades = [];
    for (let i = 0; i < 50; i++) {
      const row = traceRow({
        traceId: `trace-${i}`,
        strategyId: 91 + (i % 12),
        strategyName: `Strat_${i}`,
        gate: "MAX_OPEN",
        reason: "max-open cap",
        side: i % 2 === 0 ? "LONG" : "SHORT",
      });
      const t = buildMockTradeFromTrace({ row, currentPrice: 60_000, config: baseConfig, now: T0 });
      expect(t).not.toBeNull();
      if (t) trades.push(t);
    }
    expect(trades.length).toBe(50);
    expect(trades.every((t) => t.status === "OPEN")).toBe(true);
  });
});

// ── Required: PnL updates when BTC price changes ─────────────────────────────
describe("PnL updates when BTC price changes", () => {
  it("long unrealized PnL goes positive when price rises", () => {
    const trade = buildOpenTrade();
    const updated = applyPriceTickToTrade({
      trade,
      price: 60_300, // +0.5%
      config: baseConfig,
      now: T0 + 60_000,
    });
    expect(updated.status).toBe("OPEN");
    expect(updated.unrealizedPnl).toBeGreaterThan(0);
    expect(updated.currentPrice).toBe(60_300);
  });

  it("short unrealized PnL goes positive when price drops", () => {
    const trade = buildOpenTrade({ side: "SELL" });
    const updated = applyPriceTickToTrade({
      trade,
      price: 59_700, // -0.5%
      config: baseConfig,
      now: T0 + 60_000,
    });
    expect(updated.status).toBe("OPEN");
    expect(updated.unrealizedPnl).toBeGreaterThan(0);
  });

  it("PnL maths: $100 notional, 1% upward move on long ≈ +$1", () => {
    const trade = buildOpenTrade();
    const updated = applyPriceTickToTrade({
      trade,
      price: 60_600, // +1.0%
      config: baseConfig,
      now: T0 + 60_000,
    });
    expect(updated.unrealizedPnl).toBeCloseTo(1.0, 1);
  });
});

// ── Exit logic ───────────────────────────────────────────────────────────────
describe("mock exit logic", () => {
  it("closes long with TAKE_PROFIT when price hits tp threshold", () => {
    const trade = buildOpenTrade();
    const tpPrice = trade.entryPrice * (1 + baseConfig.takeProfitPct / 100) + 1;
    const updated = applyPriceTickToTrade({
      trade,
      price: tpPrice,
      config: baseConfig,
      now: T0 + 60_000,
    });
    expect(updated.status).toBe("CLOSED");
    expect(updated.exitReason).toBe("TAKE_PROFIT");
    expect(updated.realizedPnl).toBeGreaterThan(0);
  });

  it("closes long with STOP_LOSS when price drops past sl threshold", () => {
    const trade = buildOpenTrade();
    const slPrice = trade.entryPrice * (1 - baseConfig.stopLossPct / 100) - 1;
    const updated = applyPriceTickToTrade({
      trade,
      price: slPrice,
      config: baseConfig,
      now: T0 + 60_000,
    });
    expect(updated.status).toBe("CLOSED");
    expect(updated.exitReason).toBe("STOP_LOSS");
    expect(updated.realizedPnl).toBeLessThan(0);
  });

  it("closes with MAX_HOLD when age exceeds maxHoldMinutes", () => {
    const trade = buildOpenTrade();
    const updated = applyPriceTickToTrade({
      trade,
      price: trade.entryPrice + 5,
      config: baseConfig,
      now: T0 + baseConfig.maxHoldMinutes * 60_000 + 1_000,
    });
    expect(updated.status).toBe("CLOSED");
    expect(updated.exitReason).toBe("MAX_HOLD");
  });

  it("does not re-close a CLOSED trade", () => {
    const trade = buildOpenTrade();
    const closed = closeMockTrade(trade, trade.entryPrice + 10, T0 + 1000);
    const reapplied = applyPriceTickToTrade({
      trade: closed,
      price: closed.exitPrice ?? 0,
      config: baseConfig,
      now: T0 + 2_000,
    });
    expect(reapplied).toBe(closed);
  });
});

// ── Analytics ────────────────────────────────────────────────────────────────
describe("analytics roll-up", () => {
  it("counts open, closed, wins, losses, and profit factor", () => {
    const open1 = buildOpenTrade({ id: "o1", traceId: "t1" });
    const open2 = buildOpenTrade({ id: "o2", traceId: "t2" });

    const winRow = traceRow({ traceId: "t3", strategyId: 95 });
    const winTrade = buildMockTradeFromTrace({
      row: winRow,
      currentPrice: 60_000,
      config: baseConfig,
      now: T0,
    })!;
    const closedWin = closeMockTrade(winTrade, 60_000 * 1.02, T0 + 30_000);

    const lossRow = traceRow({ traceId: "t4", strategyId: 95 });
    const lossTrade = buildMockTradeFromTrace({
      row: lossRow,
      currentPrice: 60_000,
      config: baseConfig,
      now: T0,
    })!;
    const closedLoss = closeMockTrade(lossTrade, 60_000 * 0.995, T0 + 30_000);

    const tickedOpen1 = applyPriceTickToTrade({
      trade: open1,
      price: 60_120,
      config: baseConfig,
      now: T0 + 1_000,
    });

    const analytics = computeAnalytics([tickedOpen1, open2, closedWin, closedLoss]);
    expect(analytics.totalTrades).toBe(4);
    expect(analytics.openTrades).toBe(2);
    expect(analytics.closedTrades).toBe(2);
    expect(analytics.winRate).toBeCloseTo(0.5, 5);
    expect(analytics.realizedPnl).toBeCloseTo(closedWin.realizedPnl + closedLoss.realizedPnl, 5);
    expect(analytics.profitFactor).not.toBeNull();
    expect(analytics.perStrategy.length).toBe(2);
    expect(analytics.perBlocker.some((b) => b.gate === "REGIME")).toBe(true);
  });
});

// ── Filters ──────────────────────────────────────────────────────────────────
describe("filterMockTrades", () => {
  it("filters by status, side, strategy, blocker, and profitability", () => {
    const winTrade = buildOpenTrade({ id: "w", traceId: "tw" });
    const closedWin = closeMockTrade(winTrade, winTrade.entryPrice * 1.02, T0 + 30_000);
    const lossTrade = buildOpenTrade({ id: "l", traceId: "tl", side: "SELL" });
    const closedLoss = closeMockTrade(lossTrade, lossTrade.entryPrice * 1.005, T0 + 30_000);
    const stillOpen = buildOpenTrade({ id: "o", traceId: "to" });

    const all = [closedWin, closedLoss, stillOpen];
    expect(filterMockTrades(all, { status: "OPEN" }).map((t) => t.id)).toEqual(["o"]);
    expect(filterMockTrades(all, { side: "SELL" }).map((t) => t.id)).toEqual(["l"]);
    expect(filterMockTrades(all, { profitability: "profit" }).map((t) => t.id)).toEqual(["w"]);
    expect(filterMockTrades(all, { profitability: "loss" }).map((t) => t.id)).toEqual(["l"]);
    expect(filterMockTrades(all, { blockerGate: "REGIME" }).length).toBe(3);
    expect(filterMockTrades(all, { blockerGate: "DOES_NOT_EXIST" }).length).toBe(0);
  });
});

// ── PnL math sanity ──────────────────────────────────────────────────────────
describe("computeMockPnl", () => {
  it("returns 0 for invalid inputs", () => {
    expect(computeMockPnl("BUY", NaN, 100, 1)).toBe(0);
    expect(computeMockPnl("BUY", 100, 100, 0)).toBe(0);
  });
  it("computes long PnL correctly", () => {
    expect(computeMockPnl("BUY", 100, 110, 2)).toBe(20);
  });
  it("computes short PnL correctly", () => {
    expect(computeMockPnl("SELL", 100, 90, 2)).toBe(20);
  });
});

// ── Default config sanity ────────────────────────────────────────────────────
describe("DEFAULT_MOCK_EXIT", () => {
  it("uses non-zero TP, SL, hold, and notional", () => {
    expect(DEFAULT_MOCK_EXIT.takeProfitPct).toBeGreaterThan(0);
    expect(DEFAULT_MOCK_EXIT.stopLossPct).toBeGreaterThan(0);
    expect(DEFAULT_MOCK_EXIT.maxHoldMinutes).toBeGreaterThan(0);
    expect(DEFAULT_MOCK_EXIT.notionalUsd).toBeGreaterThan(0);
  });
});
