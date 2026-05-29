/**
 * Smoke tests for the Mock Trading engine.
 *
 * Each test proves a requirement from the Mock Trading spec:
 *   - $1,000,000 default starting balance.
 *   - Trade sizing mirrors the production paper desk (fixed-% of equity).
 *   - Opening a mock trade reserves margin + exposure on the account.
 *   - Closing winners adds to cash/equity; closing losers reduces them.
 *   - Live price movement updates equity and unrealized PnL.
 *   - Signals blocked by REGIME / SIGNAL / ATR_FEES still create mock trades.
 *   - Fees and slippage are applied at entry and exit.
 *   - Persistence validators reject corrupt trade rows.
 */

import { describe, expect, it } from "vitest";
import { createTraceRow, type SignalTraceGate, type StrategySignalTraceRow } from "@/lib/strategySignalTrace";
import {
  applyPriceTickToTrade,
  buildMockTradeFromTrace,
  buildMockTradeFromResearchSignal,
  canOpenAdditionalMockTrade,
  closeMockTrade,
  computeAccountState,
  computeAnalytics,
  computeMockNetPnlAtExitMark,
  computeMockNotional,
  computeMockPnl,
  countOpenMockTrades,
  DEFAULT_MOCK_TRADING_CONFIG,
  filterMockTrades,
  isStrategySignalRaised,
  isValidMockConfig,
  isValidMockTrade,
  maxOpenMockTradesFromConfig,
  MOCK_IGNORED_GATES,
  MOCK_PERSIST_VERSION,
  MOCK_TRADE_SORT_OPTIONS,
  mockTradePnl,
  sortMockTrades,
  type MockTrade,
  type MockTradingConfig,
} from "@/lib/mockTradingEngine";

const T0 = 1_700_000_000_000;
const ENTRY = 60_000;

const baseConfig: MockTradingConfig = { ...DEFAULT_MOCK_TRADING_CONFIG };
const wideExitConfig: MockTradingConfig = {
  ...DEFAULT_MOCK_TRADING_CONFIG,
  takeProfitPct: 100,
  stopLossPct: 100,
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

function build(opts: Partial<{
  row: StrategySignalTraceRow;
  currentPrice: number;
  config: MockTradingConfig;
  equity: number;
  now: number;
}> = {}): MockTrade {
  const trade = buildMockTradeFromTrace({
    row: opts.row ?? traceRow(),
    currentPrice: opts.currentPrice ?? ENTRY,
    config: opts.config ?? baseConfig,
    equity: opts.equity,
    now: opts.now ?? T0,
  });
  if (!trade) throw new Error("expected a mock trade");
  return trade;
}

// ── Defaults: $1,000,000 starting balance ────────────────────────────────────
describe("DEFAULT_MOCK_TRADING_CONFIG", () => {
  it("uses a $1,000,000 starting balance", () => {
    expect(DEFAULT_MOCK_TRADING_CONFIG.startingBalanceUsd).toBe(1_000_000);
  });
  it("uses 25× leverage (matching BTC FT)", () => {
    expect(DEFAULT_MOCK_TRADING_CONFIG.leverage).toBe(25);
  });
  it("defaults to fixed-% of equity sizing at 1%", () => {
    expect(DEFAULT_MOCK_TRADING_CONFIG.sizingMode).toBe("fixed_pct_equity");
    expect(DEFAULT_MOCK_TRADING_CONFIG.fixedPctOfEquity).toBe(1);
  });
  it("applies a non-zero taker fee + slippage", () => {
    expect(DEFAULT_MOCK_TRADING_CONFIG.takerFeePct).toBeGreaterThan(0);
    expect(DEFAULT_MOCK_TRADING_CONFIG.slippageBpsPerSide).toBeGreaterThan(0);
  });
  it("uses percentage-distance TP/SL defaults", () => {
    expect(DEFAULT_MOCK_TRADING_CONFIG.takeProfitPct).toBe(1.5);
    expect(DEFAULT_MOCK_TRADING_CONFIG.stopLossPct).toBe(0.6);
  });
  it("allows 5,000 simultaneous open mock trades by default", () => {
    expect(DEFAULT_MOCK_TRADING_CONFIG.maxOpenMockTrades).toBe(5_000);
    expect(maxOpenMockTradesFromConfig(DEFAULT_MOCK_TRADING_CONFIG)).toBe(5_000);
  });
});

// ── Sizing reflects $1M account ──────────────────────────────────────────────
describe("computeMockNotional with $1M account", () => {
  it("at default 1% of equity yields ~$10,000 notional per trade", () => {
    const n = computeMockNotional({ config: baseConfig, equity: 1_000_000 });
    expect(n).toBeCloseTo(10_000, 0);
  });
  it("respects fixed_notional override", () => {
    const cfg = { ...baseConfig, sizingMode: "fixed_notional" as const, fixedNotionalUsd: 25_000 };
    expect(computeMockNotional({ config: cfg, equity: 1_000_000 })).toBe(25_000);
  });
  it("risk_pct_equity scales notional inversely with SL distance", () => {
    const cfg = { ...baseConfig, sizingMode: "risk_pct_equity" as const, riskPctOfEquity: 1, stopLossPct: 1 };
    const small = computeMockNotional({ config: cfg, equity: 1_000_000, slPct: 0.5 });
    const big = computeMockNotional({ config: cfg, equity: 1_000_000, slPct: 2 });
    expect(small).toBeGreaterThan(big);
  });
});

// ── Required: blocked-by-REGIME/SIGNAL/ATR_FEES still create mock trades ─────
describe("blockers do not prevent mock trade creation", () => {
  for (const gate of ["REGIME", "SIGNAL", "ATR_FEES", "CONFIRM", "MTF", "COOLDOWN", "MAX_OPEN"] as const) {
    it(`gate=${gate} still produces a mock trade and records the blocker`, () => {
      const trade = buildMockTradeFromTrace({
        row: traceRow({ gate: gate as SignalTraceGate, reason: `${gate} rejection` }),
        currentPrice: ENTRY,
        config: baseConfig,
        now: T0,
      });
      expect(trade).not.toBeNull();
      expect(trade?.blockers.some((b) => b.gate === gate)).toBe(true);
    });
  }
});

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
      "OCCUPIED",
      "SAME_SIDE",
      "CATEGORY",
      "QUALITY",
    ];
    for (const gate of required) expect(MOCK_IGNORED_GATES).toContain(gate);
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
  it("returns true for FIRED and REJECTED rows with side+score", () => {
    expect(isStrategySignalRaised(traceRow({ status: "FIRED", gate: "CONFIRM" }))).toBe(true);
    expect(isStrategySignalRaised(traceRow({ status: "REJECTED", gate: "REGIME" }))).toBe(true);
  });
});

