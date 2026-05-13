import { describe, expect, it } from "vitest";
import {
  buildDeskHoldTuningDumpPayload,
  rankWorstTimeOffenders,
  reduceTradesToStrategyDeskBuckets,
  type TradeRowForHoldTuning,
} from "./futuresHoldTuningAnalysis";

describe("reduceTradesToStrategyDeskBuckets", () => {
  const widened = new Map<number, boolean>([
    [1, true],
    [2, false],
  ]);
  const cats = new Map<number, string>([
    [1, "MeanRev"],
    [2, "Trend"],
  ]);

  it("keys by strategyId and deskTpWidened from map", () => {
    const trades: TradeRowForHoldTuning[] = [
      { strategyId: 1, strategyName: "A", category: "MeanRev", exitReason: "TIME", netPnl: -1 },
      { strategyId: 1, strategyName: "A", category: "MeanRev", exitReason: "TIME", netPnl: -2 },
      { strategyId: 1, strategyName: "A", category: "MeanRev", exitReason: "TP", netPnl: 3 },
    ];
    const buckets = reduceTradesToStrategyDeskBuckets(trades, widened, cats, 400);
    expect(buckets).toHaveLength(1);
    const b = buckets[0];
    expect(b?.key).toBe("1:w");
    expect(b?.perReason.TIME?.count).toBe(2);
    expect(b?.perReason.TIME?.sumNet).toBe(-3);
    expect(b?.perReason.TP?.count).toBe(1);
    expect(b?.perReason.TP?.sumNet).toBe(3);
  });

  it("splits same id when widened flag differs from map (two buckets)", () => {
    const w = new Map<number, boolean>([[9, false]]);
    const trades: TradeRowForHoldTuning[] = [
      { strategyId: 9, strategyName: "X", category: "Vol", exitReason: "TIME", netPnl: -1 },
    ];
    const b = reduceTradesToStrategyDeskBuckets(trades, w, cats, 400);
    expect(b).toHaveLength(1);
    expect(b[0]?.key).toBe("9:nw");
  });

  it("uses lastN window slice (last row only)", () => {
    const trades: TradeRowForHoldTuning[] = [
      { strategyId: 1, strategyName: "A", category: "MeanRev", exitReason: "SL", netPnl: 0 },
      { strategyId: 1, strategyName: "A", category: "MeanRev", exitReason: "TIME", netPnl: -5 },
    ];
    const b = reduceTradesToStrategyDeskBuckets(trades, widened, cats, 1);
    expect(b).toHaveLength(1);
    expect(b[0]?.perReason.SL).toBeUndefined();
    expect(b[0]?.perReason.TIME?.count).toBe(1);
    expect(b[0]?.perReason.TIME?.sumNet).toBe(-5);
  });
});

describe("rankWorstTimeOffenders", () => {
  it("orders by most negative TIME sumNet first", () => {
    const buckets = reduceTradesToStrategyDeskBuckets(
      [
        { strategyId: 1, strategyName: "Lo", category: "A", exitReason: "TIME", netPnl: -10 },
        { strategyId: 2, strategyName: "Hi", category: "B", exitReason: "TIME", netPnl: -1 },
        { strategyId: 1, strategyName: "Lo", category: "A", exitReason: "TP", netPnl: 2 },
      ],
      new Map([
        [1, false],
        [2, false],
      ]),
      new Map(),
      400,
    );
    const r = rankWorstTimeOffenders(buckets, 5);
    expect(r[0]?.strategyId).toBe(1);
    expect(r[0]?.timeSumNet).toBe(-10);
    expect(r[1]?.strategyId).toBe(2);
    expect(r[0]?.tpMeanNet).toBeCloseTo(2, 5);
  });
});

describe("buildDeskHoldTuningDumpPayload", () => {
  it("includes meanNet per exitReason in dump buckets", () => {
    const p = buildDeskHoldTuningDumpPayload(
      [{ strategyId: 3, strategyName: "BB", category: "MeanRev", exitReason: "TIME", netPnl: -4 }],
      new Map([[3, true]]),
      new Map([[3, "MeanRev"]]),
      400,
    );
    expect(p.buckets[0]?.deskTpWidened).toBe(true);
    expect(p.buckets[0]?.exitReasons.TIME?.meanNet).toBe(-4);
  });
});

/**
 * Synthetic ledger of exactly 400 closes: 394 padding trades + 6 TIME trades.
 * Hand TIME totals: id10 (nw) −2+(−4)=−6; id30 (nw) −5; id20 (w) −1×3=−3 → rank 10, 30, 20.
 */
describe("last-400 worst-TIME ranking (synthetic ledger)", () => {
  it("matches hand-calculated TIME sumNet order", () => {
    const widened = new Map<number, boolean>([
      [10, false],
      [20, true],
      [30, false],
      [100, false],
    ]);
    const cats = new Map<number, string>([
      [10, "MeanRev"],
      [20, "Momentum"],
      [30, "Trend"],
      [100, "Vol"],
    ]);
    const filler: TradeRowForHoldTuning[] = Array.from({ length: 394 }, () => ({
      strategyId: 100,
      strategyName: "Pad",
      category: "Vol",
      exitReason: "TP",
      netPnl: 0.001,
    }));
    const tail: TradeRowForHoldTuning[] = [
      { strategyId: 10, strategyName: "S10", category: "MeanRev", exitReason: "TIME", netPnl: -2 },
      { strategyId: 10, strategyName: "S10", category: "MeanRev", exitReason: "TIME", netPnl: -4 },
      { strategyId: 30, strategyName: "S30", category: "Trend", exitReason: "TIME", netPnl: -5 },
      { strategyId: 20, strategyName: "S20", category: "Momentum", exitReason: "TIME", netPnl: -1 },
      { strategyId: 20, strategyName: "S20", category: "Momentum", exitReason: "TIME", netPnl: -1 },
      { strategyId: 20, strategyName: "S20", category: "Momentum", exitReason: "TIME", netPnl: -1 },
    ];
    const ledger400 = [...filler, ...tail];
    expect(ledger400).toHaveLength(400);

    const buckets = reduceTradesToStrategyDeskBuckets(ledger400, widened, cats, 400);
    const ranked = rankWorstTimeOffenders(buckets, 5).map((r) => ({
      strategyId: r.strategyId,
      category: r.category,
      deskTpWidened: r.deskTpWidened,
      timeCount: r.timeCount,
      timeSumNet: r.timeSumNet,
      timeMeanNet: r.timeMeanNet,
    }));

    expect(ranked).toMatchSnapshot();

    expect(ranked[0]).toEqual({
      strategyId: 10,
      category: "MeanRev",
      deskTpWidened: false,
      timeCount: 2,
      timeSumNet: -6,
      timeMeanNet: -3,
    });
    expect(ranked[1]).toEqual({
      strategyId: 30,
      category: "Trend",
      deskTpWidened: false,
      timeCount: 1,
      timeSumNet: -5,
      timeMeanNet: -5,
    });
    expect(ranked[2]).toEqual({
      strategyId: 20,
      category: "Momentum",
      deskTpWidened: true,
      timeCount: 3,
      timeSumNet: -3,
      timeMeanNet: -1,
    });
  });
});
