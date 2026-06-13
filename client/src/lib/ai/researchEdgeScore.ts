/**
 * Research-grade per-strategy edge scorer.
 *
 * Pure function — no I/O, no MongoDB. Takes closed paper trades and returns
 * a ResearchEdgeScore per strategy.
 *
 * Hard invariants:
 *   - Probe, bootstrap, canary, and smoke trades are excluded before scoring.
 *   - PROMOTE requires ≥12 real trades, positive expectancy, fee/gross < 50%,
 *     and profit factor ≥ 1.1.
 *   - DISABLE fires when ≥5 trades, expectancy < 0, and fee/gross > 100%.
 *   - No threshold lowering, no gate bypassing, no guaranteed profit claims.
 *   - Recommendations only — operator decides whether to apply.
 */

import type { PaperTradeDbRow } from "@/lib/portfolio/paperTradesTypes";

// ─── Public types ─────────────────────────────────────────────────────────────

export interface RegimeStats {
  trades: number;
  expectancy: number;
  winRate: number;
}

export interface ResearchEdgeScore {
  strategyId: number;
  strategyName: string;
  trades: number;
  netPnl: number;
  expectancy: number;
  winRate: number;
  profitFactor: number;
  feePctOfAbsGross: number;
  maxDrawdownEstimate: number;
  tpHits: number;
  slHits: number;
  regimeBreakdown: Record<string, RegimeStats>;
  confidence: number;
  status: "PROMOTE" | "KEEP" | "WATCH" | "DISABLE" | "INSUFFICIENT";
  reason: string;
}

// ─── Constants ────────────────────────────────────────────────────────────────

const PROBE_PATTERN = /probe|bootstrap|canary|smoke|paper_entry_canary/i;
const MIN_TRADES_FOR_ANY_VERDICT = 5;
const MIN_TRADES_TO_PROMOTE = 12;
const MAX_FEE_PCT_TO_PROMOTE = 50;
const MIN_PROFIT_FACTOR = 1.1;
const HIGH_FEE_DISABLE_PCT = 100;

const STATUS_RANK: Record<ResearchEdgeScore["status"], number> = {
  PROMOTE: 0,
  KEEP: 1,
  WATCH: 2,
  DISABLE: 3,
  INSUFFICIENT: 4,
};

// ─── Helpers ──────────────────────────────────────────────────────────────────

function isProbe(t: PaperTradeDbRow): boolean {
  if (PROBE_PATTERN.test(t.strategy_name ?? "")) return true;
  if (t.template_family && PROBE_PATTERN.test(t.template_family)) return true;
  return false;
}

function extractRegime(t: PaperTradeDbRow): string {
  try {
    const p = t.payload as Record<string, unknown> | null | undefined;
    const r = p?.regime ?? p?.regimeTag;
    if (typeof r === "string" && r.length > 0) return r;
  } catch {
    // ignore
  }
  return "unknown";
}

function computeMaxDrawdown(pnlsChronological: number[]): number {
  let equity = 0;
  let peak = 0;
  let maxDD = 0;
  for (const pnl of pnlsChronological) {
    equity += pnl;
    if (equity > peak) peak = equity;
    const dd = peak - equity;
    if (dd > maxDD) maxDD = dd;
  }
  return maxDD;
}

function toConfidence(n: number): number {
  if (n < MIN_TRADES_FOR_ANY_VERDICT) return 0;
  if (n < MIN_TRADES_TO_PROMOTE) return 0.2;
  if (n < 20) return 0.4;
  if (n < 30) return 0.6;
  if (n < 50) return 0.75;
  return 0.9;
}

function classify(
  n: number,
  expectancy: number,
  feePctOfAbsGross: number,
  profitFactor: number,
): { status: ResearchEdgeScore["status"]; reason: string } {
  if (n < MIN_TRADES_FOR_ANY_VERDICT) {
    return {
      status: "INSUFFICIENT",
      reason: `Only ${n} trade${n !== 1 ? "s" : ""} — need ≥${MIN_TRADES_FOR_ANY_VERDICT} for any verdict.`,
    };
  }

  if (expectancy < 0 && feePctOfAbsGross > HIGH_FEE_DISABLE_PCT) {
    return {
      status: "DISABLE",
      reason: `Negative expectancy ($${expectancy.toFixed(2)}/trade) with fee/gross ${feePctOfAbsGross.toFixed(0)}% over ${n} trades. Fee drag destroying edge.`,
    };
  }

  if (
    n >= MIN_TRADES_TO_PROMOTE &&
    expectancy > 0 &&
    feePctOfAbsGross < MAX_FEE_PCT_TO_PROMOTE &&
    profitFactor >= MIN_PROFIT_FACTOR
  ) {
    return {
      status: "PROMOTE",
      reason: `${n} trades, expectancy $${expectancy.toFixed(2)}/trade, PF ${profitFactor.toFixed(2)}, fee/gross ${feePctOfAbsGross.toFixed(0)}% — all promotion criteria met.`,
    };
  }

  if (n >= MIN_TRADES_TO_PROMOTE && expectancy > 0) {
    const gaps: string[] = [];
    if (feePctOfAbsGross >= MAX_FEE_PCT_TO_PROMOTE)
      gaps.push(`fee/gross ${feePctOfAbsGross.toFixed(0)}% ≥ ${MAX_FEE_PCT_TO_PROMOTE}%`);
    if (profitFactor < MIN_PROFIT_FACTOR)
      gaps.push(`PF ${profitFactor.toFixed(2)} < ${MIN_PROFIT_FACTOR}`);
    return {
      status: "KEEP",
      reason: `Positive expectancy but ${gaps.join(", ")}. Keep observing before promoting.`,
    };
  }

  if (n < MIN_TRADES_TO_PROMOTE) {
    return {
      status: "WATCH",
      reason: `${n}/${MIN_TRADES_TO_PROMOTE} trades needed to promote. Gathering data.`,
    };
  }

  return {
    status: "WATCH",
    reason: `${n} trades, expectancy $${expectancy.toFixed(2)}/trade. Not meeting criteria — continue observing.`,
  };
}