// ── Account state: starting balance and exposure on open ─────────────────────
describe("computeAccountState on an empty book", () => {
  it("equity equals startingBalance when no trades are open", () => {
    const acct = computeAccountState([], baseConfig);
    expect(acct.startingBalance).toBe(1_000_000);
    expect(acct.equity).toBe(1_000_000);
    expect(acct.cashBalance).toBe(1_000_000);
    expect(acct.exposure).toBe(0);
    expect(acct.marginUsed).toBe(0);
    expect(acct.openCount).toBe(0);
  });
});

describe("opening a mock trade updates exposure and reserved margin", () => {
  it("reserves notional / leverage as marginUsed", () => {
    const trade = build();
    const acct = computeAccountState([trade], baseConfig);
    expect(acct.openCount).toBe(1);
    expect(acct.exposure).toBeCloseTo(trade.notional, 4);
    expect(acct.marginUsed).toBeCloseTo(trade.notional / trade.leverage, 4);
    expect(acct.cashBalance).toBeCloseTo(baseConfig.startingBalanceUsd - acct.marginUsed, 4);
    expect(acct.availableBalance).toBeCloseTo(acct.equity - acct.marginUsed, 4);
  });
});

// ── Many open trades coexist ─────────────────────────────────────────────────
describe("many mock trades can remain open simultaneously", () => {
  it("opens 50 trades and aggregates exposure across them", () => {
    const trades: MockTrade[] = [];
    for (let i = 0; i < 50; i++) {
      const row = traceRow({
        traceId: `trace-${i}`,
        strategyId: 91 + (i % 12),
        strategyName: `Strat_${i}`,
        gate: "MAX_OPEN",
        reason: "max-open cap",
        side: i % 2 === 0 ? "LONG" : "SHORT",
      });
      trades.push(build({ row }));
    }
    expect(trades.length).toBe(50);
    const acct = computeAccountState(trades, baseConfig);
    expect(acct.openCount).toBe(50);
    expect(acct.exposure).toBeGreaterThan(0);
    // 50 trades × ~$10K = ~$500K exposure on $1M (sanity)
    expect(acct.exposure).toBeGreaterThan(400_000);
    expect(acct.exposure).toBeLessThan(600_000);
  });

  it("allows the 5,000th open mock trade and blocks the 5,001st", () => {
    const seed = build({ row: traceRow({ gate: "MAX_OPEN", reason: "production max-open cap" }) });
    const trades: MockTrade[] = Array.from({ length: 4_999 }, (_, i) => ({
      ...seed,
      id: `mock-limit-${i}`,
      traceId: `limit-${i}`,
      openedAt: T0 + i,
    }));

    expect(canOpenAdditionalMockTrade({ trades, config: baseConfig })).toBe(true);
    const fiveThousand = [
      ...trades,
      { ...seed, id: "mock-limit-4999", traceId: "limit-4999", openedAt: T0 + 4_999 },
    ];
    expect(countOpenMockTrades(fiveThousand)).toBe(5_000);
    expect(computeAccountState(fiveThousand, baseConfig).openCount).toBe(5_000);
    expect(canOpenAdditionalMockTrade({ trades: fiveThousand, config: baseConfig })).toBe(false);
  });

  it("does not treat the production MAX_OPEN blocker as the mock trade cap", () => {
    const trade = build({
      row: traceRow({
        traceId: "max-open-production-blocker",
        gate: "MAX_OPEN",
        reason: "production desk max-open cap",
      }),
      config: { ...baseConfig, maxOpenMockTrades: 5_000 },
    });

    expect(trade.blockers.some((blocker) => blocker.gate === "MAX_OPEN")).toBe(true);
    expect(canOpenAdditionalMockTrade({ trades: [trade], config: baseConfig })).toBe(true);
  });
});

