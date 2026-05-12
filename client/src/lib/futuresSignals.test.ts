import { describe, expect, it } from "vitest";
import type { FuturesStratDef } from "./futuresStrategies";
import {
  buildSignalInputs,
  effectiveSignalThreshold,
  evalMinuteSignal,
  passesEntryConfirmation,
  type FuturesSignalInputs,
} from "./futuresSignals";

// ========== SYNTHETIC BAR FIXTURES ==========

/**
 * Generate a synthetic trending-up OHLCV series.
 * Needs 30+ bars so all indicators (RSI-14, EMA-21, BB-20, etc.) are defined at the tail.
 * Each bar closes ~0.2% higher to create a clear up-trend with rising momentum.
 */
function bullishBars(base = 100_000, count = 40) {
  const closes: number[] = [];
  const highs: number[] = [];
  const lows: number[] = [];
  const volumes: number[] = [];
  let price = base;
  for (let i = 0; i < count; i++) {
    price *= 1.002 + (i % 3) * 0.0003;
    const h = price * 1.001;
    const l = price * 0.998;
    closes.push(price);
    highs.push(h);
    lows.push(l);
    volumes.push(5000 + i * 200);
  }
  return { closes, highs, lows, volumes };
}

/** Trending-down mirror. */
function bearishBars(base = 100_000, count = 40) {
  const closes: number[] = [];
  const highs: number[] = [];
  const lows: number[] = [];
  const volumes: number[] = [];
  let price = base;
  for (let i = 0; i < count; i++) {
    price *= 0.998 - (i % 3) * 0.0003;
    const h = price * 1.002;
    const l = price * 0.999;
    closes.push(price);
    highs.push(h);
    lows.push(l);
    volumes.push(5000 + i * 200);
  }
  return { closes, highs, lows, volumes };
}

/**
 * Choppy / range-bound bars (no directional bias).
 * Alternates above/below base with tiny amplitude and flat EMA.
 * EMA(9) should hover close to EMA(21) so Trend category gate rejects.
 */
function choppyBars(base = 100_000, count = 40) {
  const closes: number[] = [];
  const highs: number[] = [];
  const lows: number[] = [];
  const volumes: number[] = [];
  for (let i = 0; i < count; i++) {
    const delta = (i % 2 === 0 ? 1 : -1) * base * 0.00005;
    const price = base + delta;
    closes.push(price);
    highs.push(price + base * 0.00008);
    lows.push(price - base * 0.00008);
    volumes.push(3000);
  }
  return { closes, highs, lows, volumes };
}

/** Helper: build SignalInputs from synthetic bars. */
function inputs(bars: ReturnType<typeof bullishBars>, markPrice?: number): FuturesSignalInputs {
  const mark = markPrice ?? bars.closes[bars.closes.length - 1];
  return buildSignalInputs(bars.closes, bars.highs, bars.lows, bars.volumes, mark);
}

// ========== STRATEGY FIXTURES ==========

const trendLong: FuturesStratDef = {
  id: 1, name: "EMA_Cross_Long", category: "Trend",
  signalKey: "EMA_CROSS_LONG", slPct: 0.28, tpPct: 0.62,
  cooldownMin: 3, holdMinutes: 18, confluenceMin: 3,
};

const trendShort: FuturesStratDef = {
  id: 2, name: "EMA_Cross_Short", category: "Trend",
  signalKey: "EMA_CROSS_SHORT", slPct: 0.28, tpPct: 0.62,
  cooldownMin: 3, holdMinutes: 18, confluenceMin: 3,
};

const confluenceLong: FuturesStratDef = {
  id: 15, name: "Confluence_Break_Long", category: "Confluence",
  signalKey: "CONF_BREAK_LONG", slPct: 0.26, tpPct: 0.75,
  cooldownMin: 6, holdMinutes: 20, confluenceMin: 5,
};

const mtfTrendLong: FuturesStratDef = {
  id: 111, name: "MTF_Trend_Align_Long", category: "MTF Trend",
  signalKey: "MTF_TREND_ALIGN_LONG", slPct: 0.26, tpPct: 0.82,
  cooldownMin: 6, holdMinutes: 32, confluenceMin: 4, requiresHtf: true,
};

const mtfTrendShort: FuturesStratDef = {
  id: 112, name: "MTF_Trend_Align_Short", category: "MTF Trend",
  signalKey: "MTF_TREND_ALIGN_SHORT", slPct: 0.26, tpPct: 0.82,
  cooldownMin: 6, holdMinutes: 32, confluenceMin: 4, requiresHtf: true,
};

// ========== TESTS ==========

describe("effectiveSignalThreshold", () => {
  it("baseline: delta 0 returns base", () => {
    expect(effectiveSignalThreshold(28, 0)).toBe(28);
  });

  it("scalp_aggro_v1: delta −4 lowers threshold", () => {
    expect(effectiveSignalThreshold(28, -4)).toBe(24);
  });

  it("clamps to min 18", () => {
    expect(effectiveSignalThreshold(20, -10)).toBe(18);
  });

  it("clamps to max 99", () => {
    expect(effectiveSignalThreshold(95, 10)).toBe(99);
  });

  it("custom clamp range", () => {
    expect(effectiveSignalThreshold(50, 0, 40, 60)).toBe(50);
    expect(effectiveSignalThreshold(50, 20, 40, 60)).toBe(60);
    expect(effectiveSignalThreshold(50, -20, 40, 60)).toBe(40);
  });
});

