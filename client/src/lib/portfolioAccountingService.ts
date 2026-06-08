/**
 * PortfolioAccountingService — single MongoDB-authoritative accounting layer.
 * All Paper Desk dashboards must consume snapshots from this service.
 */

import {
  getClosedTradeStats,
  getEquityCurve,
  getPaperState,
  listOpenPositions,
  type ClosedTradeStats,
  type PaperStateDoc,
} from "@/lib/paperDeskClient";
import { computeExtendedMetricsFromRecords } from "@/lib/mockExtendedMetrics";
import { unrealizedPnlForOpenPosition } from "@/lib/paperDeskPositionMath";
import { computeExposureFromPositions } from "@/lib/portfolioAccountingExposure";
import type {
  DrawdownMetrics,
  PortfolioAccountingInputs,
  PortfolioAccountingSnapshot,
} from "@/lib/portfolioAccountingTypes";
import { PAPER_DESK_STARTING_BALANCE } from "@/lib/portfolioAccountingTypes";

function safeNum(v: unknown, fallback = 0): number {
  const n = Number(v);
  return Number.isFinite(n) ? n : fallback;
}

function computeDrawdownMetrics(
  equity: number,
  startingBalance: number,
  state: PaperStateDoc | null,
  equityCurve: { ts: string; equity: number; drawdown_pct?: number }[],
): DrawdownMetrics {
  const peakFromState = safeNum(state?.peak_equity, 0);
  const peakFromCurve = equityCurve.reduce((max, pt) => Math.max(max, safeNum(pt.equity)), 0);
  const peak = Math.max(startingBalance, peakFromState, peakFromCurve, equity);

  const currentDd = peak > 0 ? Math.max(0, (peak - equity) / peak) : 0;
  const maxDdState = safeNum(state?.max_drawdown, 0);
  const maxDdCurve = equityCurve.reduce((max, pt) => Math.max(max, safeNum(pt.drawdown_pct)), 0);

  const now = Date.now();
  const dayAgo = now - 24 * 60 * 60 * 1000;
  const weekAgo = now - 7 * 24 * 60 * 60 * 1000;
  const dayStart = new Date();
  dayStart.setUTCHours(0, 0, 0, 0);
  const weekStart = new Date(dayStart);
  weekStart.setUTCDate(weekStart.getUTCDate() - weekStart.getUTCDay());

  let rollingPeak24h = peak;
  let dailyPeak = peak;
  let weeklyPeak = peak;
  let rollingDd24h = 0;
  let dailyDd = 0;
  let weeklyDd = 0;

  for (const pt of equityCurve) {
    const ts = new Date(pt.ts).getTime();
    const eq = safeNum(pt.equity);
    if (ts >= dayAgo) {
      if (eq > rollingPeak24h) rollingPeak24h = eq;
      const dd = rollingPeak24h > 0 ? (rollingPeak24h - eq) / rollingPeak24h : 0;
      if (dd > rollingDd24h) rollingDd24h = dd;
    }
    if (ts >= dayStart.getTime()) {
      if (eq > dailyPeak) dailyPeak = eq;
      const dd = dailyPeak > 0 ? (dailyPeak - eq) / dailyPeak : 0;
      if (dd > dailyDd) dailyDd = dd;
    }
    if (ts >= weekStart.getTime()) {
      if (eq > weeklyPeak) weeklyPeak = eq;
      const dd = weeklyPeak > 0 ? (weeklyPeak - eq) / weeklyPeak : 0;
      if (dd > weeklyDd) weeklyDd = dd;
    }
  }

  return {
    peak_equity: peak,
    current_drawdown: currentDd,
    max_drawdown: Math.max(maxDdState, maxDdCurve, currentDd),
    rolling_drawdown_24h: rollingDd24h,
    daily_drawdown: dailyDd,
    weekly_drawdown: weeklyDd,
  };
}

/** Build snapshot from pre-fetched inputs (for API routes that already loaded data). */
export function buildPortfolioAccountingSnapshot(
  inputs: PortfolioAccountingInputs,
): PortfolioAccountingSnapshot {
  const { accountKey, state, closedStats, openPositions, markPrice, equityCurveLastTs } = inputs;
  const starting = PAPER_DESK_STARTING_BALANCE;

  const exposure = computeExposureFromPositions(openPositions, markPrice);
  let unrealized = 0;
  for (const pos of openPositions) {
    if (markPrice && markPrice > 0) {
      unrealized += unrealizedPnlForOpenPosition(pos, markPrice);
    }
  }
  if (unrealized === 0 && state) {
    unrealized = safeNum(state.unrealized_pnl);
  }

  const realized = safeNum(closedStats.realized_pnl);
  const grossPnl = safeNum(closedStats.gross_pnl, realized + safeNum(closedStats.total_fees));
  const totalFees = safeNum(closedStats.total_fees);
  const entryFees = safeNum(closedStats.entry_fees, totalFees / 2);
  const exitFees = safeNum(closedStats.exit_fees, totalFees / 2);

  const balance = state != null ? safeNum(state.balance, starting + realized) : starting + realized;
  const equity = balance + unrealized;

  const drawdown: DrawdownMetrics = {
    peak_equity: Math.max(starting, safeNum(state?.peak_equity), equity),
    current_drawdown: safeNum(state?.current_drawdown),
    max_drawdown: safeNum(state?.max_drawdown),
    rolling_drawdown_24h: 0,
    daily_drawdown: 0,
    weekly_drawdown: 0,
  };

  const stalenessSec = equityCurveLastTs
    ? Math.max(0, Math.floor((Date.now() - new Date(equityCurveLastTs).getTime()) / 1000))
    : null;

  return {
    account_key: accountKey,
    computed_at: new Date().toISOString(),
    source: "mongodb",
    balance,
    equity,
    starting_balance: starting,
    realized_pnl: realized,
    unrealized_pnl: unrealized,
    gross_pnl: grossPnl,
    net_pnl: realized,
    entry_fees: entryFees,
    exit_fees: exitFees,
    total_fees: totalFees,
    exposure,
    drawdown,
    total_trades: closedStats.total_trades,
    winning_trades: closedStats.winning_trades,
    losing_trades: closedStats.losing_trades,
    win_rate: closedStats.win_rate,
    open_position_count: openPositions.length,
    profit_factor: null,
    expectancy: closedStats.total_trades > 0 ? realized / closedStats.total_trades : 0,
    average_win: 0,
    average_loss: 0,
    sharpe: null,
    sortino: null,
    calmar: null,
    recovery_factor: null,
    equity_curve_staleness_sec: stalenessSec,
    equity_curve_last_ts: equityCurveLastTs ?? null,
  };
}

