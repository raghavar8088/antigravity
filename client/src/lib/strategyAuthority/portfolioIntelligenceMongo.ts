/**
 * APICAP — Portfolio Intelligence MongoDB Orchestration Layer
 *
 * Wires together: Correlation → Allocation → Regime → Genome → Candidate Queue → Portfolio Construction
 *
 * Entry point: runFullPortfolioIntelligence()
 * Individual getters for each sub-system.
 */

import type { Db } from "mongodb";
import { getDb } from "@/lib/mongoTradesClient";
import { ISPAP_PROFILES_COLLECTION } from "./strategyAuthorityMongo";
import type { StrategyProfileDoc } from "./types";
import { computeAuthorityScore } from "./authorityScore";
import { readSepStrategyEvidence } from "@/lib/sep/sepPipeline";

// Sub-engines
import {
  computeAndStoreCorrelations,
  getDiversificationSummaries,
  STRATEGY_DIVERSIFICATION_COLLECTION,
} from "./correlationEngine";
import {
  computeAllocations,
  buildAllocationSummary,
  STRATEGY_ALLOCATIONS_COLLECTION,
} from "./allocationEngine";
import {
  computeAndStoreRegimeMetrics,
  getAllRegimeMetrics,
  getCurrentRegime,
  STRATEGY_REGIME_METRICS_COLLECTION,
} from "./regimeEngine";
import {
  buildGenome,
  storeGenomes,
  getAllGenomes,
  getCandidateGenomes,
  STRATEGY_GENOMES_COLLECTION,
} from "./genomeEngine";
import type {
  StrategyAllocationWeight,
  StrategyCorrelationSummary,
  StrategyRegimeMetrics,
  StrategyGenome,
  CandidateQueueEntry,
  CandidateQueueSummary,
  PortfolioConstructionResult,
  PortfolioComputeResult,
  PortfolioAllocationSummary,
  MarketRegime,
} from "./portfolioTypes";

export const CANDIDATE_QUEUE_COLLECTION = "candidate_queue";
export const PORTFOLIO_CONSTRUCTION_COLLECTION = "portfolio_construction";

// ── Metrics computation (reused from V2) ──────────────────────────────────────

async function computeMetricsForStrategy(db: Db, strategyName: string) {
  const [aggResult] = await db.collection("mock_trades").aggregate([
    { $match: { strategy_name: strategyName, status: "CLOSED" } },
    {
      $group: {
        _id: null,
        totalTrades: { $sum: 1 },
        wins: { $sum: { $cond: [{ $gt: ["$realized_pnl", 0] }, 1, 0] } },
        grossWin: { $sum: { $cond: [{ $gt: ["$realized_pnl", 0] }, "$realized_pnl", 0] } },
        grossLoss: { $sum: { $cond: [{ $lte: ["$realized_pnl", 0] }, "$realized_pnl", 0] } },
        totalPnl: { $sum: "$realized_pnl" },
        pnlArray: { $push: "$realized_pnl" },
      },
    },
  ]).toArray();

  if (!aggResult) {
    return { closedTrades: 0, winRate: 0, profitFactor: 0, expectancy: 0, maxDrawdown: 0, sharpeRatio: 0, totalPnl: 0 };
  }

  const closedTrades = aggResult.totalTrades as number;
  const winRate = closedTrades > 0 ? ((aggResult.wins as number) / closedTrades) * 100 : 0;
  const profitFactor = Math.abs(aggResult.grossLoss as number) > 0
    ? (aggResult.grossWin as number) / Math.abs(aggResult.grossLoss as number)
    : (aggResult.grossWin as number) > 0 ? 99 : 0;
  const expectancy = closedTrades > 0 ? (aggResult.totalPnl as number) / closedTrades : 0;

  const pnlArr = (aggResult.pnlArray as number[]) ?? [];
  let maxDrawdown = 0;
  let peak = 0;
  let cumulative = 0;
  for (const pnl of pnlArr) {
    cumulative += pnl;
    if (cumulative > peak) peak = cumulative;
    const dd = peak > 0 ? ((peak - cumulative) / peak) * 100 : 0;
    if (dd > maxDrawdown) maxDrawdown = dd;
  }

  const mean = pnlArr.length > 0 ? pnlArr.reduce((a, p) => a + p, 0) / pnlArr.length : 0;
  const variance = pnlArr.length > 1
    ? pnlArr.reduce((a, p) => a + Math.pow(p - mean, 2), 0) / (pnlArr.length - 1)
    : 0;
  const stdDev = Math.sqrt(variance);
  const sharpeRatio = stdDev > 0 ? (mean / stdDev) * Math.sqrt(252) : 0;

  return {
    closedTrades,
    winRate: +winRate.toFixed(1),
    profitFactor: +profitFactor.toFixed(3),
    expectancy: +expectancy.toFixed(2),
    maxDrawdown: +maxDrawdown.toFixed(1),
    sharpeRatio: +sharpeRatio.toFixed(3),
    totalPnl: +(aggResult.totalPnl as number).toFixed(2),
  };
}