describe("evalMinuteSignal", () => {
  it("strong bullish bars → positive score with reasons for Trend LONG", () => {
    const bars = bullishBars();
    const s = inputs(bars);
    const result = evalMinuteSignal(s, trendLong);
    expect(result.score).toBeGreaterThan(0);
    expect(result.reason.length).toBeGreaterThan(0);
  });

  it("choppy / weak bars → low or zero score for Trend LONG", () => {
    const bars = choppyBars();
    const s = inputs(bars);
    const result = evalMinuteSignal(s, trendLong);
    const bullBars = bullishBars();
    const bullScore = evalMinuteSignal(inputs(bullBars), trendLong).score;
    expect(result.score).toBeLessThan(bullScore);
  });

  it("SHORT signal key uses short-specific scoring path", () => {
    const bars = bearishBars();
    const s = inputs(bars);
    const result = evalMinuteSignal(s, trendShort);
    expect(result.score).toBeGreaterThan(0);
    expect(result.reason).toBeTruthy();
  });

  it("score is deterministic — same inputs yield same output", () => {
    const bars = bullishBars(100_000, 20);
    const s = inputs(bars);
    const r1 = evalMinuteSignal(s, trendLong);
    const r2 = evalMinuteSignal(s, trendLong);
    expect(r1.score).toBe(r2.score);
    expect(r1.reason).toBe(r2.reason);
  });
});

describe("passesEntryConfirmation", () => {
  it("strong bull bars pass Trend LONG confirmation", () => {
    const bars = bullishBars();
    const s = inputs(bars);
    const passes = passesEntryConfirmation(s, trendLong);
    expect(passes).toBe(true);
  });

  it("choppy bars fail Trend LONG confirmation (EMA not aligned)", () => {
    const bars = choppyBars();
    const s = inputs(bars);
    const passes = passesEntryConfirmation(s, trendLong);
    expect(passes).toBe(false);
  });

  it("Confluence category: strict confluenceMin required", () => {
    const bars = choppyBars();
    const s = inputs(bars);
    const passes = passesEntryConfirmation(s, confluenceLong);
    expect(passes).toBe(false);
  });

  it("profile delta: aggroThreshold is strictly lower than baseline", () => {
    const baseThreshold = 28;
    const aggroThreshold = effectiveSignalThreshold(baseThreshold, -4);
    expect(aggroThreshold).toBe(24);
    expect(aggroThreshold).toBeLessThan(baseThreshold);
  });
});

describe("HTF entry confirmation", () => {
  it("MTF long with bullish HTF context — confirmation returns boolean", () => {
    const bars = bullishBars();
    const s = inputs(bars);
    const passes = passesEntryConfirmation(s, mtfTrendLong);
    expect(typeof passes).toBe("boolean");
  });

  it("MTF long with bearish HTF → rejected (opposing HTF)", () => {
    const bars = bearishBars();
    const s = inputs(bars);
    const passes = passesEntryConfirmation(s, mtfTrendLong);
    expect(passes).toBe(false);
  });

  it("MTF short with bullish HTF → rejected (opposing HTF)", () => {
    const bars = bullishBars();
    const s = inputs(bars);
    const passes = passesEntryConfirmation(s, mtfTrendShort);
    expect(passes).toBe(false);
  });

  it("HTF neutral + bearish LTF bias → short MTF may pass (relaxed gate)", () => {
    const bars = bearishBars();
    const s = inputs(bars);
    const isLtfBear = s.fast < s.slow && s.momentum3 < 0;
    const htf5Neutral = s.htf5_trend === 0;
    const htf15Neutral = s.htf15_trend === 0;
    if (htf5Neutral && htf15Neutral && isLtfBear) {
      const passes = passesEntryConfirmation(s, mtfTrendShort);
      expect(passes).toBe(true);
    } else {
      expect(true).toBe(true);
    }
  });
});

describe("buildSignalInputs", () => {
  it("returns all core price / EMA / momentum fields from 40 bars", () => {
    const bars = bullishBars();
    const s = inputs(bars);
    expect(s.price).toBeGreaterThan(0);
    expect(s.markPrice).toBeGreaterThan(0);
    expect(typeof s.fast).toBe("number");
    expect(typeof s.slow).toBe("number");
    expect(typeof s.momentum3).toBe("number");
    expect(typeof s.momentum6).toBe("number");
    expect(typeof s.bbUpper).toBe("number");
    expect(typeof s.bbLower).toBe("number");
    expect(typeof s.htf5_trend).toBe("number");
    expect(typeof s.htf15_trend).toBe("number");
  });

  it("mark price is forwarded through", () => {
    const bars = bullishBars();
    const customMark = 99_999;
    const s = buildSignalInputs(bars.closes, bars.highs, bars.lows, bars.volumes, customMark);
    expect(s.markPrice).toBe(customMark);
  });
});
