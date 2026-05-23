/**
 * Pure signal scoring helpers for the 160-strategy research pool.
 *
 * Each category has a dedicated scorer that consumes pre-computed
 * FuturesSignalInputs (built by buildSignalInputs in futuresSignals.ts).
 * All functions are side-effect-free and fully unit-testable.
 *
 * Return shape matches evalMinuteSignal: { score, reason }
 * score ≥ signalThreshold → entry candidate; confirmation gate is separate.
 *
 * Scoring philosophy:
 *   - Each check adds a specific point value to score.
 *   - The PRIMARY signal (the defining indicator for this strategy) scores 12–16.
 *   - Secondary confirms score 4–8.
 *   - Volume/regime quality adds 4–6.
 *   - A strong signal alone (~14 pts) + 2 confirms (~12 pts) = ~26 pts ≥ threshold.
 *   - Chop/noise conditions should consistently score ≤18.
 */

import type { FuturesSignalInputs } from "@/lib/futuresSignals";
import type { FuturesStratDef } from "@/lib/futuresStratTypes";

export type ScoringResult = { score: number; reason: string };

const NO_SIGNAL: ScoringResult = { score: 0, reason: "no_signal" };

// ─── Scalping (600–619) ───────────────────────────────────────────────────────

/**
 * Routes a Scalping strategy (SCP_ prefix) to its dedicated scoring branch.
 * Each branch implements the specific indicator thesis for that strategy.
 */
