/**
 * APICAP Phase 6 — Regime Intelligence Engine
 *
 * Computes regime-conditioned performance for each strategy by joining
 * mock_trades (by timestamp) with regime_snapshots (TRENDING/RANGING/
 * HIGH_VOLATILITY_BREAKOUT/LOW_VOLATILITY_CHOP).
 *
 * Data sources:
 *   - mock_trades collection: strategy_name, closed_at, gross_pnl, fees, funding_costs
 *   - regime_snapshots collection: account_key, snapshot.regime, snapshot.timestamp
 */

import type { Db } from "mongodb";
import type { MarketRegime, RegimePerformance, StrategyRegimeMetrics } from "./portfolioTypes";

export const STRATEGY_REGIME_METRICS_COLLECTION = "strategy_regime_metrics";

const ALL_REGIMES: MarketRegime[] = [
  "TRENDING",
  "RANGING",
  "HIGH_VOLATILITY_BREAKOUT",
  "LOW_VOLATILITY_CHOP",
];

// ── Regime snapshot helpers ────────────────────────────────────────────────────

interface RegimeSnapshot {
  timestamp: number;   // epoch ms
  regime: MarketRegime;
}

async function loadRegimeSnapshots(db: Db): Promise<RegimeSnapshot[]> {
  const raw = await db.collection("regime_snapshots")
    .find({}, { projection: { "snapshot.timestamp": 1, "snapshot.regime": 1, _id: 0 } })
    .sort({ "snapshot.timestamp": 1 })
    .limit(10_000)
    .toArray();

  return raw
    .filter((r) => r?.snapshot?.timestamp && r?.snapshot?.regime)
    .map((r) => ({ timestamp: r.snapshot.timestamp as number, regime: r.snapshot.regime as MarketRegime }));
}

/**
 * Binary search to find the regime closest to (but not after) a given timestamp.
 */
function findRegimeAtTime(snapshots: RegimeSnapshot[], tsMs: number): MarketRegime | null {
  if (snapshots.length === 0) return null;
  let lo = 0;
  let hi = snapshots.length - 1;
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1;
    if (snapshots[mid].timestamp <= tsMs) lo = mid;
    else hi = mid - 1;
  }
  // Accept if within 2 hours
  if (Math.abs(snapshots[lo].timestamp - tsMs) > 7_200_000) return null;
  return snapshots[lo].regime;
}

// ── Trade-level regime assignment ──────────────────────────────────────────────

interface RegimeTrade {
  strategy_name: string;
  regime: MarketRegime;
  pnl: number;
}

async function getTradesWithRegime(
  db: Db,
  snapshots: RegimeSnapshot[],
  lookbackDays = 180
): Promise<RegimeTrade[]> {
  const cutoff = new Date(Date.now() - lookbackDays * 86_400_000).toISOString();

  const trades = await db.collection("mock_trades").aggregate([
    { $match: { status: "CLOSED", closed_at: { $gte: cutoff } } },
    {
      $project: {
        strategy_name: 1,
        tsMs: { $toLong: { $dateFromString: { dateString: "$closed_at" } } },
        pnl: {
          $subtract: [
            { $ifNull: ["$gross_pnl", 0] },
            { $add: [{ $ifNull: ["$fees", 0] }, { $ifNull: ["$funding_costs", 0] }] },
          ],
        },
      },
    },
  ]).toArray();

  const result: RegimeTrade[] = [];
  for (const t of trades) {
    const regime = findRegimeAtTime(snapshots, t.tsMs as number);
    if (!regime) continue;
    result.push({ strategy_name: t.strategy_name as string, regime, pnl: t.pnl as number });
  }
  return result;
}

// ── Per-regime metric computation ─────────────────────────────────────────────

function computeRegimePerformance(pnls: number[]): RegimePerformance {
  if (pnls.length === 0) return { tradeCount: 0, winRate: 0, profitFactor: 0, expectancy: 0, sharpeRatio: 0 };

  const wins = pnls.filter((p) => p > 0);
  const losses = pnls.filter((p) => p <= 0);
  const grossWin = wins.reduce((a, p) => a + p, 0);
  const grossLoss = Math.abs(losses.reduce((a, p) => a + p, 0));

  const winRate = (wins.length / pnls.length) * 100;
  const profitFactor = grossLoss === 0 ? (grossWin > 0 ? 99 : 0) : grossWin / grossLoss;
  const expectancy = pnls.reduce((a, p) => a + p, 0) / pnls.length;

  // Sharpe (annualized assuming ~252 trading periods)
  const mean = expectancy;
  const variance = pnls.reduce((a, p) => a + Math.pow(p - mean, 2), 0) / pnls.length;
  const stdDev = Math.sqrt(variance);
  const sharpeRatio = stdDev > 0 ? (mean / stdDev) * Math.sqrt(252) : 0;

  return {
    tradeCount: pnls.length,
    winRate: +winRate.toFixed(1),
    profitFactor: +profitFactor.toFixed(3),
    expectancy: +expectancy.toFixed(2),
    sharpeRatio: +sharpeRatio.toFixed(3),
  };
}

// ── Regime strength score ──────────────────────────────────────────────────────

/**
 * Regime strength score 0–100.
 * Rewards strategies that perform consistently across regimes.
 * Current regime gets 2× weight.
 */
