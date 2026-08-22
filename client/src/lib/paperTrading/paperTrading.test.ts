import { describe, expect, it } from "vitest";

import {
  accrueCarry,
  findLevelHit,
  findLiquidation,
  fundingSettlements,
  limitFill,
  liquidationPrice,
  marginUsd,
  notionalUsd,
  pnlUsd,
  roundSize,
  roundToTick,
  stopFill,
  swapCharges,
  usdPerQuoteUnit,
  walkBook,
} from "./matching";
import { marketOpen } from "./venues/forex";
import type { Candle, Instrument, OrderBook } from "./types";

// ── fixtures ────────────────────────────────────────────────────────────────

function fx(over: Partial<Instrument> = {}): Instrument {
  return {
    venue: "forex",
    symbol: "EURUSD",
    displayName: "Euro / US Dollar",
    kind: "fx",
    contractSize: 100_000,
    sizeUnit: "lots",
    minSize: 0.01,
    sizeStep: 0.01,
    tickSize: 0.00001,
    pricePrecision: 5,
    maxLeverage: 2000,
    maintenanceMarginPct: 0,
    takerFeeRate: 0,
    makerFeeRate: 0,
    commissionPerLotUsd: 0,
    carryKind: "swap",
    fundingRatePct8h: null,
    swapLongPointsPerDay: -7.2,
    swapShortPointsPerDay: 2.1,
    last: 1.1,
    bid: 1.09995,
    ask: 1.10005,
    markPrice: 1.1,
    change24hPct: 0,
    high24h: null,
    low24h: null,
    quoteCurrency: "USD",
    spreadIsModelled: true,
    source: "test",
    ...over,
  };
}

function perp(over: Partial<Instrument> = {}): Instrument {
  return {
    ...fx(),
    venue: "delta",
    symbol: "BTCUSD",
    kind: "perp",
    contractSize: 0.001,
    sizeUnit: "contracts",
    minSize: 1,
    sizeStep: 1,
    tickSize: 0.5,
    pricePrecision: 1,
    maintenanceMarginPct: 0.5,
    takerFeeRate: 0.00059,
    makerFeeRate: 0.000236,
    carryKind: "funding",
    fundingRatePct8h: 0.01,
    swapLongPointsPerDay: null,
    swapShortPointsPerDay: null,
    last: 77_000,
    bid: 76_999,
    ask: 77_001,
    markPrice: 77_000,
    spreadIsModelled: false,
    ...over,
  };
}

function bar(o: number, h: number, l: number, c: number, time = 1_000): Candle {
  return { time, open: o, high: h, low: l, close: c, volume: 1 };
}

// ── the forex bug everyone writes ───────────────────────────────────────────

describe("usdPerQuoteUnit — converting out of the quote currency", () => {
  const quotes = new Map([
    ["EURUSD", 1.1678],
    ["GBPUSD", 1.3648],
    ["USDJPY", 158.94],
    ["USDCAD", 1.3764],
    ["AUDUSD", 0.7175],
  ]);

  it("is 1 for a USD-quoted pair", () => {
    expect(usdPerQuoteUnit(fx({ symbol: "EURUSD" }), quotes)).toBe(1);
  });

  it("inverts USDJPY for a yen-quoted pair", () => {
    expect(usdPerQuoteUnit(fx({ symbol: "USDJPY" }), quotes)).toBeCloseTo(1 / 158.94, 10);
  });

  it("uses USDJPY for a CROSS quoted in yen, not the cross's own price", () => {
    // GBPJPY profits in yen. Its own rate is ~216; using it would misprice the
    // P&L by about 36%.
    expect(usdPerQuoteUnit(fx({ symbol: "GBPJPY" }), quotes)).toBeCloseTo(1 / 158.94, 10);
    expect(usdPerQuoteUnit(fx({ symbol: "EURJPY" }), quotes)).toBeCloseTo(1 / 158.94, 10);
  });

  it("uses the direct pair when the quote currency is quoted against USD", () => {
    expect(usdPerQuoteUnit(fx({ symbol: "EURGBP" }), quotes)).toBeCloseTo(1.3648, 10);
    expect(usdPerQuoteUnit(fx({ symbol: "EURAUD" }), quotes)).toBeCloseTo(0.7175, 10);
  });

  it("returns NULL rather than 1 when the rate is missing", () => {
    // Defaulting to 1 would report a yen P&L as dollars, inflating it 159-fold.
    expect(usdPerQuoteUnit(fx({ symbol: "EURSEK" }), quotes)).toBeNull();
  });

  it("is 1 for crypto perpetuals and for USD-quoted metals", () => {
    expect(usdPerQuoteUnit(perp(), new Map())).toBe(1);
    expect(usdPerQuoteUnit(fx({ symbol: "XAUUSD", kind: "metal" }), new Map())).toBe(1);
  });

  it("values 100 pips on one standard lot correctly per pair", () => {
    const cases: [string, number, number, number][] = [
      // symbol, pip, contract, expected USD for 100 pips on 1.00 lot
      ["EURUSD", 0.0001, 100_000, 1_000],
      ["USDJPY", 0.01, 100_000, 100 * 0.01 * 100_000 / 158.94],
      ["GBPJPY", 0.01, 100_000, 100 * 0.01 * 100_000 / 158.94],
      ["EURGBP", 0.0001, 100_000, 100 * 0.0001 * 100_000 * 1.3648],
    ];
    for (const [symbol, pip, contract, expected] of cases) {
      const inst = fx({ symbol, contractSize: contract });
      const conv = usdPerQuoteUnit(inst, quotes)!;
      const got = pnlUsd(inst, "long", 1, 1, 1 + 100 * pip, conv);
      expect(got).toBeCloseTo(expected, 6);
    }
  });
});

