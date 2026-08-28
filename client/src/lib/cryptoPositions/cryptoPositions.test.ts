import { describe, expect, it } from "vitest";
import {
  initialMarginUsd,
  legPremium,
  liquidationFor,
  marginFor,
  maxLeverageFor,
  scanGrid,
  SCAN_RANGE_PCT,
  type MarginLeg,
} from "./margin";
import { displayNameOf, feeFor, formatExpiry, type Instrument } from "./types";

/**
 * The margin model and the fee model are where this desk makes claims that
 * cannot be read off the venue. Everything here pins a decision that would
 * otherwise be silently wrong: whether a hedge actually reduces the
 * requirement, whether a bought option is charged twice, and whether a cheap
 * option is priced out of existence by a fee on notional.
 */

const SPOT = 80_000;

function option(
  optionType: "CALL" | "PUT",
  strike: number,
  mark: number,
  greeks: Partial<Instrument["greeks"]> & { delta: number; gamma: number },
): Instrument {
  return {
    symbol: `${optionType === "CALL" ? "C" : "P"}-BTC-${strike}-010127`,
    kind: "OPTION",
    underlying: "BTC",
    expiry: "2027-01-01",
    strike,
    optionType,
    // 0.001 BTC per contract, as Delta actually lists them.
    contractValue: 0.001,
    tickSize: 0.1,
    markPrice: mark,
    bid: mark * 0.99,
    ask: mark * 1.01,
    spot: SPOT,
    openInterest: 100,
    turnoverUsd: 1000,
    change24hPct: 0,
    ivPct: 50,
    greeks: { delta: greeks.delta, gamma: greeks.gamma, theta: -1, vega: 1, rho: 0 },
    fundingRatePct8h: null,
    // Delta's real published parameters for BTC contracts.
    initialMarginPct: 0.5,
    maintenanceMarginPct: 0.25,
    imScalingFactor: 0.000002,
    mmScalingFactor: 0.000001,
    defaultLeverage: 200,
    maxLeverage: 200,
    penaltyFactor: 0.5,
  };
}

function perp(mark: number): Instrument {
  return {
    ...option("CALL", 0, mark, { delta: 1, gamma: 0 }),
    symbol: "BTCUSD",
    kind: "PERPETUAL",
    expiry: null,
    strike: null,
    optionType: null,
    greeks: null,
    ivPct: null,
    fundingRatePct8h: 0.01,
    spot: mark,
  };
}

function leg(i: Instrument, side: "BUY" | "SELL", lots = 1, leverage = 1): MarginLeg {
  return {
    symbol: i.symbol,
    underlying: i.underlying,
    side,
    lots,
    contractValue: i.contractValue,
    price: i.markPrice,
    instrument: i,
    leverage,
  };
}

describe("scan grid", () => {
  it("evaluates the unshocked book and spans the full range both ways", () => {
    const g = scanGrid();
    expect(g).toContain(0);
    expect(Math.min(...g)).toBeCloseTo(-SCAN_RANGE_PCT / 100, 10);
    expect(Math.max(...g)).toBeCloseTo(SCAN_RANGE_PCT / 100, 10);
  });
});

describe("premium uses the contract multiplier", () => {
  it("prices one contract as mark x contractValue, not mark", () => {
    // Delta quotes per unit of underlying. A $3,000 call on a 0.001 BTC
    // contract costs $3, and reading the quote as the per-contract price would
    // overstate it a thousandfold.
    const call = option("CALL", 80_000, 3_000, { delta: 0.5, gamma: 0.00002 });
    expect(legPremium(leg(call, "BUY"))).toBeCloseTo(-3, 10);
    expect(legPremium(leg(call, "SELL"))).toBeCloseTo(3, 10);
  });
});