// ── Live price movement → equity and unrealized PnL ──────────────────────────
describe("PnL updates when BTC price changes", () => {
  it("long unrealized PnL rises when price rises", () => {
    const trade = build({ config: wideExitConfig });
    const tick = applyPriceTickToTrade({
      trade,
      price: ENTRY * 1.005,
      config: wideExitConfig,
      now: T0 + 60_000,
    });
    expect(tick.status).toBe("OPEN");
    expect(tick.unrealizedPnl).toBeGreaterThan(0);
  });
  it("short unrealized PnL rises when price drops", () => {
    const trade = build({ row: traceRow({ side: "SHORT" }), config: wideExitConfig });
    const tick = applyPriceTickToTrade({
      trade,
      price: ENTRY * 0.995,
      config: wideExitConfig,
      now: T0 + 60_000,
    });
    expect(tick.unrealizedPnl).toBeGreaterThan(0);
  });
  it("equity rises and falls with the mark price for an open long", () => {
    const trade = build();
    const up = applyPriceTickToTrade({ trade, price: ENTRY * 1.01, config: baseConfig, now: T0 + 60_000 });
    const down = applyPriceTickToTrade({ trade, price: ENTRY * 0.99, config: baseConfig, now: T0 + 60_000 });
    const acctUp = computeAccountState([up], baseConfig);
    const acctDown = computeAccountState([down], baseConfig);
    expect(acctUp.equity).toBeGreaterThan(acctDown.equity);
  });
});

// ── Closing a winning trade increases cash/equity ────────────────────────────
describe("close lifecycle", () => {
  it("closing a winning trade increases cash and equity", () => {
    const open = build();
    const acctOpen = computeAccountState([open], baseConfig);
    const closed = closeMockTrade(open, ENTRY * 1.02, T0 + 60_000, baseConfig);
    const acctClosed = computeAccountState([closed], baseConfig);
    expect(closed.realizedPnl).toBeGreaterThan(0);
    expect(acctClosed.equity).toBeGreaterThan(acctOpen.startingBalance);
    expect(acctClosed.cashBalance).toBeGreaterThan(acctOpen.startingBalance);
    expect(acctClosed.marginUsed).toBe(0); // margin released
  });

  it("closing a losing trade decreases cash and equity", () => {
    const open = build();
    const closed = closeMockTrade(open, ENTRY * 0.98, T0 + 60_000, baseConfig);
    const acct = computeAccountState([closed], baseConfig);
    expect(closed.realizedPnl).toBeLessThan(0);
    expect(acct.equity).toBeLessThan(baseConfig.startingBalanceUsd);
    expect(acct.cashBalance).toBeLessThan(baseConfig.startingBalanceUsd);
  });

  it("fees are applied to realized PnL on close", () => {
    const open = build();
    const closed = closeMockTrade(open, ENTRY * 1.02, T0 + 60_000, baseConfig);
    expect(closed.fees).toBeGreaterThan(0);
    // Realized = gross - fees, so realized < gross
    const grossLong = computeMockPnl("BUY", open.entryPrice, ENTRY * 1.02, open.quantity);
    expect(closed.realizedPnl).toBeLessThan(grossLong);
  });

  it("slippage is applied at entry — entry price differs from signal price for a buy", () => {
    const open = build();
    expect(open.entryPrice).toBeGreaterThan(open.signalPrice); // BUY pays a worse fill
  });

  it("slippage is applied at exit — exit price is worsened by configured bps", () => {
    const open = build();
    const closed = closeMockTrade(open, ENTRY * 1.02, T0 + 60_000, baseConfig);
    // On a long sell-to-close, exit slippage makes you receive LESS than mark.
    expect(closed.exitPrice ?? 0).toBeLessThan(ENTRY * 1.02);
  });
});

