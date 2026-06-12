/**
 * APICAP Phase 2 — Capital Allocation Authority Engine
 *
 * Computes per-strategy capital weights using:
 *   1. Risk parity (inverse volatility)
 *   2. Kelly criterion (fractional Kelly sizing)
 *   3. Authority score multiplier
 *   4. Diversification multiplier
 *
 * Constraints:
 *   - Max single-strategy weight: 25%
 *   - Max family weight: 40%
 *   - Min weight if included: 1%
 *   - Kelly fraction must be > 0 to be included
 */

import type { StrategyGradeMetrics, StrategyStatus } from "./types";
import type {
  AllocationTier,
  PortfolioAllocationSummary,
  StrategyAllocationWeight,
} from "./portfolioTypes";

export const STRATEGY_ALLOCATIONS_COLLECTION = "strategy_allocations";

const MAX_SINGLE_WEIGHT = 25;   // %
const MAX_FAMILY_WEIGHT = 40;   // %
const MIN_WEIGHT = 1;           // %
const KELLY_FRACTION = 0.25;    // 25% of full Kelly (conservative)

// ── Kelly criterion ────────────────────────────────────────────────────────────

/**
 * Compute full Kelly fraction for a strategy.
 * Returns negative or zero if strategy has negative edge.
 */
export function computeKelly(metrics: StrategyGradeMetrics): number {
  if (metrics.closedTrades < 10) return 0;
  const wr = metrics.winRate / 100;
  const lr = 1 - wr;

  // Estimate avg win / avg loss from profit factor and win rate
  // PF = (wr * avgWin) / (lr * avgLoss)
  // → avgWin/avgLoss = PF * lr/wr
  if (wr <= 0 || lr <= 0 || metrics.profitFactor <= 0) return 0;

  const oddsRatio = metrics.profitFactor * (lr / wr); // avgWin/avgLoss

  // Kelly: b*p - q where b = net odds, p = win prob, q = loss prob
  // With avgWin/avgLoss as "b"
  const kelly = wr - lr / oddsRatio;
  return Math.max(0, kelly);
}

// ── PnL volatility from metrics ────────────────────────────────────────────────

/**
 * Estimate annualized PnL volatility from max drawdown as proxy.
 * σ ≈ maxDrawdown / 1.65 (assuming ~5% tail, 1.65σ event)
 * Clamped to avoid division-by-zero and extreme values.
 */
export function estimateVolatility(metrics: StrategyGradeMetrics): number {
  if (metrics.sharpeRatio > 0 && metrics.expectancy > 0) {
    // Better: vol = expectancy / sharpe (per-trade unit)
    return Math.max(0.01, metrics.expectancy / Math.max(0.01, metrics.sharpeRatio));
  }
  // Fallback: max drawdown proxy
  return Math.max(1, metrics.maxDrawdown / 1.65);
}

// ── Authority multiplier ───────────────────────────────────────────────────────

/**
 * Maps authority score 0–100 → multiplier 0.6–1.0.
 * Top-tier strategies get full weight; low-tier get reduced weight.
 */
function authorityMultiplier(authorityScore: number): number {
  return 0.6 + (authorityScore / 100) * 0.4;
}

// ── Diversification multiplier ─────────────────────────────────────────────────

/**
 * Maps diversification score 0–100 → multiplier 0.4–1.0.
 * Highly correlated strategies get significantly downweighted.
 */
function diversificationMultiplier(divScore: number): number {
  return 0.4 + (divScore / 100) * 0.6;
}

// ── Allocation tier ────────────────────────────────────────────────────────────

function classifyTier(weight: number, kelly: number): AllocationTier {
  if (kelly <= 0) return "EXCLUDED";
  if (weight >= 8) return "CORE";
  if (weight >= 3) return "SATELLITE";
  if (weight >= 1) return "WATCH";
  return "EXCLUDED";
}

// ── Main allocation computation ────────────────────────────────────────────────

export interface AllocationInput {
  strategy_id: string;
  strategy_name: string;
  family: string;
  current_status: StrategyStatus;
  metrics: StrategyGradeMetrics;
  authority_score: number;
  diversification_score: number;
}

/**
 * Compute capital allocation weights for a set of strategies.
 * Returns sorted by weight desc.
 */