// ── Candidate Queue computation ────────────────────────────────────────────────

async function computeCandidateQueue(
  db: Db,
  grade1Profiles: StrategyProfileDoc[],
  diversificationMap: Map<string, StrategyCorrelationSummary>,
  regimeMap: Map<string, StrategyRegimeMetrics>,
  allocationMap: Map<string, StrategyAllocationWeight>,
  mainEngineCount: number
): Promise<CandidateQueueSummary> {
  const now = new Date().toISOString();
  const entries: CandidateQueueEntry[] = [];

  // Family count in Main Engine
  const mainEngineProfiles = await db.collection<StrategyProfileDoc>(ISPAP_PROFILES_COLLECTION)
    .find({ current_status: "MAIN_ENGINE" })
    .toArray();
  const familyInMain: Record<string, number> = {};
  for (const p of mainEngineProfiles) {
    familyInMain[p.family] = (familyInMain[p.family] ?? 0) + 1;
  }
  const maxFamilyMain = Math.max(5, Math.round(mainEngineCount * 0.30));

  for (const profile of grade1Profiles) {
    const divScore = diversificationMap.get(profile.strategy_name)?.diversification_score ?? 0;
    const regime = regimeMap.get(profile.strategy_id);
    const allocation = allocationMap.get(profile.strategy_id);
    const metrics = await computeMetricsForStrategy(db, profile.strategy_name);
    const auth = computeAuthorityScore(metrics, "GRADE_1");

    // Gate 1: Correlation/Diversification
    const gateCorr = {
      pass: divScore > 35,
      score: divScore,
      reason: divScore > 35
        ? `Diversification score ${divScore}/100 — acceptable correlation profile`
        : `Diversification score ${divScore}/100 — too correlated with existing strategies`,
    };

    // Gate 2: Family concentration
    const familyCount = familyInMain[profile.family] ?? 0;
    const gateFamily = {
      pass: familyCount < maxFamilyMain,
      score: Math.max(0, 100 - (familyCount / maxFamilyMain) * 100),
      reason: familyCount < maxFamilyMain
        ? `Family ${profile.family} has ${familyCount}/${maxFamilyMain} slots used`
        : `Family ${profile.family} at concentration limit (${familyCount}/${maxFamilyMain})`,
    };

    // Gate 3: Regime gate
    const currentRegimePf = regime?.current_regime_pf ?? 0;
    const gateRegime = {
      pass: currentRegimePf > 1.0 || currentRegimePf === 0,
      score: currentRegimePf > 0 ? Math.min(100, currentRegimePf * 50) : 50,
      reason: currentRegimePf === 0
        ? "No regime data — gate waived"
        : currentRegimePf > 1.0
        ? `PF ${currentRegimePf.toFixed(2)} in current regime`
        : `PF ${currentRegimePf.toFixed(2)} in current regime — below 1.0 threshold`,
    };

    // Gate 4: Portfolio fit (proxy: diversification + authority)
    const portfolioFitScore = Math.round((divScore * 0.6 + auth.total * 0.4));
    const gatePortfolioFit = {
      pass: portfolioFitScore > 50,
      score: portfolioFitScore,
      reason: portfolioFitScore > 50
        ? `Portfolio fit score ${portfolioFitScore}/100 — improves portfolio Sharpe`
        : `Portfolio fit score ${portfolioFitScore}/100 — marginal portfolio improvement`,
    };

    // Gate 5: Allocation gate
    const allocWeight = allocation?.allocation_weight ?? 0;
    const kellyFraction = allocation?.kelly_fraction ?? 0;
    const gateAllocation = {
      pass: kellyFraction > 0,
      score: kellyFraction > 0 ? Math.min(100, allocWeight * 4) : 0,
      reason: kellyFraction > 0
        ? `Positive Kelly fraction (${(kellyFraction * 100).toFixed(1)}%) — ${allocWeight.toFixed(1)}% projected weight`
        : "Zero or negative Kelly — no positive edge to allocate",
    };

    const gates = [gateCorr, gateFamily, gateRegime, gatePortfolioFit, gateAllocation];
    const gatesPassed = gates.filter((g) => g.pass).length;
    const admissionScore = Math.round(
      gateCorr.score * 0.20 +
      gateFamily.score * 0.15 +
      gateRegime.score * 0.20 +
      gatePortfolioFit.score * 0.25 +
      gateAllocation.score * 0.20
    );

    entries.push({
      strategy_id: profile.strategy_id,
      strategy_name: profile.strategy_name,
      family: profile.family,
      current_status: "GRADE_1",
      gate_correlation: gateCorr,
      gate_family_concentration: gateFamily,
      gate_regime: gateRegime,
      gate_portfolio_fit: gatePortfolioFit,
      gate_allocation: gateAllocation,
      gates_passed: gatesPassed,
      all_gates_pass: gatesPassed === 5,
      admission_score: admissionScore,
      authority_score: auth.total,
      genome_score: 0, // filled after genome computation
      allocation_weight: allocWeight,
      queued_at: now,
    });
  }

  entries.sort((a, b) => b.admission_score - a.admission_score);

  const summary: CandidateQueueSummary = {
    total_candidates: entries.length,
    fully_eligible: entries.filter((e) => e.all_gates_pass).length,
    partial_eligible: entries.filter((e) => e.gates_passed >= 3 && !e.all_gates_pass).length,
    ineligible: entries.filter((e) => e.gates_passed < 3).length,
    entries,
    computed_at: now,
  };

  // Store
  const coll = db.collection(CANDIDATE_QUEUE_COLLECTION);
  await coll.deleteMany({});
  if (entries.length > 0) await coll.insertMany(entries);
  await coll.createIndex({ strategy_id: 1 }, { unique: true, background: true });
  await coll.createIndex({ admission_score: -1 }, { background: true });
  await coll.createIndex({ all_gates_pass: 1 }, { background: true });

  return summary;
}

