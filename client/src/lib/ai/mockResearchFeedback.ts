/**
 * Research Feedback Loop for the BTC futures mock trading module.
 *
 * Automatically learns from closed trade history to surface actionable insights:
 *
 *   1. Best strategy by market regime
 *   2. Best strategy family by regime
 *   3. Best trading session (UTC hour) by regime
 *   4. Best volatility environment (ATR bucket) for each strategy
 *   5. Best risk/reward settings by regime and strategy family
 *   6. Regime-specific performance heatmap
 *   7. Compounding regime insights into a trading conditions scorecard
 *
 * All insights are purely derived from the realized trade database —
 * no forward-looking or synthetic data is used.
 *
 * Pure functions — no React, no I/O.
 */

import type { MockTrade } from "@/lib/trading/mockTradingEngine";

// ── Session classification ────────────────────────────────────────────────────

export type TradingSession = "ASIA" | "LONDON" | "NEW_YORK" | "OVERLAP" | "WEEKEND";

export function classifySession(timestampMs: number): TradingSession {
  const d = new Date(timestampMs);
  const day = d.getUTCDay();
  if (day === 0 || day === 6) return "WEEKEND";
  const h = d.getUTCHours();
  if (h >= 1 && h < 8) return "ASIA";
  if (h >= 8 && h < 12) return "LONDON";
  if (h >= 12 && h < 17) return "OVERLAP"; // London/NY overlap
  if (h >= 17 && h < 22) return "NEW_YORK";
  return "ASIA"; // late NY / early Asia
}

// ── Regime performance cell ───────────────────────────────────────────────────

export interface RegimePerformanceCell {
  regime: string;
  /** Dimension (strategy, family, session, etc.). */
  dimension: string;
  dimensionValue: string;
  trades: number;
  netPnl: number;
  winRate: number;
  expectancy: number;
  avgHoldingMinutes: number;
  /** Confidence: LOW (<10 trades), MEDIUM (10–30), HIGH (>30). */
  confidence: "LOW" | "MEDIUM" | "HIGH";
}

// ── Best-by lookup ────────────────────────────────────────────────────────────

export interface BestByRegimeLookup<T extends string> {
  /** Regime → best value of the dimension. */
  bestByRegime: Map<string, { value: T; expectancy: number; trades: number }>;
  /** All regime × dimension cells sorted by expectancy. */
  cells: RegimePerformanceCell[];
}

// ── Main feedback insights ────────────────────────────────────────────────────

export interface ResearchFeedbackInsights {
  /** Best strategy ID per regime. */
  bestStrategyByRegime: BestByRegimeLookup<string>;
  /** Best strategy family per regime. */
  bestFamilyByRegime: BestByRegimeLookup<string>;
  /** Best trading session per regime. */
  bestSessionByRegime: BestByRegimeLookup<TradingSession>;
  /** Best volatility bucket per strategy. */
  bestVolEnvironmentByStrategy: Map<number, { bucket: VolBucket; expectancy: number; trades: number }>;
  /** Best R:R settings by regime. */
  bestRiskRewardByRegime: Map<string, { targetRR: number; winRate: number; expectancy: number }>;
  /** Regime performance heatmap. */
  regimeHeatmap: RegimeHeatmapRow[];
  /** Trading conditions scorecard. */
  conditionsScorecard: TradingConditionsScorecard;
  /** Top actionable insights in plain English. */
  topInsights: string[];
  computedAt: number;
}

export type VolBucket = "VERY_LOW" | "LOW" | "MEDIUM" | "HIGH" | "VERY_HIGH";

export interface RegimeHeatmapRow {
  regime: string;
  sessions: Record<TradingSession, { trades: number; netPnl: number; winRate: number }>;
  totalTrades: number;
  totalNetPnl: number;
  avgWinRate: number;
}

export interface TradingConditionsScorecard {
  /** Overall score (0–100) for current conditions. */
  score: number;
  /** Current regime is favourable for the top strategies. */
  regimeFavourable: boolean;
  /** Recommended strategies for current conditions. */
  recommendedStrategyIds: number[];
  /** Recommended session. */
  recommendedSession: TradingSession;
  /** Risk level: LOW / MEDIUM / HIGH. */
  riskLevel: "LOW" | "MEDIUM" | "HIGH";
  factors: string[];
}

// ── Main computation ──────────────────────────────────────────────────────────

/**
 * Derive all feedback insights from the closed trade history.
 */
