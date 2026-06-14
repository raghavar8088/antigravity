/**
 * Shared types for the Strategy Authority system.
 * Strategies move through a promotion pipeline from Grade 5 → Main Engine.
 */

export type StrategyStatus =
  | "GRADE_5"
  | "GRADE_4"
  | "GRADE_3"
  | "GRADE_2"
  | "GRADE_1"
  | "MAIN_ENGINE"
  | "RETIRED";

export interface StrategyMetrics {
  totalPnl: number;
  profitFactor: number;
  sharpeRatio: number;
  winRate: number;
  maxDrawdown: number;
  maxDrawdownPct: number;
  totalTrades: number;
  closedTrades: number;
  winCount: number;
  lossCount: number;
}

export interface StrategyWithMetrics {
  strategy_id: number;
  strategy_name: string;
  status: StrategyStatus;
  metrics: StrategyMetrics;
  demotionRisk: boolean;
  promotionEligible: boolean;
  lastTradeAt?: number | null;
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
