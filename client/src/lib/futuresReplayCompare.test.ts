import { describe, expect, it } from "vitest";
import { formatReplayCompareTable, statsFromClosedTrades } from "./futuresReplayCompare";
import type { BTCFuturesTrade } from "./btcFuturesTrade.types";

const mockTrade = (net: number, reason: BTCFuturesTrade["exitReason"]): BTCFuturesTrade => ({
  id: "x",
  symbol: "BTCUSD",
  strategyId: 91,
  strategyName: "T",
  side: "LONG",
  entryPrice: 100_000,
  exitPrice: 100_100,
  contracts: 50,
  notional: 50,
  marginUsed: 2,
  realizedPnl: net,
  fees: 0.1,
  netPnl: net,
  netPnlPct: 0,
  priceMovePct: 0,
  fundingCosts: 0,
  openedAt: "2026-05-16T10:00:00.000Z",
  closedAt: "2026-05-16T10:05:00.000Z",
  exitReason: reason,
  liquidationPrice: 0,
  liquidationDistancePct: 0,
});

describe("formatReplayCompareTable", () => {
  it("formats live vs replay metrics", () => {
    const live = statsFromClosedTrades([mockTrade(2, "TP"), mockTrade(-0.5, "SL")]);
    const replay = statsFromClosedTrades([mockTrade(1, "PROFIT_LOCK")]);
    const table = formatReplayCompareTable(live, replay);
    expect(table).toContain("tradeCount");
    expect(table).toContain("2");
    expect(table).toContain("1");
    expect(table).toContain("sumNet");
    expect(table).toContain("TP×1");
  });
});
