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

/**
 * Adverse fill adjustment in basis points (1 bps = 0.01%).
 * `slippageUsd = price × (slippageBps / 10_000)`.
 *
 * Entry (taker worse fill):
 * - LONG: `price × (1 + bps/10_000)`
 * - SHORT: `price × (1 − bps/10_000)`
 *
 * Exit (taker worse fill vs level/mark):
 * - LONG: `price × (1 − bps/10_000)` (receive less)
 * - SHORT: `price × (1 + bps/10_000)` (cover higher)
 *
 * `slippageBps ≤ 0` or non-finite inputs → unchanged `price`.
 */
export function paperApplyEntrySlippage(side: PaperSide, price: number, slippageBps: number): number {
  if (!Number.isFinite(price) || price <= 0 || !Number.isFinite(slippageBps) || slippageBps <= 0) return price;
  const mul = slippageBps / 10_000;
  return side === "LONG" ? price * (1 + mul) : price * (1 - mul);
}

export function paperApplyExitSlippage(side: PaperSide, price: number, slippageBps: number): number {
  if (!Number.isFinite(price) || price <= 0 || !Number.isFinite(slippageBps) || slippageBps <= 0) return price;
  const mul = slippageBps / 10_000;
  return side === "LONG" ? price * (1 - mul) : price * (1 + mul);
}

/**
 * |last − mark| / mark × 100 (%). Used to skip entries when last/mark diverge too far.
 * Invalid or non-positive mark → `Infinity` (callers should treat as over any finite cap).
 */
export function paperLastMarkSpreadPct(last: number, mark: number): number {
  if (!Number.isFinite(last) || !Number.isFinite(mark) || mark <= 0) return Infinity;
  return (Math.abs(last - mark) / mark) * 100;
}

/**
 * Volatility-scaled slippage in bps (P2.2.4).
 *
 * Scales base slippage by `clamp(currentAtr / avgAtr30, 0.5, 2.5)`:
 * - Low vol regime (curATR < avgATR): slippage is reduced (down to 50% of base)
 * - High vol regime (curATR > avgATR): slippage is widened (up to 250% of base)
 *
 * Returns `baseBps` unchanged when ATR inputs are invalid (no behavior change).
 */
export function paperDynamicSlippageBps(
  baseBps: number,
  currentAtr: number,
  avgAtr30: number,
): number {
  if (!Number.isFinite(baseBps) || baseBps <= 0) return baseBps;
  if (!Number.isFinite(currentAtr) || currentAtr <= 0) return baseBps;
  if (!Number.isFinite(avgAtr30) || avgAtr30 <= 0) return baseBps;
  const ratio = Math.min(2.5, Math.max(0.5, currentAtr / avgAtr30));
  return baseBps * ratio;
}

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
  if (!Number.isFinite(entryPrice) || entryPrice <= 0) return 0;
  // Leverage must be >= 1; below 1x isolated margin the formula produces a negative liq price
  const lev = Math.max(1, leverage);
  if (side === "LONG") {
    return entryPrice * (1 - 1 / lev + maintenanceMarginPct);
  }
  return entryPrice * (1 + 1 / lev - maintenanceMarginPct);
}

/** True when mark has crossed through modeled liquidation (first liq check in runtime). */
export function paperLiquidationCrossed(side: PaperSide, markPrice: number, liquidationPrice: number): boolean {
  if (side === "LONG") return markPrice <= liquidationPrice;
  return markPrice >= liquidationPrice;
}

/** % distance from price to liquidation (used for stats / trade record), same as legacy hook. */
export function paperLiquidationDistancePct(price: number, liquidationPrice: number, side: PaperSide): number {
  if (!Number.isFinite(price) || price <= 0) return Infinity;
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
  if (!Number.isFinite(entryPrice) || entryPrice <= 0 || !Number.isFinite(notional) || notional <= 0) return 0;
  const pct =
    side === "LONG"
      ? (markOrExitPrice - entryPrice) / entryPrice
      : (entryPrice - markOrExitPrice) / entryPrice;
  return pct * notional;
}

