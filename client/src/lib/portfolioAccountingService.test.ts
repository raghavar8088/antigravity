import { describe, expect, it } from "vitest";
import { canonicalNetPnl, canonicalTradeFees } from "@/lib/portfolioAccountingFees";
import { computeExposureFromPositions } from "@/lib/portfolioAccountingExposure";
import { buildPortfolioAccountingSnapshot } from "@/lib/portfolioAccountingService";
import type { ClosedTradeStats, PaperPositionDoc, PaperStateDoc } from "@/lib/paperDeskClient";

describe("canonicalTradeFees", () => {
  it("computes entry and exit fees separately", () => {
    const fees = canonicalTradeFees(100_000, 101_000, 0.01);
    // notional entry = 100k * 0.01 = $1000 → fee 0.05% = $0.50
    expect(fees.entry_fee).toBeCloseTo(0.5, 4);
    expect(fees.exit_fee).toBeCloseTo(0.505, 4);
    expect(fees.total_fee).toBeCloseTo(1.005, 4);
  });

  it("net = gross - entry - exit", () => {
    const gross = 100;
    const fees = canonicalTradeFees(50_000, 50_100, 0.02);
    expect(canonicalNetPnl(gross, fees)).toBeCloseTo(gross - fees.total_fee, 6);
  });
});

describe("computeExposureFromPositions", () => {
  it("sums long and short BTC exposure", () => {
    const positions: PaperPositionDoc[] = [
      {
        account_key: "u1",
        position_id: "p1",
        order_id: "o1",
        strategy_id: "s1",
        symbol: "BTCUSD",
        side: "LONG",
        size: 1.2,
        entry_price: 100_000,
        stop_loss: 99_000,
        take_profit: 101_000,
        status: "OPEN",
        opened_at: "",
      },
      {
        account_key: "u1",
        position_id: "p2",
        order_id: "o2",
        strategy_id: "s2",
        symbol: "BTCUSD",
        side: "SHORT",
        size: 0.5,
        entry_price: 100_000,
        stop_loss: 101_000,
        take_profit: 99_000,
        status: "OPEN",
        opened_at: "",
      },
    ];
    const exp = computeExposureFromPositions(positions, 100_000);
    expect(exp.long_exposure_btc).toBeCloseTo(1.2, 6);
    expect(exp.short_exposure_btc).toBeCloseTo(0.5, 6);
    expect(exp.net_exposure_btc).toBeCloseTo(0.7, 6);
    expect(exp.gross_exposure_btc).toBeCloseTo(1.7, 6);
  });
});

describe("buildPortfolioAccountingSnapshot", () => {
  const state: PaperStateDoc = {
    account_key: "u1",
    balance: 997_303,
    equity: 997_303,
    unrealized_pnl: 0,
    realized_pnl: -392,
    peak_equity: 1_000_000,
    current_drawdown: 0,
    max_drawdown: 0,
    open_position_count: 1,
    total_exposure_btc: 0,
    long_exposure_btc: 0,
    short_exposure_btc: 0,
    total_trades: 1852,
    winning_trades: 620,
    losing_trades: 1232,
    win_rate: 0.33,
    total_fees: 5293,
    session_start: "",
    snapped_at: "",
    updated_at: "",
  };

  const closedStats: ClosedTradeStats = {
    total_trades: 1852,
    winning_trades: 620,
    losing_trades: 1232,
    win_rate: 620 / 1852,
    realized_pnl: -2697,
    gross_pnl: 19852,
    total_fees: 22549,
    entry_fees: 11274.5,
    exit_fees: 11274.5,
  };

  it("uses Mongo SUM(net_pnl) for realized PnL, not journal", () => {
    const snap = buildPortfolioAccountingSnapshot({
      accountKey: "u1",
      state,
      closedStats,
      openPositions: [],
    });
    expect(snap.realized_pnl).toBe(-2697);
    expect(snap.total_fees).toBe(22549);
    expect(snap.total_trades).toBe(1852);
  });
});
