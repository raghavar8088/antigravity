/**
 * APICAP Phase 7 — Strategy Genome Engine
 *
 * Aggregates all intelligence dimensions into a single Genome document:
 *   - Authority Score (V2)
 *   - SEP Evidence Score
 *   - Diversification Score (Phase 3/4)
 *   - Regime Strength Score (Phase 6)
 *   - Allocation Weight (Phase 2)
 *   - Candidate Eligibility (Phase 9)
 *
 * Genome Score formula:
 *   authority    × 0.30
 *   diversification × 0.25
 *   regime       × 0.20
 *   allocation   × 0.15   (normalized: allocationWeight / 25 × 100)
 *   evidence     × 0.10   (normalized: evidenceScore / 15 × 100)
 */

import type { Db } from "mongodb";
import type { StrategyGradeMetrics, StrategyStatus } from "./types";
import type {
  AllocationTier,
  GenomeTier,
  MarketRegime,
  RegimePerformance,
  StrategyGenome,
} from "./portfolioTypes";
import { computeAuthorityScore } from "./authorityScore";

export const STRATEGY_GENOMES_COLLECTION = "strategy_genomes";

const GENOME_WEIGHTS = {
  authority: 0.30,
  diversification: 0.25,
  regime: 0.20,
  allocation: 0.15,
  evidence: 0.10,
};

function genomeTierFromScore(score: number): GenomeTier {
  if (score >= 80) return "ELITE";
  if (score >= 65) return "STRONG";
  if (score >= 50) return "ADEQUATE";
  if (score >= 35) return "MARGINAL";
  return "WEAK";
}

export interface GenomeInput {
  strategy_id: string;
  strategy_name: string;
  family: string;
  category: string;
  timeframe: string;
  current_status: StrategyStatus;
  metrics: StrategyGradeMetrics;
  evidence_score: number;
  diversification_score: number;
  allocation_weight: number;
  allocation_tier: AllocationTier;
  regime_strength_score: number;
  regime_metrics: Partial<Record<MarketRegime, RegimePerformance>>;
  best_regime: MarketRegime | null;
  current_regime_pf: number;
  promotion_count: number;
  demotion_count: number;
  last_promoted_at: string | null;
  last_demoted_at: string | null;
}

/**
 * Compute genome score from component scores.
 */
export function computeGenomeScore(
  authorityScore: number,
  diversificationScore: number,
  regimeScore: number,
  allocationWeight: number,  // 0–25% max → normalize to 0–100
  evidenceScore: number      // 0–15 → normalize to 0–100
): number {
  const normAllocation = Math.min(100, (allocationWeight / 25) * 100);
  const normEvidence = (evidenceScore / 15) * 100;

  const raw =
    authorityScore * GENOME_WEIGHTS.authority +
    diversificationScore * GENOME_WEIGHTS.diversification +
    regimeScore * GENOME_WEIGHTS.regime +
    normAllocation * GENOME_WEIGHTS.allocation +
    normEvidence * GENOME_WEIGHTS.evidence;

  return Math.max(0, Math.min(100, Math.round(raw)));
}

/**
 * Compute candidate eligibility based on 5-gate checks.
 * Returns gates_passed count and overall eligibility.
 */
export function checkCandidateEligibility(input: GenomeInput): {
  eligible: boolean;
  gates_passed: number;
} {
  let passed = 0;

  // Gate 1: Correlation/Diversification (score > 35)
  if (input.diversification_score > 35) passed++;

  // Gate 2: Regime gate (current regime PF > 1.0 or no regime data)
  if (input.current_regime_pf > 1.0 || input.current_regime_pf === 0) passed++;

  // Gate 3: Allocation gate (allocation_weight > 0, positive Kelly)
  if (input.allocation_weight > 0) passed++;

  // Gate 4: Authority gate (authority > 40)
  const auth = computeAuthorityScore(input.metrics, input.current_status, input.evidence_score);
  if (auth.total > 40) passed++;

  // Gate 5: Evidence gate (evidence_score > 0 or enough trades to waive)
  if (input.evidence_score > 0 || input.metrics.closedTrades >= 100) passed++;

  return { eligible: passed >= 5, gates_passed: passed };
}