describe("notional and margin", () => {
  const quotes = new Map([["USDJPY", 158.94], ["EURUSD", 1.1678]]);

  it("prices a EURUSD lot in dollars through the price", () => {
    expect(notionalUsd(fx(), 1, 1.1678, 1)).toBeCloseTo(116_780, 4);
  });

  it("prices a USDJPY lot at its BASE value, not its yen value", () => {
    // 1 lot of USDJPY is 100,000 USD. Multiplying by 158.94 would call it
    // $15.9m.
    const inst = fx({ symbol: "USDJPY", contractSize: 100_000 });
    const conv = usdPerQuoteUnit(inst, quotes)!;
    expect(notionalUsd(inst, 1, 158.94, conv)).toBeCloseTo(100_000, 3);
  });

  it("prices a Delta contract by its contract value", () => {
    expect(notionalUsd(perp(), 200, 77_000, 1)).toBeCloseTo(15_400, 6);
  });

  it("divides notional by leverage", () => {
    expect(marginUsd(15_400, 10)).toBeCloseTo(1_540, 9);
    expect(marginUsd(15_400, 1)).toBeCloseTo(15_400, 9);
  });
});

describe("liquidationPrice", () => {
  it("sits below a long and above a short", () => {
    expect(liquidationPrice("long", 100, 10, 0.5)).toBeCloseTo(100 * (1 - (0.1 - 0.005)), 9);
    expect(liquidationPrice("short", 100, 10, 0.5)).toBeCloseTo(100 * (1 + (0.1 - 0.005)), 9);
  });

  it("is null when maintenance margin eats the whole cushion", () => {
    // 50x leaves 2% of margin; a 2.5% maintenance requirement is already past it.
    expect(liquidationPrice("long", 100, 50, 2.5)).toBeNull();
  });

  it("is null when the venue published no maintenance margin — never a guess", () => {
    expect(liquidationPrice("long", 100, 10, 0)).toBeNull();
  });
});

// ── book walking ────────────────────────────────────────────────────────────

const book: OrderBook = {
  symbol: "BTCUSD",
  bids: [
    { price: 100, size: 5 },
    { price: 99, size: 10 },
    { price: 98, size: 20 },
  ],
  asks: [
    { price: 101, size: 5 },
    { price: 102, size: 10 },
    { price: 103, size: 20 },
  ],
  asOf: 0,
  modelled: false,
};

describe("walkBook — a market order eats depth", () => {
  it("fills at the touch when it fits in the best level", () => {
    const f = walkBook(book, "buy", 3)!;
    expect(f.avgPrice).toBe(101);
    expect(f.slippage).toBe(0);
    expect(f.levelsConsumed).toBe(1);
  });

  it("takes a size-weighted average across levels", () => {
    // 5 @ 101 + 5 @ 102 = 1015 over 10 = 101.5
    const f = walkBook(book, "buy", 10)!;
    expect(f.avgPrice).toBeCloseTo((5 * 101 + 5 * 102) / 10, 9);
    expect(f.avgPrice).toBeCloseTo(101.5, 9);
    expect(f.slippage).toBeCloseTo(0.5, 9);
    expect(f.levelsConsumed).toBe(2);
  });

  it("walks the bids downward for a sell", () => {
    const f = walkBook(book, "sell", 10)!;
    expect(f.avgPrice).toBeCloseTo(99.5, 9);
    // Slippage is a cost on both sides: a sell receives LESS than the touch.
    expect(f.slippage).toBeCloseTo(0.5, 9);
  });

  it("flags exhaustion instead of inventing depth past what the venue published", () => {
    const f = walkBook(book, "buy", 100)!;
    expect(f.exhausted).toBe(true);
    expect(f.filledSize).toBe(35);
  });

  it("returns null on an empty side", () => {
    expect(walkBook({ ...book, asks: [] }, "buy", 1)).toBeNull();
  });
});

