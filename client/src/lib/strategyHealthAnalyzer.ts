/**
 * Strategy health analyzer — UPGRADE 3 of PR-SELF-HEALING-DESK.
 *
 * Pure function. Takes a flat list of closed paper trades and groups them by
 * `strategy_id`, computing per-strategy win-rate, mean/std net PnL, and a
 * classification used by the operator-facing healing recommendation engine.
 *
 * Hard invariants:
 *   - Paper trades only. The analyzer does not touch live execution.
 *   - Pure: no MongoDB, no env reads, no Date.now. Deterministic from inputs.
 *   - Recommendation-only: classifying a strategy as `loser` does NOT disable
 *     it. The operator (or a future env-gated automation step) reviews the
 *     report and decides whether to add the strategy ID to
 *     `paper_state.disabled_strategies`.
 *   - Never recommends lowering thresholds, bypassing gates, or modifying
 *     PnL math. Only acts on observed closed-trade outcomes.
 */

// ─── Public types ────────────────────────────────────────────────────────────

/**
 * Minimum trade shape the analyzer needs. Matches PaperTradeDbRow but is
 * declared structurally here so the analyzer has no runtime dependency on
 * the Mongo schema and is trivially testable.
 */
export interface AnalyzerTradeInput {
  strategy_id: number;
  strategy_name?: string | null;
  net_pnl: number;
  closed_at: string;
  /** Excluded probes/bootstraps/CANARY rows still pass through — the
   *  caller filters before calling the analyzer. */
}

export type StrategyHealthClassification =
  | "winner"
  | "loser"
  | "volatile"
  | "thin";

export interface StrategyHealthReport {
  strategyId: number;
  strategyName: string | null;
  trades: number;
  wins: number;
  losses: number;
  winRate: number;
  meanPnlUsd: number;
  stdPnlUsd: number;
  totalNetPnlUsd: number;
  classification: StrategyHealthClassification;
  recommendation: string;
}

export interface AnalyzerOptions {
  /** Rolling window per strategy — only the most recent N trades are scored. */
  windowSize?: number;
  /** Below this many trades, classification is forced to `thin`. */
  minTradesForVerdict?: number;
  /** Win-rate threshold (0..1) at or above which a strategy is `winner`. */
  winnerWinRate?: number;
  /** Win-rate threshold (0..1) strictly below which a strategy is `loser`. */
  loserWinRate?: number;
  /** Coefficient-of-variation threshold above which a strategy is `volatile`. */
  volatileCoefficient?: number;
}

// ─── Defaults ────────────────────────────────────────────────────────────────

const DEFAULT_WINDOW_SIZE = 20;
const DEFAULT_MIN_TRADES = 20;
const DEFAULT_WINNER_RATE = 0.55;
const DEFAULT_LOSER_RATE = 0.30;
const DEFAULT_VOLATILE_COEF = 3.0;

// ─── Math helpers ────────────────────────────────────────────────────────────

function mean(xs: number[]): number {
  if (xs.length === 0) return 0;
  let sum = 0;
  for (const x of xs) sum += x;
  return sum / xs.length;
}

function stdev(xs: number[]): number {
  if (xs.length < 2) return 0;
  const m = mean(xs);
  let sumSq = 0;
  for (const x of xs) sumSq += (x - m) ** 2;
  return Math.sqrt(sumSq / (xs.length - 1));
}

// ─── Classifier ──────────────────────────────────────────────────────────────

function classify(
  trades: number,
  winRate: number,
  meanPnl: number,
  stdPnl: number,
  opts: Required<AnalyzerOptions>,
): { classification: StrategyHealthClassification; recommendation: string } {
  if (trades < opts.minTradesForVerdict) {
    return {
      classification: "thin",
      recommendation: `Keep gathering data — ${trades}/${opts.minTradesForVerdict} trades. No action.`,
    };
  }

  if (winRate < opts.loserWinRate) {
    return {
      classification: "loser",
      recommendation: `Win rate ${(winRate * 100).toFixed(0)}% over ${trades} trades is below ${(opts.loserWinRate * 100).toFixed(0)}%. Consider disabling this strategy via paper_state.disabled_strategies.`,
    };
  }

  if (winRate >= opts.winnerWinRate) {
    // Even winners can be volatile — flag that, but keep classification as winner.
    if (
      stdPnl > 0 &&
      Math.abs(meanPnl) > 0 &&
      Math.abs(stdPnl / meanPnl) > opts.volatileCoefficient
    ) {
      return {
        classification: "volatile",
        recommendation: `Win rate ${(winRate * 100).toFixed(0)}% but PnL is volatile (σ/μ=${(stdPnl / Math.abs(meanPnl)).toFixed(1)}). Consider lower position-size multiplier.`,
      };
    }
    return {
      classification: "winner",
      recommendation: `Healthy winner: ${(winRate * 100).toFixed(0)}% over ${trades} trades. Keep as-is.`,
    };
  }

  // Mid-range win rate. If volatility is extreme, flag; otherwise neutral.
  if (
    stdPnl > 0 &&
    Math.abs(meanPnl) > 0 &&
    Math.abs(stdPnl / meanPnl) > opts.volatileCoefficient
  ) {
    return {
      classification: "volatile",
      recommendation: `Mid-range win rate ${(winRate * 100).toFixed(0)}% with high PnL volatility (σ/μ=${(stdPnl / Math.abs(meanPnl)).toFixed(1)}). Consider lower position-size multiplier.`,
    };
  }

  // Mid-range, low volatility → keep collecting data.
  return {
    classification: "thin",
    recommendation: `Win rate ${(winRate * 100).toFixed(0)}% — neither clearly winning nor losing. Keep observing.`,
  };
}

