/**
 * Shared per-poll position mark + exit resolution (live hook + offline replay).
 * Keep in sync with `useBTCFuturesScalperEngine` exit constants.
 */

import {
  applyFundingAccrual,
  DELTA_PAPER_FUNDING_INTERVAL_MS,
  paperApplyExitSlippage,
  paperApplyFuturesExitPatches,
  paperFuturesProgressTowardTp,
  paperLinearGrossPnl,
  paperNetPnlOnClose,
  paperResolveHardExit,
  paperReturnOnMargin,
  type PaperFuturesExitPatchConsts,
  type PaperSide,
} from "./futuresPaperMath";

export const DESK_EXIT_BREAKEVEN_TRIGGER_FRAC = 0.4;
export const DESK_EXIT_TRAIL_ACTIVATION_PCT = 0.3;
export const DESK_EXIT_TRAIL_GIVEBACK_SHARE = 0.18;
export const DESK_EXIT_PROFIT_LOCK_PROGRESS = 0.55;
export const DESK_EXIT_PROFIT_LOCK_SHARE = 0.6;
export const DESK_EXIT_LATE_EXIT_MIN_GAIN = 0.22;
export const DESK_EXIT_MTF_HOLD_BONUS = 1.3;
/**
 * Default minimum projected USD net (after exit slippage + round-trip fees + funding)
 * required to fire PROFIT_LOCK. Skips lock-outs that would book a micro-loss.
 * Overridable via `NEXT_PUBLIC_DESK_PROFIT_LOCK_MIN_NET_USD`.
 */
export const DESK_EXIT_PROFIT_LOCK_MIN_NET_USD = 0.05;

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
 * Knobs passed into {@link resolveFuturesExitStep}. Production wires the live
 * hook's slippage + fee constants; replay wires its fixture-matching defaults.
 */
export type FuturesExitStepOpts = {
  /**
   * Minimum projected USD net (after exit slippage, round-trip fees, funding) required to
   * book a PROFIT_LOCK exit. Below this, the lock is skipped and we let TRAIL / TP / SL
   * decide. Default {@link DESK_EXIT_PROFIT_LOCK_MIN_NET_USD}.
   */
  profitLockMinNetUsd?: number;
  /** Progress-toward-TP gate before lock can fire. Default {@link DESK_EXIT_PROFIT_LOCK_PROGRESS}. */
  profitLockMinProgress?: number;
  /** Taker round-trip fee % used for the net-projection. Default 0.001 (0.10%). */
  takerFeePct?: number;
  /** Adverse exit slippage in bps applied to mark before the net projection. Default 5. */
  exitSlippageBps?: number;
  /**
   * Current bar's ATR14 in USD. When provided and > 0, activates ATR-based trailing:
   * once the position reaches TRAIL_ACTIVATION progress, the trail stop is set at
   * peakMarkPrice − 0.5×ATR (LONG) / peakMarkPrice + 0.5×ATR (SHORT).
   * Falls back to pct-giveback trail when omitted.
   */
  atr14?: number;
  /** Skip SL until position age ≥ this (paper discovery; TP/TIME/liq still apply). */
  minAgeBeforeSlMs?: number;
  /** Override breakeven progress gate (default DESK_EXIT_BREAKEVEN_TRIGGER_FRAC). */
  breakevenTriggerProgress?: number;
};

/**
 * One step: trail / breakeven / peak → hard exits (liq→SL→TP→TIME) → profit-lock → trail giveback.
 */
