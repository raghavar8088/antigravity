import { randomUUID } from "crypto";
import type { Db } from "mongodb";
import { getDb } from "@/lib/broker/mongoTradesClient";
import { STRATEGY_CATALOG } from "./strategyCatalog";
import {
  checkDemotionRisk,
  checkPromotionEligible,
  checkRetirementCandidate,
  computePromotionProgress,
  getDemotedStatus,
  getTargetStatus,
} from "./gradeThresholds";
import type {
  GradePromotionProgress,
  PipelineStageState,
  PipelineState,
  StrategyEventDoc,
  StrategyGradeMetrics,
  StrategyLeaderboardEntry,
  StrategyProfileDoc,
  StrategyStatus,
  StrategyWithMetrics,
} from "./types";

// ── Collection names ────────────────────────────────────────────────────────
export const ISPAP_PROFILES_COLLECTION = "strategy_authority_profiles";
export const ISPAP_EVENTS_COLLECTION = "strategy_authority_events";

const MOCK_TRADES_COLLECTION = "mock_trades";

const STAGE_LABELS: Record<StrategyStatus, string> = {
  GRADE_5: "Grade 5 — Discovery",
  GRADE_4: "Grade 4 — Initial Validation",
  GRADE_3: "Grade 3 — Robustness Testing",
  GRADE_2: "Grade 2 — Institutional Screening",
  GRADE_1: "Grade 1 — Capital Candidate",
  MAIN_ENGINE: "Main Mock Trading Engine",
  RETIRED: "Retired",
};

const ORDERED_STAGES: StrategyStatus[] = [
  "GRADE_5", "GRADE_4", "GRADE_3", "GRADE_2", "GRADE_1", "MAIN_ENGINE",
];

// ── Database helpers ─────────────────────────────────────────────────────────

function db(): Promise<Db> {
  return getDb();
}

/** Ensure indexes on first use. */
async function ensureIndexes(d: Db): Promise<void> {
  const profiles = d.collection(ISPAP_PROFILES_COLLECTION);
  const events = d.collection(ISPAP_EVENTS_COLLECTION);

  await Promise.all([
    profiles.createIndex({ strategy_id: 1 }, { unique: true, background: true }),
    profiles.createIndex({ current_status: 1 }, { background: true }),
    profiles.createIndex({ family: 1 }, { background: true }),
    events.createIndex({ strategy_id: 1 }, { background: true }),
    events.createIndex({ event_type: 1, created_at: -1 }, { background: true }),
    events.createIndex({ created_at: -1 }, { background: true }),
  ]);
}

// ── Migration ────────────────────────────────────────────────────────────────

/** Seed all 305 catalog strategies into Grade 5 (idempotent). */
export async function migrateAllToGrade5(): Promise<{ inserted: number; alreadyExisted: number }> {
  const d = await db();
  await ensureIndexes(d);

  const profiles = d.collection<StrategyProfileDoc>(ISPAP_PROFILES_COLLECTION);
  const events = d.collection<StrategyEventDoc>(ISPAP_EVENTS_COLLECTION);

  const now = new Date().toISOString();
  let inserted = 0;
  let alreadyExisted = 0;

  for (const entry of STRATEGY_CATALOG) {
    const existing = await profiles.findOne({ strategy_id: entry.id });
    if (existing) {
      alreadyExisted++;
      continue;
    }

    const profile: StrategyProfileDoc = {
      strategy_id: entry.id,
      strategy_name: entry.name,
      family: entry.family,
      category: entry.category,
      timeframe: entry.timeframe,
      current_status: "GRADE_5",
      current_grade: 5,
      migrated_at: now,
      last_evaluated_at: null,
      last_promoted_at: null,
      last_demoted_at: null,
      promotion_count: 0,
      demotion_count: 0,
      retired_at: null,
      retirement_reason: null,
    };

    await profiles.insertOne(profile);
    await events.insertOne({
      event_id: randomUUID(),
      strategy_id: entry.id,
      strategy_name: entry.name,
      event_type: "MIGRATION",
      from_status: "GRADE_5",
      to_status: "GRADE_5",
      reason: "Initial ISPAP migration — all strategies enter at Grade 5",
      metrics_snapshot: null,
      created_at: now,
    });
    inserted++;
  }

  return { inserted, alreadyExisted };
}

// ── Grade Metrics Computation ────────────────────────────────────────────────

