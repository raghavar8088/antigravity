import { describe, expect, it } from "vitest";
import type { BTCFuturesTrade } from "./btcFuturesTrade.types";
import {
  formatReplaySummary,
  mapReplayTradesToTableRows,
  parsePaperReplayApiResponse,
} from "./futuresReplayUi";
import type { PaperReplayStats } from "./futuresReplayEngine";

const sampleTrade: BTCFuturesTrade = {
  id: "t1",
  symbol: "BTCUSD",
  strategyId: 91,
  strategyName: "Trend A",
  side: "LONG",
  entryPrice: 100_000,
  exitPrice: 100_100,
  contracts: 100,
  notional: 100,
  marginUsed: 4,
  realizedPnl: 0.08,
  fees: 0.2,
  netPnl: -0.12,
  netPnlPct: -0.12,
  priceMovePct: 0.1,
  fundingCosts: 0,
  openedAt: "2026-05-01T10:00:00.000Z",
  closedAt: "2026-05-01T10:05:00.000Z",
  exitReason: "TIME",
  liquidationPrice: 96_000,
  liquidationDistancePct: 4,
};

const sampleStats: PaperReplayStats = {
  count: 2,
  sumNet: 1.5,
  expectancy: 0.75,
  exitReasonCounts: { TIME: 1, TP: 1 },
};

describe("formatReplaySummary", () => {
  it("formats trade count, PnL, expectancy, and exit reason line", () => {
    const v = formatReplaySummary(sampleStats);
    expect(v.tradeCount).toBe(2);
    expect(v.sumNet).toBe(1.5);
    expect(v.expectancy).toBe(0.75);
    expect(v.exitReasonLine).toContain("TIME×1");
    expect(v.exitReasonLine).toContain("TP×1");
  });
});

describe("mapReplayTradesToTableRows", () => {
  it("maps trade shape and sorts by closedAt desc", () => {
    const older = { ...sampleTrade, closedAt: "2026-05-01T09:00:00.000Z", strategyName: "Old" };
    const newer = { ...sampleTrade, closedAt: "2026-05-01T11:00:00.000Z", strategyName: "New" };
    const rows = mapReplayTradesToTableRows([older, newer]);
    expect(rows[0]!.strategyName).toBe("New");
    expect(rows[0]).toMatchObject({
      side: "LONG",
      netPnl: -0.12,
      exitReason: "TIME",
    });
  });
});

describe("parsePaperReplayApiResponse", () => {
  it("accepts ok payload with trades and stats", () => {
    const parsed = parsePaperReplayApiResponse({
      ok: true,
      symbol: "BTCUSD",
      bars: 500,
      finalBalance: 1001,
      trades: [sampleTrade],
      stats: { ...sampleStats, count: 1 },
    });
    expect(parsed.ok).toBe(true);
    if (parsed.ok) {
      expect(parsed.data.trades).toHaveLength(1);
      expect(parsed.data.stats.count).toBe(1);
    }
  });

  it("rejects malformed payload", () => {
    expect(parsePaperReplayApiResponse({ ok: true, trades: [] }).ok).toBe(false);
    expect(parsePaperReplayApiResponse({ ok: false, error: "nope" }).ok).toBe(false);
  });
});