// ─── Main entry ──────────────────────────────────────────────────────────────

/**
 * Group trades by strategy_id, take the most recent `windowSize` trades per
 * group (by `closed_at` descending), and emit a StrategyHealthReport per
 * strategy. Results are sorted by net PnL descending (winners first).
 */
export function analyzeStrategyHealth(
  trades: ReadonlyArray<AnalyzerTradeInput>,
  options: AnalyzerOptions = {},
): StrategyHealthReport[] {
  const opts: Required<AnalyzerOptions> = {
    windowSize: options.windowSize ?? DEFAULT_WINDOW_SIZE,
    minTradesForVerdict: options.minTradesForVerdict ?? DEFAULT_MIN_TRADES,
    winnerWinRate: options.winnerWinRate ?? DEFAULT_WINNER_RATE,
    loserWinRate: options.loserWinRate ?? DEFAULT_LOSER_RATE,
    volatileCoefficient: options.volatileCoefficient ?? DEFAULT_VOLATILE_COEF,
  };

  // Group by strategy_id
  const byStrategy = new Map<number, AnalyzerTradeInput[]>();
  for (const t of trades) {
    if (!Number.isFinite(t.strategy_id)) continue;
    const arr = byStrategy.get(t.strategy_id) ?? [];
    arr.push(t);
    byStrategy.set(t.strategy_id, arr);
  }

  const reports: StrategyHealthReport[] = [];

  for (const [strategyId, all] of byStrategy) {
    // Most-recent `windowSize` trades, by closed_at descending
    const sorted = [...all].sort((a, b) =>
      b.closed_at.localeCompare(a.closed_at),
    );
    const window = sorted.slice(0, opts.windowSize);

    const pnls = window.map((t) => t.net_pnl);
    const wins = pnls.filter((p) => p > 0).length;
    const losses = pnls.filter((p) => p < 0).length;
    const tradesN = window.length;
    const winRate = tradesN > 0 ? wins / tradesN : 0;
    const meanPnl = mean(pnls);
    const stdPnl = stdev(pnls);
    const totalNetPnl = pnls.reduce((acc, p) => acc + p, 0);

    const { classification, recommendation } = classify(
      tradesN,
      winRate,
      meanPnl,
      stdPnl,
      opts,
    );

    reports.push({
      strategyId,
      strategyName: window[0]?.strategy_name ?? null,
      trades: tradesN,
      wins,
      losses,
      winRate,
      meanPnlUsd: meanPnl,
      stdPnlUsd: stdPnl,
      totalNetPnlUsd: totalNetPnl,
      classification,
      recommendation,
    });
  }

  reports.sort((a, b) => b.totalNetPnlUsd - a.totalNetPnlUsd);
  return reports;
}

// ─── Roll-up summary ─────────────────────────────────────────────────────────

export interface RosterHealthSummary {
  total: number;
  winners: number[];
  losers: number[];
  volatile: number[];
  thin: number[];
}

/** Convenience: bucket strategy IDs by classification for quick UI rendering. */
export function summarizeRosterHealth(
  reports: ReadonlyArray<StrategyHealthReport>,
): RosterHealthSummary {
  const summary: RosterHealthSummary = {
    total: reports.length,
    winners: [],
    losers: [],
    volatile: [],
    thin: [],
  };
  for (const r of reports) {
    switch (r.classification) {
      case "winner":
        summary.winners.push(r.strategyId);
        break;
      case "loser":
        summary.losers.push(r.strategyId);
        break;
      case "volatile":
        summary.volatile.push(r.strategyId);
        break;
      case "thin":
        summary.thin.push(r.strategyId);
        break;
    }
  }
  return summary;
}