/**
 * Underlying **price** return % from entry → exit (not leverage-scaled).
 * Same sign convention as {@link paperLinearGrossPnl} before notional multiply.
 */
export function paperPriceMovePctOnNotional(entryPrice: number, exitPrice: number, side: PaperSide): number {
  if (!Number.isFinite(entryPrice) || entryPrice <= 0 || !Number.isFinite(exitPrice)) return 0;
  return side === "LONG"
    ? ((exitPrice - entryPrice) / entryPrice) * 100
    : ((entryPrice - exitPrice) / entryPrice) * 100;
}

export function paperRoundTripTakerFees(notional: number, takerFeePct: number): number {
  return notional * takerFeePct * 2;
}

/**
 * Delta India maker fee on perpetuals — 0.05% per leg (matches `MAKER_FEE_PCT`
 * in `useBTCFuturesScalperEngine.ts`). Used by `paperRoundTripBlendedFees`
 * to model a maker-first entry path with taker fallback.
 */
export const PAPER_MAKER_FEE_PCT_DEFAULT = 0.0005;

/**
 * Probability a limit (post-only) entry actually fills as maker before timeout.
 * 0.7 is conservative for 1m scalps — fills happen when price ticks through the limit,
 * which is common but not guaranteed within an N-second window.
 */
export const PAPER_MAKER_FILL_PROBABILITY_DEFAULT = 0.7;

/**
 * Round-trip fees under a maker-first entry path with taker fallback.
 *
 * Blended per-leg fee = `p_maker * makerFeePct + (1 - p_maker) * takerFeePct`.
 * Round-trip = `2 * blended * notional`.
 *
 * Used by paper math when `deskPaperMakerFillModelEnabled()` is on. With Delta's
 * standard rates (0.02% maker, 0.10% taker) and `p_maker = 0.7`, RT drops from
 * 0.20% (all-taker) to ~0.088% — a ~56% reduction in fee drag.
 *
 * IMPORTANT: this is a *model* of expected fee outcomes under a maker-first
 * implementation. The live execution layer must actually place post-only orders
 * with a taker-fallback timeout for the model to be honest. This function
 * intentionally returns a higher number than pure-maker math would, so paper
 * results stay conservative against optimistic execution assumptions.
 */
