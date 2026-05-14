import { describe, expect, it } from "vitest";
import {
  applyFundingAccrual,
  DELTA_PAPER_FUNDING_INTERVAL_MS,
  paperApplyFuturesExitPatches,
  paperContracts,
  paperEstimatedMaxLossAtStopSl,
  paperFuturesProgressTowardTp,
  paperLiquidationCrossed,
  paperLiquidationPrice,
  paperLinearGrossPnl,
  paperMarginRequired,
  paperNetPnlOnClose,
  paperNotional,
  paperMinExpectedMoveVsFees,
  paperPriceMovePctOnNotional,
  paperResolveHardExit,
  paperReturnOnMargin,
  paperRoundTripTakerFees,
  paperSameDirNotionalWouldExceedCap,
  paperWidenTpToMinSlRatio,
  type PaperFuturesExitPatchConsts,
} from "./futuresPaperMath";

describe("applyFundingAccrual (time-scaled periodic funding)", () => {
  const interval = DELTA_PAPER_FUNDING_INTERVAL_MS; // 8h
  const notional = 50;
  const rate = 0.0001; // 0.01% per full period on notional

  it("60s hold accrues << one naive per-poll full notional*rate charge", () => {
    const elapsedMs = 60_000;
    const longAcc = applyFundingAccrual({
      side: "LONG",
      notional,
      fundingRate: rate,
      elapsedMs,
      fundingIntervalMs: interval,
    });
    const naivePerPoll = notional * rate;
    expect(longAcc).toBeCloseTo(notional * rate * (elapsedMs / interval), 12);
    expect(Math.abs(longAcc)).toBeLessThan(naivePerPoll * 0.01);
  });

  it("LONG pays positive rate; SHORT receives (opposite sign)", () => {
    const elapsedMs = 60_000;
    const longAcc = applyFundingAccrual({
      side: "LONG",
      notional,
      fundingRate: rate,
      elapsedMs,
      fundingIntervalMs: interval,
    });
    const shortAcc = applyFundingAccrual({
      side: "SHORT",
      notional,
      fundingRate: rate,
      elapsedMs,
      fundingIntervalMs: interval,
    });
    expect(longAcc).toBeGreaterThan(0);
    expect(shortAcc).toBeLessThan(0);
    expect(shortAcc).toBeCloseTo(-longAcc, 12);
  });

  it("negative rate flips payer/receiver", () => {
    const elapsedMs = 120_000;
    const r = -0.0001;
    const longAcc = applyFundingAccrual({
      side: "LONG",
      notional,
      fundingRate: r,
      elapsedMs,
      fundingIntervalMs: interval,
    });
    const shortAcc = applyFundingAccrual({
      side: "SHORT",
      notional,
      fundingRate: r,
      elapsedMs,
      fundingIntervalMs: interval,
    });
    expect(longAcc).toBeLessThan(0);
    expect(shortAcc).toBeGreaterThan(0);
  });

  it("returns 0 for non-positive elapsed or notional", () => {
    expect(
      applyFundingAccrual({
        side: "LONG",
        notional: 50,
        fundingRate: 0.0001,
        elapsedMs: 0,
        fundingIntervalMs: interval,
      }),
    ).toBe(0);
    expect(
      applyFundingAccrual({
        side: "LONG",
        notional: 0,
        fundingRate: 0.0001,
        elapsedMs: 60_000,
        fundingIntervalMs: interval,
      }),
    ).toBe(0);
  });
});

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

  it("short: TIME when neither liq/SL/TP and age exceeds hold window", () => {
    const holdMinutes = 15;
    const mtf = 1.3;
    const holdTimeMul = 1;
    const holdExtendMs = holdMinutes * mtf * holdTimeMul * 60_000;
    const r = paperResolveHardExit({
      side: "SHORT",
      markPrice: 99_500,
      liquidationPrice: 130_000,
      adaptiveSl: 102_000,
      tpPrice: 90_000,
      entryPrice: 100_000,
      openedAtMs: 0,
      nowMs: holdExtendMs + 1,
      holdMinutes,
      mtfHoldBonus: mtf,
      holdTimeMul,
    });
    expect(r).toEqual({ shouldClose: true, reason: "TIME", exitPrice: 99_500 });
  });

  it("no exit when all conditions are safe and time not elapsed", () => {
    const r = paperResolveHardExit({
      side: "LONG",
      markPrice: 100,
      liquidationPrice: 50,
      adaptiveSl: 90,
      tpPrice: 115,
      entryPrice: 100,
      openedAtMs: 0,
      nowMs: 1,
      holdMinutes: 60,
      mtfHoldBonus: 1.3,
      holdTimeMul: 1,
    });
    expect(r).toEqual({ shouldClose: false });
  });
});