/** Full async snapshot — queries Mongo and computes all metrics. */
export async function getPortfolioAccountingSnapshot(
  accountKey: string,
  opts?: { markPrice?: number },
): Promise<PortfolioAccountingSnapshot> {
  const [state, closedStats, openPositions, equityCurve] = await Promise.all([
    getPaperState(accountKey),
    getClosedTradeStats(accountKey),
    listOpenPositions(accountKey),
    getEquityCurve(accountKey, 2016),
  ]);

  const lastCurvePt = equityCurve.length > 0 ? equityCurve[equityCurve.length - 1] : null;
  const snapshot = buildPortfolioAccountingSnapshot({
    accountKey,
    state,
    closedStats,
    openPositions,
    markPrice: opts?.markPrice,
    equityCurveLastTs: lastCurvePt?.ts ?? null,
  });

  snapshot.drawdown = computeDrawdownMetrics(
    snapshot.equity,
    snapshot.starting_balance,
    state,
    equityCurve.map((pt) => ({
      ts: pt.ts,
      equity: pt.equity,
      drawdown_pct: pt.drawdown_pct,
    })),
  );

  if (closedStats.total_trades > 0) {
    const tradeRecords = await loadClosedTradeRecordsForMetrics(accountKey);
    const extended = computeExtendedMetricsFromRecords(tradeRecords, snapshot.starting_balance);
    snapshot.profit_factor = extended.profitFactor;
    snapshot.expectancy = extended.expectancy;
    snapshot.average_win = extended.averageWin;
    snapshot.average_loss = extended.averageLoss;
    snapshot.sharpe = extended.sharpeRatio;
    snapshot.sortino = extended.sortinoRatio;
    snapshot.calmar = extended.calmarRatio;
    snapshot.recovery_factor = extended.recoveryFactor;
  }

  return snapshot;
}

async function loadClosedTradeRecordsForMetrics(
  accountKey: string,
): Promise<{ realizedPnl: number; openedAt: number; closedAt: number }[]> {
  const { getDb } = await import("@/lib/mongoTradesClient");
  const db = await getDb();
  const docs = await db
    .collection("paper_trades")
    .find({ account_key: accountKey })
    .project({ net_pnl: 1, entry_at: 1, closed_at: 1 })
    .sort({ closed_at: 1 })
    .limit(5000)
    .toArray();

  return docs.map((d) => ({
    realizedPnl: safeNum(d.net_pnl),
    openedAt: new Date(String(d.entry_at ?? d.closed_at)).getTime(),
    closedAt: new Date(String(d.closed_at)).getTime(),
  }));
}

/** Merge authoritative Mongo accounting into paper_state for dashboard display. */
export function mergeAccountingIntoPaperState(
  state: PaperStateDoc | null,
  stats: ClosedTradeStats,
  snapshot: Pick<
    PortfolioAccountingSnapshot,
    "exposure" | "drawdown" | "equity" | "balance" | "unrealized_pnl"
  >,
): PaperStateDoc | null {
  if (!state) return null;
  return {
    ...state,
    balance: snapshot.balance,
    equity: snapshot.equity,
    unrealized_pnl: snapshot.unrealized_pnl,
    realized_pnl: stats.realized_pnl,
    total_fees: stats.total_fees,
    total_trades: stats.total_trades,
    winning_trades: stats.winning_trades,
    losing_trades: stats.losing_trades,
    win_rate: stats.win_rate,
    open_position_count: snapshot.exposure.gross_exposure_btc > 0
      ? state.open_position_count
      : state.open_position_count,
    total_exposure_btc: snapshot.exposure.gross_exposure_btc,
    long_exposure_btc: snapshot.exposure.long_exposure_btc,
    short_exposure_btc: snapshot.exposure.short_exposure_btc,
    peak_equity: snapshot.drawdown.peak_equity,
    current_drawdown: snapshot.drawdown.current_drawdown,
    max_drawdown: snapshot.drawdown.max_drawdown,
  };
}
