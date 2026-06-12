/**
 * APICAP Phase 3 — Correlation Intelligence Engine
 *
 * Computes pairwise Pearson correlation between strategy PnL streams.
 * Derives diversification scores. Assigns correlation clusters.
 *
 * Data source: mock_trades collection (strategy_name, closed_at, realized_pnl)
 * Output: strategy_correlations + strategy_diversification collections
 */

import type { Db } from "mongodb";
import type { StrategyCorrelationPair, StrategyCorrelationSummary } from "./portfolioTypes";

export const STRATEGY_CORRELATIONS_COLLECTION = "strategy_correlations";
export const STRATEGY_DIVERSIFICATION_COLLECTION = "strategy_diversification";

// Minimum overlap days before we trust a correlation
const MIN_OVERLAP_DAYS = 10;
// Families that get an extra concentration penalty
const HIGH_DENSITY_FAMILY_THRESHOLD = 5;

// ── Math helpers ──────────────────────────────────────────────────────────────

function pearson(a: number[], b: number[]): number {
  const n = a.length;
  if (n < 2) return 0;
  let sumA = 0, sumB = 0;
  for (let i = 0; i < n; i++) { sumA += a[i]; sumB += b[i]; }
  const mA = sumA / n;
  const mB = sumB / n;
  let cov = 0, varA = 0, varB = 0;
  for (let i = 0; i < n; i++) {
    const da = a[i] - mA;
    const db = b[i] - mB;
    cov += da * db;
    varA += da * da;
    varB += db * db;
  }
  const denom = Math.sqrt(varA * varB);
  return denom < 1e-10 ? 0 : Math.max(-1, Math.min(1, cov / denom));
}

// Cluster assignment via single-linkage at 0.70 absolute correlation threshold
function assignClusters(
  names: string[],
  corrMap: Map<string, number>
): Map<string, number> {
  const THRESHOLD = 0.70;
  const clusterOf = new Map<string, number>();
  let nextCluster = 1;

  for (const name of names) {
    if (!clusterOf.has(name)) {
      clusterOf.set(name, nextCluster++);
    }
  }

  let merged = true;
  while (merged) {
    merged = false;
    for (const a of names) {
      for (const b of names) {
        if (a >= b) continue;
        const key = `${a}|||${b}`;
        const corr = corrMap.get(key);
        if (corr === undefined) continue;
        if (Math.abs(corr) >= THRESHOLD) {
          const ca = clusterOf.get(a)!;
          const cb = clusterOf.get(b)!;
          if (ca !== cb) {
            // Merge larger cluster id into smaller
            const keep = Math.min(ca, cb);
            const replace = Math.max(ca, cb);
            for (const [n, c] of clusterOf) {
              if (c === replace) clusterOf.set(n, keep);
            }
            merged = true;
          }
        }
      }
    }
  }
  return clusterOf;
}

// ── Main computation ───────────────────────────────────────────────────────────

/**
 * Compute daily PnL for all strategies from mock_trades.
 * Returns: Map<strategyName, Map<dateKey, totalPnl>>
 */
export async function buildDailyPnlMatrix(
  db: Db,
  lookbackDays = 180
): Promise<Map<string, Map<string, number>>> {
  const cutoff = new Date(Date.now() - lookbackDays * 86_400_000).toISOString();

  const agg = await db.collection("mock_trades").aggregate([
    {
      $match: {
        status: "CLOSED",
        closed_at: { $gte: cutoff },
      },
    },
    {
      $project: {
        strategy_name: 1,
        dateKey: { $dateToString: { format: "%Y-%m-%d", date: { $dateFromString: { dateString: "$closed_at" } } } },
        pnl: {
          $subtract: [
            { $ifNull: ["$gross_pnl", 0] },
            { $add: [{ $ifNull: ["$fees", 0] }, { $ifNull: ["$funding_costs", 0] }] },
          ],
        },
      },
    },
    {
      $group: {
        _id: { strategy: "$strategy_name", date: "$dateKey" },
        dailyPnl: { $sum: "$pnl" },
      },
    },
  ]).toArray();

  const matrix = new Map<string, Map<string, number>>();
  for (const row of agg) {
    const name = row._id.strategy as string;
    const date = row._id.date as string;
    if (!matrix.has(name)) matrix.set(name, new Map());
    matrix.get(name)!.set(date, row.dailyPnl as number);
  }
  return matrix;
}

/**
 * Compute all pairwise correlations from a daily PnL matrix.
 * Only pairs with >= MIN_OVERLAP_DAYS common trading days are computed.
 */
