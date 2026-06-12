import type { GradeRequirement, StrategyGradeMetrics, StrategyStatus } from "./types";

export const GRADE_REQUIREMENTS: GradeRequirement[] = [
  {
    grade: 5,
    minClosedTrades: 50,
    minWinRate: 55,
    minProfitFactor: 1.10,
    minExpectancy: 0,
    maxDrawdown: 100,
    minSharpe: -999,
  },
  {
    grade: 4,
    minClosedTrades: 75,
    minWinRate: 55,
    minProfitFactor: 1.15,
    minExpectancy: 0,
    maxDrawdown: 25,
    minSharpe: -999,
  },
  {
    grade: 3,
    minClosedTrades: 100,
    minWinRate: 55,
    minProfitFactor: 1.20,
    minExpectancy: 0,
    maxDrawdown: 20,
    minSharpe: -999,
  },
  {
    grade: 2,
    minClosedTrades: 150,
    minWinRate: 55,
    minProfitFactor: 1.25,
    minExpectancy: 0,
    maxDrawdown: 15,
    minSharpe: 0.50,
  },
  {
    grade: 1,
    minClosedTrades: 200,
    minWinRate: 55,
    minProfitFactor: 1.30,
    minExpectancy: 0,
    maxDrawdown: 10,
    minSharpe: 0.75,
  },
  {
    grade: "MAIN_ENGINE",
    minClosedTrades: 575,
    minWinRate: 55,
    minProfitFactor: 1.30,
    minExpectancy: 0,
    maxDrawdown: 10,
    minSharpe: 0.75,
  },
];

/** Rolling window for demotion evaluation */
export const DEMOTION_ROLLING_TRADES = 50;

export const DEMOTION_TRIGGERS = {
  minWinRate: 50,
  minProfitFactor: 1.0,
  minExpectancy: 0,
};

export const RETIREMENT_TRIGGERS = {
  minClosedTrades: 200,
  maxProfitFactor: 0.90,
  maxExpectancy: 0,
};

export function statusToGradeNumber(status: StrategyStatus): number {
  switch (status) {
    case "GRADE_5": return 5;
    case "GRADE_4": return 4;
    case "GRADE_3": return 3;
    case "GRADE_2": return 2;
    case "GRADE_1": return 1;
    case "MAIN_ENGINE": return 0;
    case "RETIRED": return -1;
  }
}

export function gradeNumberToStatus(grade: number): StrategyStatus {
  switch (grade) {
    case 5: return "GRADE_5";
    case 4: return "GRADE_4";
    case 3: return "GRADE_3";
    case 2: return "GRADE_2";
    case 1: return "GRADE_1";
    case 0: return "MAIN_ENGINE";
    default: return "RETIRED";
  }
}

export function getPromotionRequirement(currentStatus: StrategyStatus): GradeRequirement | null {
  switch (currentStatus) {
    case "GRADE_5": return GRADE_REQUIREMENTS.find((r) => r.grade === 4) ?? null;
    case "GRADE_4": return GRADE_REQUIREMENTS.find((r) => r.grade === 3) ?? null;
    case "GRADE_3": return GRADE_REQUIREMENTS.find((r) => r.grade === 2) ?? null;
    case "GRADE_2": return GRADE_REQUIREMENTS.find((r) => r.grade === 1) ?? null;
    case "GRADE_1": return GRADE_REQUIREMENTS.find((r) => r.grade === "MAIN_ENGINE") ?? null;
    default: return null;
  }
}

export function getTargetStatus(currentStatus: StrategyStatus): StrategyStatus | null {
  switch (currentStatus) {
    case "GRADE_5": return "GRADE_4";
    case "GRADE_4": return "GRADE_3";
    case "GRADE_3": return "GRADE_2";
    case "GRADE_2": return "GRADE_1";
    case "GRADE_1": return "MAIN_ENGINE";
    default: return null;
  }
}

export function getDemotedStatus(currentStatus: StrategyStatus): StrategyStatus {
  switch (currentStatus) {
    case "MAIN_ENGINE": return "GRADE_1";
    case "GRADE_1": return "GRADE_2";
    case "GRADE_2": return "GRADE_3";
    case "GRADE_3": return "GRADE_4";
    default: return "GRADE_5";
  }
}

export function checkPromotionEligible(
  metrics: StrategyGradeMetrics,
  currentStatus: StrategyStatus
): boolean {
  const req = getPromotionRequirement(currentStatus);
  if (!req) return false;
  return (
    metrics.closedTrades >= req.minClosedTrades &&
    metrics.winRate >= req.minWinRate &&
    metrics.profitFactor >= req.minProfitFactor &&
    metrics.expectancy >= req.minExpectancy &&
    metrics.maxDrawdown <= req.maxDrawdown &&
    metrics.sharpeRatio >= req.minSharpe
  );
}

export function checkDemotionRisk(metrics: StrategyGradeMetrics): boolean {
  return (
    metrics.winRate < DEMOTION_TRIGGERS.minWinRate ||
    metrics.profitFactor < DEMOTION_TRIGGERS.minProfitFactor ||
    metrics.expectancy < DEMOTION_TRIGGERS.minExpectancy
  );
}

export function checkRetirementCandidate(metrics: StrategyGradeMetrics): boolean {
  return (
    metrics.closedTrades >= RETIREMENT_TRIGGERS.minClosedTrades &&
    (metrics.profitFactor < RETIREMENT_TRIGGERS.maxProfitFactor ||
      metrics.expectancy < RETIREMENT_TRIGGERS.maxExpectancy)
  );
}

export function computePromotionProgress(
  metrics: StrategyGradeMetrics,
  currentStatus: StrategyStatus
): { targetGrade: StrategyStatus; requirement: GradeRequirement; tradesProgress: number; winRatePass: boolean; profitFactorPass: boolean; expectancyPass: boolean; drawdownPass: boolean; sharpePass: boolean; overallProgress: number } | null {
  const req = getPromotionRequirement(currentStatus);
  const target = getTargetStatus(currentStatus);
  if (!req || !target) return null;

  const tradesProgress = Math.min(100, (metrics.closedTrades / req.minClosedTrades) * 100);
  const winRatePass = metrics.winRate >= req.minWinRate;
  const profitFactorPass = metrics.profitFactor >= req.minProfitFactor;
  const expectancyPass = metrics.expectancy >= req.minExpectancy;
  const drawdownPass = metrics.maxDrawdown <= req.maxDrawdown;
  const sharpePass = req.minSharpe <= -999 || metrics.sharpeRatio >= req.minSharpe;

  const gates = [winRatePass, profitFactorPass, expectancyPass, drawdownPass, sharpePass];
  const gatePct = (gates.filter(Boolean).length / gates.length) * 100;
  const overallProgress = Math.round((tradesProgress * 0.5) + (gatePct * 0.5));

  return {
    targetGrade: target,
    requirement: req,
    tradesProgress: Math.round(tradesProgress),
    winRatePass,
    profitFactorPass,
    expectancyPass,
    drawdownPass,
    sharpePass,
    overallProgress,
  };
}
