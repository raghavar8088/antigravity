import { describe, expect, it } from "vitest";
import type { BtcFtTemplateId, FuturesStratDef } from "@/lib/futuresStratTypes";
import { BTC_FT_EXTENDED_DEFS_PROOF } from "@/lib/btcFtStrategyTemplates";
import { buildSignalInputs, evalMinuteSignal, passesEntryConfirmation } from "./futuresSignals";

const THRESHOLD = 26;

function stratFirstForTpl(tpl: BtcFtTemplateId): FuturesStratDef {
  const s = BTC_FT_EXTENDED_DEFS_PROOF.find((x) => x.btcFtTemplate === tpl);
  if (!s) throw new Error(`No proof strat for template ${tpl}`);
  return s;
}

/** Clone for opposite side (proof batch may only include one side per template index). */
function stratSameTplSide(base: FuturesStratDef, side: "LONG" | "SHORT"): FuturesStratDef {
  const tpl = base.btcFtTemplate!;
  const variant = base.btcFtVariant ?? 0;
  return {
    ...base,
    signalKey: `BTCFT_${tpl}_${variant}_${side}`,
    name: `${base.name}_TESTFLIP`,
  };
}

function bullishBars(base = 100_000, count = 120) {
  const opens: number[] = [];
  const closes: number[] = [];
  const highs: number[] = [];
  const lows: number[] = [];
  const volumes: number[] = [];
  let price = base;
  for (let i = 0; i < count; i++) {
    price *= 1.0018 + (i % 4) * 0.00025;
    const h = price * 1.0012;
    const l = price * 0.9985;
    const o = i === 0 ? l : closes[i - 1];
    opens.push(o);
    closes.push(price);
    highs.push(h);
    lows.push(l);
    volumes.push(6000 + i * 180);
  }
  return { opens, closes, highs, lows, volumes };
}

function bearishBars(base = 100_000, count = 120) {
  const opens: number[] = [];
  const closes: number[] = [];
  const highs: number[] = [];
  const lows: number[] = [];
  const volumes: number[] = [];
  let price = base;
  for (let i = 0; i < count; i++) {
    price *= 0.9982 - (i % 4) * 0.00025;
    const h = price * 1.0015;
    const l = price * 0.9985;
    const o = i === 0 ? h : closes[i - 1];
    opens.push(o);
    closes.push(price);
    highs.push(h);
    lows.push(l);
    volumes.push(6000 + i * 180);
  }
  return { opens, closes, highs, lows, volumes };
}

/** Sharp dip so price pierces lower Bollinger band (MR long). */
function bbOversoldBars(base = 100_000, count = 120) {
  const opens: number[] = [];
  const closes: number[] = [];
  const highs: number[] = [];
  const lows: number[] = [];
  const volumes: number[] = [];
  let price = base;
  for (let i = 0; i < count - 1; i++) {
    const delta = (i % 2 === 0 ? 1 : -1) * base * 0.00008;
    price = base + delta;
    const o = i === 0 ? price : closes[i - 1];
    opens.push(o);
    closes.push(price);
    highs.push(price + base * 0.00015);
    lows.push(price - base * 0.00015);
    volumes.push(3500);
  }
  const drop = base * 0.035;
  const lastClose = closes[count - 2]! - drop;
  opens.push(closes[count - 2]!);
  closes.push(lastClose);
  lows.push(lastClose - base * 0.0005);
  highs.push(closes[count - 2]!);
  volumes.push(25_000);
  return { opens, closes, highs, lows, volumes };
}

/** Extended rally then blow-off volume spike for flow proxy long. */
function flowBurstLongBars(base = 100_000, count = 120) {
  const b = bullishBars(base, count - 1);
  const prev = b.closes[b.closes.length - 1]!;
  const jump = prev * 0.0045;
  b.opens.push(prev);
  b.closes.push(prev + jump);
  b.highs.push(prev + jump * 1.001);
  b.lows.push(prev * 0.999);
  b.volumes.push(95_000);
  return b;
}

function flowBurstShortBars(base = 100_000, count = 120) {
  const b = bearishBars(base, count - 1);
  const prev = b.closes[b.closes.length - 1]!;
  const drop = prev * 0.0045;
  b.opens.push(prev);
  b.closes.push(prev - drop);
  b.lows.push(prev - drop * 1.001);
  b.highs.push(prev * 1.001);
  b.volumes.push(95_000);
  return b;
}

/** Closes stay high vs VWAP proxy — for VWAP revert SHORT (rich). */
function vwapRichShortBars(base = 100_000, count = 120) {
  const b = bullishBars(base, count - 1);
  const prev = b.closes[b.closes.length - 1]!;
  b.opens.push(prev);
  b.closes.push(prev + base * 0.003);
  b.highs.push(prev + base * 0.0035);
  b.lows.push(prev);
  b.volumes.push(88_000);
  return b;
}

