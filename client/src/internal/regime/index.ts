import type { FuturesSignalInputs } from "@/lib/futuresSignals";

export type RegimeType =
  | "TRENDING_BULL"
  | "TRENDING_BEAR"
  | "RANGING"
  | "VOLATILE"
  | "LOW_VOL";

export interface RegimeState {
  regime: RegimeType;
  adx: number;
  atrPct: number;
  emaSlope: number;
  vwapDeviationPct: number;
  volumeRatio: number;
  confidence: number;
  detectedAt: number;
}

function safePct(numerator: number, denominator: number): number {
  if (!Number.isFinite(numerator) || !Number.isFinite(denominator) || denominator === 0) return 0;
  return (numerator / denominator) * 100;
}

function clamp(n: number, min = 0, max = 100): number {
  return Math.min(max, Math.max(min, Number.isFinite(n) ? n : 0));
}

export class MarketRegimeEngine {
  detect(input: FuturesSignalInputs, now = Date.now()): RegimeState {
    const price = input.markPrice > 0 ? input.markPrice : input.price;
    const adx = input.adxProxy;
    const atrPct = safePct(input.atr14, price);
    const emaSlope = input.ema5 - input.prevEma5 + (input.ema13 - input.prevEma13);
    const vwapDeviationPct = safePct(input.vwapDev, price);
    const volumeRatio = input.volRatio;

    let regime: RegimeType = "RANGING";
    if (atrPct >= 1.2 || volumeRatio >= 2.2) {
      regime = "VOLATILE";
    } else if (atrPct <= 0.18 && adx < 14) {
      regime = "LOW_VOL";
    } else if (adx >= 24 && emaSlope > 0 && vwapDeviationPct >= -0.25) {
      regime = "TRENDING_BULL";
    } else if (adx >= 24 && emaSlope < 0 && vwapDeviationPct <= 0.25) {
      regime = "TRENDING_BEAR";
    }

    const confidence = clamp(
      adx * 1.8 +
      Math.min(30, atrPct * 12) +
      Math.min(20, Math.abs(emaSlope / Math.max(price, 1)) * 10000) +
      Math.min(20, volumeRatio * 8),
    );

    return {
      regime,
      adx,
      atrPct,
      emaSlope,
      vwapDeviationPct,
      volumeRatio,
      confidence,
      detectedAt: now,
    };
  }
}
