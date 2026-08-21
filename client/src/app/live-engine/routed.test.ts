import { describe, expect, it } from "vitest";

/**
 * The rule this board got wrong, stated as data.
 *
 * `enabled` is an owner preference stored per stream and it SURVIVES a roster
 * change. `routed` is whether the engine's allow-list contains the stream at
 * all. Only the second decides whether an order can be placed, and conflating
 * them made 13 history rows render as "on" against a 9-stream roster.
 */
type Row = { strategy: string; symbol: string; enabled: boolean; routed: boolean; trades: number };

const onCount = (rows: Row[]) => rows.filter((r) => r.routed && r.enabled).length;
const offCount = (rows: Row[]) => rows.filter((r) => r.routed && !r.enabled).length;
const liveLabel = (r: Row) => (!r.routed ? "not routed" : r.enabled ? "on" : "off");

describe("live roster vs live record", () => {
  // Modelled on the real state: 9 routed, 31 with history, 1 in both.
  const rows: Row[] = [
    { strategy: "MTF_4h_DoubleTopBottom_Long", symbol: "AVAXUSD", enabled: true, routed: true, trades: 0 },
    { strategy: "MTF_1d_Keltner_Long", symbol: "ETHUSD", enabled: true, routed: true, trades: 0 },
    { strategy: "MTF_1h_Keltner_Long", symbol: "AVAXUSD", enabled: false, routed: true, trades: 3 },
    // History only: filled under an older roster, never switched off.
    { strategy: "MTF_10m_TrendPullback_Short", symbol: "BLESSUSD", enabled: true, routed: false, trades: 18 },
    { strategy: "MTF_10m_FibRetrace_Short", symbol: "ARCUSD", enabled: true, routed: false, trades: 16 },
  ];

  it("does not count an unrouted stream as live, however its switch reads", () => {
    // Both history rows carry enabled:true. Counting them is the bug.
    expect(onCount(rows)).toBe(2);
    expect(offCount(rows)).toBe(1);
    expect(onCount(rows) + offCount(rows)).toBe(rows.filter((r) => r.routed).length);
  });

  it("labels an unrouted stream distinctly, never as off", () => {
    // "off" would imply switching it on could make it trade. It could not:
    // the bridge consults the allow-list, and this stream is not on it.
    expect(liveLabel(rows[3])).toBe("not routed");
    expect(liveLabel(rows[3])).not.toBe("off");
    expect(liveLabel(rows[2])).toBe("off");
    expect(liveLabel(rows[0])).toBe("on");
  });

  it("keeps history rows on the board — they are the record, not clutter", () => {
    // The 30 unrouted rows are where every real fill lives. Hiding them would
    // leave a board whose realized P&L came from nowhere.
    const withHistory = rows.filter((r) => r.trades > 0);
    expect(withHistory.length).toBe(3);
    expect(withHistory.some((r) => !r.routed)).toBe(true);
  });

  // A routed stream that has never filled must still appear, or a promotion
  // cannot be confirmed from the board — "enabled and never filled" and
  // "not enabled" look identical if the first is hidden.
  it("shows routed streams that have not filled yet", () => {
    const routedUnfilled = rows.filter((r) => r.routed && r.trades === 0);
    expect(routedUnfilled.length).toBe(2);
  });
});
