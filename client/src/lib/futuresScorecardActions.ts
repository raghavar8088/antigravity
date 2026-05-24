/**
 * futuresScorecardActions.ts
 * One recommended desk fix from rolling scorecard — never auto-applied.
 */

import type { DeskRollingPnLScorecard } from "./futuresDeskPnLTracker";
import type { TuneRecommendation, TuneTarget } from "./futuresParameterTuner";
import type { RotationReport } from "./futuresStrategyRotation";

export type ScorecardActionSeverity = "OK" | "WARN" | "ACT";

export type ScorecardActionType =
  | "NO_CHANGE"
  | "RAISE_THRESHOLD"
  | "TIGHTEN_EXITS"
  | "REDUCE_ROSTER"
  | "BLOCK_WORST_HOUR"
  | "SUSPEND_WORST_STRAT";

export interface ScorecardAction {
  severity: ScorecardActionSeverity;
  action: ScorecardActionType;
  rationale: string;
  suggestedEnv?: Record<string, string>;
  worstStrategyId?: number;
  worstStrategyName?: string;
}

const FEE_ACT_PCT = 60;

function tuneToAction(tune: TuneRecommendation): ScorecardAction | null {
  if (tune.target === "NO_CHANGE") return null;

  const mapTarget = (target: TuneTarget): ScorecardActionType => {
    if (target === "SIGNAL_THRESHOLD") return "RAISE_THRESHOLD";
    if (target === "TP_PCT" || target === "SL_PCT" || target === "HOLD_MINUTES") {
      return "TIGHTEN_EXITS";
    }
    if (target === "SAME_SIDE_CAP") return "REDUCE_ROSTER";
    return "NO_CHANGE";
  };

  const action = mapTarget(tune.target);
  const suggestedEnv: Record<string, string> = {};

  if (tune.target === "SIGNAL_THRESHOLD") {
    suggestedEnv.NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD = String(Math.round(tune.suggestedValue));
  }
  if (tune.target === "SAME_SIDE_CAP") {
    suggestedEnv.NEXT_PUBLIC_DESK_PROFIT_MAX_SAME_SIDE_CHOP = String(
      Math.max(1, Math.floor(tune.suggestedValue)),
    );
  }
  if (tune.target === "TP_PCT" || tune.target === "SL_PCT") {
    suggestedEnv.NEXT_PUBLIC_DESK_PROFIT_EXIT_LOCK_MIN_NET = "0.20";
    suggestedEnv.NEXT_PUBLIC_DESK_PROFIT_EXIT_LOCK_PROGRESS = "0.60";
  }

  return {
    severity: tune.confidence === "HIGH" ? "ACT" : "WARN",
    action,
    rationale: `Parameter tuner: ${tune.rationale}`,
    suggestedEnv: Object.keys(suggestedEnv).length ? suggestedEnv : undefined,
  };
}

function worstFromRotation(rotation: RotationReport | null): {
  id?: number;
  name?: string;
} {
  const suspended = rotation?.suspended ?? [];
  const worst = suspended[0] ?? rotation?.scores[rotation.scores.length - 1];
  if (!worst) return {};
  return { id: worst.strategyId, name: worst.strategyName };
}

export function deriveScorecardAction(
  scorecard: DeskRollingPnLScorecard | null,
  tune: TuneRecommendation | null,
  rotation: RotationReport | null,
  currentThreshold = 28,
): ScorecardAction {
  if (!scorecard) {
    return {
      severity: "WARN",
      action: "NO_CHANGE",
      rationale: "No scorecard yet — need ≥5 production closes.",
    };
  }

  const { last50, last20, closes48h } = scorecard;

  if (
    last50.tradeCount >= 10 &&
    scorecard.passesFeeTarget50 &&
    scorecard.passesExpectancyTarget50
  ) {
    return {
      severity: "OK",
      action: "NO_CHANGE",
      rationale:
        `Last ${last50.tradeCount} closes on track: fee/gross ${last50.feePctOfAbsGross.toFixed(1)}%, ` +
        `expectancy $${last50.expectancy.toFixed(2)}/trade.`,
    };
  }

  if (closes48h < 10) {
    return {
      severity: "WARN",
      action: "NO_CHANGE",
      rationale: `Only ${closes48h} closes in 48h — collect more data before changing parameters.`,
    };
  }

  if (last50.tradeCount >= 10 && last50.feePctOfAbsGross > FEE_ACT_PCT) {
    return {
      severity: "ACT",
      action: "RAISE_THRESHOLD",
      rationale:
        `Fee drag ${last50.feePctOfAbsGross.toFixed(1)}% of |gross| on last ${last50.tradeCount} ` +
        `(target < ${scorecard.targets.feePctMax}%). Raise entry bar or tighten exits.`,
      suggestedEnv: {
        NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD: String(currentThreshold + 2),
        NEXT_PUBLIC_DESK_PROFIT_MIN_QUALITY: "75",
        NEXT_PUBLIC_DESK_PROFIT_EXIT_LOCK_MIN_NET: "0.20",
      },
    };
  }

  if (last20.tradeCount >= 10 && last20.expectancy < -5) {
    const worst = worstFromRotation(rotation);
    return {
      severity: "ACT",
      action: "SUSPEND_WORST_STRAT",
      rationale:
        `Last ${last20.tradeCount} closes: expectancy $${last20.expectancy.toFixed(2)}/trade. ` +
        `Review or runtime-block worst performer (do not auto-apply).`,
      worstStrategyId: worst.id,
      worstStrategyName: worst.name,
      suggestedEnv: worst.id
        ? {
            "# Manual": `Use Desk Monitor blocklist for strategy ${worst.id} (${worst.name ?? "unknown"})`,
          }
        : undefined,
    };
  }

  if (last50.tradeCount >= 10 && !scorecard.passesFeeTarget50) {
    return {
      severity: "ACT",
      action: "TIGHTEN_EXITS",
      rationale:
        `Fee/gross ${last50.feePctOfAbsGross.toFixed(1)}% still above ${scorecard.targets.feePctMax}% target. ` +
        `Tighten profit-lock and disable quick-TP micro exits.`,
      suggestedEnv: {
        NEXT_PUBLIC_DESK_PROFIT_EXIT_LOCK_MIN_NET: "0.20",
        NEXT_PUBLIC_DESK_PROFIT_EXIT_LOCK_PROGRESS: "0.60",
        NEXT_PUBLIC_DESK_PROFIT_DISABLE_QUICK_TP: "1",
      },
    };
  }

  const fromTune = tune ? tuneToAction(tune) : null;
  if (fromTune) return fromTune;

  if (scorecard.paperReadyHint === "REVIEW") {
    return {
      severity: "WARN",
      action: "NO_CHANGE",
      rationale:
        "Scorecard REVIEW — check win rate / profit factor on last 50 closes before promoting roster.",
    };
  }

  return {
    severity: "WARN",
    action: "NO_CHANGE",
    rationale: "Collect more closes to stabilize rolling metrics.",
  };
}

export function formatScorecardActionEnv(action: ScorecardAction): string {
  if (!action.suggestedEnv || !Object.keys(action.suggestedEnv).length) {
    return "# No env lines for this action — use Desk Monitor controls.";
  }
  return Object.entries(action.suggestedEnv)
    .map(([k, v]) => `${k}=${v}`)
    .join("\n");
}
