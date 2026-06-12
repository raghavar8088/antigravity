/** ISPAP — Institutional Strategy Promotion Authority Program */

export type StrategyGrade = 5 | 4 | 3 | 2 | 1;
export type StrategyStatus = "GRADE_5" | "GRADE_4" | "GRADE_3" | "GRADE_2" | "GRADE_1" | "MAIN_ENGINE" | "RETIRED";

export type StrategyFamily =
  | "Trend" | "Mean Reversion" | "Mean Rev Elite" | "Breakout" | "Breakout Elite"
  | "Volatility" | "Momentum" | "Momentum Elite" | "Multi-Signal" | "Microstructure"
  | "Price Action Elite" | "Adaptive Elite" | "Statistical" | "Time-of-Day"
  | "Funding" | "Liquidity" | "Structure" | "Smart Money" | "Market Profile"
  | "Session" | "Liquidations" | "Phase 11 Liquidity" | "Phase 11 Derivatives"
  | "Phase 11 Order Flow" | "Phase 11 Liquidations" | "Phase 11 Structure"
  | "Phase 11 Smart Money" | "Intraday";

export interface StrategyGradeMetrics {
  closedTrades: number;
  winRate: number;         // 0–100
  profitFactor: number;
  expectancy: number;      // mean PnL per trade USD
  maxDrawdown: number;     // 0–100 pct
  sharpeRatio: number;
  totalPnl: number;
}

export interface GradeRequirement {
  grade: StrategyGrade | "MAIN_ENGINE";
  minClosedTrades: number;
  minWinRate: number;      // %
  minProfitFactor: number;
  minExpectancy: number;
  maxDrawdown: number;     // %
  minSharpe: number;
}

export interface StrategyProfileDoc {
  strategy_id: string;           // constructor slug
  strategy_name: string;         // display name
  family: string;
  category: string;
  timeframe: string;
  current_status: StrategyStatus;
  current_grade: number;         // 1–5, 0=MAIN, -1=RETIRED
  migrated_at: string;           // ISO date
  last_evaluated_at: string | null;
  last_promoted_at: string | null;
  last_demoted_at: string | null;
  promotion_count: number;
  demotion_count: number;
  retired_at: string | null;
  retirement_reason: string | null;
}

export interface StrategyEventDoc {
  event_id: string;
  strategy_id: string;
  strategy_name: string;
  event_type: "MIGRATION" | "PROMOTION" | "DEMOTION" | "RETIREMENT";
  from_status: StrategyStatus;
  to_status: StrategyStatus;
  reason: string;
  metrics_snapshot: StrategyGradeMetrics | null;
  created_at: string;
}

export interface StrategyWithMetrics extends StrategyProfileDoc {
  metrics: StrategyGradeMetrics;
  promotionEligible: boolean;
  demotionRisk: boolean;
  retirementCandidate: boolean;
  promotionProgress: GradePromotionProgress | null;
}

export interface GradePromotionProgress {
  targetGrade: StrategyStatus;
  requirement: GradeRequirement;
  tradesProgress: number;        // 0–100 %
  winRatePass: boolean;
  profitFactorPass: boolean;
  expectancyPass: boolean;
  drawdownPass: boolean;
  sharpePass: boolean;
  overallProgress: number;       // 0–100 %
}

export interface PipelineStageState {
  status: StrategyStatus;
  label: string;
  count: number;
  avgWinRate: number;
  avgProfitFactor: number;
  avgExpectancy: number;
  avgDrawdown: number;
  promotionCandidates: number;
  demotionRisk: number;
  topPerformers: StrategyWithMetrics[];
  bottomPerformers: StrategyWithMetrics[];
}

export interface PipelineState {
  stages: PipelineStageState[];
  totalStrategies: number;
  totalActive: number;
  totalRetired: number;
  totalMainEngine: number;
  promotionsToday: number;
  demotionsToday: number;
  retirementsToday: number;
  lastUpdated: string;
}

export interface StrategyLeaderboardEntry {
  rank: number;
  strategy_id: string;
  strategy_name: string;
  family: string;
  current_status: StrategyStatus;
  metrics: StrategyGradeMetrics;
  promotionEligible: boolean;
  demotionRisk: boolean;
}

export interface ISPAPCatalogEntry {
  id: string;
  name: string;
  family: string;
  category: string;
  timeframe: string;
}
