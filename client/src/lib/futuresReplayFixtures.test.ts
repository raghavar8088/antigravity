import { describe, expect, it } from "vitest";
import {
  filterCandlesByTimeRange,
  parseReplayFixtureFile,
  utcDayBoundsMs,
} from "./futuresReplayFixtures";

describe("parseReplayFixtureFile (live JSON shape)", () => {
  it("parses Delta-style fixture payload", () => {
    const parsed = parseReplayFixtureFile({
      symbol: "BTCUSD",
      barMs: 60_000,
      fetchedAt: "2026-05-16T12:00:00.000Z",
      source: "delta-exchange-futures-1m",
      fundingRate: 0.0001,
      candles: [
        {
          time: 1_700_000_060_000,
          open: 100_000,
          high: 100_100,
          low: 99_900,
          close: 100_050,
          volume: 12,
        },
      ],
    });
    expect(parsed.symbol).toBe("BTCUSD");
    expect(parsed.candles).toHaveLength(1);
    expect(parsed.candles[0]!.close).toBe(100_050);
    expect(parsed.fundingRate).toBe(0.0001);
  });

  it("utcDayBoundsMs and filterCandlesByTimeRange", () => {
    const { startMs, endMs } = utcDayBoundsMs("2026-05-16");
    const candles = [
      { time: startMs - 60_000, open: 1, high: 1, low: 1, close: 1, volume: 1 },
      { time: startMs + 60_000, open: 2, high: 2, low: 2, close: 2, volume: 2 },
      { time: endMs, open: 3, high: 3, low: 3, close: 3, volume: 3 },
    ];
    const day = filterCandlesByTimeRange(candles, startMs, endMs);
    expect(day).toHaveLength(1);
    expect(day[0]!.close).toBe(2);
  });
});