// ── Portfolio Construction ─────────────────────────────────────────────────────

function buildPortfolioConstruction(
  genomes: StrategyGenome[],
  weights: StrategyAllocationWeight[]
): PortfolioConstructionResult {
  const now = new Date().toISOString();
  const weightMap = new Map(weights.map((w) => [w.strategy_id, w]));

  // Include MAIN_ENGINE strategies + candidates that pass all gates
  const eligible = genomes.filter((g) =>
    g.current_status === "MAIN_ENGINE" ||
    (g.candidate_eligible && g.current_status === "GRADE_1")
  );

  if (eligible.length === 0) {
    return {
      recommended_strategy_ids: [],
      recommended_strategy_names: [],
      weights: {},
      total_weight: 0,
      expected_portfolio_pf: 0,
      expected_portfolio_sharpe: 0,
      expected_max_drawdown: 0,
      family_exposure: {},
      max_single_weight: 0,
      min_single_weight: 0,
      hhi: 10000,
      diversification_ratio: 0,
      excluded_reasons: {},
      computed_at: now,
    };
  }

  const recIds: string[] = [];
  const recNames: string[] = [];
  const weightRecord: Record<string, number> = {};
  const familyExposure: Record<string, number> = {};
  const excludedReasons: Record<string, string> = {};

  let totalPf = 0, totalSharpe = 0, totalDd = 0, totalWt = 0;

  for (const g of eligible) {
    const w = weightMap.get(g.strategy_id);
    const allocWeight = w?.allocation_weight ?? 0;
    if (allocWeight <= 0) {
      excludedReasons[g.strategy_id] = "Zero allocation weight (negative Kelly)";
      continue;
    }
    recIds.push(g.strategy_id);
    recNames.push(g.strategy_name);
    weightRecord[g.strategy_id] = allocWeight;
    familyExposure[g.family] = (familyExposure[g.family] ?? 0) + allocWeight;

    const wFrac = allocWeight / 100;
    totalPf += g.metrics.profitFactor * wFrac;
    totalSharpe += g.metrics.sharpeRatio * wFrac;
    totalDd += g.metrics.maxDrawdown * wFrac;
    totalWt += wFrac;
  }

  const includedWeights = recIds.map((id) => weightRecord[id] ?? 0);
  const hhi = includedWeights.reduce((acc, w) => acc + Math.pow(w, 2), 0);
  const divRatio = includedWeights.length > 1
    ? Math.max(0, 1 - hhi / 10000)
    : 0;

  return {
    recommended_strategy_ids: recIds,
    recommended_strategy_names: recNames,
    weights: weightRecord,
    total_weight: +includedWeights.reduce((a, w) => a + w, 0).toFixed(1),
    expected_portfolio_pf: totalWt > 0 ? +(totalPf / totalWt).toFixed(3) : 0,
    expected_portfolio_sharpe: totalWt > 0 ? +(totalSharpe / totalWt).toFixed(3) : 0,
    expected_max_drawdown: totalWt > 0 ? +(totalDd / totalWt).toFixed(1) : 0,
    family_exposure: familyExposure,
    max_single_weight: Math.max(0, ...includedWeights),
    min_single_weight: includedWeights.length > 0 ? Math.min(...includedWeights) : 0,
    hhi: +hhi.toFixed(1),
    diversification_ratio: +divRatio.toFixed(3),
    excluded_reasons: excludedReasons,
    computed_at: now,
  };
}

