import type { BTCFuturesTrade } from "@/lib/btcFuturesTrade.types";
import type {
  PaperTradeClientPayload,
  PaperTradeDbRow,
  PaperTradeModuleKey,
} from "@/lib/paperTradesTypes";
import { paperTradeExitReasonSchema, paperTradeSideSchema } from "@/lib/paperTradesTypes";

export type PaperTradeInsertRow = {
  account_key: string;
  client_trade_id: string;
  opened_at: string;
  closed_at: string;
  symbol: string;
  strategy_id: number;
  strategy_name: string;
  side: string;
  entry_price: number;
  exit_price: number;
  contracts: number;
  notional: number;
  margin_used: number;
  gross_pnl: number;
  fees: number;
  funding_costs: number;
  net_pnl: number;
  exit_reason: string;
  payload: Record<string, unknown>;
  module_key?: PaperTradeModuleKey;
  /** Parent algorithm family — used for cross-strategy research leaderboards. */
  template_family?: string;
  /** Exchange where execution was simulated (e.g. "binance", "delta", "nse"). */
  exchange?: string;
};

export function btcFuturesTradeToClientPayload(trade: BTCFuturesTrade): PaperTradeClientPayload {
  const clientTradeId = trade.clientTradeId ?? trade.id;
  return {
    clientTradeId,
    id: trade.id,
    symbol: trade.symbol,
    strategyId: trade.strategyId,
    strategyName: trade.strategyName,
    side: trade.side,
    entryPrice: trade.entryPrice,
    exitPrice: trade.exitPrice,
    contracts: trade.contracts,
    notional: trade.notional,
    marginUsed: trade.marginUsed,
    realizedPnl: trade.realizedPnl,
    fees: trade.fees,
    netPnl: trade.netPnl,
    netPnlPct: trade.netPnlPct,
    priceMovePct: trade.priceMovePct,
    fundingCosts: trade.fundingCosts,
    openedAt: trade.openedAt,
    closedAt: trade.closedAt,
    exitReason: trade.exitReason,
    liquidationPrice: trade.liquidationPrice,
    liquidationDistancePct: trade.liquidationDistancePct,
    lastFundingAppliedAt: trade.lastFundingAppliedAt,
    fundingSinceOpenMs: trade.fundingSinceOpenMs,
  };
}

export function clientPayloadToInsertRow(
  accountKey: string,
  trade: PaperTradeClientPayload,
  moduleKey?: PaperTradeModuleKey,
  templateFamily?: string,
  exchange?: string,
): PaperTradeInsertRow {
  const row: PaperTradeInsertRow = {
    account_key: accountKey,
    client_trade_id: trade.clientTradeId,
    opened_at: trade.openedAt,
    closed_at: trade.closedAt,
    symbol: trade.symbol,
    strategy_id: trade.strategyId,
    strategy_name: trade.strategyName,
    side: trade.side,
    entry_price: trade.entryPrice,
    exit_price: trade.exitPrice,
    contracts: trade.contracts,
    notional: trade.notional,
    margin_used: trade.marginUsed,
    gross_pnl: trade.realizedPnl,
    fees: trade.fees,
    funding_costs: trade.fundingCosts,
    net_pnl: trade.netPnl,
    exit_reason: trade.exitReason,
    payload: { ...trade },
  };
  if (moduleKey) row.module_key = moduleKey;
  if (templateFamily) row.template_family = templateFamily;
  if (exchange) row.exchange = exchange;
  return row;
}

export function dbRowToBtcFuturesTrade(row: PaperTradeDbRow): BTCFuturesTrade {
  const payload = row.payload;
  if (payload && typeof payload === "object" && !Array.isArray(payload)) {
    const p = payload as Record<string, unknown>;
    const side = paperTradeSideSchema.safeParse(p.side);
    const exitReason = paperTradeExitReasonSchema.safeParse(p.exitReason);
    if (side.success && exitReason.success) {
      return {
        ...(p as unknown as BTCFuturesTrade),
        clientTradeId: row.client_trade_id,
        side: side.data,
        exitReason: exitReason.data,
      };
    }
  }
  const side = paperTradeSideSchema.parse(row.side);
  const exitReason = paperTradeExitReasonSchema.parse(row.exit_reason);
  return {
    clientTradeId: row.client_trade_id,
    id: String(row.client_trade_id),
    symbol: row.symbol,
    strategyId: row.strategy_id,
    strategyName: row.strategy_name,
    side,
    entryPrice: row.entry_price,
    exitPrice: row.exit_price,
    contracts: row.contracts,
    notional: row.notional,
    marginUsed: row.margin_used,
    realizedPnl: row.gross_pnl,
    fees: row.fees,
    netPnl: row.net_pnl,
    netPnlPct: row.margin_used > 0 ? (row.net_pnl / row.margin_used) * 100 : 0,
    priceMovePct: 0,
    fundingCosts: row.funding_costs,
    openedAt: row.opened_at,
    closedAt: row.closed_at,
    exitReason,
    liquidationPrice: 0,
    liquidationDistancePct: 0,
  };
}
