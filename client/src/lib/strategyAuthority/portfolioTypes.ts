/**
 * APICAP — Autonomous Portfolio Intelligence & Capital Allocation Program
 * V3 type definitions — portfolio-level analytics layer on top of ISPAP V2
 */

import type { StrategyGradeMetrics, StrategyStatus } from "./types";

// ── Regime ─────────────────────────────────────────────────────────────────────

export type MarketRegime =
  | "TRENDING"
  | "RANGING"
  | "HIGH_VOLATILITY_BREAKOUT"
  | "LOW_VOLATILITY_CHOP";

export interface RegimePerformance {
  tradeCount: number;
  winRate: number;        // 0–100
  profitFactor: number;
  expectancy: number;     // mean PnL per trade
  sharpeRatio: number;
}

export interface StrategyRegimeMetrics {
  strategy_id: string;
  strategy_name: string;
  family: string;
  regimes: Partial<Record<MarketRegime, RegimePerformance>>;
  regime_strength_score: number;  // 0–100
  best_regime: MarketRegime | null;
  worst_regime: MarketRegime | null;
  current_regime: MarketRegime | null;
  current_regime_pf: number;      // PF in current market regime (0 if no data)
  computed_at: string;
}

// ── Correlation ────────────────────────────────────────────────────────────────

export interface StrategyCorrelationPair {
  strategy_id_a: string;
  strategy_name_a: string;
  strategy_id_b: string;
  strategy_name_b: string;
  pearson_correlation: number;    // −1 to 1
  overlap_days: number;           // trading days both strategies had trades
  computed_at: string;
}

export interface StrategyCorrelationSummary {
  strategy_id: string;
  strategy_name: string;
  family: string;
  avg_abs_correlation: number;    // mean |corr| across all peers with ≥10 overlap days
  max_corr_peer_name: string;     // most correlated strategy name
  max_corr_value: number;         // highest |correlation| with any peer
  correlation_cluster: number;    // cluster group ID (0 = independent)
  diversification_score: number;  // 0–100 (100 = fully uncorrelated)
  family_concentration_penalty: number; // 0–20 subtracted from diversification
  computed_at: string;
}

// ── Allocation ─────────────────────────────────────────────────────────────────

export type AllocationTier = "CORE" | "SATELLITE" | "WATCH" | "EXCLUDED";

export interface StrategyAllocationWeight {
  strategy_id: string;
  strategy_name: string;
  family: string;
  current_status: StrategyStatus;
  // Raw components
  inverse_vol_weight: number;     // 1/σ normalized (risk parity)
  kelly_fraction: number;         // full Kelly fraction (positive = include)
  fractional_kelly: number;       // 25% Kelly (conservative sizing)
  authority_score: number;        // 0–100 from authorityScore.ts
  diversification_score: number;  // 0–100 from correlationEngine
  // Composite
  composite_score: number;        // risk-parity × authority × diversification
  allocation_weight: number;      // final 0–100% after normalization + caps
  allocation_tier: AllocationTier;
  // Constraints applied
  weight_capped: boolean;         // true if hit 25% single-strategy cap
  family_capped: boolean;         // true if family hit 40% cap
  computed_at: string;
}

export interface PortfolioAllocationSummary {
  strategies: StrategyAllocationWeight[];
  total_allocated_strategies: number;
  excluded_strategies: number;
  expected_portfolio_pf: number;
  expected_portfolio_sharpe: number;
  family_exposure: Record<string, number>;  // family → % of portfolio
  max_single_weight: number;
  hhi: number;                    // Herfindahl-Hirschman Index 0–10000
  diversification_ratio: number;  // 0–1 (1 = fully diversified)
  computed_at: string;
}

// ── Strategy Genome ────────────────────────────────────────────────────────────

export type GenomeTier = "ELITE" | "STRONG" | "ADEQUATE" | "MARGINAL" | "WEAK";

export interface StrategyGenome {
  strategy_id: string;
  strategy_name: string;
  family: string;
  category: string;
  timeframe: string;
  current_status: StrategyStatus;

  // Individual performance
  metrics: StrategyGradeMetrics;
  authority_score: number;
  authority_tier: string;
  evidence_score: number;         // 0–15 from SEP

  // Portfolio intelligence
  diversification_score: number;  // 0–100
  correlation_penalty: number;    // points deducted for correlation
  allocation_weight: number;      // % of portfolio (0 if excluded)
  allocation_tier: AllocationTier;

  // Regime intelligence
  regime_strength_score: number;  // 0–100
  regime_metrics: Partial<Record<MarketRegime, { pf: number; wr: number; trades: number }>>;
  best_regime: MarketRegime | null;
  current_regime_pf: number;

  // Lifecycle
  promotion_count: number;
  demotion_count: number;
  last_promoted_at: string | null;
  last_demoted_at: string | null;

  // Composite genome score
  genome_score: number;           // 0–100 composite across all dimensions
  genome_tier: GenomeTier;
  candidate_eligible: boolean;    // passes all 5 candidate queue gates
  main_engine_eligible: boolean;

  computed_at: string;
}

// ── Candidate Queue ────────────────────────────────────────────────────────────

export interface CandidateGateResult {
  pass: boolean;
  score: number;    // gate-specific score 0–100
  reason: string;
}

export interface CandidateQueueEntry {
  strategy_id: string;
  strategy_name: string;
  family: string;
  current_status: StrategyStatus;
  // Five gates
  gate_correlation: CandidateGateResult;
  gate_family_concentration: CandidateGateResult;
  gate_regime: CandidateGateResult;
  gate_portfolio_fit: CandidateGateResult;
  gate_allocation: CandidateGateResult;
  // Summary
  gates_passed: number;           // 0–5
  all_gates_pass: boolean;
  admission_score: number;        // 0–100 composite
  authority_score: number;
  genome_score: number;
  allocation_weight: number;      // projected % if admitted
  queued_at: string;
}

export interface CandidateQueueSummary {
  total_candidates: number;       // strategies in GRADE_1 evaluated
  fully_eligible: number;         // all 5 gates pass
  partial_eligible: number;       // 3–4 gates pass
  ineligible: number;             // ≤2 gates pass
  entries: CandidateQueueEntry[];
  computed_at: string;
}

// ── Portfolio Construction ─────────────────────────────────────────────────────

export interface PortfolioConstructionResult {
  recommended_strategy_ids: string[];
  recommended_strategy_names: string[];
  weights: Record<string, number>;        // strategy_id → weight %
  total_weight: number;                   // should be ~100
  expected_portfolio_pf: number;
  expected_portfolio_sharpe: number;
  expected_max_drawdown: number;
  family_exposure: Record<string, number>;
  max_single_weight: number;
  min_single_weight: number;
  hhi: number;                            // concentration (lower = better)
  diversification_ratio: number;          // 0–1
  excluded_reasons: Record<string, string>; // strategy_id → exclusion reason
  computed_at: string;
}

// ── Portfolio Intelligence Compute Request ─────────────────────────────────────

export interface PortfolioComputeResult {
  correlations_computed: number;
  diversification_scores_computed: number;
  regime_profiles_computed: number;
  allocations_computed: number;
  genomes_computed: number;
  candidates_evaluated: number;
  portfolio_constructed: boolean;
  elapsed_ms: number;
  computed_at: string;
}