describe("paperPriceMovePctOnNotional", () => {
  it("LONG: exit above entry is positive %", () => {
    expect(paperPriceMovePctOnNotional(100_000, 101_000, "LONG")).toBeCloseTo(1, 8);
  });
  it("SHORT: exit below entry is positive %", () => {
    expect(paperPriceMovePctOnNotional(100_000, 99_000, "SHORT")).toBeCloseTo(1, 8);
  });
  it("matches linear gross direction on notional", () => {
    const entry = 80_000;
    const exit = 81_000;
    const notional = 500;
    const pct = paperPriceMovePctOnNotional(entry, exit, "LONG");
    const gross = paperLinearGrossPnl(entry, exit, notional, "LONG");
    expect(pct).toBeCloseTo(1.25, 8);
    expect(gross).toBeCloseTo(notional * (pct / 100), 6);
  });
});

describe("booked LONG close: gross, net, margin % vs price %", () => {
  it("hand math: favorable move, fees only, no funding", () => {
    const entry = 80_000;
    const exit = 81_000;
    const notional = 500;
    const leverage = 25;
    const margin = notional / leverage;
    const taker = 0.001;
    const { grossPnl, fees, netPnl } = paperNetPnlOnClose({
      entryPrice: entry,
      exitPrice: exit,
      notional,
      side: "LONG",
      takerFeePct: taker,
      fundingCosts: 0,
      minAbsNetWinUsd: 0,
    });
    expect(grossPnl).toBeCloseTo(6.25, 8);
    expect(fees).toBeCloseTo(notional * taker * 2, 8);
    expect(netPnl).toBeCloseTo(grossPnl - fees, 8);
    const netPctOnMargin = (netPnl / margin) * 100;
    const pricePct = paperPriceMovePctOnNotional(entry, exit, "LONG");
    expect(pricePct).toBeCloseTo(1.25, 8);
    expect(Math.abs(netPctOnMargin)).toBeGreaterThan(Math.abs(pricePct));
  });
});