/** Compute grade metrics from mock_trades for a given strategy_name. */
async function computeMetricsForStrategy(
  d: Db,
  strategyName: string,
  strategyId?: string,
): Promise<StrategyGradeMetrics> {
  const matchConditions: Record<string, unknown>[] = [{ strategy_name: strategyName }];
  if (strategyId) {
    matchConditions.push({ ispap_strategy_id: strategyId });
    matchConditions.push({ "raw_trade.ispapStrategyId": strategyId });
  }

  const result = await d.collection(MOCK_TRADES_COLLECTION).aggregate([
    {
      $match: {
        $or: matchConditions,
        status: "CLOSED",
      },
    },
    {
      $group: {
        _id: null,
        closedTrades: { $sum: 1 },
        wins: { $sum: { $cond: [{ $gt: ["$realized_pnl", 0] }, 1, 0] } },
        totalPnl: { $sum: "$realized_pnl" },
        grossProfit: {
          $sum: { $cond: [{ $gt: ["$realized_pnl", 0] }, "$realized_pnl", 0] },
        },
        grossLoss: {
          $sum: { $cond: [{ $lt: ["$realized_pnl", 0] }, { $abs: "$realized_pnl" }, 0] },
        },
        pnlValues: { $push: "$realized_pnl" },
      },
    },
  ]).toArray();

  if (!result.length || result[0].closedTrades === 0) {
    return {
      closedTrades: 0,
      winRate: 0,
      profitFactor: 0,
      expectancy: 0,
      maxDrawdown: 0,
      sharpeRatio: 0,
      totalPnl: 0,
    };
  }

  const r = result[0];
  const closedTrades: number = r.closedTrades;
  const winRate = (r.wins / closedTrades) * 100;
  const profitFactor = r.grossLoss > 0 ? r.grossProfit / r.grossLoss : r.grossProfit > 0 ? 999 : 0;
  const expectancy = r.totalPnl / closedTrades;
  const totalPnl: number = r.totalPnl;

  // Compute max drawdown and Sharpe from the PnL array
  const pnlValues: number[] = r.pnlValues ?? [];
  let maxDrawdown = 0;
  let peak = 0;
  let runningPnl = 0;
  for (const p of pnlValues) {
    runningPnl += p;
    if (runningPnl > peak) peak = runningPnl;
    const dd = peak > 0 ? ((peak - runningPnl) / peak) * 100 : 0;
    if (dd > maxDrawdown) maxDrawdown = dd;
  }

  let sharpeRatio = 0;
  if (pnlValues.length > 1) {
    const mean = totalPnl / pnlValues.length;
    const variance =
      pnlValues.reduce((acc, v) => acc + Math.pow(v - mean, 2), 0) / pnlValues.length;
    const std = Math.sqrt(variance);
    sharpeRatio = std > 0 ? (mean / std) * Math.sqrt(252) : 0;
  }

  return {
    closedTrades,
    winRate: +winRate.toFixed(2),
    profitFactor: +profitFactor.toFixed(3),
    expectancy: +expectancy.toFixed(2),
    maxDrawdown: +maxDrawdown.toFixed(2),
    sharpeRatio: +sharpeRatio.toFixed(3),
    totalPnl: +totalPnl.toFixed(2),
  };
}

// ── Grade Evaluation ─────────────────────────────────────────────────────────