// ── Exit logic uses setup-derived TP / SL / MaxHold ──────────────────────────
describe("exit reasons", () => {
  it("BUY closes at TP with setup-derived profit", () => {
    const trade = build();
    const expected = computeMockNetPnlAtExitMark({
      side: trade.side,
      entryPrice: trade.entryPrice,
      exitMarkPrice: trade.takeProfitPrice,
      quantity: trade.quantity,
      notional: trade.notional,
      config: baseConfig,
    });
    const tick = applyPriceTickToTrade({ trade, price: trade.takeProfitPrice, config: baseConfig, now: T0 + 60_000 });
    expect(tick.status).toBe("CLOSED");
    expect(tick.exitReason).toBe("TAKE_PROFIT");
    expect(trade.takeProfitUsd).toBeCloseTo(expected, 6);
    expect(tick.realizedPnl).toBeCloseTo(expected, 6);
    expect(tick.realizedPnl).not.toBeCloseTo(10, 2);
  });

  it("BUY closes at SL with setup-derived loss", () => {
    const trade = build();
    const expected = computeMockNetPnlAtExitMark({
      side: trade.side,
      entryPrice: trade.entryPrice,
      exitMarkPrice: trade.stopLossPrice,
      quantity: trade.quantity,
      notional: trade.notional,
      config: baseConfig,
    });
    const tick = applyPriceTickToTrade({ trade, price: trade.stopLossPrice, config: baseConfig, now: T0 + 60_000 });
    expect(tick.status).toBe("CLOSED");
    expect(tick.exitReason).toBe("STOP_LOSS");
    expect(trade.stopLossUsd).toBeCloseTo(Math.abs(expected), 6);
    expect(tick.realizedPnl).toBeCloseTo(expected, 6);
    expect(tick.realizedPnl).not.toBeCloseTo(-5, 2);
  });

  it("SELL closes at TP with setup-derived profit", () => {
    const trade = build({ row: traceRow({ side: "SHORT" }) });
    const expected = computeMockNetPnlAtExitMark({
      side: trade.side,
      entryPrice: trade.entryPrice,
      exitMarkPrice: trade.takeProfitPrice,
      quantity: trade.quantity,
      notional: trade.notional,
      config: baseConfig,
    });
    const tick = applyPriceTickToTrade({ trade, price: trade.takeProfitPrice, config: baseConfig, now: T0 + 60_000 });
    expect(tick.status).toBe("CLOSED");
    expect(tick.exitReason).toBe("TAKE_PROFIT");
    expect(tick.realizedPnl).toBeCloseTo(expected, 6);
    expect(tick.realizedPnl).not.toBeCloseTo(10, 2);
  });

  it("SELL closes at SL with setup-derived loss", () => {
    const trade = build({ row: traceRow({ side: "SHORT" }) });
    const expected = computeMockNetPnlAtExitMark({
      side: trade.side,
      entryPrice: trade.entryPrice,
      exitMarkPrice: trade.stopLossPrice,
      quantity: trade.quantity,
      notional: trade.notional,
      config: baseConfig,
    });
    const tick = applyPriceTickToTrade({ trade, price: trade.stopLossPrice, config: baseConfig, now: T0 + 60_000 });
    expect(tick.status).toBe("CLOSED");
    expect(tick.exitReason).toBe("STOP_LOSS");
    expect(tick.realizedPnl).toBeCloseTo(expected, 6);
    expect(tick.realizedPnl).not.toBeCloseTo(-5, 2);
  });

  it("uses configurable TP/SL percentages for new trades", () => {
    const cfg = { ...baseConfig, takeProfitPct: 2, stopLossPct: 1 };
    const trade = build({ config: cfg });
    expect(trade.takeProfitPrice).toBeCloseTo(trade.entryPrice * 1.02, 6);
    expect(trade.stopLossPrice).toBeCloseTo(trade.entryPrice * 0.99, 6);
    expect(trade.takeProfitUsd).toBeGreaterThan(0);
    expect(trade.stopLossUsd).toBeGreaterThan(0);
    expect(trade.riskRewardRatio).toBeCloseTo(trade.takeProfitUsd / trade.stopLossUsd, 6);

    const tick = applyPriceTickToTrade({ trade, price: trade.takeProfitPrice, config: cfg, now: T0 + 60_000 });
    expect(tick.realizedPnl).toBeCloseTo(trade.takeProfitUsd, 6);
  });

  it("MAX_HOLD triggers when age >= maxHoldMinutes", () => {
    const trade = build({ config: wideExitConfig });
    const tick = applyPriceTickToTrade({
      trade,
      price: trade.entryPrice + 1,
      config: wideExitConfig,
      now: T0 + wideExitConfig.maxHoldMinutes * 60_000 + 1_000,
    });
    expect(tick.exitReason).toBe("MAX_HOLD");
  });
  it("CLOSED trade is not re-closed", () => {
    const open = build();
    const closed = closeMockTrade(open, ENTRY * 1.02, T0 + 60_000, baseConfig);
    const reapplied = applyPriceTickToTrade({
      trade: closed,
      price: ENTRY * 1.05,
      config: baseConfig,
      now: T0 + 70_000,
    });
    expect(reapplied).toBe(closed);
  });
});

