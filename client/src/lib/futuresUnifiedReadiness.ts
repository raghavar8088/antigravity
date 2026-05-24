/**
 * futuresUnifiedReadiness.ts
 * Single paper-desk readiness state from scorecard + go-live + soak + replay.
 */

import type { GoLiveGateReport } from "./futuresGoLiveGates";
import type { DeskRollingPnLScorecard } from "./futuresDeskPnLTracker";
import type { soakTrendSummary } from "./futuresSoakTracker";

export type UnifiedReadiness =
  | "NOT_READY"
  | "COLLECT_DATA"
  | "PAPER_EDGE_OK"
  | "PAPER_READY"
  | "TESTNET_SOAK_READY";

const MAX_REPLAY_SIGN_FLIP = 0.15;
const MIN_GREEN_SOAK_DAYS = 7;

export function computeUnifiedReadiness(inputs: {
  scorecard: DeskRollingPnLScorecard | null;
  goLive: GoLiveGateReport | null;
  soak: ReturnType<typeof soakTrendSummary>;
  replaySignFlipRate: number | null;
  profitModeEnabled: boolean;
}): { state: UnifiedReadiness; blockers: string[]; nextStep: string } {
  const blockers: string[] = [];
  const { scorecard, goLive, soak, replaySignFlipRate } = inputs;

  if (!scorecard || scorecard.last20.tradeCount < 5) {
    return {
      state: "COLLECT_DATA",
      blockers: ["Need ≥5 production closes for scorecard"],
      nextStep: "Keep profit mode on and let the desk run until 20+ closes accumulate.",
    };
  }

  if (scorecard.closes48h < 10) {
    blockers.push(`Only ${scorecard.closes48h} closes in 48h (target ≥10/day pace)`);
  }

  const edgeOk =
    scorecard.paperReadyHint === "ON_TRACK" ||
    (scorecard.passesFeeTarget50 && scorecard.passesExpectancyTarget50);

  if (!edgeOk) {
    blockers.push("Rolling scorecard not ON_TRACK (fee/expectancy targets)");
    if (scorecard.last50.feePctOfAbsGross > scorecard.targets.feePctMax) {
      blockers.push(`Fee/gross ${scorecard.last50.feePctOfAbsGross.toFixed(1)}% > ${scorecard.targets.feePctMax}%`);
    }
    if (scorecard.last50.expectancy <= scorecard.targets.expectancyMin) {
      blockers.push(`Expectancy $${scorecard.last50.expectancy.toFixed(2)} ≤ $0`);
    }
  }

  const goLivePass = goLive?.allBlockersPass === true;
  if (!goLivePass) {
    blockers.push("Go-live blocker gates failing (see Go-live panel)");
    const failed = goLive?.blockers.filter((g) => !g.pass) ?? [];
    for (const g of failed.slice(0, 3)) {
      blockers.push(`${g.label}: ${g.value}`);
    }
  }

  if (replaySignFlipRate == null) {
    if (inputs.profitModeEnabled) {
      blockers.push("Replay compare not run (enable NEXT_PUBLIC_DESK_REPLAY_GATE=1)");
    }
  } else if (replaySignFlipRate > MAX_REPLAY_SIGN_FLIP) {
    blockers.push(
      `Replay sign-flip ${(replaySignFlipRate * 100).toFixed(1)}% > ${(MAX_REPLAY_SIGN_FLIP * 100).toFixed(0)}%`,
    );
  }

  if (soak.greenDays < MIN_GREEN_SOAK_DAYS) {
    blockers.push(`Soak: ${soak.greenDays}/${MIN_GREEN_SOAK_DAYS} green days in last 7`);
  }

  if (blockers.length && !edgeOk) {
    return {
      state: "NOT_READY",
      blockers,
      nextStep:
        "Fix fee bleed and negative expectancy first — review Scorecard action panel and strict exits.",
    };
  }

  if (blockers.some((b) => b.includes("48h")) || scorecard.last50.tradeCount < 50) {
    return {
      state: "COLLECT_DATA",
      blockers,
      nextStep: "Continue paper trading daily; target 50 closes and 7 green soak days before testnet.",
    };
  }

  if (edgeOk && !goLivePass) {
    return {
      state: "PAPER_EDGE_OK",
      blockers,
      nextStep: "Edge metrics look good — clear remaining go-live blockers (sample size, readiness, replay).",
    };
  }

  if (edgeOk && goLivePass && (replaySignFlipRate == null || replaySignFlipRate > MAX_REPLAY_SIGN_FLIP)) {
    return {
      state: "PAPER_READY",
      blockers,
      nextStep:
        "Run replay compare on recent UTC days: npm run replay:compare -- --account_key=<key> --date=YYYY-MM-DD. " +
        "Set NEXT_PUBLIC_DESK_REPLAY_GATE=1 for automatic gate feed.",
    };
  }

  if (edgeOk && goLivePass && soak.greenDays < MIN_GREEN_SOAK_DAYS) {
    return {
      state: "PAPER_READY",
      blockers,
      nextStep: `Maintain green daily soak grades (${soak.greenDays}/${MIN_GREEN_SOAK_DAYS} so far).`,
    };
  }

  if (edgeOk && goLivePass && replaySignFlipRate != null && soak.greenDays >= MIN_GREEN_SOAK_DAYS) {
    return {
      state: "TESTNET_SOAK_READY",
      blockers: [],
      nextStep:
        "Paper evidence supports testnet soak — export validation report and proceed per LIVE_TRADING_PHASE.md §2.",
    };
  }

  return {
    state: edgeOk ? "PAPER_EDGE_OK" : "NOT_READY",
    blockers,
    nextStep: "Review unified blockers and 7-day soak table daily.",
  };
}

export function unifiedReadinessLabel(state: UnifiedReadiness): string {
  return state.replace(/_/g, " ");
}
