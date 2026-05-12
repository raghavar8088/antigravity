import { describe, expect, it } from "vitest";
import {
  paperLiquidationCrossed,
  paperLiquidationPrice,
  paperNetPnlOnClose,
  paperResolveHardExit,
  paperRoundTripTakerFees,
} from "./futuresPaperMath";

describe("paperLiquidationPrice + paperLiquidationCrossed", () => {
  it("long: modeled liq below entry; crossed when mark at or below liq", () => {
    const entry = 100_000;
    const liq = paperLiquidationPrice(entry, "LONG", 25);
    expect(liq).toBeLessThan(entry);
    expect(paperLiquidationCrossed("LONG", liq - 1, liq)).toBe(true);
    expect(paperLiquidationCrossed("LONG", liq, liq)).toBe(true);
    expect(paperLiquidationCrossed("LONG", liq + 1, liq)).toBe(false);
  });

  it("short: modeled liq above entry; crossed when mark at or above liq", () => {
    const entry = 100_000;
    const liq = paperLiquidationPrice(entry, "SHORT", 25);
    expect(liq).toBeGreaterThan(entry);
    expect(paperLiquidationCrossed("SHORT", liq + 1, liq)).toBe(true);
    expect(paperLiquidationCrossed("SHORT", liq, liq)).toBe(true);
    expect(paperLiquidationCrossed("SHORT", liq - 1, liq)).toBe(false);
  });
});

describe("paperResolveHardExit precedence (matches hook: liq → SL → TP → TIME)", () => {
  const base = (over: Partial<Parameters<typeof paperResolveHardExit>[0]>) => ({
    side: "LONG" as const,
    markPrice: 100,
    liquidationPrice: 70,
    adaptiveSl: 95,
    tpPrice: 110,
    entryPrice: 100,
    openedAtMs: 0,
    nowMs: 1,
    holdMinutes: 60,
    mtfHoldBonus: 1.3,
    holdTimeMul: 1,
    ...over,
  });

  it("long: SL hits when mark through adaptiveSl (before TP)", () => {
    const r = paperResolveHardExit(base({ markPrice: 94 }));
    expect(r).toEqual({ shouldClose: true, reason: "SL", exitPrice: 95 });
  });

  it("long: TP when mark at or above tp", () => {
    const r = paperResolveHardExit(base({ markPrice: 111 }));
    expect(r).toEqual({ shouldClose: true, reason: "TP", exitPrice: 110 });
  });

  it("long: liq beats SL when both conditions true (mark at or below liq first)", () => {
    const r = paperResolveHardExit(
      base({
        markPrice: 65,
        liquidationPrice: 70,
        adaptiveSl: 90,
      }),
    );
    expect(r).toEqual({ shouldClose: true, reason: "LIQUIDATION_RISK", exitPrice: 70 });
  });

  it("long: TIME when neither liq/SL/TP and age exceeds hold window", () => {
    const holdMinutes = 10;
    const mtf = 1.3;
    const holdExtendMs = holdMinutes * mtf * 60_000;
    const r = paperResolveHardExit(
      base({
        markPrice: 100,
        liquidationPrice: 50,
        adaptiveSl: 90,
        tpPrice: 115,
        openedAtMs: 0,
        nowMs: holdExtendMs + 1,
        holdMinutes,
        mtfHoldBonus: mtf,
      }),
    );
    expect(r).toEqual({ shouldClose: true, reason: "TIME", exitPrice: 100 });
  });

  it("short: SL when mark at or above adaptiveSl", () => {
    const r = paperResolveHardExit({
      side: "SHORT",
      markPrice: 101_500,
      liquidationPrice: 130_000,
      adaptiveSl: 101_000,
      tpPrice: 95_000,
      entryPrice: 100_000,
      openedAtMs: 0,
      nowMs: 1,
      holdMinutes: 60,
      mtfHoldBonus: 1.3,
      holdTimeMul: 1,
    });
    expect(r).toEqual({ shouldClose: true, reason: "SL", exitPrice: 101_000 });
  });

  it("short: TP when mark at or below tp", () => {
    const r = paperResolveHardExit({
      side: "SHORT",
      markPrice: 89_000,
      liquidationPrice: 130_000,
      adaptiveSl: 102_000,
      tpPrice: 90_000,
      entryPrice: 100_000,
      openedAtMs: 0,
      nowMs: 1,
      holdMinutes: 60,
      mtfHoldBonus: 1.3,
      holdTimeMul: 1,
    });
    expect(r).toEqual({ shouldClose: true, reason: "TP", exitPrice: 90_000 });
  });

  it("short: liq before SL when mark crosses liq upward", () => {
    const r = paperResolveHardExit({
      side: "SHORT",
      markPrice: 120_500,
      liquidationPrice: 120_000,
      adaptiveSl: 102_000,
      tpPrice: 90_000,
      entryPrice: 100_000,
      openedAtMs: 0,
      nowMs: 1,
      holdMinutes: 60,
      mtfHoldBonus: 1.3,
      holdTimeMul: 1,
    });
    expect(r).toEqual({ shouldClose: true, reason: "LIQUIDATION_RISK", exitPrice: 120_000 });
  });
});

describe("paperNetPnlOnClose (fees + win floor)", () => {
  const TAKER = 0.001;
  const MIN_WIN = 2;
  const notional = 50;
  const entry = 100_000;

  it("tiny loser after fees (~$50 notional)", () => {
    const exit = entry + 40;
    const out = paperNetPnlOnClose({
      entryPrice: entry,
      exitPrice: exit,
      notional,
      side: "LONG",
      takerFeePct: TAKER,
      fundingCosts: 0,
      minAbsNetWinUsd: MIN_WIN,
    });
    expect(out.fees).toBeCloseTo(paperRoundTripTakerFees(notional, TAKER), 9);
    expect(out.grossPnl).toBeCloseTo((40 / entry) * notional, 9);
    expect(out.netPnl).toBeCloseTo(out.grossPnl - out.fees, 9);
    expect(out.netPnl).toBeLessThan(0);
  });

  it("tiny gross win floored to minAbsNetWinUsd", () => {
    const exit = entry + 500;
    const out = paperNetPnlOnClose({
      entryPrice: entry,
      exitPrice: exit,
      notional,
      side: "LONG",
      takerFeePct: TAKER,
      fundingCosts: 0,
      minAbsNetWinUsd: MIN_WIN,
    });
    const rawNet = out.grossPnl - out.fees;
    expect(rawNet).toBeGreaterThan(0);
    expect(rawNet).toBeLessThan(MIN_WIN);
    expect(out.netPnl).toBe(MIN_WIN);
  });

  it("larger win not floored", () => {
    const exit = entry + 50_000;
    const out = paperNetPnlOnClose({
      entryPrice: entry,
      exitPrice: exit,
      notional,
      side: "LONG",
      takerFeePct: TAKER,
      fundingCosts: 0,
      minAbsNetWinUsd: MIN_WIN,
    });
    expect(out.netPnl).toBeCloseTo(out.grossPnl - out.fees, 6);
    expect(out.netPnl).toBeGreaterThan(MIN_WIN);
  });
});