describe("paperNetPnlOnClose (fees + win floor)", () => {
  const TAKER = 0.001;
  const MIN_WIN = 2;
  const notional = 50;
  const entry = 100_000;
  const expectedFees = notional * TAKER * 2;

  it("fees = notional × takerFee × 2 always", () => {
    expect(paperRoundTripTakerFees(notional, TAKER)).toBe(expectedFees);
    expect(expectedFees).toBe(0.1);
  });

  it("tiny loser after fees (~$50 notional, LONG)", () => {
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
    expect(out.fees).toBe(expectedFees);
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
    expect(out.fees).toBe(expectedFees);
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
    expect(out.fees).toBe(expectedFees);
    expect(out.netPnl).toBeCloseTo(out.grossPnl - out.fees, 6);
    expect(out.netPnl).toBeGreaterThan(MIN_WIN);
  });

  it("small genuine winner: netPnl > 0 after fees (~$50 notional)", () => {
    // Exit far enough above entry that gross > fees (need grossPnl > 0.10)
    // %Δ × notional > 0.10 → %Δ > 0.002 → exit > entry × 1.002 = 100_200
    const exit = entry + 10_000; // ~10% move — gross = 5.0, fees = 0.10
    const out = paperNetPnlOnClose({
      entryPrice: entry,
      exitPrice: exit,
      notional,
      side: "LONG",
      takerFeePct: TAKER,
      fundingCosts: 0,
      minAbsNetWinUsd: MIN_WIN,
    });
    expect(out.fees).toBe(expectedFees);
    expect(out.grossPnl).toBeGreaterThan(0);
    expect(out.netPnl).toBeGreaterThan(0);
    expect(out.netPnl).toBeCloseTo(out.grossPnl - out.fees, 9);
  });

  it("breakeven move: grossPnl ≈ 0, netPnl ≈ −fees", () => {
    const out = paperNetPnlOnClose({
      entryPrice: entry,
      exitPrice: entry, // exact breakeven
      notional,
      side: "LONG",
      takerFeePct: TAKER,
      fundingCosts: 0,
      minAbsNetWinUsd: MIN_WIN,
    });
    expect(out.fees).toBe(expectedFees);
    expect(out.grossPnl).toBe(0);
    expect(out.netPnl).toBeCloseTo(-expectedFees, 9);
  });

  it("SHORT tiny loser: mark moved up", () => {
    const exit = entry + 500; // adverse for short
    const out = paperNetPnlOnClose({
      entryPrice: entry,
      exitPrice: exit,
      notional,
      side: "SHORT",
      takerFeePct: TAKER,
      fundingCosts: 0,
      minAbsNetWinUsd: MIN_WIN,
    });
    expect(out.fees).toBe(expectedFees);
    expect(out.grossPnl).toBeLessThan(0);
    expect(out.netPnl).toBeLessThan(0);
  });

  it("SHORT winner: mark moved down", () => {
    const exit = entry - 10_000;
    const out = paperNetPnlOnClose({
      entryPrice: entry,
      exitPrice: exit,
      notional,
      side: "SHORT",
      takerFeePct: TAKER,
      fundingCosts: 0,
      minAbsNetWinUsd: MIN_WIN,
    });
    expect(out.fees).toBe(expectedFees);
    expect(out.grossPnl).toBeGreaterThan(0);
    expect(out.netPnl).toBeCloseTo(out.grossPnl - out.fees, 9);
    expect(out.netPnl).toBeGreaterThan(0);
  });

  it("funding costs reduce net PnL", () => {
    const exit = entry + 10_000;
    const funding = 1.5;
    const out = paperNetPnlOnClose({
      entryPrice: entry,
      exitPrice: exit,
      notional,
      side: "LONG",
      takerFeePct: TAKER,
      fundingCosts: funding,
      minAbsNetWinUsd: MIN_WIN,
    });
    expect(out.fees).toBe(expectedFees);
    expect(out.netPnl).toBeCloseTo(out.grossPnl - out.fees - funding, 9);
  });
});

describe("paperFuturesProgressTowardTp + paperApplyFuturesExitPatches + paperEstimatedMaxLossAtStopSl", () => {
  const patchC: PaperFuturesExitPatchConsts = {
    breakevenTriggerProgress: 0.4,
    trailActivationProgress: 0.3,
    trailGivebackShare: 0.18,
  };

  it("progress = |return on margin %| / entry→TP distance (%)", () => {
    expect(paperFuturesProgressTowardTp(10, 100_000, 102_000)).toBeCloseTo(5, 9);
  });

  it("breakeven raises LONG adaptiveSl toward entry when threshold met", () => {
    const out = paperApplyFuturesExitPatches(
      {
        side: "LONG",
        entryPrice: 100,
        markPrice: 101,
        adaptiveSl: 95,
        breakevenMoved: false,
        returnPctOnMargin: 10,
        peakReturnPctOnMargin: 10,
        progressTowardTp: 0.5,
      },
      patchC,
    );
    expect(out.breakevenMoved).toBe(true);
    expect(out.adaptiveSl).toBeGreaterThanOrEqual(100);
  });

  it("estimated max loss at SL = adverse gross + round-trip fees", () => {
    const est = paperEstimatedMaxLossAtStopSl(100_000, 99_000, 50, "LONG", 0.001);
    expect(est).toBeCloseTo(0.5 + 0.1, 9);
  });
});