describe("BTC FT template signals (dedicated scorer + confirm)", () => {
  it("MTF_TREND long scores and confirms on sustained rally", () => {
    const bars = bullishBars();
    const strat = stratFirstForTpl("MTF_TREND");
    const t = Date.UTC(2026, 4, 17, 14, 5, 0);
    const s = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!, t);
    const ev = evalMinuteSignal(s, strat);
    expect(ev.score).toBeGreaterThanOrEqual(THRESHOLD);
    expect(passesEntryConfirmation(s, strat)).toBe(true);
  });

  it("MTF_TREND short scores and confirms on sustained selloff (synthetic side)", () => {
    const bars = bearishBars();
    const strat = stratSameTplSide(stratFirstForTpl("MTF_TREND"), "SHORT");
    const t = Date.UTC(2026, 4, 17, 8, 12, 0);
    const s = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!, t);
    const ev = evalMinuteSignal(s, strat);
    expect(ev.score).toBeGreaterThanOrEqual(THRESHOLD);
    expect(passesEntryConfirmation(s, strat)).toBe(true);
  });

  it("MTF_BREAK short scores on breakdown vs rolling lows (proof roster side)", () => {
    const bars = bearishBars();
    const strat = stratFirstForTpl("MTF_BREAK");
    const s = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    const ev = evalMinuteSignal(s, strat);
    expect(ev.score).toBeGreaterThanOrEqual(THRESHOLD);
    expect(passesEntryConfirmation(s, strat)).toBe(true);
  });

  it("MEAN_REVERT_BB long on BB pierce + RSI soft", () => {
    const bars = bbOversoldBars();
    const strat = stratFirstForTpl("MEAN_REVERT_BB");
    const s = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    const ev = evalMinuteSignal(s, strat);
    expect(ev.score).toBeGreaterThanOrEqual(THRESHOLD);
    expect(passesEntryConfirmation(s, strat)).toBe(true);
  });

  it("VWAP_REVERT short when trade rich vs VWAP proxy (proof roster side)", () => {
    const bars = vwapRichShortBars();
    const strat = stratFirstForTpl("VWAP_REVERT");
    const s = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    const ev = evalMinuteSignal(s, strat);
    expect(ev.score).toBeGreaterThanOrEqual(THRESHOLD);
    expect(passesEntryConfirmation(s, strat)).toBe(true);
  });

  it("MOMENTUM_IMPULSE long on ATR-scaled thrust + OBV", () => {
    const bars = flowBurstLongBars();
    const strat = stratFirstForTpl("MOMENTUM_IMPULSE");
    const s = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    const ev = evalMinuteSignal(s, strat);
    expect(ev.score).toBeGreaterThanOrEqual(THRESHOLD);
    expect(passesEntryConfirmation(s, strat)).toBe(true);
  });

  it("SESSION_OPEN requires UTC liquidity hour when lastBarTimeMs set (proof SHORT)", () => {
    const bars = flowBurstShortBars();
    const strat = stratFirstForTpl("SESSION_OPEN");
    const nyMs = Date.UTC(2026, 4, 17, 14, 4, 0);
    const sOk = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!, nyMs);
    expect(evalMinuteSignal(sOk, strat).score).toBeGreaterThanOrEqual(THRESHOLD);
    expect(passesEntryConfirmation(sOk, strat)).toBe(true);

    const quietMs = Date.UTC(2026, 4, 17, 3, 0, 0);
    const sQuiet = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!, quietMs);
    expect(passesEntryConfirmation(sQuiet, strat)).toBe(false);
  });

  it("WYCKOFF_TRAP long: spring-like CCI + mid structure", () => {
    const bars = bbOversoldBars();
    const strat = stratFirstForTpl("WYCKOFF_TRAP");
    const s = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    const ev = evalMinuteSignal(s, strat);
    expect(ev.score).toBeGreaterThanOrEqual(THRESHOLD);
    expect(passesEntryConfirmation(s, strat)).toBe(true);
  });

  it("ORDERFLOW_PROXY short on volume thrust (proof roster side)", () => {
    const bars = flowBurstShortBars();
    const strat = stratFirstForTpl("ORDERFLOW_PROXY");
    const s = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    const ev = evalMinuteSignal(s, strat);
    expect(ev.score).toBeGreaterThanOrEqual(THRESHOLD);
    expect(passesEntryConfirmation(s, strat)).toBe(true);
  });

  it("MTF_TREND long does not confirm on bearish tape", () => {
    const bars = bearishBars();
    const strat = stratFirstForTpl("MTF_TREND");
    const s = buildSignalInputs(bars.opens, bars.closes, bars.highs, bars.lows, bars.volumes, bars.closes.at(-1)!);
    expect(passesEntryConfirmation(s, strat)).toBe(false);
  });
});