/** Evaluate all active strategies: promote, demote, or retire as needed. */
export async function evaluateAndUpdateGrades(): Promise<{
  promoted: number;
  demoted: number;
  retired: number;
  evaluated: number;
}> {
  const d = await db();
  const profiles = d.collection<StrategyProfileDoc>(ISPAP_PROFILES_COLLECTION);
  const events = d.collection<StrategyEventDoc>(ISPAP_EVENTS_COLLECTION);
  const now = new Date().toISOString();

  const activeProfiles = await profiles
    .find({ current_status: { $nin: ["RETIRED"] } })
    .toArray();

  let promoted = 0;
  let demoted = 0;
  let retired = 0;

  for (const profile of activeProfiles) {
    const metrics = await computeMetricsForStrategy(d, profile.strategy_name, profile.strategy_id);

    // Retirement check
    if (
      profile.current_status !== "MAIN_ENGINE" &&
      checkRetirementCandidate(metrics)
    ) {
      await profiles.updateOne(
        { strategy_id: profile.strategy_id },
        {
          $set: {
            current_status: "RETIRED",
            current_grade: -1,
            retired_at: now,
            retirement_reason: `PF=${metrics.profitFactor.toFixed(2)} < 0.90 with ${metrics.closedTrades} closed trades`,
            last_evaluated_at: now,
          },
        }
      );
      await events.insertOne({
        event_id: randomUUID(),
        strategy_id: profile.strategy_id,
        strategy_name: profile.strategy_name,
        event_type: "RETIREMENT",
        from_status: profile.current_status,
        to_status: "RETIRED",
        reason: `Retired: PF ${metrics.profitFactor.toFixed(2)} < 0.90 after ${metrics.closedTrades} trades`,
        metrics_snapshot: metrics,
        created_at: now,
      });
      retired++;
      continue;
    }

    // Promotion check
    if (
      profile.current_status !== "MAIN_ENGINE" &&
      checkPromotionEligible(metrics, profile.current_status as StrategyStatus)
    ) {
      const targetStatus = getTargetStatus(profile.current_status as StrategyStatus);
      if (targetStatus) {
        const targetGrade =
          targetStatus === "MAIN_ENGINE"
            ? 0
            : parseInt(targetStatus.replace("GRADE_", ""), 10);
        await profiles.updateOne(
          { strategy_id: profile.strategy_id },
          {
            $set: {
              current_status: targetStatus,
              current_grade: targetGrade,
              last_promoted_at: now,
              last_evaluated_at: now,
            },
            $inc: { promotion_count: 1 },
          }
        );
        await events.insertOne({
          event_id: randomUUID(),
          strategy_id: profile.strategy_id,
          strategy_name: profile.strategy_name,
          event_type: "PROMOTION",
          from_status: profile.current_status as StrategyStatus,
          to_status: targetStatus,
          reason: `Promoted: WR=${metrics.winRate.toFixed(1)}% PF=${metrics.profitFactor.toFixed(2)} Trades=${metrics.closedTrades}`,
          metrics_snapshot: metrics,
          created_at: now,
        });
        promoted++;
        continue;
      }
    }

    // Demotion check (only if enough trades)
    if (
      profile.current_status !== "GRADE_5" &&
      profile.current_status !== "RETIRED" &&
      metrics.closedTrades >= 30 &&
      checkDemotionRisk(metrics)
    ) {
      const demotedStatus = getDemotedStatus(profile.current_status as StrategyStatus);
      const demotedGrade = demotedStatus === "RETIRED" ? -1 : parseInt(demotedStatus.replace("GRADE_", ""), 10);
      await profiles.updateOne(
        { strategy_id: profile.strategy_id },
        {
          $set: {
            current_status: demotedStatus,
            current_grade: demotedGrade,
            last_demoted_at: now,
            last_evaluated_at: now,
          },
          $inc: { demotion_count: 1 },
        }
      );
      await events.insertOne({
        event_id: randomUUID(),
        strategy_id: profile.strategy_id,
        strategy_name: profile.strategy_name,
        event_type: "DEMOTION",
        from_status: profile.current_status as StrategyStatus,
        to_status: demotedStatus,
        reason: `Demoted: WR=${metrics.winRate.toFixed(1)}% PF=${metrics.profitFactor.toFixed(2)} below threshold`,
        metrics_snapshot: metrics,
        created_at: now,
      });
      demoted++;
      continue;
    }

    // Just update evaluation timestamp
    await profiles.updateOne(
      { strategy_id: profile.strategy_id },
      { $set: { last_evaluated_at: now } }
    );
  }

  return { promoted, demoted, retired, evaluated: activeProfiles.length };
}

// ── Pipeline State ───────────────────────────────────────────────────────────

