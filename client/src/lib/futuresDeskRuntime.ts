/**
 * Shared per-poll position mark + exit resolution (live hook + offline replay).
 * Keep in sync with `useBTCFuturesScalperEngine` exit constants.
 */

import {
  applyFundingAccrual,
  DELTA_PAPER_FUNDING_INTERVAL_MS,
  paperApplyFuturesExitPatches,
  paperFuturesProgressTowardTp,
  paperLinearGrossPnl,
  paperResolveHardExit,
  paperReturnOnMargin,
  type PaperFuturesExitPatchConsts,
  type PaperSide,
} from "./futuresPaperMath";

export const DESK_EXIT_BREAKEVEN_TRIGGER_FRAC = 0.4;
export const DESK_EXIT_TRAIL_ACTIVATION_PCT = 0.3;
export const DESK_EXIT_TRAIL_GIVEBACK_SHARE = 0.18;
export const DESK_EXIT_PROFIT_LOCK_PROGRESS = 0.6;
export const DESK_EXIT_PROFIT_LOCK_SHARE = 0.6;
export const DESK_EXIT_LATE_EXIT_MIN_GAIN = 0.22;
export const DESK_EXIT_MTF_HOLD_BONUS = 1.3;

export const FUTURES_EXIT_PATCH_CONSTS: PaperFuturesExitPatchConsts = {
  breakevenTriggerProgress: DESK_EXIT_BREAKEVEN_TRIGGER_FRAC,
  trailActivationProgress: DESK_EXIT_TRAIL_ACTIVATION_PCT,
  trailGivebackShare: DESK_EXIT_TRAIL_GIVEBACK_SHARE,
};

export type FuturesMarkPosition = {
  side: PaperSide;
  entryPrice: number;
  markPrice: number;
  lastPrice: number;
  notional: number;
  marginUsed: number;
  fundingCosts: number;
  lastFundingAppliedAt: number;
  unrealizedPnl: number;
  unrealizedPnlPct: number;
  returnPct: number;
  peakReturnPct: number;
};

export type FuturesExitStepPosition = FuturesMarkPosition & {
  tpPrice: number;
  slPrice: number;
  adaptiveSl: number;
  breakevenMoved: boolean;
  liquidationPrice: number;
  openedAt: string;
  holdMinutes: number;
};

export type FuturesExitStepResult = {
  patched: FuturesExitStepPosition;
  close: {
    shouldClose: boolean;
    reason?: "TP" | "SL" | "TIME" | "TRAIL" | "LIQUIDATION_RISK" | "PROFIT_LOCK";
    exitPrice: number;
  };
};

export function applyMarkToFuturesPosition<P extends FuturesMarkPosition>(
  p: P,
  markPrice: number,
  lastPrice: number,
  ctx: { fundingRate: number; nowMs: number },
): P {
  const unrealizedPnL = paperLinearGrossPnl(p.entryPrice, markPrice, p.notional, p.side);
  const returnPct = paperReturnOnMargin(unrealizedPnL, p.marginUsed);
  const unrealizedPnLPct = p.notional > 0 ? (unrealizedPnL / p.notional) * 100 : 0;

  const lastAcc = Number.isFinite(p.lastFundingAppliedAt) ? p.lastFundingAppliedAt : ctx.nowMs;
  const elapsedMs = Math.max(0, ctx.nowMs - lastAcc);
  const fundingDelta = applyFundingAccrual({
    side: p.side,
    notional: p.notional,
    fundingRate: ctx.fundingRate,
    elapsedMs,
    fundingIntervalMs: DELTA_PAPER_FUNDING_INTERVAL_MS,
  });

  return {
    ...p,
    markPrice,
    lastPrice,
    unrealizedPnl: unrealizedPnL,
    unrealizedPnlPct: unrealizedPnLPct,
    returnPct,
    fundingCosts: p.fundingCosts + fundingDelta,
    lastFundingAppliedAt: ctx.nowMs,
  };
}

/**
 * One step: trail / breakeven / peak → hard exits (liq→SL→TP→TIME) → profit-lock → trail giveback.
 */
export function resolveFuturesExitStep(
  p: FuturesExitStepPosition,
  holdTimeMul: number,
  nowMs: number,
): FuturesExitStepResult {
  const progress = paperFuturesProgressTowardTp(p.returnPct, p.entryPrice, p.tpPrice);
  const soft = paperApplyFuturesExitPatches(
    {
      side: p.side,
      entryPrice: p.entryPrice,
      markPrice: p.markPrice,
      adaptiveSl: p.adaptiveSl,
      breakevenMoved: p.breakevenMoved,
      returnPctOnMargin: p.returnPct,
      peakReturnPctOnMargin: p.peakReturnPct ?? p.returnPct,
      progressTowardTp: progress,
    },
    FUTURES_EXIT_PATCH_CONSTS,
  );
  const q: FuturesExitStepPosition = { ...p, ...soft, peakReturnPct: soft.peakReturnPctOnMargin };

  const hard = paperResolveHardExit({
    side: q.side,
    markPrice: q.markPrice,
    liquidationPrice: q.liquidationPrice,
    adaptiveSl: q.adaptiveSl,
    tpPrice: q.tpPrice,
    entryPrice: q.entryPrice,
    openedAtMs: new Date(q.openedAt).getTime(),
    nowMs,
    holdMinutes: q.holdMinutes,
    mtfHoldBonus: DESK_EXIT_MTF_HOLD_BONUS,
    holdTimeMul,
  });
  if (hard.shouldClose) {
    return {
      patched: q,
      close: { shouldClose: true, reason: hard.reason, exitPrice: hard.exitPrice },
    };
  }

  const tpPctAbs = Math.abs((q.tpPrice - q.entryPrice) / q.entryPrice) * 100;
  const lockTh = Math.max(DESK_EXIT_LATE_EXIT_MIN_GAIN, tpPctAbs * DESK_EXIT_PROFIT_LOCK_SHARE);
  if (progress >= DESK_EXIT_PROFIT_LOCK_PROGRESS && q.returnPct >= lockTh) {
    return { patched: q, close: { shouldClose: true, reason: "PROFIT_LOCK", exitPrice: q.markPrice } };
  }

  const peak = soft.peakReturnPctOnMargin;
  if (
    progress >= DESK_EXIT_TRAIL_ACTIVATION_PCT &&
    peak > DESK_EXIT_LATE_EXIT_MIN_GAIN &&
    q.returnPct <= peak * (1 - DESK_EXIT_TRAIL_GIVEBACK_SHARE)
  ) {
    return { patched: q, close: { shouldClose: true, reason: "TRAIL", exitPrice: q.markPrice } };
  }

  return { patched: q, close: { shouldClose: false, exitPrice: q.markPrice } };
}
