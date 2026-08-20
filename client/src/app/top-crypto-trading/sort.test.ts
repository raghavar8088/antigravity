import { describe, expect, it } from "vitest";
import { sideOf, streamMetric, type SortKey, type Stream } from "./page";

function row(p: Partial<Stream>): Stream {
  return {
    strategy: "MTF_1h_Flag_Long",
    symbol: "BTCUSD",
    live: false,
    trades: 0,
    wins: 0,
    grossUsd: 0,
    feesUsd: 0,
    netUsd: 0,
    shareOfEquityPct: 0,
    ...p,
  };
}

describe("streamMetric", () => {
  // The two DERIVED columns are the ones worth testing. Win rate and fee drag
  // are computed at render time and are not fields on the row, so indexing the
  // row for them yields undefined — and sorting by undefined does nothing at
  // all, silently. A header whose arrows move and whose rows do not is the bug
  // this guards.
  it("computes win rate rather than reading a field that does not exist", () => {
    expect(streamMetric(row({ trades: 4, wins: 1 }), "wr")).toBe(25);
    expect(streamMetric(row({ trades: 8, wins: 6 }), "wr")).toBe(75);
  });

  it("computes fee drag as a share of gross", () => {
    expect(streamMetric(row({ grossUsd: 100, feesUsd: 25 }), "drag")).toBe(25);
  });

  // A stream with no gross has no drag. Sorting it as 0% would park every
  // untraded stream at the top of a cheapest-first ordering, which is the
  // opposite of what "lowest fee drag" is being asked for.
  it("sends a stream with no gross to the bottom rather than calling it 0% drag", () => {
    const none = streamMetric(row({ grossUsd: 0, feesUsd: 5 }), "drag");
    const real = streamMetric(row({ grossUsd: 100, feesUsd: 90 }), "drag");
    expect(none).toBe(Number.NEGATIVE_INFINITY);
    expect(none).toBeLessThan(real);
  });

  it("reads the plain numeric columns straight off the row", () => {
    const r = row({ trades: 7, grossUsd: 12, feesUsd: 3, netUsd: 9 });
    const cases: Array<[SortKey, number]> = [["n", 7], ["gross", 12], ["fees", 3], ["net", 9]];
    for (const [k, want] of cases) expect(streamMetric(r, k)).toBe(want);
  });

  // An untraded stream must not outrank a losing one on win rate. At 0 trades
  // the rate is undefined, and returning 0 would tie it with a strategy that
  // genuinely lost every trade.
  it("ranks an untraded stream below a 0% win rate", () => {
    expect(streamMetric(row({ trades: 0, wins: 0 }), "wr")).toBeLessThan(
      streamMetric(row({ trades: 5, wins: 0 }), "wr"),
    );
  });
});

describe("sideOf", () => {
  it("reads the side off the strategy name", () => {
    expect(sideOf("MTF_1h_Flag_Long")).toBe("LONG");
    expect(sideOf("MTF_4h_Wedge_Short")).toBe("SHORT");
  });

  // A name that encodes no side must not be silently filed as LONG — it would
  // vanish from the board whenever either side filter was applied.
  it("returns ALL for a name with no side suffix", () => {
    expect(sideOf("SomeOtherStrategy")).toBe("ALL");
  });
});