export async function getPipelineState(): Promise<PipelineState> {
  const d = await db();
  const profiles = d.collection<StrategyProfileDoc>(ISPAP_PROFILES_COLLECTION);
  const events = d.collection<StrategyEventDoc>(ISPAP_EVENTS_COLLECTION);

  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const todayIso = today.toISOString();

  const [allProfiles, todayEvents] = await Promise.all([
    profiles.find({}).toArray(),
    events.find({ created_at: { $gte: todayIso } }).toArray(),
  ]);

  const totalStrategies = allProfiles.length;
  const totalRetired = allProfiles.filter((p) => p.current_status === "RETIRED").length;
  const totalMainEngine = allProfiles.filter((p) => p.current_status === "MAIN_ENGINE").length;
  const totalActive = totalStrategies - totalRetired;

  const promotionsToday = todayEvents.filter((e) => e.event_type === "PROMOTION").length;
  const demotionsToday = todayEvents.filter((e) => e.event_type === "DEMOTION").length;
  const retirementsToday = todayEvents.filter((e) => e.event_type === "RETIREMENT").length;

  const stages: PipelineStageState[] = [];

  for (const status of ORDERED_STAGES) {
    const stageProfiles = allProfiles.filter((p) => p.current_status === status);

    // Compute metrics for this stage's strategies (sample top 10 for performance)
    const metricsMap = new Map<string, StrategyGradeMetrics>();
    const sample = stageProfiles.slice(0, 20);
    await Promise.all(
      sample.map(async (p) => {
        const m = await computeMetricsForStrategy(d, p.strategy_name, p.strategy_id);
        metricsMap.set(p.strategy_id, m);
      })
    );

    const strategyItems: StrategyWithMetrics[] = sample.map((p) => {
      const metrics = metricsMap.get(p.strategy_id) ?? {
        closedTrades: 0, winRate: 0, profitFactor: 0, expectancy: 0,
        maxDrawdown: 0, sharpeRatio: 0, totalPnl: 0,
      };
      const promoEligible = checkPromotionEligible(metrics, p.current_status as StrategyStatus);
      const demRisk = checkDemotionRisk(metrics);
      const retCand = checkRetirementCandidate(metrics);
      const progress = computePromotionProgress(metrics, p.current_status as StrategyStatus);
      return {
        ...p,
        metrics,
        promotionEligible: promoEligible,
        demotionRisk: demRisk,
        retirementCandidate: retCand,
        promotionProgress: progress as GradePromotionProgress | null,
      };
    });

    const metricsArr = strategyItems.map((s) => s.metrics);
    const avgWinRate = metricsArr.length
      ? metricsArr.reduce((a, m) => a + m.winRate, 0) / metricsArr.length
      : 0;
    const avgPF = metricsArr.length
      ? metricsArr.reduce((a, m) => a + m.profitFactor, 0) / metricsArr.length
      : 0;
    const avgExp = metricsArr.length
      ? metricsArr.reduce((a, m) => a + m.expectancy, 0) / metricsArr.length
      : 0;
    const avgDD = metricsArr.length
      ? metricsArr.reduce((a, m) => a + m.maxDrawdown, 0) / metricsArr.length
      : 0;

    const sorted = [...strategyItems].sort((a, b) => b.metrics.profitFactor - a.metrics.profitFactor);

    stages.push({
      status,
      label: STAGE_LABELS[status],
      count: stageProfiles.length,
      avgWinRate: +avgWinRate.toFixed(1),
      avgProfitFactor: +avgPF.toFixed(3),
      avgExpectancy: +avgExp.toFixed(2),
      avgDrawdown: +avgDD.toFixed(1),
      promotionCandidates: strategyItems.filter((s) => s.promotionEligible).length,
      demotionRisk: strategyItems.filter((s) => s.demotionRisk).length,
      topPerformers: sorted.slice(0, 5),
      bottomPerformers: sorted.slice(-3).reverse(),
    });
  }

  return {
    stages,
    totalStrategies,
    totalActive,
    totalRetired,
    totalMainEngine,
    promotionsToday,
    demotionsToday,
    retirementsToday,
    lastUpdated: new Date().toISOString(),
  };
}

// ── Leaderboard ──────────────────────────────────────────────────────────────

export async function getLeaderboard(limit = 100): Promise<StrategyLeaderboardEntry[]> {
  const d = await db();
  const profiles = d.collection<StrategyProfileDoc>(ISPAP_PROFILES_COLLECTION);

  const allProfiles = await profiles
    .find({ current_status: { $nin: ["RETIRED"] } })
    .limit(limit)
    .toArray();

  const entries: StrategyLeaderboardEntry[] = [];
  for (const profile of allProfiles) {
    const metrics = await computeMetricsForStrategy(d, profile.strategy_name, profile.strategy_id);
    entries.push({
      rank: 0,
      strategy_id: profile.strategy_id,
      strategy_name: profile.strategy_name,
      family: profile.family,
      current_status: profile.current_status as StrategyStatus,
      metrics,
      promotionEligible: checkPromotionEligible(metrics, profile.current_status as StrategyStatus),
      demotionRisk: checkDemotionRisk(metrics),
    });
  }

  // Sort by profitFactor desc, then assign ranks
  entries.sort((a, b) => b.metrics.profitFactor - a.metrics.profitFactor);
  entries.forEach((e, i) => (e.rank = i + 1));

  return entries;
}