// ── resting orders ──────────────────────────────────────────────────────────

describe("limitFill — a limit needs price to trade THROUGH it", () => {
  it("does not fill a buy on a mere touch", () => {
    // Touching the level puts you at the back of a queue that may never clear.
    expect(limitFill(bar(105, 106, 100, 104), "buy", 100)).toBeNull();
  });

  it("fills a buy when price trades below the limit", () => {
    expect(limitFill(bar(105, 106, 99.9, 104), "buy", 100)).toBe(100);
  });

  it("fills at the OPEN when the bar gapped below — better than the limit", () => {
    expect(limitFill(bar(98, 99, 97, 98.5), "buy", 100)).toBe(98);
  });

  it("mirrors for a sell", () => {
    expect(limitFill(bar(95, 100, 94, 96), "sell", 100)).toBeNull();
    expect(limitFill(bar(95, 100.1, 94, 96), "sell", 100)).toBe(100);
    expect(limitFill(bar(102, 103, 101, 102.5), "sell", 100)).toBe(102);
  });
});

describe("stopFill — a stop triggers on TOUCH and fills like a market order", () => {
  it("fills a sell stop at the level when price trades to it", () => {
    expect(stopFill(bar(105, 106, 100, 101), "sell", 100)).toBe(100);
  });

  it("fills at the OPEN when price gapped past it — worse than the stop", () => {
    const fill = stopFill(bar(95, 96, 94, 95.5), "sell", 100)!;
    expect(fill).toBe(95);
    expect(fill).toBeLessThan(100);
  });

  it("mirrors for a buy stop", () => {
    expect(stopFill(bar(95, 100, 94, 99), "buy", 100)).toBe(100);
    expect(stopFill(bar(105, 106, 104, 105.5), "buy", 100)).toBe(105);
  });
});

describe("findLevelHit — take-profit and stop-loss on an open position", () => {
  const bars = [bar(100, 101, 99, 100, 10), bar(100, 112, 88, 105, 20)];

  it("assumes the STOP when one bar holds both levels", () => {
    const hit = findLevelHit(bars, "long", 110, 90, 0)!;
    expect(hit.reason).toBe("stop_loss");
    expect(hit.ambiguous).toBe(true);
    expect(hit.at).toBe(20);
  });

  it("takes the target when only the target printed", () => {
    const hit = findLevelHit(bars, "long", 110, 50, 0)!;
    expect(hit.reason).toBe("take_profit");
    expect(hit.ambiguous).toBe(false);
  });

  it("ignores bars before the position existed", () => {
    expect(findLevelHit([bar(100, 200, 50, 100, 5)], "long", 110, 90, 10)).toBeNull();
  });

  it("returns null when the position has no levels attached", () => {
    expect(findLevelHit(bars, "long", null, null, 0)).toBeNull();
  });
});

describe("findLiquidation", () => {
  it("catches a long liquidated on the low", () => {
    const l = findLiquidation([bar(100, 101, 89, 92, 10)], "long", 90, 0)!;
    expect(l.at).toBe(10);
    expect(l.fill).toBe(90);
  });
  it("fills worse than the level when price gapped through it", () => {
    const l = findLiquidation([bar(85, 86, 84, 85, 10)], "long", 90, 0)!;
    expect(l.fill).toBe(85);
  });
  it("is null when there is no liquidation price to reach", () => {
    expect(findLiquidation([bar(1, 2, 0.1, 1)], "long", null, 0)).toBeNull();
  });
});

// ── carry ───────────────────────────────────────────────────────────────────