// ── Drawdown / return % ──────────────────────────────────────────────────────
describe("max drawdown and return %", () => {
  it("return % = (equity - startingBalance) / startingBalance", () => {
    const win = closeMockTrade(build(), ENTRY * 1.01, T0 + 30_000, baseConfig);
    const acct = computeAccountState([win], baseConfig);
    expect(acct.returnPct).toBeCloseTo(win.realizedPnl / baseConfig.startingBalanceUsd, 6);
  });

  it("max drawdown tracks the worst trough after a winning streak", () => {
    const win1 = closeMockTrade(build({ row: traceRow({ traceId: "a" }) }), ENTRY * 1.02, T0 + 1_000, baseConfig);
    const win2 = closeMockTrade(build({ row: traceRow({ traceId: "b" }) }), ENTRY * 1.02, T0 + 2_000, baseConfig);
    const lossRow = traceRow({ traceId: "c" });
    const loss = closeMockTrade(build({ row: lossRow }), ENTRY * 0.97, T0 + 3_000, baseConfig);
    const acct = computeAccountState([win1, win2, loss], baseConfig);
    expect(acct.maxDrawdownPct).toBeGreaterThan(0);
  });

  it("no drawdown on a clean winning streak", () => {
    const win1 = closeMockTrade(build({ row: traceRow({ traceId: "a" }) }), ENTRY * 1.01, T0 + 1_000, baseConfig);
    const win2 = closeMockTrade(build({ row: traceRow({ traceId: "b" }) }), ENTRY * 1.02, T0 + 2_000, baseConfig);
    const acct = computeAccountState([win1, win2], baseConfig);
    expect(acct.maxDrawdownPct).toBeLessThan(1e-9);
  });
});

// ── Analytics + per-strategy exposure ────────────────────────────────────────
describe("per-strategy analytics", () => {
  it("aggregates exposure per strategy across open trades", () => {
    const t1 = build({ row: traceRow({ traceId: "a", strategyId: 91 }) });
    const t2 = build({ row: traceRow({ traceId: "b", strategyId: 91 }) });
    const t3 = build({ row: traceRow({ traceId: "c", strategyId: 95 }) });
    const a = computeAnalytics([t1, t2, t3]);
    const s91 = a.perStrategy.find((s) => s.strategyId === 91)!;
    const s95 = a.perStrategy.find((s) => s.strategyId === 95)!;
    expect(s91.exposure).toBeCloseTo(t1.notional + t2.notional, 4);
    expect(s95.exposure).toBeCloseTo(t3.notional, 4);
  });
});

describe("setup-derived exit analytics", () => {
  it("counts TP wins, SL losses, hit rates, average win, average loss, and average realized PnL", () => {
    const tpTrade = build({ row: traceRow({ traceId: "tp" }) });
    const tp = applyPriceTickToTrade({
      trade: tpTrade,
      price: tpTrade.takeProfitPrice,
      config: baseConfig,
      now: T0 + 1_000,
    });
    const slTrade = build({ row: traceRow({ traceId: "sl" }) });
    const sl = applyPriceTickToTrade({
      trade: slTrade,
      price: slTrade.stopLossPrice,
      config: baseConfig,
      now: T0 + 2_000,
    });
    const analytics = computeAnalytics([tp, sl]);

    expect(analytics.takeProfitWins).toBe(1);
    expect(analytics.stopLossLosses).toBe(1);
    expect(analytics.takeProfitHitRate).toBeCloseTo(0.5, 6);
    expect(analytics.stopLossHitRate).toBeCloseTo(0.5, 6);
    expect(analytics.averageWin).toBeCloseTo(tp.realizedPnl, 6);
    expect(analytics.averageLoss).toBeCloseTo(sl.realizedPnl, 6);
    expect(analytics.averageRealizedPnl).toBeCloseTo((tp.realizedPnl + sl.realizedPnl) / 2, 6);
  });
});