export function resolveFuturesExitStep(
  p: FuturesExitStepPosition,
  holdTimeMul: number,
  nowMs: number,
  opts: FuturesExitStepOpts = {},
): FuturesExitStepResult {
  const openedAtMs = new Date(p.openedAt).getTime();
  const inEarlyHold =
    Number.isFinite(opts.minAgeBeforeSlMs) &&
    (opts.minAgeBeforeSlMs ?? 0) > 0 &&
    nowMs - openedAtMs < (opts.minAgeBeforeSlMs as number);

  const progress = paperFuturesProgressTowardTp(p.returnPct, p.entryPrice, p.tpPrice);
  const patchConsts: PaperFuturesExitPatchConsts = {
    ...FUTURES_EXIT_PATCH_CONSTS,
    ...(opts.breakevenTriggerProgress !== undefined
      ? { breakevenTriggerProgress: opts.breakevenTriggerProgress }
      : {}),
    // During paper grace: do not tighten SL via breakeven/trail patches.
    ...(inEarlyHold ? { breakevenTriggerProgress: 1.01, trailActivationProgress: 1.01 } : {}),
  };
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
    patchConsts,
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
    minAgeBeforeSlMs: opts.minAgeBeforeSlMs,
  });
  if (hard.shouldClose) {
    return {
      patched: q,
      close: { shouldClose: true, reason: hard.reason, exitPrice: hard.exitPrice },
    };
  }

  const tpPctAbs = Math.abs((q.tpPrice - q.entryPrice) / q.entryPrice) * 100;
  const lockTh = Math.max(DESK_EXIT_LATE_EXIT_MIN_GAIN, tpPctAbs * DESK_EXIT_PROFIT_LOCK_SHARE);
  const profitLockMinProgress = opts.profitLockMinProgress ?? DESK_EXIT_PROFIT_LOCK_PROGRESS;
  const profitLockMinNetUsd = opts.profitLockMinNetUsd ?? DESK_EXIT_PROFIT_LOCK_MIN_NET_USD;
  if (!inEarlyHold && progress >= profitLockMinProgress && q.returnPct >= lockTh) {
    // Project the actual net at slipped mark — if it's negative or below threshold,
    // skip the lock so we don't book a micro-loss. The trade falls through to TRAIL/TP/SL.
    const takerFeePct = opts.takerFeePct ?? 0.001;
    const exitSlippageBps = opts.exitSlippageBps ?? 5;
    const slippedMark = paperApplyExitSlippage(q.side, q.markPrice, exitSlippageBps);
    const { netPnl: projectedNet } = paperNetPnlOnClose({
      entryPrice: q.entryPrice,
      exitPrice: slippedMark,
      notional: q.notional,
      side: q.side,
      takerFeePct,
      fundingCosts: q.fundingCosts,
      minAbsNetWinUsd: 0, // raw net for the decision — don't let the floor mask a loss
    });
    if (projectedNet >= profitLockMinNetUsd) {
      return { patched: q, close: { shouldClose: true, reason: "PROFIT_LOCK", exitPrice: q.markPrice } };
    }
    // else: fall through; TRAIL / TP / SL / TIME still resolve normally
  }

  const peak = soft.peakReturnPctOnMargin;
  const atr14 = opts.atr14;
  if (
    !inEarlyHold &&
    progress >= DESK_EXIT_TRAIL_ACTIVATION_PCT &&
    peak > DESK_EXIT_LATE_EXIT_MIN_GAIN
  ) {
    if (atr14 && atr14 > 0 && q.marginUsed > 0) {
      // ATR-based trail: reconstruct approximate peak mark price from peakReturnPct.
      // peakReturnPct is % of margin; grossPnl = returnPct/100 * marginUsed.
      // For LONG: grossPnl = (peakMark - entry) / entry * notional → peakMark = entry + grossPnl * entry / notional.
      const grossPnlAtPeak = (peak / 100) * q.marginUsed;
      const peakMarkApprox =
        q.side === "LONG"
          ? q.entryPrice + (grossPnlAtPeak * q.entryPrice) / q.notional
          : q.entryPrice - (grossPnlAtPeak * q.entryPrice) / q.notional;
      const atrTrailStop =
        q.side === "LONG" ? peakMarkApprox - 0.5 * atr14 : peakMarkApprox + 0.5 * atr14;
      const atrTrailHit =
        q.side === "LONG" ? q.markPrice <= atrTrailStop : q.markPrice >= atrTrailStop;
      if (atrTrailHit) {
        return { patched: q, close: { shouldClose: true, reason: "TRAIL", exitPrice: q.markPrice } };
      }
    } else if (q.returnPct <= peak * (1 - DESK_EXIT_TRAIL_GIVEBACK_SHARE)) {
      // Fallback pct-giveback trail when ATR is not available.
      return { patched: q, close: { shouldClose: true, reason: "TRAIL", exitPrice: q.markPrice } };
    }
  }

  return { patched: q, close: { shouldClose: false, exitPrice: q.markPrice } };
}