function computeRegimeStrengthScore(
  regimes: Partial<Record<MarketRegime, RegimePerformance>>,
  currentRegime: MarketRegime | null
): number {
  const entries = Object.entries(regimes) as [MarketRegime, RegimePerformance][];
  if (entries.length === 0) return 0;

  let weightedPf = 0;
  let totalWeight = 0;

  for (const [regime, perf] of entries) {
    if (perf.tradeCount < 3) continue;
    const weight = regime === currentRegime ? 2 : 1;
    weightedPf += perf.profitFactor * weight;
    totalWeight += weight;
  }

  if (totalWeight === 0) return 0;

  const avgPf = weightedPf / totalWeight;

  // Scale: PF 0 → 0 pts, PF 1.0 → 50 pts, PF 1.5+ → 100 pts
  const rawScore = avgPf < 1.0
    ? Math.max(0, avgPf * 50)
    : Math.min(100, 50 + (avgPf - 1.0) / 0.5 * 50);

  // Penalty if any tested regime has PF < 0.8 (consistent loser in that regime)
  const worstPf = Math.min(...entries.filter(([, p]) => p.tradeCount >= 5).map(([, p]) => p.profitFactor));
  const worstPenalty = worstPf < 0.8 ? Math.min(20, (0.8 - worstPf) * 40) : 0;

  return Math.max(0, Math.min(100, Math.round(rawScore - worstPenalty)));
}

// ── Main computation ───────────────────────────────────────────────────────────

/**
 * Compute regime metrics for all strategies and store in MongoDB.
 */
export async function computeAndStoreRegimeMetrics(
  db: Db,
  strategyProfiles: { strategy_id: string; strategy_name: string; family: string }[]
): Promise<StrategyRegimeMetrics[]> {
  const now = new Date().toISOString();

  // Load regime snapshots
  const snapshots = await loadRegimeSnapshots(db);
  const currentRegime: MarketRegime | null = snapshots.length > 0
    ? snapshots[snapshots.length - 1].regime
    : null;

  // Get trades with regime labels
  const trades = await getTradesWithRegime(db, snapshots);

  // Group by (strategy_name, regime)
  const grouped = new Map<string, Map<MarketRegime, number[]>>();
  for (const t of trades) {
    if (!grouped.has(t.strategy_name)) grouped.set(t.strategy_name, new Map());
    const regMap = grouped.get(t.strategy_name)!;
    if (!regMap.has(t.regime)) regMap.set(t.regime, []);
    regMap.get(t.regime)!.push(t.pnl);
  }

  const results: StrategyRegimeMetrics[] = [];

  for (const profile of strategyProfiles) {
    const regMap = grouped.get(profile.strategy_name);
    const regimes: Partial<Record<MarketRegime, RegimePerformance>> = {};

    if (regMap) {
      for (const regime of ALL_REGIMES) {
        const pnls = regMap.get(regime);
        if (pnls && pnls.length > 0) {
          regimes[regime] = computeRegimePerformance(pnls);
        }
      }
    }

    const pfByRegime = Object.entries(regimes) as [MarketRegime, RegimePerformance][];
    const best = pfByRegime.filter(([, p]) => p.tradeCount >= 3).sort(([, a], [, b]) => b.profitFactor - a.profitFactor)[0]?.[0] ?? null;
    const worst = pfByRegime.filter(([, p]) => p.tradeCount >= 3).sort(([, a], [, b]) => a.profitFactor - b.profitFactor)[0]?.[0] ?? null;

    const currentRegimePf = currentRegime && regimes[currentRegime]?.tradeCount
      ? regimes[currentRegime]!.profitFactor
      : 0;

    const regimeStrengthScore = computeRegimeStrengthScore(regimes, currentRegime);

    results.push({
      strategy_id: profile.strategy_id,
      strategy_name: profile.strategy_name,
      family: profile.family,
      regimes,
      regime_strength_score: regimeStrengthScore,
      best_regime: best,
      worst_regime: worst,
      current_regime: currentRegime,
      current_regime_pf: +currentRegimePf.toFixed(3),
      computed_at: now,
    });
  }

  // Store in MongoDB
  const coll = db.collection<StrategyRegimeMetrics>(STRATEGY_REGIME_METRICS_COLLECTION);
  await coll.deleteMany({});
  if (results.length > 0) await coll.insertMany(results);
  await Promise.all([
    coll.createIndex({ strategy_id: 1 }, { unique: true, background: true }),
    coll.createIndex({ regime_strength_score: -1 }, { background: true }),
  ]);

  return results;
}

/**
 * Get regime metrics for a single strategy.
 */
export async function getStrategyRegimeMetrics(
  db: Db,
  strategyId: string
): Promise<StrategyRegimeMetrics | null> {
  return db.collection<StrategyRegimeMetrics>(STRATEGY_REGIME_METRICS_COLLECTION)
    .findOne({ strategy_id: strategyId });
}

/**
 * Get all regime metrics, sorted by regime_strength_score desc.
 */
export async function getAllRegimeMetrics(
  db: Db,
  limit = 200
): Promise<StrategyRegimeMetrics[]> {
  return db.collection<StrategyRegimeMetrics>(STRATEGY_REGIME_METRICS_COLLECTION)
    .find({})
    .sort({ regime_strength_score: -1 })
    .limit(limit)
    .toArray();
}

/**
 * Get the current active regime from regime_snapshots.
 */
export async function getCurrentRegime(db: Db): Promise<MarketRegime | null> {
  const snap = await db.collection("regime_snapshots")
    .find({})
    .sort({ "snapshot.timestamp": -1 })
    .limit(1)
    .toArray();
  const regime = snap[0]?.snapshot?.regime;
  return ALL_REGIMES.includes(regime) ? regime : null;
}
