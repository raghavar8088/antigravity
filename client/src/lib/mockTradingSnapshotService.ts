/**
 * Builds terminal-compatible snapshots from mock trading Mongo collections.
 * Replaces the retired paper desk snapshot layer.
 */

import {
  computeAccountFromMongo,
  getLatestMockAccountSnapshot,
  listMockTrades,
} from "@/lib/mockTradingMongo";
import {
  DEFAULT_MOCK_TRADING_CONFIG,
  normalizeMockTradingConfig,
  type MockTrade,
} from "@/lib/mockTradingEngine";
import type { PaperDeskSnapshotPayload } from "@/lib/terminal/mapSnapshotToTerminalDelta";

function mockTradeToOpenPosition(trade: MockTrade): Record<string, unknown> {
  const side = trade.side === "BUY" ? "LONG" : "SHORT";
  return {
    position_id: trade.id,
    symbol: trade.symbol,
    side,
    strategy_id: trade.strategyName,
    strategy: trade.strategyName,
    entry_price: trade.entryPrice,
    mark_price: trade.currentPrice,
    liquidation_price: 0,
    size_btc: trade.quantity,
    size: trade.quantity,
    unrealized_pnl: trade.unrealizedPnl,
    margin_used: trade.marginUsed,
    funding_costs: trade.fundingCosts,
    opened_at: trade.openedAt,
  };
}

function mockTradeToJournalRow(trade: MockTrade): Record<string, unknown> {
  const side = trade.side === "BUY" ? "LONG" : "SHORT";
  return {
    trade_id: trade.id,
    client_trade_id: trade.id,
    symbol: trade.symbol,
    side,
    strategy_id: trade.strategyName,
    strategy: trade.strategyName,
    entry_price: trade.entryPrice,
    exit_price: trade.exitPrice ?? trade.currentPrice,
    size_btc: trade.quantity,
    realized_pnl: trade.realizedPnl,
    fees: trade.fees,
    funding_costs: trade.fundingCosts,
    exit_reason: trade.exitReason ?? "CLOSE",
    opened_at: trade.openedAt,
    closed_at: trade.closedAt,
  };
}

export async function buildMockTradingSnapshot(accountKey: string): Promise<PaperDeskSnapshotPayload> {
  const { account: cached, config: cachedConfig } = await getLatestMockAccountSnapshot(accountKey);
  const config = normalizeMockTradingConfig(cachedConfig ?? DEFAULT_MOCK_TRADING_CONFIG);
  const account = cached ?? (await computeAccountFromMongo(accountKey, config));

  const [openResult, recentResult] = await Promise.all([
    listMockTrades({ account_key: accountKey, status: "OPEN", page: 1, limit: 200, sort: "oldest" }),
    listMockTrades({ account_key: accountKey, status: "CLOSED", page: 1, limit: 20, sort: "newest" }),
  ]);

  const state: Record<string, unknown> = {
    account_key: accountKey,
    balance: account.cashBalance,
    equity: account.equity,
    starting_balance: account.startingBalance,
    realized_pnl: account.realizedPnl,
    unrealized_pnl: account.unrealizedPnl,
    exposure: account.exposure,
    margin_used: account.marginUsed,
    available_balance: account.availableBalance,
    return_pct: account.returnPct,
    peak_equity: account.peakEquity,
    max_drawdown: account.maxDrawdownPct,
    current_drawdown: account.maxDrawdownPct,
    open_position_count: account.openCount,
    closed_trade_count: account.closedCount,
    snapped_at: new Date().toISOString(),
    execution_authority: "mock-trading",
  };

  const portfolio: Record<string, unknown> = {
    equity: account.equity,
    cash_balance: account.cashBalance,
    realized_pnl: account.realizedPnl,
    unrealized_pnl: account.unrealizedPnl,
    gross_exposure: account.exposure,
    net_exposure: account.longExposure - account.shortExposure,
    margin_used: account.marginUsed,
    drawdown: account.maxDrawdownPct,
    return_pct: account.returnPct,
    open_positions: account.openCount,
    closed_trades: account.closedCount,
    regime: "",
  };

  return {
    server_time: new Date().toISOString(),
    state,
    open_positions: openResult.trades.map(mockTradeToOpenPosition),
    recent_trades: recentResult.trades.map(mockTradeToJournalRow),
    health_summary: {
      healthy: 0,
      warning: 0,
      critical: 0,
      insufficient_data: 0,
    },
    portfolio,
  };
}

export async function buildMockStrategyIntelRows(accountKey: string) {
  const { trades } = await listMockTrades({
    account_key: accountKey,
    status: "CLOSED",
    page: 1,
    limit: 50_000,
    sort: "oldest",
  });

  const byStrategy = new Map<
    string,
    { wins: number; losses: number; totalPnl: number; grossWin: number; grossLoss: number }
  >();

  for (const trade of trades) {
    const key = trade.strategyName || String(trade.strategyId);
    const bucket = byStrategy.get(key) ?? { wins: 0, losses: 0, totalPnl: 0, grossWin: 0, grossLoss: 0 };
    bucket.totalPnl += trade.realizedPnl;
    if (trade.realizedPnl >= 0) {
      bucket.wins += 1;
      bucket.grossWin += trade.realizedPnl;
    } else {
      bucket.losses += 1;
      bucket.grossLoss += Math.abs(trade.realizedPnl);
    }
    byStrategy.set(key, bucket);
  }

  return [...byStrategy.entries()].map(([strategy_id, stats]) => {
    const sample = stats.wins + stats.losses;
    const winRate = sample > 0 ? stats.wins / sample : 0;
    const profitFactor = stats.grossLoss > 0 ? stats.grossWin / stats.grossLoss : stats.grossWin > 0 ? 99 : 0;
    const expectancy = sample > 0 ? stats.totalPnl / sample : 0;
    const status =
      sample < 5 ? "INSUFFICIENT_DATA" : winRate >= 0.5 && stats.totalPnl > 0 ? "HEALTHY" : winRate < 0.35 ? "CRITICAL" : "WARNING";
    return {
      strategy_id,
      status,
      enabled: status === "HEALTHY" || status === "WARNING",
      total_pnl: stats.totalPnl,
      expectancy,
      profit_factor: profitFactor,
      win_rate: winRate,
      max_drawdown: 0,
      sample_size: sample,
      evidence_score: Math.min(100, sample * 2),
      allocation_tier: status === "HEALTHY" ? "A" : status === "WARNING" ? "B" : status === "CRITICAL" ? "D" : "F",
    };
  });
}