describe("funding — 8h stamps", () => {
  const H = 3_600;
  it("counts stamps crossed, not hours held", () => {
    expect(fundingSettlements(1 * H, 7 * H)).toBe(0);
    expect(fundingSettlements(7 * H, 9 * H)).toBe(1);
    expect(fundingSettlements(0, 24 * H)).toBe(3);
  });

  it("charges a long and CREDITS a short at a positive rate", () => {
    const p = { side: "long" as const, size: 200, carryTo: 0 };
    const long = accrueCarry(perp(), p, 24 * H, 1, 77_000);
    const short = accrueCarry(perp(), { ...p, side: "short" }, 24 * H, 1, 77_000);
    expect(long.charges).toBe(3);
    expect(long.usd).toBeGreaterThan(0);
    expect(short.usd).toBeCloseTo(-long.usd, 9);
  });
});

describe("swap — daily rollover, tripled on Wednesday", () => {
  const DAY = 86_400;
  // 2026-08-19 is a Wednesday; rollover is 21:00 UTC.
  const wedRollover = Math.floor(Date.parse("2026-08-19T21:00:00Z") / 1000);

  it("charges nothing before the first rollover", () => {
    expect(swapCharges(wedRollover - 6 * 3_600, wedRollover - 3_600)).toBe(0);
  });

  it("charges one on an ordinary rollover", () => {
    const tue = wedRollover - DAY;
    expect(swapCharges(tue - 3_600, tue + 3_600)).toBe(1);
  });

  it("charges THREE across Wednesday's rollover — the weekend value date", () => {
    expect(swapCharges(wedRollover - 3_600, wedRollover + 3_600)).toBe(3);
  });

  it("accumulates across a week", () => {
    // Seven consecutive rollovers, one of which is Wednesday's triple.
    expect(swapCharges(wedRollover - 3 * DAY, wedRollover + 4 * DAY)).toBe(9);
  });

  it("makes a negative swap rate a COST to the position", () => {
    const inst = fx();
    const long = accrueCarry(inst, { side: "long", size: 1, carryTo: wedRollover - 3_600 }, wedRollover + 3_600, 1, 1.1);
    // swapLong is -7.2 points, so the long pays.
    expect(long.usd).toBeGreaterThan(0);
    const short = accrueCarry(inst, { side: "short", size: 1, carryTo: wedRollover - 3_600 }, wedRollover + 3_600, 1, 1.1);
    // swapShort is +2.1 points, so the short is credited.
    expect(short.usd).toBeLessThan(0);
  });
});

// ── venue rules ─────────────────────────────────────────────────────────────

describe("forex market hours — 24/5", () => {
  it("is shut all Saturday", () => {
    expect(marketOpen(new Date("2026-08-22T12:00:00Z"))).toBe(false);
    expect(marketOpen(new Date("2026-08-22T23:00:00Z"))).toBe(false);
  });
  it("opens Sunday at 21:00 UTC", () => {
    expect(marketOpen(new Date("2026-08-23T20:59:00Z"))).toBe(false);
    expect(marketOpen(new Date("2026-08-23T21:30:00Z"))).toBe(true);
  });
  it("closes Friday at 21:00 UTC", () => {
    expect(marketOpen(new Date("2026-08-21T20:00:00Z"))).toBe(true);
    expect(marketOpen(new Date("2026-08-21T21:30:00Z"))).toBe(false);
  });
  it("is open midweek", () => {
    expect(marketOpen(new Date("2026-08-19T03:00:00Z"))).toBe(true);
  });
});

describe("rounding to the venue's grid", () => {
  it("snaps a price to the tick", () => {
    expect(roundToTick(77_300.3, 0.5)).toBe(77_300.5);
    expect(roundToTick(1.123456, 0.00001)).toBeCloseTo(1.12346, 9);
  });
  it("floors a size to the step, never up past what was asked", () => {
    expect(roundSize(0.157, 0.01)).toBeCloseTo(0.15, 9);
    expect(roundSize(3.9, 1)).toBe(3);
    // 0.01 steps accumulate float error that would otherwise show in the UI.
    expect(roundSize(0.03, 0.01)).toBeCloseTo(0.03, 9);
  });
});

describe("pnlUsd direction", () => {
  it("pays a long when price rises and a short when it falls", () => {
    expect(pnlUsd(perp(), "long", 100, 77_000, 78_000, 1)).toBeCloseTo(100 * 0.001 * 1_000, 6);
    expect(pnlUsd(perp(), "short", 100, 77_000, 76_000, 1)).toBeCloseTo(100 * 0.001 * 1_000, 6);
    expect(pnlUsd(perp(), "long", 100, 77_000, 76_000, 1)).toBeLessThan(0);
    expect(pnlUsd(perp(), "short", 100, 77_000, 78_000, 1)).toBeLessThan(0);
  });
});
