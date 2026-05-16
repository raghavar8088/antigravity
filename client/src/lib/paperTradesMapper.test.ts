import { describe, expect, it } from "vitest";
import type { BTCFuturesTrade } from "@/lib/btcFuturesTrade.types";
import {
  btcFuturesTradeToClientPayload,
  clientPayloadToInsertRow,
  dbRowToBtcFuturesTrade,
} from "@/lib/paperTradesMapper";
import { paperTradePostBodySchema } from "@/lib/paperTradesTypes";

const sampleTrade: BTCFuturesTrade = {
  clientTradeId: "550e8400-e29b-41d4-a716-446655440000",
  id: "BTCUSD-3-123",
  symbol: "BTCUSD",
  strategyId: 3,
  strategyName: "BB_MeanRev_Long",
  side: "LONG",
  entryPrice: 80_000,
  exitPrice: 81_000,
  contracts: 500,
  notional: 500,
  marginUsed: 20,
  realizedPnl: 6.25,
  fees: 1,
  netPnl: 5.25,
  netPnlPct: 26.25,
  priceMovePct: 1.25,
  fundingCosts: 0.05,
  openedAt: "2026-05-14T10:00:00.000Z",
  closedAt: "2026-05-14T10:15:00.000Z",
  exitReason: "PROFIT_LOCK",
  liquidationPrice: 76_800,
  liquidationDistancePct: 5,
};

describe("paperTradesMapper", () => {
  it("maps trade to insert row with gross = realizedPnl", () => {
    const payload = btcFuturesTradeToClientPayload(sampleTrade);
    const row = clientPayloadToInsertRow("btc_future_trading_20", payload);
    expect(row.account_key).toBe("btc_future_trading_20");
    expect(row.client_trade_id).toBe(sampleTrade.clientTradeId);
    expect(row.gross_pnl).toBe(6.25);
    expect(row.net_pnl).toBe(5.25);
    expect(row.funding_costs).toBe(0.05);
  });

  it("POST body schema accepts mapped payload", () => {
    const payload = btcFuturesTradeToClientPayload(sampleTrade);
    const parsed = paperTradePostBodySchema.safeParse({
      accountKey: "btc_future_trading_20",
      trade: payload,
    });
    expect(parsed.success).toBe(true);
  });

  it("POST body schema accepts trade without accountKey (session sets account)", () => {
    const payload = btcFuturesTradeToClientPayload(sampleTrade);
    const parsed = paperTradePostBodySchema.safeParse({ trade: payload });
    expect(parsed.success).toBe(true);
  });

  it("dbRowToBtcFuturesTrade prefers payload snapshot", () => {
    const row = clientPayloadToInsertRow(
      "btc_future_trading_20",
      btcFuturesTradeToClientPayload(sampleTrade),
    );
    const fromDb = dbRowToBtcFuturesTrade({
      id: "db-uuid",
      created_at: "2026-05-14T10:16:00.000Z",
      account_key: row.account_key,
      client_trade_id: row.client_trade_id,
      opened_at: row.opened_at,
      closed_at: row.closed_at,
      symbol: row.symbol,
      strategy_id: row.strategy_id,
      strategy_name: row.strategy_name,
      side: row.side,
      entry_price: row.entry_price,
      exit_price: row.exit_price,
      contracts: row.contracts,
      notional: row.notional,
      margin_used: row.margin_used,
      gross_pnl: row.gross_pnl,
      fees: row.fees,
      funding_costs: row.funding_costs,
      net_pnl: row.net_pnl,
      exit_reason: row.exit_reason,
      payload: row.payload,
    });
    expect(fromDb.clientTradeId).toBe(sampleTrade.clientTradeId);
    expect(fromDb.netPnl).toBe(5.25);
    expect(fromDb.priceMovePct).toBe(1.25);
  });
});