// ── Events ───────────────────────────────────────────────────────────────────

export async function getRecentPromotions(limit = 50): Promise<StrategyEventDoc[]> {
  const d = await db();
  return d
    .collection<StrategyEventDoc>(ISPAP_EVENTS_COLLECTION)
    .find({ event_type: "PROMOTION" })
    .sort({ created_at: -1 })
    .limit(limit)
    .toArray();
}

export async function getRecentDemotions(limit = 50): Promise<StrategyEventDoc[]> {
  const d = await db();
  return d
    .collection<StrategyEventDoc>(ISPAP_EVENTS_COLLECTION)
    .find({ event_type: "DEMOTION" })
    .sort({ created_at: -1 })
    .limit(limit)
    .toArray();
}

export async function getRetirements(): Promise<StrategyProfileDoc[]> {
  const d = await db();
  return d
    .collection<StrategyProfileDoc>(ISPAP_PROFILES_COLLECTION)
    .find({ current_status: "RETIRED" })
    .sort({ retired_at: -1 })
    .toArray();
}

export async function getMainEngineStrategies(): Promise<StrategyWithMetrics[]> {
  const d = await db();
  const profiles = d.collection<StrategyProfileDoc>(ISPAP_PROFILES_COLLECTION);

  const mainProfiles = await profiles.find({ current_status: "MAIN_ENGINE" }).toArray();
  const results: StrategyWithMetrics[] = [];

  for (const profile of mainProfiles) {
    const metrics = await computeMetricsForStrategy(d, profile.strategy_name, profile.strategy_id);
    results.push({
      ...profile,
      metrics,
      promotionEligible: false,
      demotionRisk: checkDemotionRisk(metrics),
      retirementCandidate: checkRetirementCandidate(metrics),
      promotionProgress: null,
    });
  }

  return results.sort((a, b) => b.metrics.profitFactor - a.metrics.profitFactor);
}

export interface StageSummary {
  status: StrategyStatus;
  totalStrategies: number;
  promotionCandidates: number;
  demotionCandidates: number;
  avgProfitFactor: number;
  avgSharpe: number;
  avgExpectancy: number;
  avgDrawdown: number;
}

async function profileToStrategyWithMetrics(
  d: Db,
  profile: StrategyProfileDoc
): Promise<StrategyWithMetrics> {
  const metrics = await computeMetricsForStrategy(d, profile.strategy_name, profile.strategy_id);
  const status = profile.current_status as StrategyStatus;
  return {
    ...profile,
    metrics,
    promotionEligible: checkPromotionEligible(metrics, status),
    demotionRisk: checkDemotionRisk(metrics),
    retirementCandidate: checkRetirementCandidate(metrics),
    promotionProgress: computePromotionProgress(metrics, status) as GradePromotionProgress | null,
  };
}

/** Full strategy roster for a pipeline stage (MongoDB authority). */
export async function getStrategiesByStatus(status: StrategyStatus): Promise<{
  strategies: StrategyWithMetrics[];
  summary: StageSummary;
}> {
  const d = await db();
  const profiles = d.collection<StrategyProfileDoc>(ISPAP_PROFILES_COLLECTION);
  const stageProfiles = await profiles.find({ current_status: status }).toArray();

  const strategies: StrategyWithMetrics[] = [];
  for (const profile of stageProfiles) {
    strategies.push(await profileToStrategyWithMetrics(d, profile));
  }

  strategies.sort((a, b) => b.metrics.profitFactor - a.metrics.profitFactor);

  const metricsArr = strategies.map((s) => s.metrics);
  const n = metricsArr.length;
  const avg = (fn: (m: StrategyGradeMetrics) => number) =>
    n ? metricsArr.reduce((a, m) => a + fn(m), 0) / n : 0;

  return {
    strategies,
    summary: {
      status,
      totalStrategies: strategies.length,
      promotionCandidates: strategies.filter((s) => s.promotionEligible).length,
      demotionCandidates: strategies.filter((s) => s.demotionRisk).length,
      avgProfitFactor: +avg((m) => m.profitFactor).toFixed(3),
      avgSharpe: +avg((m) => m.sharpeRatio).toFixed(3),
      avgExpectancy: +avg((m) => m.expectancy).toFixed(2),
      avgDrawdown: +avg((m) => m.maxDrawdown).toFixed(1),
    },
  };
}

