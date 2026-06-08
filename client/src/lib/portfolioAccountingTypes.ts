/** Types for Mongo-authoritative Paper Desk portfolio accounting. */

import type { PaperPositionDoc, PaperStateDoc } from "@/lib/paperDeskClient";

export const PAPER_DESK_STARTING_BALANCE = 1_000_000;

export type ExposureMetrics = {
  long_exposure_btc: number;
  short_exposure_btc: number;
  net_exposure_btc: number;
  gross_exposure_btc: number;
  exposure_usd: number;
};

export type DrawdownMetrics = {
  peak_equity: number;
  current_drawdown: number;
  max_drawdown: number;
  rolling_drawdown_24h: number;
  daily_drawdown: number;
  weekly_drawdown: number;
};

export type PortfolioAccountingSnapshot = {
  account_key: string;
  computed_at: string;
  source: "mongodb";

  balance: number;
  equity: number;
  starting_balance: number;

  realized_pnl: number;
  unrealized_pnl: number;
  gross_pnl: number;
  net_pnl: number;

  entry_fees: number;
  exit_fees: number;
  total_fees: number;

  exposure: ExposureMetrics;

  drawdown: DrawdownMetrics;

  total_trades: number;
  winning_trades: number;
  losing_trades: number;
  win_rate: number;
  open_position_count: number;

  profit_factor: number | null;
  expectancy: number;
  average_win: number;
  average_loss: number;
  sharpe: number | null;
  sortino: number | null;
  calmar: number | null;
  recovery_factor: number | null;

  equity_curve_staleness_sec: number | null;
  equity_curve_last_ts: string | null;
};

export type PortfolioAccountingInputs = {
  accountKey: string;
  state: PaperStateDoc | null;
  closedStats: import("@/lib/paperDeskClient").ClosedTradeStats;
  openPositions: PaperPositionDoc[];
  markPrice?: number;
  equityCurveLastTs?: string | null;
};