export function computePairwiseCorrelations(
  matrix: Map<string, Map<string, number>>
): { pairs: Map<string, number>; overlapMap: Map<string, number> } {
  const names = Array.from(matrix.keys());
  const pairs = new Map<string, number>();       // "a|||b" → correlation
  const overlapMap = new Map<string, number>(); // "a|||b" → overlap days

  for (let i = 0; i < names.length; i++) {
    for (let j = i + 1; j < names.length; j++) {
      const nameA = names[i];
      const nameB = names[j];
      const datesA = matrix.get(nameA)!;
      const datesB = matrix.get(nameB)!;

      // Find shared dates
      const sharedDates: string[] = [];
      for (const d of datesA.keys()) {
        if (datesB.has(d)) sharedDates.push(d);
      }

      if (sharedDates.length < MIN_OVERLAP_DAYS) continue;

      const pnlA = sharedDates.map((d) => datesA.get(d)!);
      const pnlB = sharedDates.map((d) => datesB.get(d)!);
      const corr = pearson(pnlA, pnlB);

      const key = nameA < nameB ? `${nameA}|||${nameB}` : `${nameB}|||${nameA}`;
      pairs.set(key, corr);
      overlapMap.set(key, sharedDates.length);
    }
  }

  return { pairs, overlapMap };
}

/**
 * Compute diversification scores for all strategies.
 * diversificationScore = 100 − avgAbsCorr * 100 − familyConcentrationPenalty
 */
export function computeDiversificationScores(
  strategyNames: string[],
  strategyFamilies: Map<string, string>,
  pairs: Map<string, number>,
  mainEngineProfiles: { family: string }[]
): Map<string, StrategyCorrelationSummary> {
  const now = new Date().toISOString();
  const result = new Map<string, StrategyCorrelationSummary>();

  // Family count in main engine
  const familyMainCount: Record<string, number> = {};
  for (const p of mainEngineProfiles) {
    familyMainCount[p.family] = (familyMainCount[p.family] ?? 0) + 1;
  }

  for (const name of strategyNames) {
    let sumAbsCorr = 0;
    let peerCount = 0;
    let maxAbsCorr = 0;
    let maxCorrPeer = "";

    for (const other of strategyNames) {
      if (other === name) continue;
      const key = name < other ? `${name}|||${other}` : `${other}|||${name}`;
      const corr = pairs.get(key);
      if (corr === undefined) continue;
      const absCorr = Math.abs(corr);
      sumAbsCorr += absCorr;
      peerCount++;
      if (absCorr > maxAbsCorr) {
        maxAbsCorr = absCorr;
        maxCorrPeer = other;
      }
    }

    const avgAbsCorr = peerCount > 0 ? sumAbsCorr / peerCount : 0;

    // Family concentration penalty (0–20)
    const family = strategyFamilies.get(name) ?? "Unknown";
    const familyCount = familyMainCount[family] ?? 0;
    const familyPenalty = familyCount >= HIGH_DENSITY_FAMILY_THRESHOLD
      ? Math.min(20, (familyCount - HIGH_DENSITY_FAMILY_THRESHOLD + 1) * 4)
      : 0;

    const rawScore = 100 - avgAbsCorr * 100 - familyPenalty;
    const diversificationScore = Math.max(0, Math.min(100, Math.round(rawScore)));

    result.set(name, {
      strategy_id: name.toLowerCase().replace(/\s+/g, "_"),
      strategy_name: name,
      family,
      avg_abs_correlation: +avgAbsCorr.toFixed(4),
      max_corr_peer_name: maxCorrPeer,
      max_corr_value: +maxAbsCorr.toFixed(4),
      correlation_cluster: 0, // filled by cluster assignment
      diversification_score: diversificationScore,
      family_concentration_penalty: familyPenalty,
      computed_at: now,
    });
  }

  return result;
}

/**
 * Full correlation computation pipeline.
 * Stores results in MongoDB and returns summaries.
 */