export function scoreScalping(s: FuturesSignalInputs, strat: FuturesStratDef): ScoringResult {
  let score = 0;
  const parts: string[] = [];
  const add = (pts: number, label: string) => { score += pts; parts.push(label); };
  const isShort = strat.signalKey.endsWith("_SHORT");

  switch (strat.signalKey) {
    // ── EMA Cross (600/601): EMA9 crosses EMA21, momentum confirms ──────────
    case "SCP_EMA_CROSS_LONG":
    case "SCP_EMA_CROSS_SHORT": {
      if (isShort) {
        if (s.fast < s.slow && s.prevFast >= s.prevSlow) add(16, "ema_cross_dn");
        else if (s.fast < s.slow) add(8, "ema_bearish");
        if (s.momentum3 < 0) add(7, "mom3_dn");
        if (s.rsi14 > 30 && s.rsi14 < 55) add(5, "rsi_bear_zone");
        if (s.volRatio > 1.1) add(4, "vol_ok");
        if (s.macdHist < 0) add(4, "macd_neg");
      } else {
        if (s.fast > s.slow && s.prevFast <= s.prevSlow) add(16, "ema_cross_up");
        else if (s.fast > s.slow) add(8, "ema_bullish");
        if (s.momentum3 > 0) add(7, "mom3_up");
        if (s.rsi14 > 45 && s.rsi14 < 70) add(5, "rsi_bull_zone");
        if (s.volRatio > 1.1) add(4, "vol_ok");
        if (s.macdHist > 0) add(4, "macd_pos");
      }
      break;
    }

    // ── VWAP Bounce (602/603): price returns to VWAP + RSI mid ─────────────
    case "SCP_VWAP_BOUNCE_LONG":
    case "SCP_VWAP_REJECT_SHORT": {
      const vwapPct = s.price > 0 ? s.vwapDev / s.price : 0;
      if (isShort) {
        // Price above VWAP, reject expected
        if (vwapPct > 0.003) add(14, "above_vwap_reject");
        else if (vwapPct > 0.001) add(8, "near_vwap_resist");
        if (s.rsi14 > 60 && s.rsi14 < 78) add(7, "rsi_extended");
        if (s.momentum3 < 0 && s.momentum6 < 0) add(6, "both_mom_neg");
        if (s.obvSlope < 0) add(4, "obv_dn");
        if (s.volRatio > 1.0) add(3, "vol_confirm");
      } else {
        // Price below VWAP, bounce expected
        if (vwapPct < -0.003) add(14, "below_vwap_bounce");
        else if (vwapPct < -0.001) add(8, "near_vwap_sup");
        if (s.rsi14 > 22 && s.rsi14 < 40) add(7, "rsi_oversold");
        if (s.momentum3 > 0 && s.momentum6 > 0) add(6, "both_mom_pos");
        if (s.obvSlope > 0) add(4, "obv_up");
        if (s.volRatio > 1.0) add(3, "vol_confirm");
      }
      break;
    }

    // ── RSI Snap (604/605): RSI extreme snap-back ────────────────────────────
    case "SCP_RSI_SNAP_LONG":
    case "SCP_RSI_SNAP_SHORT": {
      if (isShort) {
        if (s.rsi14 > 75) add(16, "rsi_overbought");
        else if (s.rsi14 > 68) add(10, "rsi_high");
        if (s.rsi7 > s.rsi14) add(6, "rsi7_extended");
        if (s.stochK > 80) add(6, "stoch_ob");
        if (s.momentum3 < 0) add(4, "mom_reversing");
        if (s.macdHist < s.prevMacdHist && s.macdHist > 0) add(4, "macd_peak");
      } else {
        if (s.rsi14 < 25) add(16, "rsi_oversold");
        else if (s.rsi14 < 32) add(10, "rsi_low");
        if (s.rsi7 < s.rsi14) add(6, "rsi7_depressed");
        if (s.stochK < 20) add(6, "stoch_os");
        if (s.momentum3 > 0) add(4, "mom_reversing");
        if (s.macdHist > s.prevMacdHist && s.macdHist < 0) add(4, "macd_trough");
      }
      break;
    }

    // ── BB Squeeze (606/607): Bollinger squeeze exit direction ──────────────
    case "SCP_BB_SQUEEZE_LONG":
    case "SCP_BB_SQUEEZE_SHORT": {
      const squeezed = s.bbWidth < 0.015;
      if (isShort) {
        if (squeezed && s.momentum3 < -s.atr14 * 0.5) add(16, "squeeze_breakdown");
        else if (squeezed && s.momentum3 < 0) add(10, "squeeze_dn");
        if (s.price < s.bbLower) add(7, "below_bb");
        if (s.rsi14 < 45) add(5, "rsi_dropping");
        if (s.volRatio > 1.3) add(5, "vol_spike");
        if (s.macdHist < s.prevMacdHist) add(4, "macd_falling");
      } else {
        if (squeezed && s.momentum3 > s.atr14 * 0.5) add(16, "squeeze_breakout");
        else if (squeezed && s.momentum3 > 0) add(10, "squeeze_up");
        if (s.price > s.bbUpper) add(7, "above_bb");
        if (s.rsi14 > 55) add(5, "rsi_rising");
        if (s.volRatio > 1.3) add(5, "vol_spike");
        if (s.macdHist > s.prevMacdHist) add(4, "macd_rising");
      }
      break;
    }

    // ── Micro Break (608/609): 5-bar structural high/low break ──────────────
    case "SCP_MICRO_BREAK_LONG":
    case "SCP_MICRO_BREAK_SHORT": {
      if (isShort) {
        if (s.price < s.low20) add(14, "20bar_low_break");
        else if (s.price < s.donchianLow * 1.002) add(9, "near_donchian_lo");
        if (s.momentum3 < -s.atr14 * 0.3) add(7, "mom_confirm");
        if (s.volRatio > 1.2) add(6, "vol_break");
        if (s.fast < s.slow) add(4, "ema_align");
        if (s.adxProxy > 18) add(4, "trend_active");
      } else {
        if (s.price > s.high20) add(14, "20bar_high_break");
        else if (s.price > s.donchianHigh * 0.998) add(9, "near_donchian_hi");
        if (s.momentum3 > s.atr14 * 0.3) add(7, "mom_confirm");
        if (s.volRatio > 1.2) add(6, "vol_break");
        if (s.fast > s.slow) add(4, "ema_align");
        if (s.adxProxy > 18) add(4, "trend_active");
      }
      break;
    }

    // ── Order Book Imbalance (610/611): OBV surge + volume ──────────────────
    case "SCP_OBI_IMBAL_LONG":
    case "SCP_OBI_IMBAL_SHORT": {
      if (isShort) {
        if (s.obvSlope < -Math.abs(s.atr14) * 8) add(16, "obv_strong_dn");
        else if (s.obvSlope < 0) add(9, "obv_dn");
        if (s.volRatio > 1.8) add(8, "vol_imbal");
        if (s.momentum3 < 0) add(5, "price_follows");
        if (s.rsi14 > 50) add(4, "rsi_elevated");
        if (s.macdHist < 0) add(3, "macd_neg");
      } else {
        if (s.obvSlope > s.atr14 * 8) add(16, "obv_strong_up");
        else if (s.obvSlope > 0) add(9, "obv_up");
        if (s.volRatio > 1.8) add(8, "vol_imbal");
        if (s.momentum3 > 0) add(5, "price_follows");
        if (s.rsi14 < 50) add(4, "rsi_depressed");
        if (s.macdHist > 0) add(3, "macd_pos");
      }
      break;
    }

    // ── Momentum Tick (612/613): short-burst momentum surge ─────────────────
    case "SCP_MOM_TICK_LONG":
    case "SCP_MOM_TICK_SHORT": {
      if (isShort) {
        const momThresh = s.atr14 * 0.7;
        if (s.momentum3 < -momThresh) add(14, "mom3_surge_dn");
        if (s.roc10 < -0.5) add(8, "roc_neg");
        if (s.momentum6 < s.momentum3) add(6, "accel_dn");
        if (s.volZ30 > 1.0) add(5, "vol_z_elevated");
        if (s.macdHist < s.prevMacdHist) add(4, "macd_accel_dn");
      } else {
        const momThresh = s.atr14 * 0.7;
        if (s.momentum3 > momThresh) add(14, "mom3_surge_up");
        if (s.roc10 > 0.5) add(8, "roc_pos");
        if (s.momentum6 > s.momentum3) add(6, "accel_up");
        if (s.volZ30 > 1.0) add(5, "vol_z_elevated");
        if (s.macdHist > s.prevMacdHist) add(4, "macd_accel_up");
      }
      break;
    }

    // ── Liquidity Sweep (614/615): sweep of high/low then reversal ──────────
    case "SCP_LIQ_SWEEP_LONG":
    case "SCP_LIQ_SWEEP_SHORT": {
      if (isShort) {
        // Sweep above 20-bar high then price falls back
        const sweptHigh = s.prevPrice > s.high20 && s.price <= s.high20;
        if (sweptHigh) add(16, "liq_sweep_hi");
        else if (s.price > s.high20 * 0.998) add(8, "near_sweep_hi");
        if (s.momentum3 < 0 && s.prevPrice > s.price) add(7, "reversal_confirm");
        if (s.rsi14 > 65) add(5, "rsi_extended_hi");
        if (s.volRatio > 1.5) add(5, "sweep_vol");
        if (s.stochK > 75) add(3, "stoch_ob");
      } else {
        // Sweep below 20-bar low then price recovers
        const sweptLow = s.prevPrice < s.low20 && s.price >= s.low20;
        if (sweptLow) add(16, "liq_sweep_lo");
        else if (s.price < s.low20 * 1.002) add(8, "near_sweep_lo");
        if (s.momentum3 > 0 && s.prevPrice < s.price) add(7, "reversal_confirm");
        if (s.rsi14 < 35) add(5, "rsi_depressed_lo");
        if (s.volRatio > 1.5) add(5, "sweep_vol");
        if (s.stochK < 25) add(3, "stoch_os");
      }
      break;
    }

    // ── Session Open (616/617): opening drive with trend direction ───────────
    case "SCP_SESSION_OPEN_LONG":
    case "SCP_SESSION_OPEN_SHORT": {
      // Uses UTC hour from lastBarTimeMs (same approach as SESSION_OPEN template)
      const sessionHour = s.lastBarTimeMs != null
        ? new Date(s.lastBarTimeMs).getUTCHours()
        : -1;
      const isSessionOpen =
        (sessionHour >= 8 && sessionHour < 10) ||  // London
        (sessionHour >= 13 && sessionHour < 15);    // NY
      const sessionBonus = isSessionOpen ? 8 : 2;
      if (isShort) {
        add(sessionBonus, isSessionOpen ? "session_window" : "off_hours");
        if (s.fast < s.slow) add(8, "ema_bearish");
        if (s.momentum3 < 0 && s.momentum6 < 0) add(7, "mom_aligned_dn");
        if (s.volRatio > 1.3) add(5, "open_vol");
        if (s.rsi14 > 50) add(4, "rsi_elevated");
      } else {
        add(sessionBonus, isSessionOpen ? "session_window" : "off_hours");
        if (s.fast > s.slow) add(8, "ema_bullish");
        if (s.momentum3 > 0 && s.momentum6 > 0) add(7, "mom_aligned_up");
        if (s.volRatio > 1.3) add(5, "open_vol");
        if (s.rsi14 < 50) add(4, "rsi_depressed");
      }
      break;
    }

    // ── ATR Pullback (618/619): trend + ATR-measured pullback entry ──────────
    case "SCP_ATR_PULLBACK_LONG":
    case "SCP_ATR_PULLBACK_SHORT": {
      if (isShort) {
        // Trend is down, price pulls back up toward EMA, then resumes
        if (s.fast < s.slow && s.adxProxy > 20) add(12, "downtrend_adx");
        const pullback = s.price > s.fast && s.price < s.slow;
        if (pullback) add(10, "pullback_ema_zone");
        else if (s.price > s.fast * 0.998) add(5, "near_fast_ema");
        if (s.momentum3 < 0) add(7, "momentum_resumes");
        if (s.rsi14 > 40 && s.rsi14 < 60) add(5, "rsi_mid_pullback");
        if (s.volRatio < 1.2) add(3, "low_vol_pb");
      } else {
        // Trend is up, price pulls back down toward EMA
        if (s.fast > s.slow && s.adxProxy > 20) add(12, "uptrend_adx");
        const pullback = s.price < s.fast && s.price > s.slow;
        if (pullback) add(10, "pullback_ema_zone");
        else if (s.price < s.fast * 1.002) add(5, "near_fast_ema");
        if (s.momentum3 > 0) add(7, "momentum_resumes");
        if (s.rsi14 > 40 && s.rsi14 < 60) add(5, "rsi_mid_pullback");
        if (s.volRatio < 1.2) add(3, "low_vol_pb");
      }
      break;
    }

    default:
      return NO_SIGNAL;
  }

  return { score, reason: parts.join(",") || "no_signal" };
}

