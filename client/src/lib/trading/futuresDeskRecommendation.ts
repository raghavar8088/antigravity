/**
 * futuresDeskRecommendation.ts
 * Pure closed-loop recommendation from desk telemetry. Never mutates state.
 */

import type { AttributionReport } from "../analytics/futuresAttribution";
import type { RotationReport } from "./futuresStrategyRotation";
import type { TuneRecommendation } from "./futuresParameterTuner";

export type DeskRecommendationAction =
  | "NO_CHANGE"
  | "RAISE_THRESHOLD"
  | "REDUCE_SAME_SIDE"
  | "BLOCK_WORST_HOUR"
  | "PROMOTE_BEST_HOLD"
  | "SUSPEND_WORST_STRAT";

export interface DeskRecommendation {
  action: DeskRecommendationAction;
  confidence: "LOW" | "MED" | "HIGH";
  rationale: string;
  suggestedValue?: number | string;
}

export function deriveDeskRecommendation(inputs: {
  attribution: AttributionReport | null;
  rotation: RotationReport | null;
  tune: TuneRecommendation | null;
  qualitySkipCount: number;
  mtfSkipCount: number;
  totalEvaluations: number;
}): DeskRecommendation {
  const {
    attribution,
    rotation,
    tune,
    qualitySkipCount,
    mtfSkipCount,
    totalEvaluations,
  } = inputs;

  const evalDenom = Math.max(totalEvaluations, 1);
  const qualitySkipRate = qualitySkipCount / evalDenom;
  const mtfSkipRate = mtfSkipCount / evalDenom;

  if (tune && tune.target !== "NO_CHANGE") {
    if (tune.target === "SIGNAL_THRESHOLD") {
      return {
        action: "RAISE_THRESHOLD",
        confidence: tune.confidence,
        rationale: tune.rationale,
        suggestedValue: tune.suggestedValue,
      };
    }
    if (tune.target === "SAME_SIDE_CAP") {
      return {
        action: "REDUCE_SAME_SIDE",
        confidence: tune.confidence,
        rationale: tune.rationale,
        suggestedValue: tune.suggestedValue,
      };
    }
    return {
      action: "NO_CHANGE",
      confidence: tune.confidence,
      rationale: `Tuner suggests ${tune.target}: ${tune.rationale}`,
      suggestedValue: tune.suggestedValue,
    };
  }

  if (qualitySkipRate > 0.6) {
    return {
      action: "RAISE_THRESHOLD",
      confidence: "HIGH",
      rationale:
        `Signal quality rejected ${(qualitySkipRate * 100).toFixed(0)}% of gate evaluations ` +
        `(${qualitySkipCount}/${totalEvaluations}). Raise threshold ~4 pts to filter marginal entries.`,
      suggestedValue: 4,
    };
  }

  if (mtfSkipRate > 0.5) {
    return {
      action: "NO_CHANGE",
      confidence: "MED",
      rationale:
        `MTF gate blocked ${(mtfSkipRate * 100).toFixed(0)}% of evaluations ` +
        `(${mtfSkipCount}/${totalEvaluations}). Consider stricter min confluence score (e.g. 60 vs 55).`,
      suggestedValue: "60",
    };
  }

  if (
    attribution?.bestHoldBucket &&
    attribution?.worstHoldBucket &&
    attribution.bestHoldBucket !== attribution.worstHoldBucket
  ) {
    return {
      action: "PROMOTE_BEST_HOLD",
      confidence: "MED",
      rationale:
        `Hold attribution: best bucket "${attribution.bestHoldBucket}" vs worst "${attribution.worstHoldBucket}". ` +
        `Favor strategies/holds aligned with shorter winners.`,
      suggestedValue: attribution.bestHoldBucket,
    };
  }

  if (attribution?.worstHour != null && attribution.worstHour !== attribution.bestHour) {
    const worstBucket = attribution.byHourOfDay.find(
      (b) => b.label === String(attribution.worstHour) && b.trades >= 3,
    );
    if (worstBucket && worstBucket.avgNetPnl < -5) {
      return {
        action: "BLOCK_WORST_HOUR",
        confidence: "MED",
        rationale:
          `UTC hour ${attribution.worstHour}:00 avg PnL $${worstBucket.avgNetPnl.toFixed(2)} ` +
          `over ${worstBucket.trades} trades — consider session filter.`,
        suggestedValue: attribution.worstHour,
      };
    }
  }

  if (rotation && rotation.suspended.length > 3) {
    const worst = rotation.suspended[0];
    return {
      action: "SUSPEND_WORST_STRAT",
      confidence: "MED",
      rationale:
        `${rotation.suspended.length} strategies rotation-suspended. ` +
        `Worst: ${worst?.strategyName ?? "unknown"} (score ${worst?.score ?? 0}). Review roster.`,
      suggestedValue: worst?.strategyId,
    };
  }

  return {
    action: "NO_CHANGE",
    confidence: "LOW",
    rationale: "Desk telemetry within normal bounds — no single action recommended.",
  };
}
