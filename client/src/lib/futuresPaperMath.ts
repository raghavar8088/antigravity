/**
 * Pure math for the browser futures **paper** desk (no React).
 * Invariants (callers must preserve):
 * - No preemptive liquidation: only a true modeled-liq **cross** triggers LIQUIDATION_RISK.
 * - Gross PnL = %Δ(entry → exit) × notional (linear book).
 * - Fees on close: notional × takerFeePct × 2 (entry + exit), unchanged from hook semantics.
 * - REST `funding_rate` (Delta-style) is the fraction of notional exchanged **once per funding period**
 *   (e.g. 8h), not each poll. Never charge `notional * fundingRate` per tick without scaling by
 *   `elapsedMs / fundingIntervalMs`.
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

/**
 * **Minimum expected move vs round-trip fees** (paper desk entry quality gate).
 *
 * Treats **ATR14 / markPrice** as a one-bar **relative** excursion; scaled to the position:
 *
 * \[
 *   M_{\$} = \frac{\mathrm{ATR}_{14}}{\mathrm{markPrice}} \times \mathrm{notional}
 * \]
 *
 * Round-trip taker fees (open + close at the same notional, no funding in this gate):
 *
 * \[
 *   F_{\mathrm{rt}} = \mathrm{paperRoundTripTakerFees}(\mathrm{notional}, \mathrm{takerFeePct})
 * \]
 *
 * The gate **passes** when:
 *
 * \[
 *   M_{\$} \ge K \times F_{\mathrm{rt}}
 * \]
 *
 * where `safetyK` is \(K\) (typically ≥ 1). Intuition: a single-ATR move in **$ PnL** should clear
 * `K` times the **$** round-trip fee hurdle before opening.
 *
 * Invalid / non-positive `markPrice`, non-finite inputs, `notional ≤ 0`, or `ATR14 ≤ 0` → gate **fails**
 * (`ok: false`) so entries are skipped conservatively.
 */
export function paperMinExpectedMoveVsFees(
  markPrice: number,
  atr14: number,
  notional: number,
  takerFeePct: number,
  safetyK: number,
): { ok: boolean; expectedMoveUsd: number; thresholdUsd: number } {
  const feesRt = paperRoundTripTakerFees(notional, takerFeePct);
  if (
    !Number.isFinite(markPrice) ||
    markPrice <= 0 ||
    !Number.isFinite(atr14) ||
    atr14 <= 0 ||
    !Number.isFinite(notional) ||
    notional <= 0 ||
    !Number.isFinite(takerFeePct) ||
    takerFeePct < 0 ||
    !Number.isFinite(safetyK) ||
    safetyK <= 0
  ) {
    return { ok: false, expectedMoveUsd: 0, thresholdUsd: safetyK > 0 && Number.isFinite(safetyK) ? safetyK * feesRt : 0 };
  }
  const expectedMoveUsd = (atr14 / markPrice) * notional;
  const thresholdUsd = safetyK * feesRt;
  return { ok: expectedMoveUsd >= thresholdUsd, expectedMoveUsd, thresholdUsd };
}

/**
 * Sum **$ notional** on one side; returns whether adding `prospectiveNotional` on `side` would exceed
 * `equity * maxFrac` (same-direction book cap). Invalid equity / prospective → treat as blocked.
 */
export function paperSameDirNotionalWouldExceedCap(
  book: ReadonlyArray<{ side: PaperSide; notional: number }>,
  side: PaperSide,
  prospectiveNotional: number,
  equity: number,
  maxFrac: number,
): boolean {
  if (!Number.isFinite(prospectiveNotional) || prospectiveNotional <= 0) return true;
  if (!Number.isFinite(equity) || equity <= 0) return true;
  if (!Number.isFinite(maxFrac) || maxFrac <= 0) return false;
  const cap = equity * maxFrac;
  let longSum = 0;
  let shortSum = 0;
  for (const p of book) {
    if (p.side === "LONG") longSum += p.notional;
    else shortSum += p.notional;
  }
  const next = (side === "LONG" ? longSum : shortSum) + prospectiveNotional;
  return next > cap;
}

/**
 * Default funding period length for paper accrual when the REST payload does not expose interval length.
 * Delta India perpetuals commonly use 8h funding periods (some products differ — see exchange docs).
 */
export const DELTA_PAPER_FUNDING_INTERVAL_MS = 8 * 60 * 60 * 1000;

export type PaperFundingAccrualInput = {
  side: PaperSide;
  notional: number;
  /** Ticker `funding_rate` (fraction for one full funding period). */
  fundingRate: number;
  elapsedMs: number;
  fundingIntervalMs: number;
};

/**
 * Cash **paid** by the position toward `fundingCosts` (positive = cost, same sign as `paperNetPnlOnClose` subtraction).
 * Convention (Delta / common perps): **positive** `fundingRate` → longs pay, shorts receive.
 * Accrues linearly in wall time: `signedNotional * fundingRate * (elapsedMs / fundingIntervalMs)`.
 */