// ─── Main export ──────────────────────────────────────────────────────────────

/**
 * Compute a ResearchEdgeScore for every strategy present in `trades`.
 * Probe/bootstrap/canary trades are automatically excluded.
 * Results are sorted: PROMOTE → KEEP → WATCH → DISABLE → INSUFFICIENT,
 * then by expectancy descending within each status bucket.
 */
export function computeResearchEdgeScores(
  trades: ReadonlyArray<PaperTradeDbRow>,
): ResearchEdgeScore[] {
  // Step 1 — filter out probes / bootstrap / canary rows
  const real = trades.filter((t) => !isProbe(t));

  // Step 2 — group by strategy_id
  const byStrategy = new Map<number, PaperTradeDbRow[]>();
  for (const t of real) {
    if (!Number.isFinite(t.strategy_id)) continue;
    const arr = byStrategy.get(t.strategy_id) ?? [];
    arr.push(t);
    byStrategy.set(t.strategy_id, arr);
  }

  const results: ResearchEdgeScore[] = [];

  for (const [strategyId, rows] of byStrategy) {
    // Sort chronologically for drawdown computation
    const sorted = [...rows].sort((a, b) => a.closed_at.localeCompare(b.closed_at));
    const n = sorted.length;
    const strategyName = sorted[0]?.strategy_name ?? `strategy-${strategyId}`;

    // Core metrics
    const pnls = sorted.map((t) => t.net_pnl);
    const grossPnls = sorted.map((t) => t.gross_pnl);
    const feesAbs = sorted.map((t) => Math.abs(t.fees ?? 0));

    const netPnl = pnls.reduce((s, v) => s + v, 0);
    const expectancy = n > 0 ? netPnl / n : 0;

    const wins = pnls.filter((p) => p > 0);
    const winRate = n > 0 ? wins.length / n : 0;

    // Profit factor uses gross (pre-fee) PnL
    const grossWinSum = grossPnls.filter((p) => p > 0).reduce((s, v) => s + v, 0);
    const grossLossSum = Math.abs(grossPnls.filter((p) => p < 0).reduce((s, v) => s + v, 0));
    const profitFactor =
      grossLossSum > 0
        ? grossWinSum / grossLossSum
        : grossWinSum > 0
          ? 9.99 // cap — no losses
          : 0;

    // Fee as % of absolute gross turnover
    const totalFees = feesAbs.reduce((s, v) => s + v, 0);
    const absGrossSum = grossPnls.reduce((s, v) => s + Math.abs(v), 0);
    const feePctOfAbsGross =
      absGrossSum > 0 ? Math.min(999, (totalFees / absGrossSum) * 100) : 100;

    const maxDrawdownEstimate = computeMaxDrawdown(pnls);
    const tpHits = sorted.filter((t) => t.exit_reason === "TP").length;
    const slHits = sorted.filter((t) => t.exit_reason === "SL").length;

    // Regime breakdown
    const regimeMap = new Map<string, number[]>();
    for (const t of sorted) {
      const regime = extractRegime(t);
      const arr = regimeMap.get(regime) ?? [];
      arr.push(t.net_pnl);
      regimeMap.set(regime, arr);
    }
    const regimeBreakdown: Record<string, RegimeStats> = {};
    for (const [regime, rPnls] of regimeMap) {
      const rN = rPnls.length;
      const rWins = rPnls.filter((p) => p > 0).length;
      regimeBreakdown[regime] = {
        trades: rN,
        expectancy: rN > 0 ? rPnls.reduce((s, v) => s + v, 0) / rN : 0,
        winRate: rN > 0 ? rWins / rN : 0,
      };
    }

    const confidence = toConfidence(n);
    const { status, reason } = classify(n, expectancy, feePctOfAbsGross, profitFactor);

    results.push({
      strategyId,
      strategyName,
      trades: n,
      netPnl,
      expectancy,
      winRate,
      profitFactor,
      feePctOfAbsGross,
      maxDrawdownEstimate,
      tpHits,
      slHits,
      regimeBreakdown,
      confidence,
      status,
      reason,
    });
  }

  // Sort: by status bucket, then by expectancy descending within bucket
  results.sort((a, b) => {
    const sr = STATUS_RANK[a.status] - STATUS_RANK[b.status];
    return sr !== 0 ? sr : b.expectancy - a.expectancy;
  });

  return results;
}