export async function getMigrationStatus(): Promise<{
  migrated: boolean;
  totalInDb: number;
  totalCatalog: number;
}> {
  const d = await db();
  const count = await d.collection(ISPAP_PROFILES_COLLECTION).countDocuments();
  return {
    migrated: count > 0,
    totalInDb: count,
    totalCatalog: STRATEGY_CATALOG.length,
  };
}

// ── All Events (Promotion + Demotion + Retirement + Migration) ───────────────

export async function getAllEvents(limit = 500): Promise<StrategyEventDoc[]> {
  const d = await db();
  return d
    .collection<StrategyEventDoc>(ISPAP_EVENTS_COLLECTION)
    .find({})
    .sort({ created_at: -1 })
    .limit(limit)
    .toArray();
}

export async function getEventsByType(
  type: StrategyEventDoc["event_type"],
  limit = 200
): Promise<StrategyEventDoc[]> {
  const d = await db();
  return d
    .collection<StrategyEventDoc>(ISPAP_EVENTS_COLLECTION)
    .find({ event_type: type })
    .sort({ created_at: -1 })
    .limit(limit)
    .toArray();
}

// ── Family Intelligence ───────────────────────────────────────────────────────

export interface FamilyIntelligenceRow {
  family: string;
  total: number;
  byStatus: Partial<Record<StrategyStatus, number>>;
  avgProfitFactor: number;
  avgExpectancy: number;
  avgWinRate: number;
  avgDrawdown: number;
  avgSharpe: number;
  mainEngineCount: number;
  retiredCount: number;
  promotionCandidates: number;
  demotionRisk: number;
}

export async function getFamilyIntelligence(): Promise<FamilyIntelligenceRow[]> {
  const d = await db();
  const profiles = d.collection<StrategyProfileDoc>(ISPAP_PROFILES_COLLECTION);

  const allProfiles = await profiles.find({}).toArray();
  if (allProfiles.length === 0) return [];

  // Group by family
  const familyMap = new Map<string, StrategyProfileDoc[]>();
  for (const p of allProfiles) {
    const key = p.family || "Unknown";
    if (!familyMap.has(key)) familyMap.set(key, []);
    familyMap.get(key)!.push(p);
  }

  const rows: FamilyIntelligenceRow[] = [];

  for (const [family, members] of familyMap.entries()) {
    const byStatus: Partial<Record<StrategyStatus, number>> = {};
    for (const m of members) {
      const s = m.current_status as StrategyStatus;
      byStatus[s] = (byStatus[s] ?? 0) + 1;
    }

    // Sample up to 10 active members for metrics
    const activeSample = members
      .filter((m) => m.current_status !== "RETIRED")
      .slice(0, 10);

    let pfSum = 0, expSum = 0, wrSum = 0, ddSum = 0, shSum = 0, promCand = 0, demRisk = 0;
    let sampleCount = 0;

    for (const p of activeSample) {
      const m = await computeMetricsForStrategy(d, p.strategy_name, p.strategy_id);
      pfSum += m.profitFactor;
      expSum += m.expectancy;
      wrSum += m.winRate;
      ddSum += m.maxDrawdown;
      shSum += m.sharpeRatio;
      if (checkPromotionEligible(m, p.current_status as StrategyStatus)) promCand++;
      if (checkDemotionRisk(m)) demRisk++;
      sampleCount++;
    }

    rows.push({
      family,
      total: members.length,
      byStatus,
      avgProfitFactor: sampleCount ? +( pfSum / sampleCount).toFixed(3) : 0,
      avgExpectancy: sampleCount ? +(expSum / sampleCount).toFixed(2) : 0,
      avgWinRate: sampleCount ? +(wrSum / sampleCount).toFixed(1) : 0,
      avgDrawdown: sampleCount ? +(ddSum / sampleCount).toFixed(1) : 0,
      avgSharpe: sampleCount ? +(shSum / sampleCount).toFixed(3) : 0,
      mainEngineCount: byStatus["MAIN_ENGINE"] ?? 0,
      retiredCount: byStatus["RETIRED"] ?? 0,
      promotionCandidates: promCand,
      demotionRisk: demRisk,
    });
  }

  // Sort by avgProfitFactor desc
  return rows.sort((a, b) => b.avgProfitFactor - a.avgProfitFactor);
}