describe("margin", () => {
  it("charges a bought option nothing beyond the premium it already paid", () => {
    // The debit has left the account as cash. Charging margin as well would
    // count the same money twice and make a simple long option look twice as
    // expensive as it is.
    const call = option("CALL", 82_000, 2_000, { delta: 0.4, gamma: 0.00002 });
    const m = marginFor([leg(call, "BUY")]);
    expect(m.marginRequired).toBeCloseTo(0, 6);
    expect(m.netPremium).toBeLessThan(0);
  });

  it("charges a naked short option real margin", () => {
    const call = option("CALL", 82_000, 2_000, { delta: 0.4, gamma: 0.00002 });
    const m = marginFor([leg(call, "SELL")]);
    expect(m.marginRequired).toBeGreaterThan(0);
    // Far more than the premium collected — that is the whole point of margin.
    expect(m.marginRequired).toBeGreaterThan(m.netPremium);
  });

  it("gives a vertical spread a benefit over its naked short leg", () => {
    // The behaviour the Indian desk advertises: a bought option that caps a
    // sold option's risk lowers the margin blocked. A flat per-leg percentage
    // cannot express this, which is why margin is priced by scenario.
    const shortCall = option("CALL", 82_000, 2_000, { delta: 0.4, gamma: 0.00002 });
    const longCall = option("CALL", 86_000, 900, { delta: 0.22, gamma: 0.000015 });

    const naked = marginFor([leg(shortCall, "SELL")]);
    const spread = marginFor([leg(shortCall, "SELL"), leg(longCall, "BUY")]);

    expect(spread.marginRequired).toBeLessThan(naked.marginRequired);
    expect(spread.marginBenefit).toBeGreaterThan(0);
  });

  it("does not net a BTC option against an ETH option", () => {
    // Shocks are applied per underlying. Correlating them would hand out a
    // diversification benefit this desk has not measured.
    const btc = option("CALL", 82_000, 2_000, { delta: 0.4, gamma: 0.00002 });
    const eth: Instrument = { ...option("PUT", 3_000, 100, { delta: -0.4, gamma: 0.0002 }), underlying: "ETH", spot: 3_000 };

    const together = marginFor([leg(btc, "SELL"), leg(eth, "SELL")]);
    const apart = marginFor([leg(btc, "SELL")]).marginRequired + marginFor([leg(eth, "SELL")]).marginRequired;
    expect(together.marginRequired).toBeCloseTo(apart, 6);
  });

  it("never prices an option below worthless under a shock", () => {
    // The quadratic term turns negative far from the money; unfloored, a short
    // would appear to make more than the premium it collected.
    const deepOtm = option("CALL", 120_000, 5, { delta: 0.01, gamma: 0.0000001 });
    const m = marginFor([leg(deepOtm, "SELL")]);
    expect(m.marginRequired).toBeGreaterThanOrEqual(0);
    expect(Number.isFinite(m.marginRequired)).toBe(true);
  });

  it("reports no benefit when there is nothing to offset", () => {
    const a = option("CALL", 82_000, 2_000, { delta: 0.4, gamma: 0.00002 });
    const m = marginFor([leg(a, "SELL")]);
    expect(m.marginBenefit).toBe(0);
  });
});

describe("fees", () => {
  it("caps an option fee at a fraction of the premium", () => {
    // 0.03% of an $80,000 notional is $24. Charging that to buy a 50-cent
    // option would make every far-OTM contract a loser on fees alone, which is
    // why the venue caps it and why this desk does too.
    const qty = 0.001;
    const premium = 0.5 * qty;
    const fee = feeFor("OPTION", qty, 0.5, SPOT);
    expect(fee).toBeCloseTo(premium * 0.1, 10);
    expect(fee).toBeLessThan(qty * SPOT * 0.0003);
  });

  it("charges notional when the premium is large enough not to bind", () => {
    const qty = 0.001;
    const fee = feeFor("OPTION", qty, 20_000, SPOT);
    expect(fee).toBeCloseTo(qty * SPOT * 0.0003, 10);
  });

  it("charges a perpetual on notional", () => {
    expect(feeFor("PERPETUAL", 1, 80_000, 80_000)).toBeCloseTo(80_000 * 0.0005, 10);
  });
});

describe("rolling to the money", () => {
  it("costs more margin as a half-rolled pair than as either whole pair", () => {
    // This is the reason a roll is one operation per expiry group rather than a
    // loop over the rows. A short straddle rolled a leg at a time passes
    // through a state where the two shorts sit at DIFFERENT strikes, and that
    // intermediate book is wider than either the old pair or the new one. On a
    // tight account it can be refused, stranding the position half-rolled —
    // worse than where it started.
    const oldCall = option("CALL", 78_000, 1_500, { delta: 0.5, gamma: 0.00002 });
    const oldPut = option("PUT", 78_000, 1_400, { delta: -0.5, gamma: 0.00002 });
    const newCall = option("CALL", 80_000, 900, { delta: 0.38, gamma: 0.000022 });
    const newPut = option("PUT", 80_000, 2_100, { delta: -0.62, gamma: 0.000022 });

    const before = marginFor([leg(oldCall, "SELL"), leg(oldPut, "SELL")]).marginRequired;
    const after = marginFor([leg(newCall, "SELL"), leg(newPut, "SELL")]).marginRequired;
    // The half-rolled state: old put still on, call already moved.
    const halfway = marginFor([leg(newCall, "SELL"), leg(oldPut, "SELL")]).marginRequired;

    expect(halfway).toBeGreaterThan(Math.min(before, after));
  });

  it("prices a straddle below the sum of its naked legs", () => {
    // Not a hedge in the offsetting sense — both legs are short — but the two
    // cannot lose at the same time, and the scenario scan is what sees that.
    const call = option("CALL", 80_000, 1_500, { delta: 0.5, gamma: 0.00002 });
    const put = option("PUT", 80_000, 1_400, { delta: -0.5, gamma: 0.00002 });
    const both = marginFor([leg(call, "SELL"), leg(put, "SELL")]);
    const apart =
      marginFor([leg(call, "SELL")]).marginRequired + marginFor([leg(put, "SELL")]).marginRequired;
    expect(both.marginRequired).toBeLessThan(apart);
    expect(both.marginBenefit).toBeGreaterThan(0);
  });
});