// ── Filters ──────────────────────────────────────────────────────────────────
describe("filterMockTrades", () => {
  it("filters by status, side, blocker, profitability", () => {
    const open = build({ row: traceRow({ traceId: "o" }) });
    const winRow = traceRow({ traceId: "w" });
    const win = closeMockTrade(build({ row: winRow }), ENTRY * 1.02, T0 + 1_000, baseConfig);
    const lossRow = traceRow({ traceId: "l", side: "SHORT", gate: "ATR_FEES" });
    const loss = closeMockTrade(build({ row: lossRow }), ENTRY * 1.01, T0 + 1_000, baseConfig);

    const all = [open, win, loss];
    expect(filterMockTrades(all, { status: "OPEN" }).map((t) => t.id)).toEqual([open.id]);
    expect(filterMockTrades(all, { side: "SELL" }).map((t) => t.id)).toEqual([loss.id]);
    expect(filterMockTrades(all, { blockerGate: "ATR_FEES" }).map((t) => t.id)).toEqual([loss.id]);
    expect(filterMockTrades(all, { profitability: "profit" }).map((t) => t.id)).toEqual([win.id]);
    expect(filterMockTrades(all, { profitability: "loss" }).map((t) => t.id)).toEqual([loss.id]);
  });

  it("filters trades younger than the selected age in minutes", () => {
    const now = T0 + 30 * 60_000;
    const young = { ...build({ row: traceRow({ traceId: "young" }) }), openedAt: now - 5 * 60_000 };
    const old = { ...build({ row: traceRow({ traceId: "old" }) }), openedAt: now - 45 * 60_000 };

    expect(filterMockTrades([young, old], { ageMode: "less", ageMaxMinutes: 10 }, now).map((t) => t.id))
      .toEqual([young.id]);
  });

  it("filters trades older than the selected age in minutes", () => {
    const now = T0 + 60 * 60_000;
    const young = { ...build({ row: traceRow({ traceId: "young-more" }) }), openedAt: now - 10 * 60_000 };
    const old = { ...build({ row: traceRow({ traceId: "old-more" }) }), openedAt: now - 90 * 60_000 };

    expect(filterMockTrades([young, old], { ageMode: "more", ageMinMinutes: 30 }, now).map((t) => t.id))
      .toEqual([old.id]);
  });

  it("uses closedAt rather than current time for CLOSED trade age", () => {
    const now = T0 + 120 * 60_000;
    const closed = {
      ...build({ row: traceRow({ traceId: "closed-age" }) }),
      status: "CLOSED" as const,
      openedAt: T0,
      closedAt: T0 + 5 * 60_000,
      unrealizedPnl: 0,
      realizedPnl: 4,
      exitReason: "MANUAL" as const,
      exitPrice: ENTRY,
    };

    expect(filterMockTrades([closed], { ageMode: "less", ageMaxMinutes: 10 }, now).map((t) => t.id))
      .toEqual([closed.id]);
  });

  it("filters research-pack trades by family", () => {
    const rsi = buildMockTradeFromResearchSignal({
      signal: {
        strategyId: 1090,
        strategyName: "RSI Research Long",
        strategyFamily: "RsiMeanReversion",
        side: "BUY",
        confidenceScore: 72,
        params: { period: 14, oversold: 30 },
        evaluatedAt: T0,
      },
      currentPrice: ENTRY,
      config: baseConfig,
      now: T0,
    });
    const vwap = buildMockTradeFromResearchSignal({
      signal: {
        strategyId: 1060,
        strategyName: "VWAP Research Long",
        strategyFamily: "VwapStrategies",
        side: "BUY",
        confidenceScore: 66,
        params: { rsi_period: 14 },
        evaluatedAt: T0,
      },
      currentPrice: ENTRY,
      config: baseConfig,
      now: T0,
    });
    if (!rsi || !vwap) throw new Error("expected research trades");

    expect(filterMockTrades([rsi, vwap], { researchOnly: true }).map((t) => t.id)).toEqual([rsi.id, vwap.id]);
    expect(filterMockTrades([rsi, vwap], { strategyFamily: "RsiMeanReversion" }).map((t) => t.id)).toEqual([rsi.id]);
  });
});