export function paperRoundTripBlendedFees(
  notional: number,
  makerFeePct: number,
  takerFeePct: number,
  makerFillProbability: number,
): number {
  if (!Number.isFinite(notional) || notional <= 0) return 0;
  const pm = Math.min(1, Math.max(0, Number.isFinite(makerFillProbability) ? makerFillProbability : 0));
  const makerPerLeg = Number.isFinite(makerFeePct) && makerFeePct >= 0 ? makerFeePct : 0;
  const takerPerLeg = Number.isFinite(takerFeePct) && takerFeePct >= 0 ? takerFeePct : 0;
  const blendedPerLeg = pm * makerPerLeg + (1 - pm) * takerPerLeg;
  return notional * blendedPerLeg * 2;
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

export type PaperNotionalForTargetRiskArgs = {
  equityUsd: number;
  /** Context / future gates; sizing uses `slPct` + `entryPrice`. */
  atr14: number;
  markPrice: number;
  /** Fill price for SL distance; defaults to `markPrice`. */
  entryPrice?: number;
  side: PaperSide;
  /** Strategy stop % in the same units as `FuturesStratDef.slPct` (e.g. 0.4 = 0.4%). */
  slPct: number;
  takerFeePct: number;
  /** Fraction of equity to risk at SL + round-trip fees (e.g. 0.01 = 1%). */
  riskPctOfEquity: number;
  minNotional: number;
  maxNotional: number;
};

/**
 * Equity-curve aware sizing multiplier (P2.3.3, Kelly-lite).
 *
 * Adjusts position size based on the last N closed-trade outcomes (most-recent first):
 * - ≥2 consecutive losses → 0.5× (halve next notional)
 * - ≥3 consecutive wins   → restore to 1.0× (default; capped at 1.0 to avoid Kelly blow-up)
 *
 * Returns 1.0 when fewer than 2 trades in the window. Clamped to [0.5, 1.0].
 *
 * `recentNetPnls` is ordered most-recent-first (index 0 = last closed trade).
 */
export function paperEquityCurveSizingMul(recentNetPnls: ReadonlyArray<number>): number {
  if (!recentNetPnls || recentNetPnls.length < 2) return 1.0;
  let consecLosses = 0;
  for (const p of recentNetPnls) {
    if (Number.isFinite(p) && p < 0) consecLosses++;
    else break;
  }
  if (consecLosses >= 2) return 0.5;
  return 1.0;
}

/**
 * Target position notional so `paperEstimatedMaxLossAtStopSl` ≤ `equityUsd × riskPctOfEquity`,
 * then clamp to `[minNotional, maxNotional]`.
 *
 * Uses linear scaling: loss at notional `N` is `N × lossPerDollar` where `lossPerDollar` is
 * evaluated at `N = 1`. If no positive budget or invalid inputs → `minNotional`.
 */
export function paperNotionalForTargetRisk(args: PaperNotionalForTargetRiskArgs): number {
  const {
    equityUsd,
    markPrice,
    entryPrice: entryArg,
    side,
    slPct,
    takerFeePct,
    riskPctOfEquity,
    minNotional,
    maxNotional,
  } = args;

  const minN = Number.isFinite(minNotional) && minNotional > 0 ? minNotional : 100;
  const maxN = Number.isFinite(maxNotional) && maxNotional >= minN ? maxNotional : minN;

  if (!Number.isFinite(equityUsd) || equityUsd <= 0) return minN;
  if (!Number.isFinite(slPct) || slPct <= 0) return minN;
  if (!Number.isFinite(riskPctOfEquity) || riskPctOfEquity <= 0) return minN;

  const entry = Number.isFinite(entryArg) && (entryArg ?? 0) > 0 ? (entryArg as number) : markPrice;
  if (!Number.isFinite(entry) || entry <= 0) return minN;

  const slFrac = slPct / 100;
  const slPrice =
    side === "LONG" ? entry * (1 - slFrac) : entry * (1 + slFrac);
  if (!Number.isFinite(slPrice) || slPrice <= 0) return minN;

  const lossPerUnit = paperEstimatedMaxLossAtStopSl(entry, slPrice, 1, side, takerFeePct);
  if (!Number.isFinite(lossPerUnit) || lossPerUnit <= 0) return minN;

  const riskBudget = equityUsd * riskPctOfEquity;
  const raw = riskBudget / lossPerUnit;
  if (!Number.isFinite(raw) || raw <= 0) return minN;

  return Math.min(maxN, Math.max(minN, raw));
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

// ========== EXCHANGE-SPECIFIC FEE CALCULATORS ==========
// Rates locked to 2026 schedule. All return USD cost (positive = cost to trader).

/**
 * Binance USD-M Futures fees (2026 base tier).
 * Taker: 0.05%  |  Maker: 0.02%
 * Round-trip assumes both legs are taker unless `makerEntry` is specified.
 */
export type BinanceFeeParams = {
  notional: number;
  isTakerEntry?: boolean;
  isTakerExit?: boolean;
};
export function calcBinanceFutureFee(p: BinanceFeeParams): number {
  const TAKER = 0.0005;
  const MAKER = 0.0002;
  if (!Number.isFinite(p.notional) || p.notional <= 0) return 0;
  const entryFee = p.notional * (p.isTakerEntry === false ? MAKER : TAKER);
  const exitFee = p.notional * (p.isTakerExit === false ? MAKER : TAKER);
  return entryFee + exitFee;
}

/**
 * Delta Exchange fees (2026).
 * Futures: Taker 0.05% / Maker 0.02%
 * Options: 0.03% of notional, capped at min(3.5%, 7.5%) of option premium.
 *
 * Per-leg call (entry or exit) — multiply by 2 for round-trip futures.
 * For options, pass `optionPremiumUsd` and the cap is applied automatically.
 */
export type DeltaFeeParams = {
  notional: number;
  isFutures: boolean;
  isTaker?: boolean;
  /** USD value of the option premium (used only when !isFutures). */
  optionPremiumUsd?: number;
  /** Use 7.5% cap (USDT-settled indices) instead of 3.5% (default). */
  useHigherPremiumCap?: boolean;
};
export function calcDeltaFeePerLeg(p: DeltaFeeParams): number {
  if (!Number.isFinite(p.notional) || p.notional <= 0) return 0;
  if (p.isFutures) {
    const rate = p.isTaker === false ? 0.0002 : 0.0005;
    return p.notional * rate;
  }
  // Options: min(notional × 0.03%, premium × cap%)
  const notionalFee = p.notional * 0.0003;
  const premium = p.optionPremiumUsd ?? 0;
  if (premium > 0 && Number.isFinite(premium)) {
    const capPct = p.useHigherPremiumCap ? 0.075 : 0.035;
    return Math.min(notionalFee, premium * capPct);
  }
  return notionalFee;
}

/**
 * Profit-lock gate: only proceed with a TP close if net profit after
 * maximum expected slippage and all fees exceeds `minNetUsd`.
 *
 * Returns `true` when it is safe to take profit.
 */
export function paperProfitLockGate(
  grossPnl: number,
  fees: number,
  maxSlippageUsd: number,
  minNetUsd: number,
): boolean {
  if (!Number.isFinite(grossPnl) || !Number.isFinite(fees) || !Number.isFinite(maxSlippageUsd)) return false;
  const projected = grossPnl - fees - Math.abs(maxSlippageUsd);
  return projected >= minNetUsd;
}

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
  /** Paper discovery: skip SL until position age reaches this (TP/TIME/liq still apply). */
  minAgeBeforeSlMs?: number;
};

/**
 * Hard exits only, **same order as runtime** `resolveExit` (liq → SL → TP).
 * TIME exits are LEGACY ONLY — stored in DB for old trades.
 * This function MUST NOT return `TIME` for any new position.
 * `PaperHardExitReason` still includes `TIME` for type compatibility only.
 * Trailing / breakeven branches stay in the hook (need position mutation + progress).
 */
export function paperResolveHardExit(i: PaperHardExitInputs): PaperHardExitResult {
  if (i.side === "LONG" && i.markPrice <= i.liquidationPrice) {
    return { shouldClose: true, reason: "LIQUIDATION_RISK", exitPrice: i.liquidationPrice };
  }
  if (i.side === "SHORT" && i.markPrice >= i.liquidationPrice) {
    return { shouldClose: true, reason: "LIQUIDATION_RISK", exitPrice: i.liquidationPrice };
  }

  const ageMs = Math.max(0, i.nowMs - i.openedAtMs);
  const slAllowed =
    !Number.isFinite(i.minAgeBeforeSlMs) ||
    (i.minAgeBeforeSlMs ?? 0) <= 0 ||
    ageMs >= (i.minAgeBeforeSlMs as number);

  if (slAllowed && i.side === "LONG" && i.markPrice <= i.adaptiveSl) {
    return { shouldClose: true, reason: "SL", exitPrice: i.adaptiveSl };
  }
  if (slAllowed && i.side === "SHORT" && i.markPrice >= i.adaptiveSl) {
    return { shouldClose: true, reason: "SL", exitPrice: i.adaptiveSl };
  }

  if (i.side === "LONG" && i.markPrice >= i.tpPrice) {
    return { shouldClose: true, reason: "TP", exitPrice: i.tpPrice };
  }
  if (i.side === "SHORT" && i.markPrice <= i.tpPrice) {
    return { shouldClose: true, reason: "TP", exitPrice: i.tpPrice };
  }

  return { shouldClose: false };
}