export async function computeAndStoreCorrelations(
  db: Db,
  strategyProfiles: { strategy_id: string; strategy_name: string; family: string; current_status: string }[]
): Promise<{
  pairs: StrategyCorrelationPair[];
  summaries: StrategyCorrelationSummary[];
}> {
  const now = new Date().toISOString();

  // Build daily PnL matrix
  const matrix = await buildDailyPnlMatrix(db);

  // Only process strategies that have trade data
  const strategiesWithData = strategyProfiles.filter((p) => matrix.has(p.strategy_name));
  const names = strategiesWithData.map((p) => p.strategy_name);
  const familyMap = new Map(strategiesWithData.map((p) => [p.strategy_name, p.family]));

  // Compute pairwise correlations
  const { pairs: pairMap, overlapMap } = computePairwiseCorrelations(matrix);

  // Assign clusters
  const clusterMap = assignClusters(names, pairMap);

  // Compute diversification scores
  const mainEngineProfiles = strategyProfiles.filter((p) => p.current_status === "MAIN_ENGINE");
  const diversificationMap = computeDiversificationScores(names, familyMap, pairMap, mainEngineProfiles);

  // Apply cluster IDs
  for (const [name, summary] of diversificationMap) {
    summary.correlation_cluster = clusterMap.get(name) ?? 0;
  }

  // Build pair documents
  const pairDocs: StrategyCorrelationPair[] = [];
  for (const [key, corr] of pairMap) {
    const [nameA, nameB] = key.split("|||");
    const profileA = strategiesWithData.find((p) => p.strategy_name === nameA);
    const profileB = strategiesWithData.find((p) => p.strategy_name === nameB);
    if (!profileA || !profileB) continue;
    pairDocs.push({
      strategy_id_a: profileA.strategy_id,
      strategy_name_a: nameA,
      strategy_id_b: profileB.strategy_id,
      strategy_name_b: nameB,
      pearson_correlation: +corr.toFixed(4),
      overlap_days: overlapMap.get(key) ?? 0,
      computed_at: now,
    });
  }

  const summaryDocs = Array.from(diversificationMap.values());

  // Store in MongoDB
  const corrColl = db.collection<StrategyCorrelationPair>(STRATEGY_CORRELATIONS_COLLECTION);
  const divColl = db.collection<StrategyCorrelationSummary>(STRATEGY_DIVERSIFICATION_COLLECTION);

  // Upsert pairs
  if (pairDocs.length > 0) {
    await corrColl.deleteMany({});
    await corrColl.insertMany(pairDocs);
  }

  // Upsert summaries
  if (summaryDocs.length > 0) {
    await divColl.deleteMany({});
    await divColl.insertMany(summaryDocs);
  }

  // Ensure indexes
  await Promise.all([
    corrColl.createIndex({ strategy_id_a: 1, strategy_id_b: 1 }, { unique: true, background: true }),
    corrColl.createIndex({ strategy_id_a: 1 }, { background: true }),
    corrColl.createIndex({ strategy_id_b: 1 }, { background: true }),
    divColl.createIndex({ strategy_id: 1 }, { unique: true, background: true }),
    divColl.createIndex({ diversification_score: -1 }, { background: true }),
  ]);

  return { pairs: pairDocs, summaries: summaryDocs };
}

/**
 * Get correlation pairs involving a specific strategy.
 */
export async function getStrategyCorrelations(
  db: Db,
  strategyId: string
): Promise<StrategyCorrelationPair[]> {
  return db.collection<StrategyCorrelationPair>(STRATEGY_CORRELATIONS_COLLECTION)
    .find({ $or: [{ strategy_id_a: strategyId }, { strategy_id_b: strategyId }] })
    .sort({ pearson_correlation: -1 })
    .toArray();
}

/**
 * Get the full pairwise correlation matrix for a set of strategies.
 * Returns rows in format suitable for heatmap rendering.
 */
export async function getCorrelationMatrix(
  db: Db,
  strategyIds: string[]
): Promise<{ rows: StrategyCorrelationPair[]; strategies: string[] }> {
  const rows = await db.collection<StrategyCorrelationPair>(STRATEGY_CORRELATIONS_COLLECTION)
    .find({
      $or: [
        { strategy_id_a: { $in: strategyIds } },
        { strategy_id_b: { $in: strategyIds } },
      ],
    })
    .toArray();

  const strategiesInMatrix = Array.from(
    new Set(rows.flatMap((r) => [r.strategy_name_a, r.strategy_name_b]))
  ).sort();

  return { rows, strategies: strategiesInMatrix };
}

/**
 * Get all diversification summaries, sorted by score desc.
 */
export async function getDiversificationSummaries(
  db: Db,
  limit = 100
): Promise<StrategyCorrelationSummary[]> {
  return db.collection<StrategyCorrelationSummary>(STRATEGY_DIVERSIFICATION_COLLECTION)
    .find({})
    .sort({ diversification_score: -1 })
    .limit(limit)
    .toArray();
}

/**
 * Get diversification score for a single strategy.
 * Returns null if not computed yet.
 */
export async function getDiversificationScore(
  db: Db,
  strategyId: string
): Promise<number | null> {
  const doc = await db.collection<StrategyCorrelationSummary>(STRATEGY_DIVERSIFICATION_COLLECTION)
    .findOne({ strategy_id: strategyId });
  return doc?.diversification_score ?? null;
}
