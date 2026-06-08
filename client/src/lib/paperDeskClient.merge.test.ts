import { describe, expect, it } from "vitest";
import { mergeClosedTradeStatsIntoState, type ClosedTradeStats, type PaperStateDoc } from "./paperDeskClient";

describe("mergeClosedTradeStatsIntoState", () => {
  const baseState: PaperStateDoc = {
    account_key: "user-1",
    balance: 1_000_000,
    equity: 1_000_000,
    unrealized_pnl: 0,
    realized_pnl: -100,
    peak_equity: 1_000_000,
    current_drawdown: 0,
    max_drawdown: 0,
    open_position_count: 0,
    total_exposure_btc: 0,
    long_exposure_btc: 0,
    short_exposure_btc: 0,
    total_trades: 514,
    winning_trades: 172,
    losing_trades: 342,
    win_rate: 172 / 514,
    total_fees: 100,
    session_start: "2026-06-08T00:00:00.000Z",
    snapped_at: "2026-06-08T00:00:00.000Z",
    updated_at: "2026-06-08T00:00:00.000Z",
  };

  const closedStats: ClosedTradeStats = {
    total_trades: 1852,
    winning_trades: 620,
    losing_trades: 1232,
    win_rate: 620 / 1852,
    realized_pnl: -2697,
    gross_pnl: -1800,
    total_fees: 22549,
    entry_fees: 11274.5,
    exit_fees: 11274.5,
  };

  it("overrides trade counts and realized PnL with MongoDB closed-trade totals", () => {
    const merged = mergeClosedTradeStatsIntoState(baseState, closedStats);
    expect(merged?.total_trades).toBe(1852);
    expect(merged?.winning_trades).toBe(620);
    expect(merged?.losing_trades).toBe(1232);
    expect(merged?.win_rate).toBeCloseTo(620 / 1852, 6);
    expect(merged?.realized_pnl).toBe(-2697);
    expect(merged?.total_fees).toBe(22549);
    expect(merged?.balance).toBe(baseState.balance);
  });

  it("returns null when state is null", () => {
    expect(mergeClosedTradeStatsIntoState(null, closedStats)).toBeNull();
  });
});
