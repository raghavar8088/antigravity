import type { StrategyGradeMetrics, StrategyStatus } from "./types";

export interface AuthorityScoreBreakdown {
  total: number;          // 0–100
  winRateScore: number;   // 0–20
  pfScore: number;        // 0–20
  expectancyScore: number;// 0–15
  drawdownScore: number;  // 0–15
  sharpeScore: number;    // 0–15
  gradeScore: number;     // 0–15
  evidenceScore: number;  // 0 (reserved for SEP enrichment)
  tier: "S" | "A" | "B" | "C" | "D" | "F";
}

const gradePts: Record<StrategyStatus, number> = {
  GRADE_5: 0,
  GRADE_4: 3,
  GRADE_3: 6,
  GRADE_2: 9,
  GRADE_1: 12,
  MAIN_ENGINE: 15,
  RETIRED: 0,
};

function clamp(v: number, lo: number, hi: number) {
  return Math.max(lo, Math.min(hi, v));
}

function lerp(v: number, lo: number, hi: number, outLo: number, outHi: number) {
  if (v <= lo) return outLo;
  if (v >= hi) return outHi;
  return outLo + ((v - lo) / (hi - lo)) * (outHi - outLo);
}

export function computeAuthorityScore(
  metrics: StrategyGradeMetrics,
  status: StrategyStatus,
  sepEvidenceScore = 0
): AuthorityScoreBreakdown {
  // Win Rate 0–20
  const winRateScore = clamp(
    metrics.winRate < 40
      ? 0
      : metrics.winRate < 55
      ? lerp(metrics.winRate, 40, 55, 0, 10)
      : lerp(metrics.winRate, 55, 75, 10, 20),
    0,
    20
  );

  // Profit Factor 0–20
  const pfScore = clamp(
    metrics.profitFactor < 1.0
      ? 0
      : metrics.profitFactor < 1.3
      ? lerp(metrics.profitFactor, 1.0, 1.3, 0, 10)
      : lerp(metrics.profitFactor, 1.3, 1.8, 10, 20),
    0,
    20
  );

  // Expectancy 0–15
  const expectancyScore = clamp(
    metrics.expectancy <= 0
      ? 0
      : metrics.expectancy < 2
      ? lerp(metrics.expectancy, 0, 2, 0, 8)
      : lerp(metrics.expectancy, 2, 8, 8, 15),
    0,
    15
  );

  // Drawdown 0–15 (lower is better)
  const drawdownScore = clamp(
    metrics.maxDrawdown > 40
      ? 0
      : metrics.maxDrawdown > 20
      ? lerp(metrics.maxDrawdown, 40, 20, 0, 7)
      : lerp(metrics.maxDrawdown, 20, 0, 7, 15),
    0,
    15
  );

  // Sharpe 0–15
  const sharpeScore = clamp(
    metrics.sharpeRatio < 0
      ? 0
      : metrics.sharpeRatio < 0.5
      ? lerp(metrics.sharpeRatio, 0, 0.5, 0, 6)
      : lerp(metrics.sharpeRatio, 0.5, 1.5, 6, 15),
    0,
    15
  );

  // Grade progression 0–15
  const gradeScore = gradePts[status] ?? 0;

  // SEP evidence score 0–15 (scaled from 0–100 evidence score)
  const evidenceScore = clamp(Math.round((sepEvidenceScore / 100) * 15), 0, 15);

  const rawTotal = winRateScore + pfScore + expectancyScore + drawdownScore + sharpeScore + gradeScore + evidenceScore;
  // Max possible without SEP = 100, with SEP = 115. Normalise back to 100.
  const maxPoints = 100 + (evidenceScore > 0 ? 15 : 0);
  const total = clamp(Math.round((rawTotal / maxPoints) * 100), 0, 100);

  const tier =
    total >= 85 ? "S" :
    total >= 70 ? "A" :
    total >= 55 ? "B" :
    total >= 40 ? "C" :
    total >= 25 ? "D" : "F";

  return {
    total,
    winRateScore: Math.round(winRateScore),
    pfScore: Math.round(pfScore),
    expectancyScore: Math.round(expectancyScore),
    drawdownScore: Math.round(drawdownScore),
    sharpeScore: Math.round(sharpeScore),
    gradeScore: Math.round(gradeScore),
    evidenceScore: Math.round(evidenceScore),
    tier,
  };
}

export const TIER_COLOR: Record<string, string> = {
  S: "text-emerald-400",
  A: "text-sky-400",
  B: "text-blue-400",
  C: "text-amber-400",
  D: "text-orange-400",
  F: "text-rose-400",
};

export const TIER_BG: Record<string, string> = {
  S: "bg-emerald-900/50 border-emerald-700",
  A: "bg-sky-900/40 border-sky-700",
  B: "bg-blue-900/30 border-blue-800",
  C: "bg-amber-900/30 border-amber-800",
  D: "bg-orange-900/20 border-orange-800",
  F: "bg-rose-900/20 border-rose-900",
};
