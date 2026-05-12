/**
 * Pure math for the browser futures **paper** desk (no React).
 * Invariants (callers must preserve):
 * - No preemptive liquidation: only a true modeled-liq **cross** triggers LIQUIDATION_RISK.
 * - Gross PnL = %Δ(entry → exit) × notional (linear book).
 * - Fees on close: notional × takerFeePct × 2 (entry + exit), unchanged from hook semantics.
 */

export type PaperSide = "LONG" | "SHORT";

export type PaperHardExitReason = "LIQUIDATION_RISK" | "SL" | "TP" | "TIME";

export type PaperHardExitResult =
  | { shouldClose: true; reason: PaperHardExitReason; exitPrice: number }
  | { shouldClose: false };

/** Modeled isolated liquidation price (same formula as legacy hook). */
export function paperLiquidationPrice(
  entryPrice: number,
  side: PaperSide,
  leverage: number,
  maintenanceMarginPct = 0.005,
): number {
  if (side === "LONG") {
    return entryPrice * (1 - 1 / leverage + maintenanceMarginPct);
  }
  return entryPrice * (1 + 1 / leverage - maintenanceMarginPct);
}

/** True when mark has crossed through modeled liquidation (first liq check in runtime). */
export function paperLiquidationCrossed(side: PaperSide, markPrice: number, liquidationPrice: number): boolean {
  if (side === "LONG") return markPrice <= liquidationPrice;
  return markPrice >= liquidationPrice;
}

/** % distance from price to liquidation (used for stats / trade record), same as legacy hook. */
export function paperLiquidationDistancePct(price: number, liquidationPrice: number, side: PaperSide): number {
  if (side === "LONG") {
    return ((price - liquidationPrice) / price) * 100;
  }
  return ((liquidationPrice - price) / price) * 100;
}

/** Linear gross PnL in USD (same as legacy `calculateUnrealizedPnL`). */
export function paperLinearGrossPnl(
  entryPrice: number,
  markOrExitPrice: number,
  notional: number,
  side: PaperSide,
): number {
  if (!entryPrice || !Number.isFinite(notional) || notional <= 0) return 0;
  const pct =
    side === "LONG"
      ? (markOrExitPrice - entryPrice) / entryPrice
      : (entryPrice - markOrExitPrice) / entryPrice;
  return pct * notional;
}

export function paperRoundTripTakerFees(notional: number, takerFeePct: number): number {
  return notional * takerFeePct * 2;
}

export type PaperNetCloseParams = {
  entryPrice: number;
  exitPrice: number;
  notional: number;
  side: PaperSide;
  takerFeePct: number;
  fundingCosts: number;
  /** Floor tiny *wins* only (matches hook). */
  minAbsNetWinUsd: number;
};

/** Gross, fees, net after funding and win-floor — matches `closePosition` math. */
export function paperNetPnlOnClose(p: PaperNetCloseParams): { grossPnl: number; fees: number; netPnl: number } {
  const grossPnl = paperLinearGrossPnl(p.entryPrice, p.exitPrice, p.notional, p.side);
  const fees = paperRoundTripTakerFees(p.notional, p.takerFeePct);
  let netPnl = grossPnl - fees - p.fundingCosts;
  if (netPnl > 0 && netPnl < p.minAbsNetWinUsd) {
    netPnl = p.minAbsNetWinUsd;
  }
  return { grossPnl, fees, netPnl };
}

export type PaperHardExitInputs = {
  side: PaperSide;
  markPrice: number;
  liquidationPrice: number;
  adaptiveSl: number;
  tpPrice: number;
  entryPrice: number;
  openedAtMs: number;
  nowMs: number;
  holdMinutes: number;
  mtfHoldBonus: number;
  holdTimeMul: number;
};

/**
 * Hard exits only, **same order as runtime** `resolveExit` (liq → SL → TP → TIME).
 * Trailing / breakeven branches stay in the hook (need position mutation + progress).
 */
export function paperResolveHardExit(i: PaperHardExitInputs): PaperHardExitResult {
  if (i.side === "LONG" && i.markPrice <= i.liquidationPrice) {
    return { shouldClose: true, reason: "LIQUIDATION_RISK", exitPrice: i.liquidationPrice };
  }
  if (i.side === "SHORT" && i.markPrice >= i.liquidationPrice) {
    return { shouldClose: true, reason: "LIQUIDATION_RISK", exitPrice: i.liquidationPrice };
  }

  if (i.side === "LONG" && i.markPrice <= i.adaptiveSl) {
    return { shouldClose: true, reason: "SL", exitPrice: i.adaptiveSl };
  }
  if (i.side === "SHORT" && i.markPrice >= i.adaptiveSl) {
    return { shouldClose: true, reason: "SL", exitPrice: i.adaptiveSl };
  }

  if (i.side === "LONG" && i.markPrice >= i.tpPrice) {
    return { shouldClose: true, reason: "TP", exitPrice: i.tpPrice };
  }
  if (i.side === "SHORT" && i.markPrice <= i.tpPrice) {
    return { shouldClose: true, reason: "TP", exitPrice: i.tpPrice };
  }

  const ageMin = (i.nowMs - i.openedAtMs) / 60_000;
  const holdExtend = i.holdMinutes * i.mtfHoldBonus * i.holdTimeMul;
  if (ageMin >= holdExtend) {
    return { shouldClose: true, reason: "TIME", exitPrice: i.markPrice };
  }

  return { shouldClose: false };
}
