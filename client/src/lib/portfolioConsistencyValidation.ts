/**
 * Automatic data consistency validation for Paper Desk accounting.
 */

import {
  countPaperTrades,
  getClosedTradeStats,
  getEquityCurve,
  getPaperState,
  listOpenPositions,
  listPaperOrders,
} from "@/lib/paperDeskClient";
import { getPortfolioAccountingSnapshot } from "@/lib/portfolioAccountingService";
import { PAPER_DESK_STARTING_BALANCE } from "@/lib/portfolioAccountingTypes";

export type ConsistencyCheck = {
  id: string;
  status: "PASS" | "WARNING" | "FAIL";
  message: string;
  expected?: number | string;
  actual?: number | string;
  tolerance?: number;
};

export type ConsistencyValidationReport = {
  checked_at: string;
  account_key: string;
  overall: "PASS" | "WARNING" | "FAIL";
  checks: ConsistencyCheck[];
  drift_detected: boolean;
};

const PNL_TOLERANCE_USD = 50;
const EQUITY_TOLERANCE_USD = 100;

function worst(checks: ConsistencyCheck[]): "PASS" | "WARNING" | "FAIL" {
  if (checks.some((c) => c.status === "FAIL")) return "FAIL";
  if (checks.some((c) => c.status === "WARNING")) return "WARNING";
  return "PASS";
}

export async function runPortfolioConsistencyValidation(
  accountKey: string,
): Promise<ConsistencyValidationReport> {
  const checks: ConsistencyCheck[] = [];
  const [state, closedStats, positions, snapshot, equityCurve, orderCount] = await Promise.all([
    getPaperState(accountKey),
    getClosedTradeStats(accountKey),
    listOpenPositions(accountKey),
    getPortfolioAccountingSnapshot(accountKey),
    getEquityCurve(accountKey, 3),
    listPaperOrders({ accountKey, limit: 1 }).then((o) => o.length),
  ]);

  const tradeCountMongo = await countPaperTrades(accountKey);

  checks.push({
    id: "trades_vs_stats",
    status: tradeCountMongo === closedStats.total_trades ? "PASS" : "FAIL",
    message: tradeCountMongo === closedStats.total_trades
      ? "Trade count matches Mongo aggregation"
      : "Trade count drift between countDocuments and aggregate",
    expected: tradeCountMongo,
    actual: closedStats.total_trades,
  });

  if (state) {
    const pnlDrift = Math.abs(state.realized_pnl - closedStats.realized_pnl);
    checks.push({
      id: "pnl_state_vs_trades",
      status: pnlDrift <= PNL_TOLERANCE_USD ? "PASS" : "FAIL",
      message: pnlDrift <= PNL_TOLERANCE_USD
        ? "paper_state realized_pnl within tolerance of SUM(net_pnl)"
        : "Realized PnL split-brain: paper_state vs paper_trades",
      expected: closedStats.realized_pnl,
      actual: state.realized_pnl,
      tolerance: PNL_TOLERANCE_USD,
    });

    const equityExpected = (state.balance ?? PAPER_DESK_STARTING_BALANCE) + snapshot.unrealized_pnl;
    const equityDrift = Math.abs((state.equity ?? 0) - equityExpected);
    checks.push({
      id: "equity_vs_balance",
      status: equityDrift <= EQUITY_TOLERANCE_USD ? "PASS" : "WARNING",
      message: equityDrift <= EQUITY_TOLERANCE_USD
        ? "Equity ≈ balance + unrealized"
        : "Equity vs balance+unrealized drift",
      expected: equityExpected,
      actual: state.equity,
      tolerance: EQUITY_TOLERANCE_USD,
    });

    checks.push({
      id: "fees_vs_trades",
      status: Math.abs((state.total_fees ?? 0) - closedStats.total_fees) <= PNL_TOLERANCE_USD
        ? "PASS"
        : "WARNING",
      message: "Fee totals paper_state vs paper_trades",
      expected: closedStats.total_fees,
      actual: state.total_fees,
      tolerance: PNL_TOLERANCE_USD,
    });
  }

  const openFromPositions = positions.length;
  checks.push({
    id: "positions_vs_state",
    status: !state || state.open_position_count === openFromPositions ? "PASS" : "WARNING",
    message: "Open position count paper_state vs paper_positions",
    expected: openFromPositions,
    actual: state?.open_position_count,
  });

  const exposureDrift = Math.abs(
    (state?.total_exposure_btc ?? 0) - snapshot.exposure.gross_exposure_btc,
  );
  checks.push({
    id: "exposure_vs_positions",
    status: exposureDrift < 0.001 ? "PASS" : "WARNING",
    message: exposureDrift < 0.001
      ? "Exposure matches open positions"
      : "Exposure drift: paper_state vs computed positions",
    expected: snapshot.exposure.gross_exposure_btc,
    actual: state?.total_exposure_btc,
  });

  const lastEquityTs = equityCurve.length > 0 ? equityCurve[equityCurve.length - 1]?.ts : null;
  const stalenessSec = lastEquityTs
    ? (Date.now() - new Date(lastEquityTs).getTime()) / 1000
    : Infinity;
  checks.push({
    id: "equity_curve_freshness",
    status: stalenessSec <= 360 ? "PASS" : stalenessSec <= 7200 ? "WARNING" : "FAIL",
    message: stalenessSec <= 360
      ? "Equity curve is live (<6 min)"
      : `Equity curve stale (${Math.round(stalenessSec / 60)} min old)`,
    actual: lastEquityTs ?? "none",
  });

  checks.push({
    id: "snapshot_authoritative",
    status: snapshot.realized_pnl === closedStats.realized_pnl ? "PASS" : "FAIL",
    message: "PortfolioAccountingService uses Mongo SUM(net_pnl)",
    expected: closedStats.realized_pnl,
    actual: snapshot.realized_pnl,
  });

  if (orderCount === 0 && tradeCountMongo > 0) {
    checks.push({
      id: "orders_vs_trades",
      status: "WARNING",
      message: "Closed trades exist but no OMS orders found",
    });
  }

  const drift = checks.some((c) => c.status === "FAIL" || c.status === "WARNING");

  return {
    checked_at: new Date().toISOString(),
    account_key: accountKey,
    overall: worst(checks),
    checks,
    drift_detected: drift,
  };
}
