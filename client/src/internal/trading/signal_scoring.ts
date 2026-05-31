import type { Signal } from "@/internal/strategy/evaluator";

export interface SignalQualityFeatures {
  trendStrength: number;
  volumeConfirmation: number;
  volatilityContext: number;
  marketStructure: number;
  strategyHistoricalPerformance: number;
  regimeAligned: boolean;
}

export interface SignalScoreComponent {
  name: keyof SignalQualityFeatures | "confidence";
  value: number;
  weight: number;
  contribution: number;
  explanation: string;
}

export interface SignalQualityScore {
  SignalScore: number;
  signal: Signal;
  components: SignalScoreComponent[];
  explanation: string;
}

function clamp(n: number, min = 0, max = 100): number {
  return Math.min(max, Math.max(min, Number.isFinite(n) ? n : 0));
}

function component(
  name: SignalScoreComponent["name"],
  value: number,
  weight: number,
  explanation: string,
): SignalScoreComponent {
  const normalized = clamp(value);
  return {
    name,
    value: normalized,
    weight,
    contribution: normalized * weight,
    explanation,
  };
}

export class SignalQualityEngine {
  score(signal: Signal, features: SignalQualityFeatures): SignalQualityScore {
    const components = [
      component("trendStrength", features.trendStrength, 0.18, "ADX/EMA slope trend support"),
      component("volumeConfirmation", features.volumeConfirmation, 0.14, "volume and OBV confirmation"),
      component("volatilityContext", features.volatilityContext, 0.12, "ATR and volatility suitability"),
      component("marketStructure", features.marketStructure, 0.16, "breakout/range structure quality"),
      component("strategyHistoricalPerformance", features.strategyHistoricalPerformance, 0.18, "recent strategy expectancy"),
      component("regimeAligned", features.regimeAligned ? 100 : 0, 0.12, "strategy allowed in current regime"),
      component("confidence", signal.Confidence, 0.10, "raw deterministic strategy score"),
    ];
    const score = clamp(components.reduce((sum, c) => sum + c.contribution, 0));
    const strongest = [...components].sort((a, b) => b.contribution - a.contribution)[0];

    return {
      SignalScore: Math.round(score),
      signal,
      components,
      explanation: strongest
        ? `score=${Math.round(score)} led by ${strongest.explanation}`
        : `score=${Math.round(score)}`,
    };
  }
}
