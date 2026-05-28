/**
 * Perp market feature extractor.
 *
 * Pure function — no I/O. Converts raw tick data (funding rate, mark price,
 * ATR, volume) into a structured feature record suitable for:
 *   - Research edge reports
 *   - Verification track events
 *   - Signal trace context
 *   - Edge Lab UI
 *
 * Features intentionally do NOT trigger trades. No execution logic here.
 */

// ─── Public types ─────────────────────────────────────────────────────────────

export interface PerpMarketFeatures {
  /** Raw funding rate for this interval (e.g. 0.0001 = 0.01%). */
  fundingRate: number;
  /** Funding rate annualised as a percentage (3 intervals/day × 365 × fundingRate × 100). */
  fundingAnnualized: number;
  /** Z-score of current funding vs recent history; null if < 3 history points. */
  fundingZScore: number | null;
  /** (markPrice − lastPrice) / lastPrice × 100. 0 if lastPrice unavailable. */
  markLastSpreadPct: number;
  /** ATR as a % of mark price. 0 if ATR unavailable. */
  volatilityAtrPct: number;
  /** Z-score of current volume vs recent history; null if unavailable. */
  volumeZScore: number | null;
  /** Broad regime classification, passed through from the engine. */
  regime: "chop" | "trendLow" | "trendHigh";
  /** Round-trip fee as a % of underlying price (taker+taker default = 0.2%). */
  feeHurdlePct: number;
}

export interface PerpMarketFeaturesInput {
  fundingRate: number;
  markPrice: number;
  lastPrice?: number;
  atr?: number;
  volume?: number;
  /** Recent funding rate history (most-recent-last) for z-score computation. */
  recentFundingHistory?: number[];
  /** Recent volume history (most-recent-last) for z-score computation. */
  recentVolumeHistory?: number[];
  regime: "chop" | "trendLow" | "trendHigh";
  /** Taker fee fraction (default 0.001 = 0.1%). */
  takerFeePct?: number;
  /** Maker fee fraction (default 0.001 = 0.1%). */
  makerFeePct?: number;
}

// ─── Constants ────────────────────────────────────────────────────────────────

/** Delta Exchange India perp funding fires every 8 h → 3 per day → 1095/year. */
const FUNDING_INTERVALS_PER_YEAR = 3 * 365;
const DEFAULT_FEE = 0.001; // 0.1% taker

// ─── Math helpers ─────────────────────────────────────────────────────────────

function sampleZScore(value: number, history: number[]): number | null {
  if (history.length < 3) return null;
  const n = history.length;
  const mean = history.reduce((s, v) => s + v, 0) / n;
  const variance = history.reduce((s, v) => s + (v - mean) ** 2, 0) / n;
  const std = Math.sqrt(variance);
  return std > 1e-12 ? (value - mean) / std : null;
}

// ─── Main export ──────────────────────────────────────────────────────────────

/**
 * Extract perp market features from a single tick's data.
 * All fields that require history gracefully return null when data is absent.
 */
export function extractPerpMarketFeatures(
  input: PerpMarketFeaturesInput,
): PerpMarketFeatures {
  const {
    fundingRate,
    markPrice,
    lastPrice,
    atr,
    volume,
    recentFundingHistory = [],
    recentVolumeHistory = [],
    regime,
    takerFeePct = DEFAULT_FEE,
    makerFeePct = DEFAULT_FEE,
  } = input;

  const fundingAnnualized = fundingRate * FUNDING_INTERVALS_PER_YEAR * 100;
  const fundingZScore = sampleZScore(fundingRate, recentFundingHistory);

  const markLastSpreadPct =
    lastPrice && lastPrice > 0
      ? ((markPrice - lastPrice) / lastPrice) * 100
      : 0;

  const volatilityAtrPct =
    atr !== undefined && atr > 0 && markPrice > 0
      ? (atr / markPrice) * 100
      : 0;

  const volumeZScore =
    volume !== undefined ? sampleZScore(volume, recentVolumeHistory) : null;

  // Conservative hurdle: taker in + taker out (worst case for liquidity takers).
  // Use maker fees if available for a tighter estimate; we default to taker+taker.
  const feeHurdlePct = (takerFeePct + Math.max(takerFeePct, makerFeePct)) * 100;

  return {
    fundingRate,
    fundingAnnualized,
    fundingZScore,
    markLastSpreadPct,
    volatilityAtrPct,
    volumeZScore,
    regime,
    feeHurdlePct,
  };
}