describe("buildMockTradeFromResearchSignal", () => {
  it("creates a mock-only trade with research metadata and no blockers", () => {
    const trade = buildMockTradeFromResearchSignal({
      signal: {
        strategyId: 1000,
        strategyName: "Research Trend Long",
        strategyFamily: "TrendFollowing",
        side: "BUY",
        confidenceScore: 81,
        params: { fast: 5, slow: 20 },
        evaluatedAt: T0,
      },
      currentPrice: ENTRY,
      config: baseConfig,
      now: T0,
    });

    expect(trade).not.toBeNull();
    expect(trade?.researchPack).toBe(true);
    expect(trade?.strategyFamily).toBe("TrendFollowing");
    expect(trade?.confidenceScore).toBe(81);
    expect(trade?.strategyParams).toEqual({ fast: 5, slow: 20 });
    expect(trade?.blockers).toEqual([]);
    expect(trade?.side).toBe("BUY");
  });
});

// ── PnL math sanity (post-fees, post-slippage) ───────────────────────────────
describe("computeMockPnl", () => {
  it("returns 0 for invalid inputs", () => {
    expect(computeMockPnl("BUY", NaN, 100, 1)).toBe(0);
    expect(computeMockPnl("BUY", 100, 100, 0)).toBe(0);
  });
  it("long PnL = (exit - entry) × qty", () => {
    expect(computeMockPnl("BUY", 100, 110, 2)).toBeCloseTo(20, 6);
  });
  it("short PnL = (entry - exit) × qty", () => {
    expect(computeMockPnl("SELL", 100, 90, 2)).toBeCloseTo(20, 6);
  });
});

// ── Persistence validators ───────────────────────────────────────────────────
describe("isValidMockTrade", () => {
  it("accepts a well-formed open trade", () => {
    expect(isValidMockTrade(build())).toBe(true);
  });
  it("rejects a trade missing numeric fields", () => {
    const bad = { ...build(), entryPrice: "abc" };
    expect(isValidMockTrade(bad)).toBe(false);
  });
  it("rejects a trade with a corrupt side", () => {
    const bad = { ...build(), side: "LONG" };
    expect(isValidMockTrade(bad)).toBe(false);
  });
  it("rejects a CLOSED trade missing exitPrice", () => {
    const bad = { ...build(), status: "CLOSED" as const, closedAt: T0 + 1_000, exitPrice: null, exitReason: "MANUAL" as const };
    expect(isValidMockTrade(bad)).toBe(false);
  });
});

describe("isValidMockConfig", () => {
  it("accepts the default config", () => {
    expect(isValidMockConfig(DEFAULT_MOCK_TRADING_CONFIG)).toBe(true);
  });
  it("rejects an unknown sizing mode", () => {
    expect(isValidMockConfig({ ...DEFAULT_MOCK_TRADING_CONFIG, sizingMode: "moon_shot" })).toBe(false);
  });
  it("rejects a non-positive starting balance", () => {
    expect(isValidMockConfig({ ...DEFAULT_MOCK_TRADING_CONFIG, startingBalanceUsd: 0 })).toBe(false);
  });
  it("rejects a non-positive mock open trade limit", () => {
    expect(isValidMockConfig({ ...DEFAULT_MOCK_TRADING_CONFIG, maxOpenMockTrades: 0 })).toBe(false);
  });
});

describe("MOCK_PERSIST_VERSION", () => {
  it("is the current schema version (bump when MockTrade shape changes)", () => {
    expect(MOCK_PERSIST_VERSION).toBeGreaterThanOrEqual(2);
  });
});

// ── Sorting ──────────────────────────────────────────────────────────────────
describe("mockTradePnl", () => {
  it("returns unrealizedPnl for OPEN trades", () => {
    const open: MockTrade = { ...build(), unrealizedPnl: 42, realizedPnl: 999 };
    expect(mockTradePnl(open)).toBe(42);
  });
  it("returns realizedPnl for CLOSED trades", () => {
    const closed = closeMockTrade(build(), ENTRY * 1.02, T0 + 60_000, baseConfig);
    expect(mockTradePnl(closed)).toBe(closed.realizedPnl);
  });
});