// ── Per-strategy detail with authority score ──────────────────────────────────

export interface StrategyDetail extends StrategyWithMetrics {
  authorityScore: number;
  authorityTier: string;
}

export async function getStrategyDetail(strategyId: string): Promise<StrategyDetail | null> {
  const d = await db();
  const profile = await d
    .collection<StrategyProfileDoc>(ISPAP_PROFILES_COLLECTION)
    .findOne({ strategy_id: strategyId });

  if (!profile) return null;

  const metrics = await computeMetricsForStrategy(d, profile.strategy_name, profile.strategy_id);
  const { computeAuthorityScore } = await import("./authorityScore");
  const scoreBreakdown = computeAuthorityScore(metrics, profile.current_status as StrategyStatus);

  const progress = computePromotionProgress(metrics, profile.current_status as StrategyStatus);

  return {
    ...profile,
    metrics,
    promotionEligible: checkPromotionEligible(metrics, profile.current_status as StrategyStatus),
    demotionRisk: checkDemotionRisk(metrics),
    retirementCandidate: checkRetirementCandidate(metrics),
    promotionProgress: progress as GradePromotionProgress | null,
    authorityScore: scoreBreakdown.total,
    authorityTier: scoreBreakdown.tier,
  };
}

// ── Pipeline summary counts (fast path — no metrics computation) ──────────────

export async function getPipelineCounts(): Promise<{
  total: number;
  byStatus: Partial<Record<StrategyStatus, number>>;
  promotionsToday: number;
  demotionsToday: number;
  retirementsToday: number;
}> {
  const d = await db();
  const profiles = d.collection<StrategyProfileDoc>(ISPAP_PROFILES_COLLECTION);
  const events = d.collection<StrategyEventDoc>(ISPAP_EVENTS_COLLECTION);

  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const todayIso = today.toISOString();

  const [allProfiles, todayEvents] = await Promise.all([
    profiles.find({}, { projection: { current_status: 1 } }).toArray(),
    events.find({ created_at: { $gte: todayIso } }).toArray(),
  ]);

  const byStatus: Partial<Record<StrategyStatus, number>> = {};
  for (const p of allProfiles) {
    const s = p.current_status as StrategyStatus;
    byStatus[s] = (byStatus[s] ?? 0) + 1;
  }

  return {
    total: allProfiles.length,
    byStatus,
    promotionsToday: todayEvents.filter((e) => e.event_type === "PROMOTION").length,
    demotionsToday: todayEvents.filter((e) => e.event_type === "DEMOTION").length,
    retirementsToday: todayEvents.filter((e) => e.event_type === "RETIREMENT").length,
  };
}

// ── Lightweight stage list for the tower (no metrics) ────────────────────────

export async function getTowerCounts(): Promise<{
  status: StrategyStatus;
  count: number;
  promotionEventsTotal: number;
  demotionEventsTotal: number;
}[]> {
  const d = await db();

  const [profileCounts, promotionCounts, demotionCounts] = await Promise.all([
    d.collection(ISPAP_PROFILES_COLLECTION).aggregate([
      { $group: { _id: "$current_status", count: { $sum: 1 } } },
    ]).toArray(),
    d.collection(ISPAP_EVENTS_COLLECTION).aggregate([
      { $match: { event_type: "PROMOTION" } },
      { $group: { _id: "$to_status", count: { $sum: 1 } } },
    ]).toArray(),
    d.collection(ISPAP_EVENTS_COLLECTION).aggregate([
      { $match: { event_type: "DEMOTION" } },
      { $group: { _id: "$to_status", count: { $sum: 1 } } },
    ]).toArray(),
  ]);

  const statusOrder: StrategyStatus[] = ["GRADE_5", "GRADE_4", "GRADE_3", "GRADE_2", "GRADE_1", "MAIN_ENGINE", "RETIRED"];

  return statusOrder.map((status) => ({
    status,
    count: profileCounts.find((r) => r._id === status)?.count ?? 0,
    promotionEventsTotal: promotionCounts.find((r) => r._id === status)?.count ?? 0,
    demotionEventsTotal: demotionCounts.find((r) => r._id === status)?.count ?? 0,
  }));
}
