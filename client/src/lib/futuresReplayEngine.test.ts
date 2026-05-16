import { describe, expect, it } from "vitest";
import { makeSyntheticBtcusd1mCandles } from "./futuresReplayFixtures";
import {
  replayClientTradeId,
  runPaperDeskReplay,
  type PaperReplayConfig,
} from "./futuresReplayEngine";

const BTC_20 = [
  91, 92, 95, 96, 111, 112, 117, 118, 123, 124, 125, 126, 131, 132, 133, 134, 139, 140, 151, 152,
];

const goldenConfig: PaperReplayConfig = {
  initialBalance: 1000,
  leverage: 25,
  slippageBps: 0,
  strategyIds: BTC_20,
  maxPositions: 12,
  barMs: 60_000,
  symbol: "BTCUSD",
  signalThreshold: 26,
  volSizedNotional: false,
  minExpectedMoveSafetyK: 1,
  maxSameDirFracOfEquity: 0.35,
  allowFakeDiversity: false,
  minTpSlRatio: 2,
  fundingRate: 0,
  drawdownLock: false,
};

describe("runPaperDeskReplay", () => {
  const goldenCandles = makeSyntheticBtcusd1mCandles(100, {
    startMs: 1_700_000_000_000,
    barMs: 60_000,
    basePrice: 100_000,
  });

  it("golden: 100-bar fixture → stable trade count and first clientTradeId", () => {
    const a = runPaperDeskReplay(goldenCandles, goldenConfig);
    const b = runPaperDeskReplay(goldenCandles, goldenConfig);
    expect(a.trades.length).toBe(b.trades.length);
    expect(a.stats).toEqual(b.stats);
    if (a.trades.length > 0) {
      const t = a.trades[0]!;
      expect(t.clientTradeId).toBe(
        replayClientTradeId(
          t.symbol,
          t.strategyId,
          new Date(t.openedAt).getTime(),
          new Date(t.closedAt).getTime(),
        ),
      );
      expect(a.trades[0]!.clientTradeId).toBe(b.trades[0]!.clientTradeId);
    }
    expect({
      tradeCount: a.trades.length,
      firstClientTradeId: a.trades[0]?.clientTradeId ?? null,
      stats: a.stats,
      finalBalance: Number(a.finalBalance.toFixed(4)),
      trades: a.trades.map((t) => ({
        clientTradeId: t.clientTradeId,
        strategyId: t.strategyId,
        exitReason: t.exitReason,
        netPnl: Number(t.netPnl.toFixed(4)),
      })),
    }).toMatchSnapshot();
  });

  it("slippageBps>0 worsens LONG entry vs 0 bps on same series", () => {
    const candles = makeSyntheticBtcusd1mCandles(120, { basePrice: 100_000 });
    const noSlip = runPaperDeskReplay(candles, { ...goldenConfig, slippageBps: 0 });
    const slip = runPaperDeskReplay(candles, { ...goldenConfig, slippageBps: 5 });
    const longNo = noSlip.trades.find((t) => t.side === "LONG");
    const longSl = slip.trades.find((t) => t.side === "LONG" && t.strategyId === longNo?.strategyId);
    if (longNo && longSl) {
      expect(longSl.entryPrice).toBeGreaterThan(longNo.entryPrice);
    }
    expect(slip.stats.sumNet).toBeLessThanOrEqual(noSlip.stats.sumNet + 1e-6);
  });
});
