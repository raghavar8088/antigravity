import { describe, expect, it } from "vitest";

import { evaluateBar, fundingSettlementsBetween, replayResolution } from "./engine";
import { halfSpreadFraction, sizePosition, slipped, type SizingInput } from "./sizing";
import type { Bar } from "../delta";
import type { ScreenerRow } from "../universe";

function bar(o: number, h: number, l: number, c: number, ts = 1_000): Bar {
  return { ts, open: o, high: h, low: l, close: c, volume: 1 };
}

describe("evaluateBar — what a bar did to a position", () => {
  it("fills a long stop AT the stop when the bar traded through it normally", () => {
    const e = evaluateBar(bar(100, 101, 94, 96), "long", 95, 110);
    expect(e.stopHit).toBe(true);
    expect(e.targetHit).toBe(false);
    expect(e.stopFill).toBe(95);
  });

  it("fills a long stop AT THE OPEN when price gapped straight past it", () => {
    // Opening below the stop means the order could never have filled at the
    // stop. Filling there anyway is the single most common way a paper record
    // flatters itself.
    const e = evaluateBar(bar(90, 92, 88, 91), "long", 95, 110);
    expect(e.stopHit).toBe(true);
    expect(e.stopFill).toBe(90);
    expect(e.stopFill).toBeLessThan(95);
  });

  it("fills a long target AT THE OPEN on a favourable gap — honest in both directions", () => {
    const e = evaluateBar(bar(115, 118, 114, 117), "long", 95, 110);
    expect(e.targetHit).toBe(true);
    expect(e.targetFill).toBe(115);
    expect(e.targetFill).toBeGreaterThan(110);
  });

  it("mirrors the whole thing for a short", () => {
    const stopped = evaluateBar(bar(100, 106, 99, 105), "short", 105, 90);
    expect(stopped.stopHit).toBe(true);
    expect(stopped.stopFill).toBe(105);

    const gapped = evaluateBar(bar(108, 110, 107, 109), "short", 105, 90);
    expect(gapped.stopHit).toBe(true);
    expect(gapped.stopFill).toBe(108); // worse than the stop, as a gap must be

    const won = evaluateBar(bar(95, 96, 88, 89), "short", 105, 90);
    expect(won.targetHit).toBe(true);
    expect(won.targetFill).toBe(90);
  });

  it("reports BOTH when one bar's range contains the stop and the target", () => {
    // The caller must not pick a winner from this — it drops to 1m bars, and
    // assumes the stop when it cannot.
    const e = evaluateBar(bar(100, 112, 93, 105), "long", 95, 110);
    expect(e.stopHit).toBe(true);
    expect(e.targetHit).toBe(true);
  });

  it("reports neither when the bar stayed inside both levels", () => {
    const e = evaluateBar(bar(100, 104, 97, 103), "long", 95, 110);
    expect(e.stopHit).toBe(false);
    expect(e.targetHit).toBe(false);
  });
});

describe("fundingSettlementsBetween — 8h stamps at 00:00 / 08:00 / 16:00 UTC", () => {
  const H = 3_600;
  it("counts nothing inside one interval", () => {
    expect(fundingSettlementsBetween(1 * H, 7 * H)).toBe(0);
  });
  it("counts a single crossed stamp", () => {
    expect(fundingSettlementsBetween(7 * H, 9 * H)).toBe(1);
  });
  it("counts three stamps across a full day", () => {
    expect(fundingSettlementsBetween(0, 24 * H)).toBe(3);
  });
  it("counts fifteen across a five-day swing — the number that kills those plans", () => {
    expect(fundingSettlementsBetween(0, 5 * 24 * H)).toBe(15);
  });
  it("never counts backwards", () => {
    expect(fundingSettlementsBetween(9 * H, 7 * H)).toBe(0);
    expect(fundingSettlementsBetween(9 * H, 9 * H)).toBe(0);
  });
});

describe("replayResolution — fine enough to be honest, coarse enough to fit one request", () => {
  it("uses 5m for every hold this desk actually takes", () => {
    expect(replayResolution(6 * 3_600)).toBe("5m");
    expect(replayResolution(10 * 86_400)).toBe("5m");
  });
  it("steps down rather than silently receiving a truncated series", () => {
    expect(replayResolution(20 * 86_400)).toBe("15m");
    expect(replayResolution(90 * 86_400)).toBe("1h");
  });
});

// ── sizing ──────────────────────────────────────────────────────────────────

function row(over: Partial<ScreenerRow> = {}): ScreenerRow {
  return {
    symbol: "TESTUSD",
    price: 100,
    atrPct: 3,
    maintenanceMarginPct: 0.5,
    venueMaxLeverage: 100,
    micro: {
      contractValue: 1,
      tickSize: 0.01,
      stopExpressible: true,
      stopTicks: 100,
      spreadBps: 4,
    },
    ...over,
  } as unknown as ScreenerRow;
}

const base: Omit<SizingInput, "row"> = {
  side: "long",
  entry: 100,
  stop: 97,
  availableEquityUsd: 10_000,
  bookEquityUsd: 10_000,
};

