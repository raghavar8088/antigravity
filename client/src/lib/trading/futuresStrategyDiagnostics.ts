export interface StrategyDiagnosticRow {
  strategyId: number;
  strategyName: string;
  templateFamily: string;
  totalTrades: number;
  wins: number;
  losses: number;
  winRate: number;
  totalNetPnl: number;
  avgNetPnl: number;
  avgWin: number;
  avgLoss: number;
  feePctOfAbsGross: number;
  profitFactor: number;
  totalFees: number;
  avgHoldMinutes: number;
  exitReasonCounts: Record<string, number>;
  slCount: number;
  tpCount: number;
  timeCount: number;
  trailCount: number;
  profitLockCount: number;
  worstTrade: number;
  bestTrade: number;
  lastTradeAt: string;
  isProbe: boolean;
}

export interface HealthCheckResult {
  window: number;
  expectancy: number;
  expectancyPass: boolean;
  winRate: number;
  winRatePass: boolean;
  feePctOfAbsGross: number;
  feePass: boolean;
  profitFactor: number;
  pfPass: boolean;
  tpHits: number;
  tpHitPass: boolean;
  slCount: number;
  timeCount: number;
  overallPass: boolean;
  grade: "A" | "B" | "C" | "D" | "F";
}

export interface DiagnosticSummary {
  totalProduction: number;
  rows: StrategyDiagnosticRow[];
  topByExpectancy: StrategyDiagnosticRow[];
  bottomByExpectancy: StrategyDiagnosticRow[];
  slDominatedStrats: StrategyDiagnosticRow[];
  highFeeStrategies: StrategyDiagnosticRow[];
}