// ─── Stubs for categories 2–8 (filled in subsequent PRs) ──────────────────────

// ─── Day Trading (620–639) ───────────────────────────────────────────────────
// Primary TF: 5m (tests use 5m-shaped fixtures; same buildSignalInputs path).
// Confirm TF: 15m via htf15_* fields when requiresHtf.
//
// Score scale matches scalping (0–100). A canonical "valid setup" pairs the
// PRIMARY indicator (12–16 pts) with 2–3 confirms to clear a ~26-pt threshold.

export function scoreDay(s: FuturesSignalInputs, strat: FuturesStratDef): ScoringResult {
  let score = 0;
  const parts: string[] = [];
  const add = (pts: number, label: string) => { score += pts; parts.push(label); };
  const isShort = strat.signalKey.endsWith("_SHORT");

  switch (strat.signalKey) {
    // ── VWAP Trend (620/621): price vs VWAP + slope confirm ─────────────────
    case "DAY_VWAP_TREND_LONG":
    case "DAY_VWAP_TREND_SHORT": {
      const vwapPct = s.price > 0 ? s.vwapDev / s.price : 0;
      if (isShort) {
        if (vwapPct < -0.002) add(14, "below_vwap");
        else if (vwapPct < 0) add(7, "near_vwap_dn");
        if (s.fast < s.slow) add(8, "ema_bearish");
        if (s.momentum6 < 0 && s.momentum3 < 0) add(7, "mom_aligned_dn");
        if (s.htf15_fast < s.htf15_slow) add(6, "htf15_bear");
        if (s.adxProxy > 20) add(5, "trend_strength");
      } else {
        if (vwapPct > 0.002) add(14, "above_vwap");
        else if (vwapPct > 0) add(7, "near_vwap_up");
        if (s.fast > s.slow) add(8, "ema_bullish");
        if (s.momentum6 > 0 && s.momentum3 > 0) add(7, "mom_aligned_up");
        if (s.htf15_fast > s.htf15_slow) add(6, "htf15_bull");
        if (s.adxProxy > 20) add(5, "trend_strength");
      }
      break;
    }

    // ── ORB Break (622/623): first-range break with volume ──────────────────
    case "DAY_ORB_BREAK_LONG":
    case "DAY_ORB_BREAK_SHORT": {
      if (isShort) {
        if (s.price < s.low20) add(14, "orb_low_break");
        else if (s.price < s.donchianLow * 1.001) add(8, "near_orb_lo");
        if (s.volRatio > 1.2) add(7, "vol_confirm");
        if (s.momentum3 < -s.atr14 * 0.3) add(6, "thrust_dn");
        if (s.adxProxy > 18) add(4, "adx_active");
        if (s.macdHist < 0) add(3, "macd_neg");
      } else {
        if (s.price > s.high20) add(14, "orb_high_break");
        else if (s.price > s.donchianHigh * 0.999) add(8, "near_orb_hi");
        if (s.volRatio > 1.2) add(7, "vol_confirm");
        if (s.momentum3 > s.atr14 * 0.3) add(6, "thrust_up");
        if (s.adxProxy > 18) add(4, "adx_active");
        if (s.macdHist > 0) add(3, "macd_pos");
      }
      break;
    }

    // ── MTF Align (624/625): 5m + 15m same direction ────────────────────────
    case "DAY_MTF_ALIGN_LONG":
    case "DAY_MTF_ALIGN_SHORT": {
      const ltfBull = s.fast > s.slow;
      const htfBull = s.htf15_fast > s.htf15_slow;
      if (isShort) {
        if (!ltfBull && !htfBull) add(14, "both_tf_bearish");
        else if (!ltfBull) add(6, "ltf_only_bear");
        if (s.htf15_rsi < 50) add(7, "htf_rsi_bear");
        if (s.momentum3 < 0 && s.momentum6 < 0) add(6, "mom_aligned");
        if (s.htf15_macdHist < 0) add(5, "htf_macd_neg");
        if (s.adxProxy > 18) add(3, "adx_ok");
      } else {
        if (ltfBull && htfBull) add(14, "both_tf_bullish");
        else if (ltfBull) add(6, "ltf_only_bull");
        if (s.htf15_rsi > 50) add(7, "htf_rsi_bull");
        if (s.momentum3 > 0 && s.momentum6 > 0) add(6, "mom_aligned");
        if (s.htf15_macdHist > 0) add(5, "htf_macd_pos");
        if (s.adxProxy > 18) add(3, "adx_ok");
      }
      break;
    }

    // ── MACD Zero (626/627): MACD line cross zero + hist expanding ──────────
    case "DAY_MACD_ZERO_LONG":
    case "DAY_MACD_ZERO_SHORT": {
      if (isShort) {
        if (s.macdLine < 0 && s.prevMacdLine >= 0) add(16, "macd_zero_cross_dn");
        else if (s.macdLine < 0) add(8, "macd_below_zero");
        if (s.macdHist < 0 && s.macdHist < s.prevMacdHist) add(8, "macd_hist_expand_dn");
        if (s.fast < s.slow) add(5, "ema_bear");
        if (s.rsi14 < 50) add(4, "rsi_dn");
        if (s.momentum3 < 0) add(3, "mom3_dn");
      } else {
        if (s.macdLine > 0 && s.prevMacdLine <= 0) add(16, "macd_zero_cross_up");
        else if (s.macdLine > 0) add(8, "macd_above_zero");
        if (s.macdHist > 0 && s.macdHist > s.prevMacdHist) add(8, "macd_hist_expand_up");
        if (s.fast > s.slow) add(5, "ema_bull");
        if (s.rsi14 > 50) add(4, "rsi_up");
        if (s.momentum3 > 0) add(3, "mom3_up");
      }
      break;
    }

    // ── EMA Pullback (628/629): trend + pullback touch + reject ─────────────
    case "DAY_PB_EMA_LONG":
    case "DAY_PB_EMA_SHORT": {
      if (isShort) {
        if (s.htf15_fast < s.htf15_slow) add(10, "htf15_downtrend");
        const pullbackZone = s.price > s.fast && s.price < s.slow;
        if (pullbackZone) add(10, "pullback_to_ema");
        else if (s.price > s.fast * 0.998) add(5, "near_fast_ema");
        if (s.momentum3 < 0) add(7, "rejection_dn");
        if (s.rsi14 > 40 && s.rsi14 < 60) add(5, "rsi_mid");
        if (s.fast < s.slow) add(4, "ltf_aligned");
      } else {
        if (s.htf15_fast > s.htf15_slow) add(10, "htf15_uptrend");
        const pullbackZone = s.price < s.fast && s.price > s.slow;
        if (pullbackZone) add(10, "pullback_to_ema");
        else if (s.price < s.fast * 1.002) add(5, "near_fast_ema");
        if (s.momentum3 > 0) add(7, "rejection_up");
        if (s.rsi14 > 40 && s.rsi14 < 60) add(5, "rsi_mid");
        if (s.fast > s.slow) add(4, "ltf_aligned");
      }
      break;
    }

    // ── Range Expansion (630/631): BB width rising + direction ──────────────
    case "DAY_RANGE_EXP_LONG":
    case "DAY_RANGE_EXP_SHORT": {
      const expanding = s.bbWidth > 0.018 && s.atr14 > s.atr14Avg30;
      if (isShort) {
        if (expanding && s.momentum3 < -s.atr14 * 0.4) add(16, "range_expand_dn");
        else if (expanding) add(8, "expansion_dn");
        if (s.price < s.bbLower) add(7, "below_bb_dn");
        if (s.volRatio > 1.3) add(5, "vol_expand");
        if (s.fast < s.slow) add(4, "ema_dn");
        if (s.adxProxy > 22) add(3, "adx_rising");
      } else {
        if (expanding && s.momentum3 > s.atr14 * 0.4) add(16, "range_expand_up");
        else if (expanding) add(8, "expansion_up");
        if (s.price > s.bbUpper) add(7, "above_bb_up");
        if (s.volRatio > 1.3) add(5, "vol_expand");
        if (s.fast > s.slow) add(4, "ema_up");
        if (s.adxProxy > 22) add(3, "adx_rising");
      }
      break;
    }

    // ── Volume Climax (632/633): vol > 2× avg + wide-range bar direction ────
    case "DAY_VOL_CLIMAX_LONG":
    case "DAY_VOL_CLIMAX_SHORT": {
      const climax = s.volRatio > 2.0 || s.volZ30 > 1.5;
      if (isShort) {
        if (climax && s.price < s.prevPrice) add(16, "vol_climax_dn");
        else if (s.volRatio > 1.5) add(8, "high_vol");
        if (s.momentum3 < -s.atr14 * 0.5) add(7, "wide_range_dn");
        if (s.obvSlope < 0) add(5, "obv_dn");
        if (s.fast < s.slow) add(4, "ema_dn");
        if (s.macdHist < s.prevMacdHist) add(3, "macd_falling");
      } else {
        if (climax && s.price > s.prevPrice) add(16, "vol_climax_up");
        else if (s.volRatio > 1.5) add(8, "high_vol");
        if (s.momentum3 > s.atr14 * 0.5) add(7, "wide_range_up");
        if (s.obvSlope > 0) add(5, "obv_up");
        if (s.fast > s.slow) add(4, "ema_up");
        if (s.macdHist > s.prevMacdHist) add(3, "macd_rising");
      }
      break;
    }

    // ── Market Structure (634/635): HH+HL (long) / LL+LH (short) ────────────
    case "DAY_STRUCT_HH_LONG":
    case "DAY_STRUCT_LL_SHORT": {
      if (isShort) {
        // LL+LH: price below 20-bar low region + EMA aligned
        if (s.price < s.low20 * 1.005) add(12, "lower_low_zone");
        if (s.price < s.mean20) add(6, "below_mean");
        if (s.htf15_fast < s.htf15_slow) add(8, "htf_struct_dn");
        if (s.fast < s.slow && s.prevFast < s.prevSlow) add(6, "persistent_bear");
        if (s.momentum6 < 0) add(5, "momentum_aligned");
        if (s.rsi14 < 50) add(3, "rsi_dn");
      } else {
        // HH+HL: price near 20-bar high + EMA aligned
        if (s.price > s.high20 * 0.995) add(12, "higher_high_zone");
        if (s.price > s.mean20) add(6, "above_mean");
        if (s.htf15_fast > s.htf15_slow) add(8, "htf_struct_up");
        if (s.fast > s.slow && s.prevFast > s.prevSlow) add(6, "persistent_bull");
        if (s.momentum6 > 0) add(5, "momentum_aligned");
        if (s.rsi14 > 50) add(3, "rsi_up");
      }
      break;
    }

    // ── Midday Fade (636/637): mean revert in UTC 11–14 window ──────────────
    case "DAY_MIDDAY_FADE_LONG":
    case "DAY_MIDDAY_FADE_SHORT": {
      const hour = s.lastBarTimeMs != null ? new Date(s.lastBarTimeMs).getUTCHours() : -1;
      const middayWindow = hour >= 11 && hour < 14;
      const windowBonus = middayWindow ? 10 : 2;
      const vwapPct = s.price > 0 ? s.vwapDev / s.price : 0;
      if (isShort) {
        add(windowBonus, middayWindow ? "midday_window" : "off_window");
        if (vwapPct > 0.003) add(10, "above_vwap_fade");
        else if (vwapPct > 0.001) add(5, "vwap_extended_up");
        if (s.rsi14 > 60) add(6, "rsi_overextended");
        if (s.adxProxy < 22) add(5, "low_adx_revert");
        if (s.momentum3 < 0) add(4, "fade_starting");
      } else {
        add(windowBonus, middayWindow ? "midday_window" : "off_window");
        if (vwapPct < -0.003) add(10, "below_vwap_fade");
        else if (vwapPct < -0.001) add(5, "vwap_extended_dn");
        if (s.rsi14 < 40) add(6, "rsi_oversold");
        if (s.adxProxy < 22) add(5, "low_adx_revert");
        if (s.momentum3 > 0) add(4, "fade_starting");
      }
      break;
    }

    // ── Close Momentum (638/639): late-session momentum push ────────────────
    case "DAY_CLOSE_MOM_LONG":
    case "DAY_CLOSE_MOM_SHORT": {
      const hour = s.lastBarTimeMs != null ? new Date(s.lastBarTimeMs).getUTCHours() : -1;
      const lateWindow = hour >= 19 && hour < 21;
      const windowBonus = lateWindow ? 8 : 2;
      if (isShort) {
        add(windowBonus, lateWindow ? "close_window" : "off_window");
        if (s.momentum3 < -s.atr14 * 0.4) add(12, "close_mom_dn");
        if (s.fast < s.slow) add(6, "ema_aligned");
        if (s.volRatio > 1.2) add(5, "vol_confirm");
        if (s.macdHist < 0 && s.macdHist < s.prevMacdHist) add(4, "macd_accel_dn");
        if (s.rsi14 < 50) add(3, "rsi_dn");
      } else {
        add(windowBonus, lateWindow ? "close_window" : "off_window");
        if (s.momentum3 > s.atr14 * 0.4) add(12, "close_mom_up");
        if (s.fast > s.slow) add(6, "ema_aligned");
        if (s.volRatio > 1.2) add(5, "vol_confirm");
        if (s.macdHist > 0 && s.macdHist > s.prevMacdHist) add(4, "macd_accel_up");
        if (s.rsi14 > 50) add(3, "rsi_up");
      }
      break;
    }

    default:
      return NO_SIGNAL;
  }

  return { score, reason: parts.join(",") || "no_signal" };
}