describe("sizePosition — risk first, then leverage, then the venue's own liquidation", () => {
  it("risks 2% of the book regardless of how wide the stop is", () => {
    const tight = sizePosition({ ...base, row: row(), stop: 99 });
    const wide = sizePosition({ ...base, row: row(), stop: 90 });
    expect(tight.ok && wide.ok).toBe(true);
    if (!tight.ok || !wide.ok) return;
    // Both risk ~$200; the tight stop simply buys more of the contract. That is
    // what makes an R multiple comparable across contracts.
    expect(tight.riskUsd).toBeGreaterThan(180);
    expect(tight.riskUsd).toBeLessThan(210);
    expect(wide.riskUsd).toBeGreaterThan(180);
    expect(wide.riskUsd).toBeLessThan(210);
    expect(tight.notionalUsd).toBeGreaterThan(wide.notionalUsd);
  });

  it("caps notional at 3x equity so a 0.1% stop cannot buy the whole venue", () => {
    const s = sizePosition({ ...base, row: row(), stop: 99.9 });
    expect(s.ok).toBe(true);
    if (!s.ok) return;
    expect(s.notionalUsd).toBeLessThanOrEqual(30_000 + 1);
  });

  it("keeps margin inside the book's per-position budget by taking leverage", () => {
    const s = sizePosition({ ...base, row: row(), stop: 99 });
    expect(s.ok).toBe(true);
    if (!s.ok) return;
    // 1% stop -> ~$20,000 notional. At 1x that would post twice the whole book.
    expect(s.leverage).toBeGreaterThan(1);
    expect(s.marginUsd).toBeLessThanOrEqual(2_500 + 1);
  });

  it("REFUSES when the venue would liquidate before the stop could fill", () => {
    // A 9% stop against a 2.5% maintenance requirement needs leverage under
    // ~1/(0.135+0.025) = 6.2x; force the position to need far more by leaving
    // almost no free equity.
    const s = sizePosition({
      ...base,
      row: row({ maintenanceMarginPct: 2.5 }),
      stop: 91,
      availableEquityUsd: 50,
    });
    expect(s.ok).toBe(false);
    if (s.ok) return;
    expect(s.reason).toMatch(/free in this book|liquidation/i);
  });

  it("REFUSES rather than guessing when the venue published no maintenance margin", () => {
    const s = sizePosition({ ...base, row: row({ maintenanceMarginPct: null }) });
    expect(s.ok).toBe(false);
    if (s.ok) return;
    expect(s.reason).toMatch(/maintenance margin/i);
  });

  it("REFUSES a contract whose tick grid cannot hold the stop", () => {
    const s = sizePosition({
      ...base,
      row: row({ micro: { ...row().micro, stopExpressible: false, stopTicks: 4 } }),
    });
    expect(s.ok).toBe(false);
    if (s.ok) return;
    expect(s.reason).toMatch(/tick grid/i);
  });

  it("REFUSES when one whole contract is already bigger than the budget", () => {
    const s = sizePosition({
      ...base,
      row: row({ micro: { ...row().micro, contractValue: 1_000 } }),
    });
    expect(s.ok).toBe(false);
    if (s.ok) return;
    expect(s.reason).toMatch(/one contract/i);
  });

  it("always leaves liquidation further from entry than the stop", () => {
    for (const stop of [99.5, 99, 98, 95, 90, 80]) {
      for (const mm of [0.25, 0.5, 1, 2.5]) {
        const s = sizePosition({ ...base, row: row({ maintenanceMarginPct: mm }), stop });
        if (!s.ok) continue;
        expect(s.liquidationDistancePct).toBeGreaterThanOrEqual(s.stopDistancePct * 1.5 - 1e-9);
        expect(s.liquidationPrice).toBeLessThan(stop);
      }
    }
  });

  it("puts a short's liquidation ABOVE its entry", () => {
    const s = sizePosition({ ...base, row: row(), side: "short", entry: 100, stop: 103 });
    expect(s.ok).toBe(true);
    if (!s.ok) return;
    expect(s.liquidationPrice).toBeGreaterThan(103);
  });

  it("refuses a stop on the wrong side of entry instead of sizing a negative risk", () => {
    expect(sizePosition({ ...base, row: row(), stop: 101 }).ok).toBe(false);
    expect(sizePosition({ ...base, row: row(), side: "short", stop: 99 }).ok).toBe(false);
  });

  it("only ever sizes whole contracts", () => {
    const s = sizePosition({ ...base, row: row({ micro: { ...row().micro, contractValue: 0.37 } }) });
    expect(s.ok).toBe(true);
    if (!s.ok) return;
    expect(Number.isInteger(s.contracts)).toBe(true);
  });
});

describe("slippage — a taker always fills on the wrong side", () => {
  const hs = 0.001; // 20 bps spread

  it("pays the ask entering a long and the bid leaving it", () => {
    expect(slipped(100, "long", true, hs)).toBeCloseTo(100.1, 6);
    expect(slipped(100, "long", false, hs)).toBeCloseTo(99.9, 6);
  });

  it("pays the bid entering a short and the ask leaving it", () => {
    expect(slipped(100, "short", true, hs)).toBeCloseTo(99.9, 6);
    expect(slipped(100, "short", false, hs)).toBeCloseTo(100.1, 6);
  });

  it("assumes a pessimistic 25 bps when the venue quotes no book at all", () => {
    expect(halfSpreadFraction(row({ micro: { ...row().micro, spreadBps: null } }))).toBe(0.0025);
  });

  it("halves the quoted spread when there is one", () => {
    expect(halfSpreadFraction(row({ micro: { ...row().micro, spreadBps: 40 } }))).toBeCloseTo(0.002, 9);
  });
});