/**
 * Build full genome document for a strategy.
 */
export function buildGenome(input: GenomeInput): StrategyGenome {
  const auth = computeAuthorityScore(input.metrics, input.current_status, input.evidence_score);
  const genomeScore = computeGenomeScore(
    auth.total,
    input.diversification_score,
    input.regime_strength_score,
    input.allocation_weight,
    input.evidence_score
  );

  const { eligible, gates_passed } = checkCandidateEligibility(input);
  const genomeTier = genomeTierFromScore(genomeScore);

  // Regime metrics in compact form
  const compactRegimes: Partial<Record<MarketRegime, { pf: number; wr: number; trades: number }>> = {};
  for (const [regime, perf] of Object.entries(input.regime_metrics)) {
    if (perf && perf.tradeCount > 0) {
      compactRegimes[regime as MarketRegime] = {
        pf: perf.profitFactor,
        wr: perf.winRate,
        trades: perf.tradeCount,
      };
    }
  }

  return {
    strategy_id: input.strategy_id,
    strategy_name: input.strategy_name,
    family: input.family,
    category: input.category,
    timeframe: input.timeframe,
    current_status: input.current_status,
    metrics: input.metrics,
    authority_score: auth.total,
    authority_tier: auth.tier,
    evidence_score: input.evidence_score,
    diversification_score: input.diversification_score,
    correlation_penalty: Math.max(0, 100 - input.diversification_score),
    allocation_weight: input.allocation_weight,
    allocation_tier: input.allocation_tier,
    regime_strength_score: input.regime_strength_score,
    regime_metrics: compactRegimes,
    best_regime: input.best_regime,
    current_regime_pf: input.current_regime_pf,
    promotion_count: input.promotion_count,
    demotion_count: input.demotion_count,
    last_promoted_at: input.last_promoted_at,
    last_demoted_at: input.last_demoted_at,
    genome_score: genomeScore,
    genome_tier: genomeTier,
    candidate_eligible: eligible,
    main_engine_eligible: eligible && input.current_status === "GRADE_1",
    computed_at: new Date().toISOString(),
  };
}

/**
 * Store genome documents in MongoDB.
 */
export async function storeGenomes(db: Db, genomes: StrategyGenome[]): Promise<void> {
  if (genomes.length === 0) return;
  const coll = db.collection<StrategyGenome>(STRATEGY_GENOMES_COLLECTION);
  await coll.deleteMany({});
  await coll.insertMany(genomes);
  await Promise.all([
    coll.createIndex({ strategy_id: 1 }, { unique: true, background: true }),
    coll.createIndex({ genome_score: -1 }, { background: true }),
    coll.createIndex({ genome_tier: 1 }, { background: true }),
    coll.createIndex({ candidate_eligible: 1 }, { background: true }),
    coll.createIndex({ family: 1 }, { background: true }),
  ]);
}

/**
 * Get genome for a single strategy.
 */
export async function getGenome(db: Db, strategyId: string): Promise<StrategyGenome | null> {
  return db.collection<StrategyGenome>(STRATEGY_GENOMES_COLLECTION)
    .findOne({ strategy_id: strategyId });
}

/**
 * Get all genomes, sorted by genome_score desc.
 */
export async function getAllGenomes(db: Db, limit = 305): Promise<StrategyGenome[]> {
  return db.collection<StrategyGenome>(STRATEGY_GENOMES_COLLECTION)
    .find({})
    .sort({ genome_score: -1 })
    .limit(limit)
    .toArray();
}

/**
 * Get candidate-eligible genomes (all 5 gates pass).
 */
export async function getCandidateGenomes(db: Db): Promise<StrategyGenome[]> {
  return db.collection<StrategyGenome>(STRATEGY_GENOMES_COLLECTION)
    .find({ candidate_eligible: true })
    .sort({ genome_score: -1 })
    .toArray();
}