export function scoreSwing(_s: FuturesSignalInputs, _strat: FuturesStratDef): ScoringResult {
  return NO_SIGNAL;
}

export function scorePosition(_s: FuturesSignalInputs, _strat: FuturesStratDef): ScoringResult {
  return NO_SIGNAL;
}

export function scoreTrend(_s: FuturesSignalInputs, _strat: FuturesStratDef): ScoringResult {
  return NO_SIGNAL;
}

export function scoreRange(_s: FuturesSignalInputs, _strat: FuturesStratDef): ScoringResult {
  return NO_SIGNAL;
}

export function scoreBreakout(_s: FuturesSignalInputs, _strat: FuturesStratDef): ScoringResult {
  return NO_SIGNAL;
}

export function scoreMomentum(_s: FuturesSignalInputs, _strat: FuturesStratDef): ScoringResult {
  return NO_SIGNAL;
}

/**
 * Top-level dispatcher — called from evalMinuteSignal for SCP_/DAY_/... keys.
 * Returns NO_SIGNAL for unimplemented categories (safe: score=0 never triggers).
 */
export function scoreCategoryStrategy(
  s: FuturesSignalInputs,
  strat: FuturesStratDef,
): ScoringResult {
  const key = strat.signalKey;
  if (key.startsWith("SCP_")) return scoreScalping(s, strat);
  if (key.startsWith("DAY_")) return scoreDay(s, strat);
  if (key.startsWith("SWG_")) return scoreSwing(s, strat);
  if (key.startsWith("POS_")) return scorePosition(s, strat);
  if (key.startsWith("TRD_")) return scoreTrend(s, strat);
  if (key.startsWith("RNG_")) return scoreRange(s, strat);
  if (key.startsWith("BRK_")) return scoreBreakout(s, strat);
  if (key.startsWith("MOM_")) return scoreMomentum(s, strat);
  return NO_SIGNAL;
}
