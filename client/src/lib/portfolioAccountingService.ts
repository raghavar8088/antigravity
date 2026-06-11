/**
 * Portfolio accounting backed by mock trading Mongo collections.
 */

import { computeExtendedMetricsFromRecords } from "@/lib/mockExtendedMetrics";
import {
  computeAccountFromMongo,
  getLatestMockAccountSnapshot,
  listEquityCurvePoints,
  listMockTrades,
} from "@/lib/mockTradingMongo";
import { DEFAULT_MOCK_TRADING_CONFIG, normalizeMockTradingConfig } from "@/lib/mockTradingEngine";
import type { PortfolioAccountingSnapshot } from "@/lib/portfolioAccountingTypes";

const STARTING_BALANCE = 1_000_000;

function safeNum(v: unknown, fallback = 0): number {
  const n = Number(v);
  return Number.isFinite(n) ? n : fallback;
}

export function utcDayBounds(now = new Date()): { start: Date; end: Date } {
  const start = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()));
  const end = new Date(start);
  end.setUTCDate(end.getUTCDate() + 1);
  return { start, end };
}

export async function getTodayRealizedPnlUtc(accountKey: string, now = new Date()): Promise<number> {
  const { start, end } = utcDayBounds(now);
  const { trades } = await listMockTrades({
    account_key: accountKey,
    status: "CLOSED",
    page: 1,
    limit: 50_000,
    sort: "newest",
  });
  return trades
    .filter((t) => t.closedAt != null && t.closedAt >= start.getTime() && t.closedAt < end.getTime())
    .reduce((sum, t) => sum + t.realizedPnl, 0);
}

export type PortfolioExtendedMetrics = {
  profit_factor: number | null;
  sharpe: number | null;
};

export async function getPortfolioExtendedMetrics(accountKey: string): Promise<PortfolioExtendedMetrics> {
  const { trades } = await listMockTrades({
    account_key: accountKey,
    status: "CLOSED",
    page: 1,
    limit: 50_000,
    sort: "oldest",
  });
  if (trades.length === 0) return { profit_factor: null, sharpe: null };
  const extended = computeExtendedMetricsFromRecords(
    trades.map((t) => ({ realizedPnl: t.realizedPnl, openedAt: t.openedAt, closedAt: t.closedAt ?? t.openedAt })),
    STARTING_BALANCE,
  );
  return { profit_factor: extended.profitFactor, sharpe: extended.sharpeRatio };
}

export async function getPortfolioAccountingSnapshot(
  accountKey: string,
): Promise<PortfolioAccountingSnapshot> {
  const { account: cached, config: cachedConfig } = await getLatestMockAccountSnapshot(accountKey);
  const config = normalizeMockTradingConfig(cachedConfig ?? DEFAULT_MOCK_TRADING_CONFIG);
  const account = cached ?? (await computeAccountFromMongo(accountKey, config));
  const closed = await listMockTrades({
    account_key: accountKey,
    status: "CLOSED",
    page: 1,
    limit: 50_000,
    sort: "oldest",
  });
  const open = await listMockTrades({
    account_key: accountKey,
    status: "OPEN",
    page: 1,
    limit: 200,
    sort: "oldest",
  });
  const curve = await listEquityCurvePoints(accountKey, 500);
  const wins = closed.trades.filter((t) => t.realizedPnl >= 0).length;
  const extended = closed.trades.length
    ? computeExtendedMetricsFromRecords(
        closed.trades.map((t) => ({
          realizedPnl: t.realizedPnl,
          openedAt: t.openedAt,
          closedAt: t.closedAt ?? t.openedAt,
        })),
        STARTING_BALANCE,
      )
    : null;

  return {
    account_key: accountKey,
    computed_at: new Date().toISOString(),
    source: "mock-trading",
    balance: account.cashBalance,
    equity: account.equity,
    starting_balance: account.startingBalance,
    realized_pnl: account.realizedPnl,
    unrealized_pnl: account.unrealizedPnl,
    gross_pnl: account.realizedPnl + account.unrealizedPnl,
    net_pnl: account.realizedPnl,
    entry_fees: closed.trades.reduce((s, t) => s + t.fees / 2, 0),
    exit_fees: closed.trades.reduce((s, t) => s + t.fees / 2, 0),
    total_fees: closed.trades.reduce((s, t) => s + t.fees, 0),
    exposure: {
      gross_exposure_btc: open.trades.reduce((s, t) => s + t.quantity, 0),
      long_exposure_btc: open.trades.filter((t) => t.side === "BUY").reduce((s, t) => s + t.quantity, 0),
      short_exposure_btc: open.trades.filter((t) => t.side === "SELL").reduce((s, t) => s + t.quantity, 0),
      net_exposure_btc:
        open.trades.filter((t) => t.side === "BUY").reduce((s, t) => s + t.quantity, 0) -
        open.trades.filter((t) => t.side === "SELL").reduce((s, t) => s + t.quantity, 0),
      gross_exposure_usd: account.exposure,
      net_exposure_usd: account.longExposure - account.shortExposure,
    },
    drawdown: {
      peak_equity: account.peakEquity,
      current_drawdown: account.maxDrawdownPct,
      max_drawdown: account.maxDrawdownPct,
      rolling_drawdown_24h: 0,
      daily_drawdown: 0,
      weekly_drawdown: 0,
    },
    total_trades: closed.trades.length,
    winning_trades: wins,
    losing_trades: closed.trades.length - wins,
    win_rate: closed.trades.length > 0 ? wins / closed.trades.length : 0,
    open_position_count: open.trades.length,
    profit_factor: extended?.profitFactor ?? null,
    expectancy: extended?.expectancy ?? 0,
    average_win: extended?.averageWin ?? 0,
    average_loss: extended?.averageLoss ?? 0,
    sharpe: extended?.sharpeRatio ?? null,
    sortino: extended?.sortinoRatio ?? null,
    calmar: extended?.calmarRatio ?? null,
    recovery_factor: extended?.recoveryFactor ?? null,
    equity_curve_staleness_sec: curve.length
      ? Math.max(0, Math.floor((Date.now() - safeNum(curve[curve.length - 1]?.timestamp)) / 1000))
      : null,
    equity_curve_last_ts: curve.length ? new Date(safeNum(curve[curve.length - 1]?.timestamp)).toISOString() : null,
  };
}