// ── Full pipeline orchestration ────────────────────────────────────────────────

/**
 * Run the full APICAP pipeline. Typically called from evaluate endpoint.
 * Returns a summary of what was computed.
 */
export async function runFullPortfolioIntelligence(): Promise<PortfolioComputeResult> {
  const startMs = Date.now();
  const db = await getDb();

  // Load all strategy profiles
  const allProfiles = await db.collection<StrategyProfileDoc>(ISPAP_PROFILES_COLLECTION)
    .find({})
    .toArray();

  const activeProfiles = allProfiles.filter((p) => p.current_status !== "RETIRED");
  const mainEngineProfiles = allProfiles.filter((p) => p.current_status === "MAIN_ENGINE");
  const grade1Profiles = allProfiles.filter((p) => p.current_status === "GRADE_1");

  // SEP evidence
  const sepData = readSepStrategyEvidence();
  const sepMap = new Map(sepData?.map((r) => [r.strategy_id, r]) ?? []);

  // Phase 3: Correlation + Diversification
  const { summaries: divSummaries } = await computeAndStoreCorrelations(db, activeProfiles);
  const diversificationMap = new Map(divSummaries.map((s) => [s.strategy_name, s]));

  // Phase 6: Regime Intelligence
  const regimeMetrics = await computeAndStoreRegimeMetrics(db, activeProfiles);
  const regimeByStratId = new Map(regimeMetrics.map((r) => [r.strategy_id, r]));
  const regimeByName = new Map(regimeMetrics.map((r) => [r.strategy_name, r]));

  // Compute authority scores and metrics for allocation
  const allocationInputs = await Promise.all(
    (mainEngineProfiles.length > 0 ? mainEngineProfiles : activeProfiles.slice(0, 50)).map(async (p) => {
      const metrics = await computeMetricsForStrategy(db, p.strategy_name);
      const auth = computeAuthorityScore(metrics, p.current_status as any);
      const divScore = diversificationMap.get(p.strategy_name)?.diversification_score ?? 50;
      return {
        strategy_id: p.strategy_id,
        strategy_name: p.strategy_name,
        family: p.family,
        current_status: p.current_status as any,
        metrics,
        authority_score: auth.total,
        diversification_score: divScore,
      };
    })
  );

  // Phase 2: Allocation
  const allocationWeights = computeAllocations(allocationInputs);
  const allocationByStratId = new Map(allocationWeights.map((w) => [w.strategy_id, w]));

  // Store allocations
  const allocColl = db.collection(STRATEGY_ALLOCATIONS_COLLECTION);
  await allocColl.deleteMany({});
  if (allocationWeights.length > 0) await allocColl.insertMany(allocationWeights);
  await allocColl.createIndex({ strategy_id: 1 }, { unique: true, background: true });

  // Phase 7: Genomes — build for ALL active strategies
  const genomes: StrategyGenome[] = [];
  for (const profile of activeProfiles) {
    const metrics = await computeMetricsForStrategy(db, profile.strategy_name);
    const divSummary = diversificationMap.get(profile.strategy_name);
    const regimeSummary = regimeByStratId.get(profile.strategy_id);
    const allocation = allocationByStratId.get(profile.strategy_id);
    const sepRow = sepMap.get(profile.strategy_id);

    const genome = buildGenome({
      strategy_id: profile.strategy_id,
      strategy_name: profile.strategy_name,
      family: profile.family,
      category: profile.category,
      timeframe: profile.timeframe,
      current_status: profile.current_status as any,
      metrics,
      evidence_score: sepRow ? (sepRow.evidence_score / 100) * 15 : 0,
      diversification_score: divSummary?.diversification_score ?? 50,
      allocation_weight: allocation?.allocation_weight ?? 0,
      allocation_tier: allocation?.allocation_tier ?? "EXCLUDED",
      regime_strength_score: regimeSummary?.regime_strength_score ?? 0,
      regime_metrics: regimeSummary?.regimes ?? {},
      best_regime: regimeSummary?.best_regime ?? null,
      current_regime_pf: regimeSummary?.current_regime_pf ?? 0,
      promotion_count: profile.promotion_count,
      demotion_count: profile.demotion_count,
      last_promoted_at: profile.last_promoted_at,
      last_demoted_at: profile.last_demoted_at,
    });
    genomes.push(genome);
  }
  await storeGenomes(db, genomes);

  // Phase 9: Candidate Queue
  const candidateResult = await computeCandidateQueue(
    db,
    grade1Profiles,
    diversificationMap,
    regimeByName,
    allocationByStratId,
    mainEngineProfiles.length
  );

  // Phase 10: Portfolio Construction
  const construction = buildPortfolioConstruction(genomes, allocationWeights);
  const constructColl = db.collection(PORTFOLIO_CONSTRUCTION_COLLECTION);
  await constructColl.deleteMany({});
  await constructColl.insertOne(construction);

  return {
    correlations_computed: divSummaries.length,
    diversification_scores_computed: divSummaries.length,
    regime_profiles_computed: regimeMetrics.length,
    allocations_computed: allocationWeights.length,
    genomes_computed: genomes.length,
    candidates_evaluated: candidateResult.total_candidates,
    portfolio_constructed: true,
    elapsed_ms: Date.now() - startMs,
    computed_at: new Date().toISOString(),
  };
}

