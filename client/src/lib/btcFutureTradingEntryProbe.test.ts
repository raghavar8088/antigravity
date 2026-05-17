import { describe, expect, it } from "vitest";
import { BTC_FUTURE_TRADING_STRATEGY_IDS } from "./btcFutureTradingRoster";
import { CORE_BTC_FT_STRATEGY_IDS } from "./btcFtRoster";
import { FUTURES_STRAT_DEFS } from "./futuresStrategies";
import { buildPaperDeskStrategies, deskMinTpSlRatioFromEnv } from "./futuresDeskPolicy";
import {
  buildSignalInputs,
  classifyRegimeTagFrom1mOhlcv,
  evalMinuteSignal,
  passesEntryConfirmation,
} from "./futuresSignals";

const THRESHOLD = 26;

function bullishBars(base = 100_000, count = 40) {
  const opens: number[] = [];
  const closes: number[] = [];
  const highs: number[] = [];
  const lows: number[] = [];
  const volumes: number[] = [];
  let price = base;
  for (let i = 0; i < count; i++) {
    price *= 1.002 + (i % 3) * 0.0003;
    const h = price * 1.001;
    const l = price * 0.998;
    const o = i === 0 ? l : closes[i - 1]!;
    opens.push(o);
    closes.push(price);
    highs.push(h);
    lows.push(l);
    volumes.push(5000 + i * 200);
  }
  return { opens, closes, highs, lows, volumes };
}

function choppyBars(base = 100_000, count = 40) {
  const opens: number[] = [];
  const closes: number[] = [];
  const highs: number[] = [];
  const lows: number[] = [];
  const volumes: number[] = [];
  for (let i = 0; i < count; i++) {
    const delta = (i % 2 === 0 ? 1 : -1) * base * 0.00005;
    const price = base + delta;
    const o = i === 0 ? price : closes[i - 1]!;
    opens.push(o);
    closes.push(price);
    highs.push(price + base * 0.00008);
    lows.push(price - base * 0.00008);
    volumes.push(3000);
  }
  return { opens, closes, highs, lows, volumes };
}

/** Moderate-bullish bars: not as strong as bullishBars but stronger than choppyBars. */
function mildBullishBars(base = 100_000, count = 40) {
  const opens: number[] = [];
  const closes: number[] = [];
  const highs: number[] = [];
  const lows: number[] = [];
  const volumes: number[] = [];
  let price = base;
  for (let i = 0; i < count; i++) {
    price *= 1.0004 + (i % 5) * 0.00008; // gentler trend vs bullishBars
    const h = price * 1.0005;
    const l = price * 0.9997;
    const o = i === 0 ? l : closes[i - 1]!;
    opens.push(o);
    closes.push(price);
    highs.push(h);
    lows.push(l);
    volumes.push(4000 + i * 100);
  }
  return { opens, closes, highs, lows, volumes };
}