export function computeAllocations(
  strategies: AllocationInput[]
): StrategyAllocationWeight[] {
  const now = new Date().toISOString();

  // Step 1: compute raw scores
  const rawItems: Array<{
    input: AllocationInput;
    kelly: number;
    fracKelly: number;
    invVol: number;
    composite: number;
  }> = [];

  for (const s of strategies) {
    const kelly = computeKelly(s.metrics);
    if (kelly <= 0) continue; // Excluded — negative edge

    const invVol = 1 / estimateVolatility(s.metrics);
    const authMult = authorityMultiplier(s.authority_score);
    const divMult = diversificationMultiplier(s.diversification_score);
    const composite = invVol * authMult * divMult;

    rawItems.push({
      input: s,
      kelly,
      fracKelly: kelly * KELLY_FRACTION,
      invVol,
      composite,
    });
  }

  if (rawItems.length === 0) {
    return strategies.map((s) => ({
      strategy_id: s.strategy_id,
      strategy_name: s.strategy_name,
      family: s.family,
      current_status: s.current_status,
      inverse_vol_weight: 0,
      kelly_fraction: 0,
      fractional_kelly: 0,
      authority_score: s.authority_score,
      diversification_score: s.diversification_score,
      composite_score: 0,
      allocation_weight: 0,
      allocation_tier: "EXCLUDED" as AllocationTier,
      weight_capped: false,
      family_capped: false,
      computed_at: now,
    }));
  }

  // Step 2: normalize to 100%
  const totalComposite = rawItems.reduce((a, r) => a + r.composite, 0);
  let weights = rawItems.map((r) => ({
    ...r,
    weight: (r.composite / totalComposite) * 100,
  }));

  // Step 3: apply single-strategy cap (25%)
  let capped = false;
  for (const w of weights) {
    if (w.weight > MAX_SINGLE_WEIGHT) {
      w.weight = MAX_SINGLE_WEIGHT;
      capped = true;
    }
  }

  // Step 4: apply family cap (40%)
  const familyWeight: Record<string, number> = {};
  for (const w of weights) {
    familyWeight[w.input.family] = (familyWeight[w.input.family] ?? 0) + w.weight;
  }

  for (const family in familyWeight) {
    if (familyWeight[family] > MAX_FAMILY_WEIGHT) {
      const excess = familyWeight[family] - MAX_FAMILY_WEIGHT;
      const familyItems = weights.filter((w) => w.input.family === family);
      const totalFamilyComposite = familyItems.reduce((a, w) => a + w.composite, 0);
      for (const w of familyItems) {
        const reduction = (w.composite / totalFamilyComposite) * excess;
        w.weight = Math.max(0, w.weight - reduction);
      }
      capped = true;
    }
  }

  // Step 5: renormalize after caps
  if (capped) {
    const totalAfterCap = weights.reduce((a, w) => a + w.weight, 0);
    if (totalAfterCap > 0) {
      for (const w of weights) {
        w.weight = (w.weight / totalAfterCap) * 100;
      }
    }
  }

  // Step 6: build result (include excluded strategies too with 0 weight)
  const includedIds = new Set(rawItems.map((r) => r.input.strategy_id));
  const result: StrategyAllocationWeight[] = [];

  // Included strategies
  for (const w of weights) {
    result.push({
      strategy_id: w.input.strategy_id,
      strategy_name: w.input.strategy_name,
      family: w.input.family,
      current_status: w.input.current_status,
      inverse_vol_weight: +(w.invVol).toFixed(4),
      kelly_fraction: +(w.kelly).toFixed(4),
      fractional_kelly: +(w.fracKelly).toFixed(4),
      authority_score: w.input.authority_score,
      diversification_score: w.input.diversification_score,
      composite_score: +(w.composite).toFixed(4),
      allocation_weight: +w.weight.toFixed(2),
      allocation_tier: classifyTier(w.weight, w.kelly),
      weight_capped: w.weight >= MAX_SINGLE_WEIGHT - 0.1,
      family_capped: false, // simplified
      computed_at: now,
    });
  }

  // Excluded strategies (negative Kelly)
  for (const s of strategies) {
    if (!includedIds.has(s.strategy_id)) {
      result.push({
        strategy_id: s.strategy_id,
        strategy_name: s.strategy_name,
        family: s.family,
        current_status: s.current_status,
        inverse_vol_weight: 0,
        kelly_fraction: 0,
        fractional_kelly: 0,
        authority_score: s.authority_score,
        diversification_score: s.diversification_score,
        composite_score: 0,
        allocation_weight: 0,
        allocation_tier: "EXCLUDED" as AllocationTier,
        weight_capped: false,
        family_capped: false,
        computed_at: now,
      });
    }
  }

  return result.sort((a, b) => b.allocation_weight - a.allocation_weight);
}

// ── Portfolio summary ──────────────────────────────────────────────────────────

/**
 * Build portfolio allocation summary from computed weights.
 */
export function buildAllocationSummary(
  weights: StrategyAllocationWeight[],
  strategyMetrics: Map<string, StrategyGradeMetrics>
): PortfolioAllocationSummary {
  const included = weights.filter((w) => w.allocation_weight > 0);
  const excluded = weights.filter((w) => w.allocation_weight === 0).length;

  // Family exposure
  const familyExp: Record<string, number> = {};
  for (const w of included) {
    familyExp[w.family] = (familyExp[w.family] ?? 0) + w.allocation_weight;
  }

  // Portfolio metrics (weighted avg)
  let wtPf = 0, wtSharpe = 0, totalWt = 0;
  for (const w of included) {
    const m = strategyMetrics.get(w.strategy_id);
    if (!m) continue;
    const wt = w.allocation_weight / 100;
    wtPf += m.profitFactor * wt;
    wtSharpe += m.sharpeRatio * wt;
    totalWt += wt;
  }

  const expectedPf = totalWt > 0 ? wtPf / totalWt : 0;
  const expectedSharpe = totalWt > 0 ? wtSharpe / totalWt : 0;

  // HHI: Σ(w_i²) — lower means more diversified
  const hhi = included.reduce((acc, w) => acc + Math.pow(w.allocation_weight, 2), 0);

  // Max diversification ratio: avg correlation between included pairs (placeholder)
  const divRatio = included.length > 1
    ? Math.max(0, 1 - (hhi / 10000) * 2)
    : 0;

  return {
    strategies: weights,
    total_allocated_strategies: included.length,
    excluded_strategies: excluded,
    expected_portfolio_pf: +expectedPf.toFixed(3),
    expected_portfolio_sharpe: +expectedSharpe.toFixed(3),
    family_exposure: familyExp,
    max_single_weight: Math.max(0, ...included.map((w) => w.allocation_weight)),
    hhi: +hhi.toFixed(1),
    diversification_ratio: +divRatio.toFixed(3),
    computed_at: new Date().toISOString(),
  };
}
