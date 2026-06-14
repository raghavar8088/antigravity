/**
 * Shared types for the Strategy Authority system.
 * Trade Engine is the only active strategy surface. Legacy stage statuses are
 * retained only so existing persisted MongoDB documents continue to parse.
 */

/**
 * Trade Engine uses two statuses only.
 * Grade statuses retained for backward-compatibility with existing
 * MongoDB documents — never surfaced in UI.
 */
export type StrategyStatus =
  | "TRADE_ENGINE"
  | "MAIN_ENGINE"
  | "GRADE_5"
  | "GRADE_4"
  | "GRADE_3"
  | "GRADE_2"
  | "GRADE_1"
  | "RETIRED";

export interface StrategyMetrics {
  totalPnl: number;
  profitFactor: number;
  sharpeRatio: number;
  winRate: number;
  expectancy: number;
  maxDrawdown: number;
  maxDrawdownPct: number;
  totalTrades: number;
  closedTrades: number;
  winCount: number;
  lossCount: number;
}

export interface PromotionRequirement {
  minClosedTrades: number;
  minWinRate: number;
  minProfitFactor: number;
  minSharpe: number;
  maxDrawdown: number;
}

export interface PromotionProgress {
  targetGrade: StrategyStatus;
  overallProgress: number;
  requirement: PromotionRequirement;
  winRatePass: boolean;
  profitFactorPass: boolean;
  expectancyPass: boolean;
  sharpePass: boolean;
  drawdownPass: boolean;
}

export interface StrategyWithMetrics {
  strategy_id: string;
  strategy_name: string;
  status: StrategyStatus;
  current_status: StrategyStatus;
  family: string;
  timeframe: string;
  category?: string;
  metrics: StrategyMetrics;
  demotionRisk: boolean;
  promotionEligible: boolean;
  retirementCandidate?: boolean;
  promotionProgress?: PromotionProgress | null;
  promotion_count?: number;
  demotion_count?: number;
  migrated_at?: string | null;
  last_evaluated_at?: string | null;
  lastTradeAt?: number | null;
}

export type StrategyEventType = "PROMOTION" | "DEMOTION" | "RETIREMENT" | "MIGRATION";

export interface StrategyEventDoc {
  event_id: string;
  event_type: StrategyEventType;
  strategy_id: string;
  strategy_name: string;
  from_status: StrategyStatus;
  to_status: StrategyStatus;
  created_at: string;
  reason?: string | null;
  metrics_snapshot?: Partial<StrategyMetrics> | null;
}

export interface StrategyProfileDoc {
  strategy_id: string;
  strategy_name: string;
  family: string;
  timeframe: string;
  current_status: StrategyStatus;
  retirement_reason?: string | null;
  retired_at?: string | null;
  promotion_count?: number;
  demotion_count?: number;
}

/** Catalog entry for the ISPAP (Institutional Strategy Promotion & Authority Program) pipeline. */
export interface ISPAPCatalogEntry {
  id: number;
  name: string;
  family: string;
  status: StrategyStatus;
  signalKey: string;
  category?: string;
  regimes?: string[];
}
