import type { RegimeType } from "@/internal/regime";

export interface CouncilClassifierFeatures {
  adx: number;
  rsi: number;
  emaAlignment: "BULLISH" | "BEARISH" | "MIXED";
  vwapDistancePct: number;
  volumeRatio: number;
  atrPct: number;
  regime: RegimeType;
}

export interface CouncilClassifierResult {
  decision: "Approve" | "Reject";
  confidence: number;
  score: number;
}

function clamp(n: number, min = 0, max = 1): number {
  return Math.min(max, Math.max(min, Number.isFinite(n) ? n : 0));
}

export class CouncilClassifier {
  infer(features: CouncilClassifierFeatures): CouncilClassifierResult {
    let score = 0.45;
    score += clamp(features.adx / 50) * 0.18;
    score += features.volumeRatio >= 1 ? 0.1 : -0.08;
    score += features.atrPct > 0.15 && features.atrPct < 1.2 ? 0.12 : -0.08;
    score += features.rsi > 35 && features.rsi < 65 ? 0.08 : -0.05;
    score += features.emaAlignment === "MIXED" ? -0.04 : 0.06;
    score += Math.abs(features.vwapDistancePct) <= 0.8 ? 0.06 : -0.04;
    score += features.regime === "VOLATILE" ? -0.08 : 0.04;

    const bounded = clamp(score);
    return {
      decision: bounded >= 0.55 ? "Approve" : "Reject",
      confidence: Math.round(Math.abs(bounded - 0.5) * 200),
      score: bounded,
    };
  }
}