export function applyFundingAccrual(p: PaperFundingAccrualInput): number {
  const { side, notional, fundingRate, elapsedMs, fundingIntervalMs } = p;
  if (!Number.isFinite(notional) || notional <= 0) return 0;
  if (!Number.isFinite(fundingRate) || fundingRate === 0) return 0;
  if (!Number.isFinite(elapsedMs) || elapsedMs <= 0) return 0;
  if (!Number.isFinite(fundingIntervalMs) || fundingIntervalMs <= 0) return 0;

  const signedNotional = side === "LONG" ? notional : -notional;
  const frac = elapsedMs / fundingIntervalMs;
  return signedNotional * fundingRate * frac;
}

/**
 * Progress toward TP using the paper desk convention: |return on margin %| divided by absolute
 * entry→TP distance as % of entry (price space). Unbounded above 1 when leverage magnifies margin return.
 */
export function paperFuturesProgressTowardTp(
  returnPctOnMargin: number,
  entryPrice: number,
  tpPrice: number,
): number {
  if (!entryPrice || !Number.isFinite(entryPrice)) return 0;
  const tpMovePct = Math.abs((tpPrice - entryPrice) / entryPrice) * 100;
  if (!Number.isFinite(tpMovePct) || tpMovePct < 1e-12) return 0;
  return Math.abs(returnPctOnMargin) / tpMovePct;
}

export type PaperFuturesExitPatchConsts = {
  breakevenTriggerProgress: number;
  trailActivationProgress: number;
  trailGivebackShare: number;
};

export type PaperFuturesExitPatchIn = {
  side: PaperSide;
  entryPrice: number;
  markPrice: number;
  adaptiveSl: number;
  breakevenMoved: boolean;
  returnPctOnMargin: number;
  peakReturnPctOnMargin: number;
  progressTowardTp: number;
};

/**
 * Persist trail tightening + breakeven SL + peak return (on margin) in one pass before hard SL/TP checks.
 * Breakeven runs before trail so SL can step to entry when the BE threshold fires first on a poll.
 */
export function paperApplyFuturesExitPatches(
  p: PaperFuturesExitPatchIn,
  c: PaperFuturesExitPatchConsts,
): { adaptiveSl: number; breakevenMoved: boolean; peakReturnPctOnMargin: number } {
  let { adaptiveSl, breakevenMoved } = p;
  const peakReturnPctOnMargin = Math.max(p.peakReturnPctOnMargin, p.returnPctOnMargin);

  if (p.progressTowardTp >= c.breakevenTriggerProgress && !breakevenMoved) {
    if (p.side === "LONG") adaptiveSl = Math.max(adaptiveSl, p.entryPrice);
    else adaptiveSl = Math.min(adaptiveSl, p.entryPrice);
    breakevenMoved = true;
  }

  if (p.progressTowardTp >= c.trailActivationProgress) {
    const newSl =
      p.side === "LONG"
        ? p.entryPrice + (p.markPrice - p.entryPrice) * (1 - c.trailGivebackShare)
        : p.entryPrice - (p.entryPrice - p.markPrice) * (1 - c.trailGivebackShare);
    if (p.side === "LONG") adaptiveSl = Math.max(adaptiveSl, newSl);
    else adaptiveSl = Math.min(adaptiveSl, newSl);
  }

  return { adaptiveSl, breakevenMoved, peakReturnPctOnMargin };
}

/**
 * Conservative max cash loss if the position is closed at `stopPrice` (e.g. initial SL): adverse gross + full round-trip taker fees.
 */
export function paperEstimatedMaxLossAtStopSl(
  entryPrice: number,
  stopPrice: number,
  notional: number,
  side: PaperSide,
  takerFeePct: number,
): number {
  const g = paperLinearGrossPnl(entryPrice, stopPrice, notional, side);
  const grossLoss = g < 0 ? -g : 0;
  return grossLoss + paperRoundTripTakerFees(notional, takerFeePct);
}

/**
 * Enforce minimum TP/SL reward:risk in **percent space** (same units as `FuturesStratDef.tpPct` / `slPct`).
 * If widening would exceed `maxTpPctAbs`, returns `included: false` (caller should drop the strat from the desk).
 */
export function paperWidenTpToMinSlRatio(
  slPct: number,
  tpPct: number,
  minRatio: number,
  maxTpPctAbs: number,
): { tpPct: number; included: boolean } {
  if (!Number.isFinite(slPct) || slPct <= 0 || !Number.isFinite(tpPct)) return { tpPct, included: false };
  const ratio = minRatio >= 1 && Number.isFinite(minRatio) ? minRatio : 2;
  const current = tpPct / slPct;
  if (current >= ratio - 1e-12) return { tpPct, included: true };
  const needTp = slPct * ratio;
  if (needTp > maxTpPctAbs) return { tpPct, included: false };
  return { tpPct: needTp, included: true };
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

// ========== MARGIN / SIZING ==========

/** Isolated margin required for a position. */
export function paperMarginRequired(notional: number, leverage: number): number {
  return notional / leverage;
}

/** Contract count (whole units) from a desired notional at a given contract size. */
export function paperContracts(notional: number, contractSize: number): number {
  return Math.floor(notional / contractSize);
}

/** Notional USD from contract count. */
export function paperNotional(contracts: number, contractSize: number): number {
  return contracts * contractSize;
}

/** Return-on-margin percentage (used for unrealized PnL display). */
export function paperReturnOnMargin(unrealizedPnl: number, marginUsed: number): number {
  return marginUsed > 0 ? (unrealizedPnl / marginUsed) * 100 : 0;
}

// ========== HARD EXIT ==========

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