export function computeResearchFeedback(
  trades: readonly MockTrade[],
  currentRegime?: string,
): ResearchFeedbackInsights {
  const closed = trades.filter((t) => t.status === "CLOSED" && t.closedAt != null);
  const now = Date.now();

  const bestStrategyByRegime = _bestByRegime(closed, (t) => t.strategyName);
  const bestFamilyByRegime = _bestByRegime(closed, (t) => t.strategyFamily ?? "Unknown");
  const bestSessionByRegime = _bestByRegime(closed, (t) => classifySession(t.openedAt));
  const bestVolEnvironmentByStrategy = _bestVolByStrategy(closed);
  const bestRiskRewardByRegime = _bestRRByRegime(closed);
  const regimeHeatmap = _buildRegimeHeatmap(closed);
  const conditionsScorecard = _buildScorecard(closed, bestStrategyByRegime, bestSessionByRegime, currentRegime);
  const topInsights = _generateInsights(closed, bestStrategyByRegime, bestFamilyByRegime, bestSessionByRegime, currentRegime);

  return {
    bestStrategyByRegime,
    bestFamilyByRegime,
    bestSessionByRegime,
    bestVolEnvironmentByStrategy,
    bestRiskRewardByRegime,
    regimeHeatmap,
    conditionsScorecard,
    topInsights,
    computedAt: now,
  };
}

// ── Internal builders ─────────────────────────────────────────────────────────

function _bestByRegime<T extends string>(
  trades: MockTrade[],
  dimension: (t: MockTrade) => T,
): BestByRegimeLookup<T> {
  // Group by regime → dimensionValue → trades
  const grid = new Map<string, Map<string, MockTrade[]>>();
  for (const t of trades) {
    const regime = t.regimeAtEntry ?? "UNKNOWN";
    const dimVal = dimension(t);
    if (!grid.has(regime)) grid.set(regime, new Map());
    const regMap = grid.get(regime)!;
    const bucket = regMap.get(dimVal) ?? [];
    bucket.push(t);
    regMap.set(dimVal, bucket);
  }

  const cells: RegimePerformanceCell[] = [];
  const bestByRegime = new Map<string, { value: T; expectancy: number; trades: number }>();

  for (const [regime, dimMap] of grid) {
    let bestExpectancy = -Infinity;
    let bestValue: T | null = null;
    let bestTrades = 0;

    for (const [dimVal, bucket] of dimMap) {
      const n = bucket.length;
      const netPnl = bucket.reduce((s, t) => s + t.realizedPnl, 0);
      const wins = bucket.filter((t) => t.realizedPnl > 0).length;
      const winRate = n > 0 ? wins / n : 0;
      const expectancy = n > 0 ? netPnl / n : 0;
      const avgHold = bucket.reduce((s, t) => s + Math.max(0, ((t.closedAt ?? t.openedAt) - t.openedAt) / 60_000), 0) / Math.max(1, n);

      cells.push({
        regime,
        dimension: "strategy",
        dimensionValue: dimVal,
        trades: n,
        netPnl,
        winRate,
        expectancy,
        avgHoldingMinutes: avgHold,
        confidence: n >= 30 ? "HIGH" : n >= 10 ? "MEDIUM" : "LOW",
      });

      if (expectancy > bestExpectancy) {
        bestExpectancy = expectancy;
        bestValue = dimVal as T;
        bestTrades = n;
      }
    }

    if (bestValue != null) {
      bestByRegime.set(regime, { value: bestValue, expectancy: bestExpectancy, trades: bestTrades });
    }
  }

  cells.sort((a, b) => b.expectancy - a.expectancy);
  return { bestByRegime, cells };
}