// ── Read helpers ───────────────────────────────────────────────────────────────

export async function getAllocations(): Promise<StrategyAllocationWeight[]> {
  const db = await getDb();
  return db.collection<StrategyAllocationWeight>(STRATEGY_ALLOCATIONS_COLLECTION)
    .find({})
    .sort({ allocation_weight: -1 })
    .toArray();
}

export async function getAllocationSummary(): Promise<PortfolioAllocationSummary> {
  const db = await getDb();
  const weights = await getAllocations();
  // Build metric map for summary
  const metricMap = new Map<string, any>();
  for (const w of weights) {
    const metrics = await computeMetricsForStrategy(db, w.strategy_name);
    metricMap.set(w.strategy_id, metrics);
  }
  return buildAllocationSummary(weights, metricMap);
}

export async function getCorrelationSummaries(limit = 100): Promise<StrategyCorrelationSummary[]> {
  const db = await getDb();
  return getDiversificationSummaries(db, limit);
}

export async function getRegimeIntelligence(limit = 200): Promise<StrategyRegimeMetrics[]> {
  const db = await getDb();
  return getAllRegimeMetrics(db, limit);
}

export async function getPortfolioGenomes(limit = 305): Promise<StrategyGenome[]> {
  const db = await getDb();
  return getAllGenomes(db, limit);
}

export async function getCandidateQueue(): Promise<CandidateQueueSummary> {
  const db = await getDb();
  const entries = await db.collection<CandidateQueueEntry>(CANDIDATE_QUEUE_COLLECTION)
    .find({})
    .sort({ admission_score: -1 })
    .toArray();

  if (entries.length === 0) {
    return {
      total_candidates: 0,
      fully_eligible: 0,
      partial_eligible: 0,
      ineligible: 0,
      entries: [],
      computed_at: new Date().toISOString(),
    };
  }

  return {
    total_candidates: entries.length,
    fully_eligible: entries.filter((e) => e.all_gates_pass).length,
    partial_eligible: entries.filter((e) => e.gates_passed >= 3 && !e.all_gates_pass).length,
    ineligible: entries.filter((e) => e.gates_passed < 3).length,
    entries,
    computed_at: entries[0]?.queued_at ?? new Date().toISOString(),
  };
}

export async function getPortfolioConstruction(): Promise<PortfolioConstructionResult | null> {
  const db = await getDb();
  const doc = await db.collection<PortfolioConstructionResult>(PORTFOLIO_CONSTRUCTION_COLLECTION)
    .findOne({});
  return doc ?? null;
}

export async function getGenomeById(strategyId: string): Promise<StrategyGenome | null> {
  const db = await getDb();
  const { getGenome } = await import("./genomeEngine");
  return getGenome(db, strategyId);
}

export async function getCurrentMarketRegime() {
  const db = await getDb();
  return getCurrentRegime(db);
}