describe("paperWidenTpToMinSlRatio", () => {
  it("leaves strat unchanged when ratio already >= min", () => {
    const r = paperWidenTpToMinSlRatio(0.3, 0.7, 2, 5);
    expect(r.included).toBe(true);
    expect(r.tpPct).toBeCloseTo(0.7, 9);
  });

  it("widens TP to sl*minRatio when below floor", () => {
    const r = paperWidenTpToMinSlRatio(0.32, 0.58, 2, 5);
    expect(r.included).toBe(true);
    expect(r.tpPct).toBeCloseTo(0.64, 9);
  });

  it("excludes when required TP exceeds cap", () => {
    const r = paperWidenTpToMinSlRatio(3.0, 2.0, 2, 4.8);
    expect(r.included).toBe(false);
  });
});

describe("paperMarginRequired / paperContracts / paperNotional / paperReturnOnMargin", () => {
  it("margin = notional / leverage", () => {
    expect(paperMarginRequired(500, 25)).toBe(20);
    expect(paperMarginRequired(100, 10)).toBe(10);
    expect(paperMarginRequired(0, 25)).toBe(0);
  });

  it("contracts = floor(notional / contractSize)", () => {
    expect(paperContracts(500, 1)).toBe(500);
    expect(paperContracts(499.9, 1)).toBe(499);
    expect(paperContracts(10, 3)).toBe(3);
  });

  it("notional = contracts × contractSize", () => {
    expect(paperNotional(500, 1)).toBe(500);
    expect(paperNotional(3, 10)).toBe(30);
  });

  it("returnOnMargin = (unrealizedPnl / marginUsed) × 100", () => {
    expect(paperReturnOnMargin(5, 20)).toBeCloseTo(25, 9);
    expect(paperReturnOnMargin(-3, 20)).toBeCloseTo(-15, 9);
    expect(paperReturnOnMargin(10, 0)).toBe(0);
  });
});

describe("paperMinExpectedMoveVsFees", () => {
  const fee = 0.001;
  const n = 1000;

  it("fails when markPrice is 0", () => {
    const r = paperMinExpectedMoveVsFees(0, 50, n, fee, 1);
    expect(r.ok).toBe(false);
  });

  it("fails when ATR14 is 0", () => {
    const r = paperMinExpectedMoveVsFees(100_000, 0, n, fee, 1);
    expect(r.ok).toBe(false);
  });

  it("computes moveUsd = (ATR/mark)×notional and compares to K×round-trip fees", () => {
    const mark = 100_000;
    const atr = 200;
    const r = paperMinExpectedMoveVsFees(mark, atr, n, fee, 1);
    const move = (atr / mark) * n;
    const rt = paperRoundTripTakerFees(n, fee);
    expect(r.expectedMoveUsd).toBeCloseTo(move, 9);
    expect(r.thresholdUsd).toBeCloseTo(rt, 9);
    expect(r.ok).toBe(move >= rt);
  });

  it("passes when move clears K×fees", () => {
    const r = paperMinExpectedMoveVsFees(50_000, 500, n, fee, 1);
    expect(r.ok).toBe(true);
  });
});

describe("paperSameDirNotionalWouldExceedCap", () => {
  it("blocks when adding notional exceeds equity×frac on that side", () => {
    const book = [
      { side: "LONG" as const, notional: 200 },
      { side: "LONG" as const, notional: 100 },
    ];
    expect(paperSameDirNotionalWouldExceedCap(book, "LONG", 50, 1000, 0.35)).toBe(false); // 350 cap, 350 next → not >
    expect(paperSameDirNotionalWouldExceedCap(book, "LONG", 51, 1000, 0.35)).toBe(true);
  });

  it("SHORT side uses short sum only", () => {
    const book = [{ side: "LONG" as const, notional: 400 }, { side: "SHORT" as const, notional: 301 }];
    expect(paperSameDirNotionalWouldExceedCap(book, "SHORT", 50, 1000, 0.35)).toBe(true); // 351 > 350
  });
});