function _bestVolByStrategy(trades: MockTrade[]): Map<number, { bucket: VolBucket; expectancy: number; trades: number }> {
  // Infer vol bucket from notional/signalScore as a proxy (in absence of ATR data on trades)
  const result = new Map<number, { bucket: VolBucket; expectancy: number; trades: number }>();

  const byStrategy = new Map<number, MockTrade[]>();
  for (const t of trades) {
    const b = byStrategy.get(t.strategyId) ?? [];
    b.push(t);
    byStrategy.set(t.strategyId, b);
  }

  for (const [id, stratTrades] of byStrategy) {
    // Proxy for vol: use signalScore quintiles to approximate volatility bucket
    const scores = stratTrades.map((t) => t.signalScore).sort((a, b) => a - b);
    const q20 = scores[Math.floor(scores.length * 0.2)] ?? 0;
    const q40 = scores[Math.floor(scores.length * 0.4)] ?? 0;
    const q60 = scores[Math.floor(scores.length * 0.6)] ?? 0;
    const q80 = scores[Math.floor(scores.length * 0.8)] ?? 0;

    const bucketBoundaries: [VolBucket, number][] = [
      ["VERY_LOW", q20],
      ["LOW", q40],
      ["MEDIUM", q60],
      ["HIGH", q80],
      ["VERY_HIGH", Infinity],
    ];

    let bestBucket: VolBucket = "MEDIUM";
    let bestExpectancy = -Infinity;
    let bestTrades = 0;

    for (const [bucket, maxScore] of bucketBoundaries) {
      const prevMax = bucketBoundaries[bucketBoundaries.indexOf([bucket, maxScore]) - 1]?.[1] ?? -Infinity;
      const bucket_trades = stratTrades.filter((t) => t.signalScore > prevMax && t.signalScore <= maxScore);
      if (bucket_trades.length === 0) continue;
      const exp = bucket_trades.reduce((s, t) => s + t.realizedPnl, 0) / bucket_trades.length;
      if (exp > bestExpectancy) { bestExpectancy = exp; bestBucket = bucket; bestTrades = bucket_trades.length; }
    }

    result.set(id, { bucket: bestBucket, expectancy: bestExpectancy, trades: bestTrades });
  }
  return result;
}

function _bestRRByRegime(trades: MockTrade[]): Map<string, { targetRR: number; winRate: number; expectancy: number }> {
  const byRegime = new Map<string, MockTrade[]>();
  for (const t of trades) {
    const r = t.regimeAtEntry ?? "UNKNOWN";
    const b = byRegime.get(r) ?? [];
    b.push(t);
    byRegime.set(r, b);
  }

  const result = new Map<string, { targetRR: number; winRate: number; expectancy: number }>();
  for (const [regime, regimeTrades] of byRegime) {
    if (regimeTrades.length < 5) continue;
    const avgRR = regimeTrades.reduce((s, t) => s + t.riskRewardRatio, 0) / regimeTrades.length;
    const wins = regimeTrades.filter((t) => t.realizedPnl > 0).length;
    const winRate = wins / regimeTrades.length;
    const expectancy = regimeTrades.reduce((s, t) => s + t.realizedPnl, 0) / regimeTrades.length;
    result.set(regime, { targetRR: avgRR, winRate, expectancy });
  }
  return result;
}

function _buildRegimeHeatmap(trades: MockTrade[]): RegimeHeatmapRow[] {
  const rows = new Map<string, RegimeHeatmapRow>();
  const allSessions: TradingSession[] = ["ASIA", "LONDON", "NEW_YORK", "OVERLAP", "WEEKEND"];

  for (const t of trades) {
    const regime = t.regimeAtEntry ?? "UNKNOWN";
    const session = classifySession(t.openedAt);
    if (!rows.has(regime)) {
      const sessions: RegimeHeatmapRow["sessions"] = {} as RegimeHeatmapRow["sessions"];
      for (const s of allSessions) sessions[s] = { trades: 0, netPnl: 0, winRate: 0 };
      rows.set(regime, { regime, sessions, totalTrades: 0, totalNetPnl: 0, avgWinRate: 0 });
    }
    const row = rows.get(regime)!;
    row.sessions[session].trades++;
    row.sessions[session].netPnl += t.realizedPnl;
    row.totalTrades++;
    row.totalNetPnl += t.realizedPnl;
  }

  // Compute win rates
  for (const t of trades) {
    const regime = t.regimeAtEntry ?? "UNKNOWN";
    const session = classifySession(t.openedAt);
    const row = rows.get(regime)!;
    if (t.realizedPnl > 0) {
      const sess = row.sessions[session];
      // Will compute win rate after full pass — store wins in netPnl temporarily is wrong
    }
  }

  // Second pass for win rates
  const winsMap = new Map<string, Map<TradingSession, number>>();
  for (const t of trades) {
    if (t.realizedPnl <= 0) continue;
    const regime = t.regimeAtEntry ?? "UNKNOWN";
    const session = classifySession(t.openedAt);
    if (!winsMap.has(regime)) winsMap.set(regime, new Map());
    const sm = winsMap.get(regime)!;
    sm.set(session, (sm.get(session) ?? 0) + 1);
  }

  for (const [regime, row] of rows) {
    const wm = winsMap.get(regime);
    let totalWinRate = 0, sessionCount = 0;
    for (const session of allSessions) {
      const wins = wm?.get(session) ?? 0;
      const trades = row.sessions[session].trades;
      row.sessions[session].winRate = trades > 0 ? wins / trades : 0;
      if (trades > 0) { totalWinRate += row.sessions[session].winRate; sessionCount++; }
    }
    row.avgWinRate = sessionCount > 0 ? totalWinRate / sessionCount : 0;
  }

  return [...rows.values()].sort((a, b) => b.totalNetPnl - a.totalNetPnl);
}