describe("sortMockTrades", () => {
  function tradeWithPnl(id: string, pnl: number, openedAt: number, status: "OPEN" | "CLOSED" = "OPEN"): MockTrade {
    const base = build({ row: traceRow({ traceId: id }) });
    return {
      ...base,
      id: `mock-${id}`,
      openedAt,
      status,
      unrealizedPnl: status === "OPEN" ? pnl : 0,
      realizedPnl: status === "CLOSED" ? pnl : 0,
      closedAt: status === "CLOSED" ? openedAt + 1_000 : null,
      exitReason: status === "CLOSED" ? "MANUAL" : null,
      exitPrice: status === "CLOSED" ? ENTRY : null,
    };
  }

  it("default option is 'most_profitable'", () => {
    expect(MOCK_TRADE_SORT_OPTIONS[0].value).toBe("most_profitable");
  });

  it("'most_profitable' orders highest net PnL first", () => {
    const a = tradeWithPnl("a", -5, T0);
    const b = tradeWithPnl("b", 10, T0);
    const c = tradeWithPnl("c", 0, T0);
    const sorted = sortMockTrades([a, b, c], "most_profitable");
    expect(sorted.map((t) => t.id)).toEqual([b.id, c.id, a.id]);
  });

  it("'least_profitable' orders lowest net PnL first", () => {
    const a = tradeWithPnl("a", -5, T0);
    const b = tradeWithPnl("b", 10, T0);
    const c = tradeWithPnl("c", 0, T0);
    const sorted = sortMockTrades([a, b, c], "least_profitable");
    expect(sorted.map((t) => t.id)).toEqual([a.id, c.id, b.id]);
  });

  it("'newest' orders by openedAt desc", () => {
    const a = tradeWithPnl("a", 0, T0);
    const b = tradeWithPnl("b", 0, T0 + 1_000);
    const c = tradeWithPnl("c", 0, T0 + 500);
    const sorted = sortMockTrades([a, b, c], "newest");
    expect(sorted.map((t) => t.id)).toEqual([b.id, c.id, a.id]);
  });

  it("'oldest' orders by openedAt asc", () => {
    const a = tradeWithPnl("a", 0, T0);
    const b = tradeWithPnl("b", 0, T0 + 1_000);
    const c = tradeWithPnl("c", 0, T0 + 500);
    const sorted = sortMockTrades([a, b, c], "oldest");
    expect(sorted.map((t) => t.id)).toEqual([a.id, c.id, b.id]);
  });

  it("uses unrealizedPnl for OPEN trades and realizedPnl for CLOSED trades", () => {
    const openWinner = tradeWithPnl("ow", 7, T0, "OPEN");
    const closedLoser = tradeWithPnl("cl", -3, T0 + 100, "CLOSED");
    const closedWinner = tradeWithPnl("cw", 12, T0 + 200, "CLOSED");
    const openLoser = tradeWithPnl("ol", -8, T0 + 300, "OPEN");
    const sorted = sortMockTrades([openWinner, closedLoser, closedWinner, openLoser], "most_profitable");
    expect(sorted.map((t) => t.id)).toEqual([closedWinner.id, openWinner.id, closedLoser.id, openLoser.id]);
  });

  it("breaks ties deterministically (newer openedAt wins)", () => {
    const a = tradeWithPnl("a", 5, T0);
    const b = tradeWithPnl("b", 5, T0 + 1_000);
    const sorted = sortMockTrades([a, b], "most_profitable");
    expect(sorted.map((t) => t.id)).toEqual([b.id, a.id]);
  });

  it("does not mutate the input array", () => {
    const original = [tradeWithPnl("a", -5, T0), tradeWithPnl("b", 10, T0)];
    const before = original.map((t) => t.id);
    sortMockTrades(original, "most_profitable");
    expect(original.map((t) => t.id)).toEqual(before);
  });

  it("composes with filterMockTrades — sort runs after the filter", () => {
    const open1 = tradeWithPnl("o1", 5, T0, "OPEN");
    const open2 = tradeWithPnl("o2", -10, T0 + 100, "OPEN");
    const closed1 = tradeWithPnl("c1", 20, T0 + 200, "CLOSED");
    const all = [open1, open2, closed1];
    const filtered = filterMockTrades(all, { status: "OPEN" });
    const sorted = sortMockTrades(filtered, "most_profitable");
    expect(sorted.map((t) => t.id)).toEqual([open1.id, open2.id]);
  });

  it("keeps profitable-first sorting after age filtering", () => {
    const now = T0 + 60 * 60_000;
    const youngWinner = { ...tradeWithPnl("young-win", 20, now - 5 * 60_000), openedAt: now - 5 * 60_000 };
    const youngLoser = { ...tradeWithPnl("young-loss", -2, now - 8 * 60_000), openedAt: now - 8 * 60_000 };
    const oldWinner = { ...tradeWithPnl("old-win", 100, now - 50 * 60_000), openedAt: now - 50 * 60_000 };

    const filtered = filterMockTrades(
      [youngLoser, oldWinner, youngWinner],
      { ageMode: "less", ageMaxMinutes: 10 },
      now,
    );
    const sorted = sortMockTrades(filtered, "most_profitable");

    expect(sorted.map((t) => t.id)).toEqual([youngWinner.id, youngLoser.id]);
  });
});
