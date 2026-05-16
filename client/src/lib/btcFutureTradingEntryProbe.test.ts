import { describe, expect, it } from "vitest";
import { FUTURES_STRAT_DEFS } from "./futuresStrategies";
import { buildPaperDeskStrategies, deskFakeDiversityEnabledViaEnv, deskMinTpSlRatioFromEnv } from "./futuresDeskPolicy";
import {
  buildSignalInputs,
  classifyRegimeTagFrom1mOhlcv,
  evalMinuteSignal,
  passesEntryConfirmation,
} from "./futuresSignals";

const BTC_FUTURE_TRADING_STRATEGY_IDS = [
  91, 92, 95, 96, 111, 112, 117, 118, 123, 124, 125, 126, 131, 132, 133, 134, 139, 140, 151, 152,
];
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

describe("BTC Future Trading 20-strategy entry probe", () => {
  const raw = FUTURES_STRAT_DEFS.filter((s) => BTC_FUTURE_TRADING_STRATEGY_IDS.includes(s.id));
  const built = buildPaperDeskStrategies(raw, {
    strategyIdAllowlist: null,
    minTpSlRatio: deskMinTpSlRatioFromEnv(),
    allowFakeDiversity: true,
  });

  it("explicit roster keeps IDs 91–96 (inside fake-diversity range 79–110)", () => {
    expect(raw.length).toBe(20);
    expect(built.fakeDiversityFilteredCount).toBe(0);
    expect(built.strategies.length).toBe(20);
    expect(built.lowRrSkippedStratIds.length).toBe(0);
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
    expect(passBoth).toBeLessThan(5);
  });
});
