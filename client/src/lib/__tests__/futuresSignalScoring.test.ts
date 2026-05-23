/**
 * Per-category signal scoring tests for the 160-strategy research pool.
 *
 * Strategy: build synthetic OHLCV that creates the indicator condition
 * the strategy is designed to detect → expect score above threshold +
 * confirmation gate passes. Mirror with opposing OHLCV → expect blocked.
 */

import { describe, expect, it } from "vitest";
import { buildSignalInputs, evalMinuteSignal, passesEntryConfirmation } from "../futuresSignals";
import { scoreCategoryStrategy, scoreScalping } from "../futuresSignalScoring";
import { SCALPING_STRATEGIES } from "../futuresCategoryStrategies";
import type { FuturesStratDef } from "../futuresStratTypes";

// ─── Bar fixtures ─────────────────────────────────────────────────────────────

function bullishBars(base = 100_000, count = 40) {
  const opens: number[] = [], closes: number[] = [], highs: number[] = [], lows: number[] = [], volumes: number[] = [];
  let price = base;
  for (let i = 0; i < count; i++) {
    price *= 1.002 + (i % 3) * 0.0003;
    const h = price * 1.001, l = price * 0.998;
    opens.push(i === 0 ? l : closes[i - 1]);
    closes.push(price); highs.push(h); lows.push(l);
    volumes.push(5000 + i * 200);
  }
  return { opens, closes, highs, lows, volumes };
}

function bearishBars(base = 100_000, count = 40) {
  const opens: number[] = [], closes: number[] = [], highs: number[] = [], lows: number[] = [], volumes: number[] = [];
  let price = base;
  for (let i = 0; i < count; i++) {
    price *= 0.998 - (i % 3) * 0.0003;
    const h = price * 1.002, l = price * 0.999;
    opens.push(i === 0 ? h : closes[i - 1]);
    closes.push(price); highs.push(h); lows.push(l);
    volumes.push(5000 + i * 200);
  }
  return { opens, closes, highs, lows, volumes };
}

function choppyBars(base = 100_000, count = 40) {
  const opens: number[] = [], closes: number[] = [], highs: number[] = [], lows: number[] = [], volumes: number[] = [];
  for (let i = 0; i < count; i++) {
    const delta = (i % 2 === 0 ? 1 : -1) * base * 0.00005;
    const price = base + delta;
    opens.push(i === 0 ? price : closes[i - 1]);
    closes.push(price);
    highs.push(price * 1.0002); lows.push(price * 0.9998);
    volumes.push(4000);
  }
  return { opens, closes, highs, lows, volumes };
}

function findStrat(id: number): FuturesStratDef {
  const s = SCALPING_STRATEGIES.find((d) => d.id === id);
  if (!s) throw new Error(`strat ${id} missing`);
  return s;
}

// ─── Scoring tests ───────────────────────────────────────────────────────────

describe("scoreScalping — dispatch + structure", () => {
  it("returns no_signal for unknown SCP key", () => {
    const bars = bullishBars();
    const inputs = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    const fake: FuturesStratDef = {
      id: 999, name: "fake", category: "Scalping", signalKey: "SCP_NOPE_LONG",
      slPct: 0.4, tpPct: 1.2, holdMinutes: 20, cooldownMin: 5, confluenceMin: 5,
      tradingCategory: "scalping", researchOnly: true,
    };
    const r = scoreScalping(inputs, fake);
    expect(r.score).toBe(0);
    expect(r.reason).toBe("no_signal");
  });

  it("scoreCategoryStrategy dispatches non-SCP_ to category stub (score=0)", () => {
    const bars = bullishBars();
    const inputs = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    const fake: FuturesStratDef = {
      id: 1001, name: "fake_day", category: "Day Trading", signalKey: "DAY_VWAP_TREND_LONG",
      slPct: 0.6, tpPct: 1.8, holdMinutes: 180, cooldownMin: 15, confluenceMin: 5,
      tradingCategory: "day_trading", researchOnly: true,
    };
    const r = scoreCategoryStrategy(inputs, fake);
    expect(r.score).toBe(0);
  });
});