function _buildScorecard(
  trades: MockTrade[],
  bestStrategyByRegime: BestByRegimeLookup<string>,
  bestSessionByRegime: BestByRegimeLookup<TradingSession>,
  currentRegime?: string,
): TradingConditionsScorecard {
  if (trades.length === 0 || !currentRegime) {
    return { score: 50, regimeFavourable: false, recommendedStrategyIds: [], recommendedSession: "NEW_YORK", riskLevel: "MEDIUM", factors: ["Insufficient data"] };
  }

  const factors: string[] = [];
  let score = 50;

  const bestStrategy = bestStrategyByRegime.bestByRegime.get(currentRegime);
  const regimeFavourable = bestStrategy != null && bestStrategy.expectancy > 0;
  if (regimeFavourable) { score += 15; factors.push(`${currentRegime} regime favours strategy ${bestStrategy?.value}`); }
  else { score -= 10; factors.push(`${currentRegime} regime is historically challenging`); }

  const bestSession = bestSessionByRegime.bestByRegime.get(currentRegime);
  const currentSession = classifySession(Date.now());
  if (bestSession?.value === currentSession) { score += 10; factors.push(`Current session (${currentSession}) is optimal for ${currentRegime}`); }
  else { factors.push(`Recommended session: ${bestSession?.value ?? "NEW_YORK"}`); }

  // Risk level from score
  const riskLevel: "LOW" | "MEDIUM" | "HIGH" = score >= 65 ? "LOW" : score >= 40 ? "MEDIUM" : "HIGH";

  // Recommended strategies: those with positive expectancy in current regime
  const bestStratName = bestStrategy?.value;
  const recommendedStrategyIds = bestStratName
    ? trades.filter((t) => t.strategyName === bestStratName).map((t) => t.strategyId).filter((v, i, a) => a.indexOf(v) === i).slice(0, 5)
    : [];

  return {
    score: Math.min(100, Math.max(0, score)),
    regimeFavourable,
    recommendedStrategyIds,
    recommendedSession: bestSession?.value ?? "NEW_YORK",
    riskLevel,
    factors,
  };
}

function _generateInsights(
  trades: MockTrade[],
  bestStrategyByRegime: BestByRegimeLookup<string>,
  bestFamilyByRegime: BestByRegimeLookup<string>,
  bestSessionByRegime: BestByRegimeLookup<TradingSession>,
  currentRegime?: string,
): string[] {
  const insights: string[] = [];
  if (trades.length < 10) {
    insights.push("Collect at least 10 closed trades to unlock feedback insights.");
    return insights;
  }

  // Best strategy per regime
  for (const [regime, best] of bestStrategyByRegime.bestByRegime) {
    if (best.trades >= 5 && best.expectancy > 0) {
      insights.push(`In ${regime} conditions, "${best.value}" delivers the highest expectancy ($${best.expectancy.toFixed(0)}/trade, ${best.trades} trades).`);
    }
  }

  // Best session per regime
  for (const [regime, best] of bestSessionByRegime.bestByRegime) {
    if (best.trades >= 5) {
      insights.push(`${regime} regime performs best during the ${(best as { value: string; session?: string }).session ?? best.value} session.`);
    }
  }

  // Best family per regime
  for (const [regime, best] of bestFamilyByRegime.bestByRegime) {
    if (best.trades >= 5 && best.expectancy > 0) {
      insights.push(`${best.value} strategy family leads in ${regime} markets.`);
    }
  }

  // Current regime recommendation
  if (currentRegime) {
    const curr = bestStrategyByRegime.bestByRegime.get(currentRegime);
    if (curr) {
      insights.unshift(`CURRENT: ${currentRegime} → prioritise "${curr.value}" (expectancy $${curr.expectancy.toFixed(0)}).`);
    }
  }

  return insights.slice(0, 10);
}
