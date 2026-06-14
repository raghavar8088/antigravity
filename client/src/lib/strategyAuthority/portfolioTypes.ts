/**
 * Portfolio-level allocation types used by the Strategy Authority system.
 */

export interface PortfolioStrategyEntry {
  strategy_name: string;
  strategy_id: number;
  allocation_weight: number;
  family: string;
  allocation_tier: "CORE" | "SATELLITE" | "WATCH" | "EXCLUDED" | (string & {});
  kelly_fraction: number;
  authority_score: number;
  diversification_score: number;
}

export type StrategyAllocationWeight = PortfolioStrategyEntry;

export interface PortfolioAllocationSummary {
  strategies: PortfolioStrategyEntry[];
  total_allocated_weight: number;
  total_allocated_strategies: number;
  excluded_strategies: number;
  expected_portfolio_pf: number;
  expected_portfolio_sharpe: number;
  max_single_weight: number;
  hhi: number;
  diversification_ratio: number;
  family_exposure: Record<string, number>;
  computed_at?: string;
}

export interface PortfolioConstructionResult {
  recommended_strategy_ids: string[];
  recommended_strategy_names: string[];
  weights: Record<string, number>;
  expected_portfolio_pf: number;
  expected_portfolio_sharpe: number;
  expected_max_drawdown: number;
  max_single_weight: number;
  hhi: number;
  diversification_ratio: number;
  family_exposure: Record<string, number>;
  excluded_reasons: Record<string, string>;
  computed_at: string;
}

export interface CandidateGateResult {
  pass: boolean;
  score: number;
  reason: string;
}

export interface CandidateQueueEntry {
  strategy_id: string;
  strategy_name: string;
  family: string;
  gates_passed: number;
  all_gates_pass: boolean;
  admission_score: number;
  authority_score: number;
  allocation_weight: number;
  gate_correlation: CandidateGateResult;
  gate_family_concentration: CandidateGateResult;
  gate_regime: CandidateGateResult;
  gate_portfolio_fit: CandidateGateResult;
  gate_allocation: CandidateGateResult;
}

export interface CandidateQueueSummary {
  total_candidates: number;
  fully_eligible: number;
  partial_eligible: number;
  ineligible: number;
  entries: CandidateQueueEntry[];
  computed_at?: string;
}

export interface StrategyCorrelationPair {
  strategy_name_a: string;
  strategy_name_b: string;
  pearson_correlation: number;
  overlap_days: number;
}

export interface StrategyCorrelationSummary {
  strategy_id: string | number;
  strategy_name: string;
  family: string;
  diversification_score: number;
  avg_abs_correlation: number;
  max_corr_value: number;
  max_corr_peer_name?: string | null;
  correlation_cluster: string;
}

export type MarketRegime =
  | "TRENDING"
  | "RANGING"
  | "HIGH_VOLATILITY_BREAKOUT"
  | "LOW_VOLATILITY_CHOP";

export interface StrategyRegimePerformance {
  tradeCount: number;
  profitFactor: number;
  winRate: number;
}

export interface StrategyRegimeMetrics {
  strategy_id: string | number;
  strategy_name: string;
  family: string;
  regime_strength_score: number;
  regimes: Partial<Record<MarketRegime, StrategyRegimePerformance>>;
  best_regime?: MarketRegime | null;
  worst_regime?: MarketRegime | null;
}
