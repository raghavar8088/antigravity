/**
 * Authority score — composite performance rating for Trade Engine strategies.
 * Legacy stage statuses are scored as the active Trade Engine tier.
 */

import type { StrategyMetrics } from "./types";
import type { StrategyStatus } from "./types";

// ── Tier definitions ──────────────────────────────────────────────────────────

export const TIER_THRESHOLDS = {
  S: 85,
  A: 70,
  B: 55,
  C: 40,
  D: 0,
} as const;

export type AuthorityTier = "S" | "A" | "B" | "C" | "D";

/** Text colour class for each tier (Tailwind). */
export const TIER_COLOR: Record<string, string> = {
  S: "text-emerald-300",
  A: "text-emerald-400",
  B: "text-sky-400",
  C: "text-amber-400",
  D: "text-rose-400",
};

/** Background colour class for each tier (Tailwind). */
export const TIER_BG: Record<string, string> = {
  S: "border-emerald-500/40 bg-emerald-950/30",
  A: "border-emerald-600/30 bg-emerald-950/20",
  B: "border-sky-600/30 bg-sky-950/20",
  C: "border-amber-600/30 bg-amber-950/20",
  D: "border-rose-600/30 bg-rose-950/20",
};

function clamp(v: number, min = 0, max = 100): number {
  if (!Number.isFinite(v)) return min;
  return Math.min(max, Math.max(min, v));
}

export interface AuthorityScore {
  total: number;
  tier: AuthorityTier;
  pnlComponent: number;
  pfComponent: number;
  wrComponent: number;
  ddComponent: number;
  sharpeComponent: number;
  winRateScore: number;
  pfScore: number;
  expectancyScore: number;
  drawdownScore: number;
  sharpeScore: number;
  gradeScore: number;
}

/** Active strategies all use the former Main Engine multiplier. */
function stageMultiplier(status: string): number {
  switch (status) {
    case "TRADE_ENGINE":
    case "MAIN_ENGINE":
    case "GRADE_1":
    case "GRADE_2":
    case "GRADE_3":
    case "GRADE_4":
    case "GRADE_5":
      return 1.10;
    case "RETIRED":
      return 1.00;
    default: return 1.00;
  }
}

function tierFromScore(score: number): AuthorityTier {
  if (score >= TIER_THRESHOLDS.S) return "S";
  if (score >= TIER_THRESHOLDS.A) return "A";
  if (score >= TIER_THRESHOLDS.B) return "B";
  if (score >= TIER_THRESHOLDS.C) return "C";
  return "D";
}

export function computeAuthorityScore(
  metrics: StrategyMetrics,
  status: StrategyStatus | string = "TRADE_ENGINE",
): AuthorityScore {
  const pnlComponent = clamp(
    metrics.totalPnl >= 1000
      ? 100
      : metrics.totalPnl >= 0
        ? (metrics.totalPnl / 1000) * 100
        : 50 + (metrics.totalPnl / 200),
  );

  const pfComponent = clamp(
    !Number.isFinite(metrics.profitFactor) || metrics.profitFactor <= 0
      ? 0
      : metrics.profitFactor >= 3
        ? 100
        : ((metrics.profitFactor - 0.5) / 2.5) * 100,
  );

  const wrComponent = clamp(metrics.winRate * 120);

  const ddComponent = clamp(
    metrics.maxDrawdownPct <= 0
      ? 100
      : metrics.maxDrawdownPct >= 30
        ? 0
        : 100 - (metrics.maxDrawdownPct / 30) * 100,
  );

  const sharpeComponent = clamp(
    metrics.sharpeRatio >= 3
      ? 100
      : metrics.sharpeRatio <= -1
        ? 0
        : ((metrics.sharpeRatio + 1) / 4) * 100,
  );

  const expectancyComponent = clamp(
    metrics.expectancy >= 50
      ? 100
      : metrics.expectancy <= -50
        ? 0
        : 50 + metrics.expectancy,
  );
  const stageBonus = clamp(((stageMultiplier(status) - 0.95) / 0.15) * 15, 0, 15);

  const raw = clamp(
    pnlComponent * 0.25 +
      pfComponent * 0.30 +
      wrComponent * 0.20 +
      ddComponent * 0.15 +
      sharpeComponent * 0.10,
  );

  const total = clamp(raw * stageMultiplier(status));

  return {
    total,
    tier: tierFromScore(total),
    pnlComponent,
    pfComponent,
    wrComponent,
    ddComponent,
    sharpeComponent,
    winRateScore: clamp(wrComponent * 0.20, 0, 20),
    pfScore: clamp(pfComponent * 0.20, 0, 20),
    expectancyScore: clamp(expectancyComponent * 0.15, 0, 15),
    drawdownScore: clamp(ddComponent * 0.15, 0, 15),
    sharpeScore: clamp(sharpeComponent * 0.15, 0, 15),
    gradeScore: stageBonus,
  };
}