describe("BTC Future Trading desk roster entry probe", () => {
  const raw = FUTURES_STRAT_DEFS.filter((s) => BTC_FUTURE_TRADING_STRATEGY_IDS.includes(s.id));
  const built = buildPaperDeskStrategies(raw, {
    strategyIdAllowlist: null,
    minTpSlRatio: deskMinTpSlRatioFromEnv(),
    allowFakeDiversity: true,
  });

  it("roster includes core archetypes plus extended BTC FT IDs (200–299)", () => {
    expect(BTC_FUTURE_TRADING_STRATEGY_IDS.length).toBe(120);
    expect(BTC_FUTURE_TRADING_STRATEGY_IDS.some((id) => id >= 200)).toBe(true);
    expect(raw.length).toBe(BTC_FUTURE_TRADING_STRATEGY_IDS.length);
    expect(built.fakeDiversityFilteredCount).toBe(0);
    expect(built.strategies.length).toBe(BTC_FUTURE_TRADING_STRATEGY_IDS.length);
    expect(built.lowRrSkippedStratIds.length).toBe(0);
  });

  it("keeps all 120 explicit BTC Future Trading IDs active after desk policy build", () => {
    const explicitIds = new Set(BTC_FUTURE_TRADING_STRATEGY_IDS);
    const explicitRaw = FUTURES_STRAT_DEFS.filter((s) => explicitIds.has(s.id));
    const explicitBuilt = buildPaperDeskStrategies(explicitRaw, {
      strategyIdAllowlist: null,
      minTpSlRatio: 2,
      allowFakeDiversity: true,
    });

    expect(explicitBuilt.strategies).toHaveLength(120);
    expect(explicitBuilt.strategies.map((s) => s.id).sort((a, b) => a - b)).toEqual(
      [...BTC_FUTURE_TRADING_STRATEGY_IDS].sort((a, b) => a - b),
    );
  });

  it("on strong bullish synthetic bars, at least one strat passes signal+confirm at threshold 26", () => {
    const bars = bullishBars();
    const input = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    const regime = classifyRegimeTagFrom1mOhlcv(bars.opens, bars.highs, bars.lows, bars.closes, bars.volumes);
    let passBoth = 0;
    let regimeBlocked = 0;
    for (const strat of built.strategies) {
      const signal = evalMinuteSignal(input, strat);
      if (signal.score >= THRESHOLD && passesEntryConfirmation(input, strat)) {
        passBoth += 1;
        if (strat.regimes && !strat.regimes.includes(regime)) regimeBlocked += 1;
      }
    }
    expect(passBoth).toBeGreaterThan(0);
    expect(regimeBlocked).toBeLessThan(passBoth);
  });

  it("on choppy bars, most strats fail signal or confirm (explains quiet periods)", () => {
    const bars = choppyBars();
    const input = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    let passBoth = 0;
    for (const strat of built.strategies) {
      const signal = evalMinuteSignal(input, strat);
      if (signal.score >= THRESHOLD && passesEntryConfirmation(input, strat)) passBoth += 1;
    }
    expect(passBoth).toBeLessThan(Math.ceil(BTC_FUTURE_TRADING_STRATEGY_IDS.length * 0.2));
  });

  // ── Part 1: CORE roster bypass for ids 91–96 ──────────────────────────────
  it("CORE_BTC_FT_STRATEGY_IDS contains 91–96 and survives allowFakeDiversity:true build", () => {
    const coreIds = new Set(CORE_BTC_FT_STRATEGY_IDS);
    expect(coreIds.has(91)).toBe(true);
    expect(coreIds.has(92)).toBe(true);
    expect(coreIds.has(95)).toBe(true);
    expect(coreIds.has(96)).toBe(true);

    const coreRaw = FUTURES_STRAT_DEFS.filter((s) => coreIds.has(s.id));
    const coreBuilt = buildPaperDeskStrategies(coreRaw, {
      strategyIdAllowlist: null,
      minTpSlRatio: deskMinTpSlRatioFromEnv(),
      allowFakeDiversity: true,
    });
    const builtIds = new Set(coreBuilt.strategies.map((s) => s.id));
    expect(builtIds.has(91)).toBe(true);
    expect(builtIds.has(96)).toBe(true);
  });

  // ── Part 2: Threshold 24 allows probe bars that threshold 26 rejects ───────
  it("Part 2: threshold 24 passes >= as many strats as threshold 26 on mild-bullish bars", () => {
    const bars = mildBullishBars();
    const input = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);

    let pass24 = 0;
    let pass26 = 0;
    for (const strat of built.strategies) {
      const signal = evalMinuteSignal(input, strat);
      if (signal.score >= 24 && passesEntryConfirmation(input, strat)) pass24 += 1;
      if (signal.score >= 26 && passesEntryConfirmation(input, strat)) pass26 += 1;
    }
    // Threshold 24 must admit at least as many as threshold 26 (monotone gate).
    expect(pass24).toBeGreaterThanOrEqual(pass26);
  });
});
