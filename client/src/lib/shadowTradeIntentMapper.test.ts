import { describe, expect, it } from "vitest";
import {
  dbRowToShadowIntentListItem,
  shadowIntentFromPaperClose,
  shadowIntentFromPaperOpen,
  shadowIntentPostBodyToInsertRow,
} from "./shadowTradeIntentMapper";
import type { BTCFuturesTrade } from "@/lib/btcFuturesTrade.types";
import type { ShadowTradeIntentDbRow } from "./shadowTradeIntentTypes";

const sampleClose: BTCFuturesTrade = {
  clientTradeId: "550e8400-e29b-41d4-a716-446655440000",
  id: "BTCUSD-3-1",
  symbol: "BTCUSD",
  strategyId: 91,
  strategyName: "Trend",
  side: "LONG",
  entryPrice: 80_000,
  exitPrice: 81_000,
  contracts: 100,
  notional: 100,
  marginUsed: 4,
  realizedPnl: 1,
  fees: 0.2,
  netPnl: 0.8,
  netPnlPct: 20,
  priceMovePct: 1.25,
  fundingCosts: 0,
  openedAt: "2026-05-16T10:00:00.000Z",
  closedAt: "2026-05-16T10:05:00.000Z",
  exitReason: "TP",
  liquidationPrice: 76_000,
  liquidationDistancePct: 5,
};

describe("shadowTradeIntentMapper", () => {
  it("maps paper close to POST body", () => {
    const body = shadowIntentFromPaperClose(sampleClose);
    expect(body.intentKind).toBe("close");
    expect(body.clientIntentId).toBe(sampleClose.clientTradeId);
    expect(body.exitReason).toBe("TP");
    expect(body.exitPrice).toBe(81_000);
  });

  it("maps paper open without exit fields", () => {
    const body = shadowIntentFromPaperOpen({
      symbol: "BTCUSD",
      side: "SHORT",
      notional: 50,
      entryPrice: 90_000,
      strategyId: 92,
      strategyName: "Breakout",
    });
    expect(body.intentKind).toBe("open");
    expect(body.exitPrice).toBeNull();
    expect(body.exitReason).toBeNull();
  });

  it("maps POST body to insert row with would_place_testnet", () => {
    const body = shadowIntentFromPaperClose(sampleClose);
    const row = shadowIntentPostBodyToInsertRow("user-uuid", body, true);
    expect(row.user_id).toBe("user-uuid");
    expect(row.would_place_testnet).toBe(true);
    expect(row.exit_price).toBe(81_000);
  });

  it("maps db row to list item", () => {
    const db: ShadowTradeIntentDbRow = {
      id: "id-1",
      created_at: "2026-05-16T12:00:00.000Z",
      user_id: "user-uuid",
      client_intent_id: "550e8400-e29b-41d4-a716-446655440000",
      intent_kind: "close",
      symbol: "BTCUSD",
      side: "LONG",
      notional: 100,
      entry_price: 80_000,
      exit_price: 81_000,
      exit_reason: "TP",
      strategy_id: 91,
      strategy_name: "Trend",
      would_place_testnet: true,
    };
    const item = dbRowToShadowIntentListItem(db);
    expect(item.wouldPlaceTestnet).toBe(true);
    expect(item.intentKind).toBe("close");
  });
});