describe("SCP_EMA_CROSS_LONG (600)", () => {
  const strat = findStrat(600);

  it("fires on bullish bars (EMA up + momentum + volume)", () => {
    const bars = bullishBars();
    const s = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    const r = scoreScalping(s, strat);
    expect(r.score).toBeGreaterThan(10);
  });

  it("does not fire on bearish bars", () => {
    const bars = bearishBars();
    const s = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    const r = scoreScalping(s, strat);
    expect(r.score).toBeLessThan(10);
  });
});

describe("SCP_EMA_CROSS_SHORT (601)", () => {
  const strat = findStrat(601);

  it("fires on bearish bars", () => {
    const bars = bearishBars();
    const s = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    const r = scoreScalping(s, strat);
    expect(r.score).toBeGreaterThan(10);
  });

  it("does not fire on bullish bars", () => {
    const bars = bullishBars();
    const s = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    const r = scoreScalping(s, strat);
    expect(r.score).toBeLessThan(10);
  });
});

describe("SCP_RSI_SNAP — extreme RSI", () => {
  it("LONG (604) fires when RSI is low (bearish stretch)", () => {
    const bars = bearishBars();
    const s = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    const r = scoreScalping(s, findStrat(604));
    // bearishBars drives RSI low → snap-back-long should score
    expect(r.score).toBeGreaterThanOrEqual(10);
  });

  it("SHORT (605) fires when RSI is high (bullish stretch)", () => {
    const bars = bullishBars();
    const s = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    const r = scoreScalping(s, findStrat(605));
    expect(r.score).toBeGreaterThanOrEqual(10);
  });
});

describe("SCP_MICRO_BREAK — structural break", () => {
  it("LONG (608) fires on strong uptrend that breaks 20-bar high", () => {
    const bars = bullishBars();
    const s = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    const r = scoreScalping(s, findStrat(608));
    expect(r.score).toBeGreaterThan(10);
  });
});

describe("SCP_MOM_TICK — momentum surge", () => {
  it("LONG (612) fires on bullish momentum", () => {
    const bars = bullishBars();
    const s = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    const r = scoreScalping(s, findStrat(612));
    expect(r.score).toBeGreaterThanOrEqual(8);
  });

  it("SHORT (613) does not fire on bullish bars", () => {
    const bars = bullishBars();
    const s = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    const r = scoreScalping(s, findStrat(613));
    expect(r.score).toBeLessThan(10);
  });
});

describe("Chop conditions — most strategies should stay quiet", () => {
  it("EMA cross longs score low in chop", () => {
    const bars = choppyBars();
    const s = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    const r = scoreScalping(s, findStrat(600));
    expect(r.score).toBeLessThanOrEqual(20);
  });

  it("Momentum tick stays quiet in chop", () => {
    const bars = choppyBars();
    const s = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    const r = scoreScalping(s, findStrat(612));
    expect(r.score).toBeLessThanOrEqual(15);
  });
});

describe("evalMinuteSignal dispatch via researchOnly flag", () => {
  it("routes SCP_ strats through scoreCategoryStrategy", () => {
    const bars = bullishBars();
    const s = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    const r = evalMinuteSignal(s, findStrat(600));
    expect(r.score).toBeGreaterThan(0);
  });

  it("returns score=0 for researchOnly with unknown signalKey prefix", () => {
    const bars = bullishBars();
    const s = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    const fake: FuturesStratDef = {
      id: 99999, name: "ghost", category: "Mystery", signalKey: "XXX_NOPE_LONG",
      slPct: 0.5, tpPct: 1.5, holdMinutes: 30, cooldownMin: 5, confluenceMin: 5,
      tradingCategory: "scalping", researchOnly: true,
    };
    const r = evalMinuteSignal(s, fake);
    expect(r.score).toBe(0);
  });
});

describe("passesEntryConfirmation — scalping gates", () => {
  it("passes on healthy bullish bars (positive volRatio, ATR)", () => {
    const bars = bullishBars();
    const s = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    expect(passesEntryConfirmation(s, findStrat(600))).toBe(true);
  });

  it("scalping rejects when ATR is zero (degenerate bars)", () => {
    const flat = Array.from({ length: 30 }, () => 100_000);
    const vol = Array.from({ length: 30 }, () => 0);
    const s = buildSignalInputs(flat, flat, flat, flat, vol, 100_000);
    // Should fail volRatio gate
    expect(passesEntryConfirmation(s, findStrat(600))).toBe(false);
  });
});