describe("leverage", () => {
  it("takes its ceiling from the venue's own initial margin, not from us", () => {
    // 0.5% initial margin is 200x AT ZERO SIZE, and Delta scales the rate up
    // with notional, so the real ceiling at $1,000 is 100 / (0.5 + 2e-6*1000)
    // = 199x. The scaling is the point: a ceiling of a flat 200 would let the
    // desk offer leverage Delta would refuse on anything but a dust position.
    const p = perp(80_000);
    expect(maxLeverageFor(p, 0)).toBe(200);
    expect(maxLeverageFor(p, 1_000)).toBe(199);
  });

  it("lowers the ceiling as the position grows, the way Delta scales it", () => {
    const p = perp(80_000);
    const small = maxLeverageFor(p, 1_000);
    const huge = maxLeverageFor(p, 50_000_000);
    expect(huge).toBeLessThan(small);
  });

  it("never posts less than the venue's floor however much leverage is asked for", () => {
    // Asking 500x on a contract margined at 0.5% must not silently succeed —
    // it would understate margin and move the liquidation price.
    const p = perp(80_000);
    const notional = 8_000;
    // The floor is the SCALED rate, not the headline 0.5%: at $8,000 that is
    // 0.5 + 2e-6*8000 = 0.516%.
    const floor = (notional * (0.5 + 0.000002 * notional)) / 100;
    expect(initialMarginUsd(p, notional, 500)).toBeCloseTo(floor, 6);
    expect(floor).toBeGreaterThan((notional * 0.5) / 100);
    // Below the ceiling the asked-for leverage is simply honoured.
    expect(initialMarginUsd(p, notional, 10)).toBeCloseTo(notional / 10, 6);
  });

  it("charges a perpetual by leverage, so the setting actually does something", () => {
    const p = perp(80_000);
    const at2 = marginFor([leg(p, "BUY", 100, 2)]).marginRequired;
    const at20 = marginFor([leg(p, "BUY", 100, 20)]).marginRequired;
    expect(at20).toBeLessThan(at2);
    expect(at2 / at20).toBeCloseTo(10, 1);
  });
});

describe("liquidation", () => {
  it("returns nothing for a bought option, because there is nothing to liquidate", () => {
    // The premium is paid in full: no borrow, no margin call, no price at which
    // the venue takes it away. A number here would invent a risk.
    const call = option("CALL", 80_000, 2_000, { delta: 0.5, gamma: 0.00002 });
    expect(liquidationFor(call, "BUY", 1, 2_000, 10, 2_000)).toBeNull();
  });

  it("puts a long perp's liquidation below entry and a short's above", () => {
    const p = perp(80_000);
    const long = liquidationFor(p, "BUY", 10, 80_000, 10, 80_000);
    const short = liquidationFor(p, "SELL", 10, 80_000, 10, 80_000);
    expect(long!.price).toBeLessThan(80_000);
    expect(short!.price).toBeGreaterThan(80_000);
  });

  it("moves liquidation closer as leverage rises", () => {
    const p = perp(80_000);
    const at2 = liquidationFor(p, "BUY", 10, 80_000, 2, 80_000)!;
    const at50 = liquidationFor(p, "BUY", 10, 80_000, 50, 80_000)!;
    expect(Math.abs(at50.distancePct)).toBeLessThan(Math.abs(at2.distancePct));
    // 10x is roughly a 10% adverse move, less the maintenance floor.
    const at10 = liquidationFor(p, "BUY", 10, 80_000, 10, 80_000)!;
    expect(Math.abs(at10.distancePct)).toBeGreaterThan(9);
    expect(Math.abs(at10.distancePct)).toBeLessThan(10);
  });

  it("puts bankruptcy beyond liquidation, never before it", () => {
    // Liquidation is where the maintenance floor is breached; bankruptcy is
    // where the margin is gone entirely. A long must reach liquidation first.
    const p = perp(80_000);
    const long = liquidationFor(p, "BUY", 10, 80_000, 10, 80_000)!;
    expect(long.bankruptcyPrice).toBeLessThan(long.price);
    const short = liquidationFor(p, "SELL", 10, 80_000, 10, 80_000)!;
    expect(short.bankruptcyPrice).toBeGreaterThan(short.price);
  });
});

describe("labels", () => {
  it("formats an expiry the way the venue writes it", () => {
    expect(formatExpiry("2026-09-26")).toBe("26SEP26");
  });

  it("names an option and a perpetual differently", () => {
    expect(
      displayNameOf({ underlying: "BTC", kind: "OPTION", expiry: "2026-09-26", strike: 80_000, optionType: "CALL", symbol: "C-BTC-80000-260926" }),
    ).toBe("BTC 26SEP26 80000 CALL");
    expect(
      displayNameOf({ underlying: "BTC", kind: "PERPETUAL", expiry: null, strike: null, optionType: null, symbol: "BTCUSD" }),
    ).toBe("BTCUSD PERP");
  });
});
