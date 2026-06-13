/**
 * Trade analytics for institutional API routes (correlation, regime, risk metrics).
 */

export type TradeRow = {
  strategy_id: string;
  net_pnl: number;
  closed_at: string;
  regime_at_entry?: string;
};

function pearson(a: number[], b: number[]): number {
  const n = Math.min(a.length, b.length);
  if (n < 2) return 0;
  const xs = a.slice(0, n);
  const ys = b.slice(0, n);
  const meanA = xs.reduce((s, v) => s + v, 0) / n;
  const meanB = ys.reduce((s, v) => s + v, 0) / n;
  let num = 0;
  let denA = 0;
  let denB = 0;
  for (let i = 0; i < n; i++) {
    const da = xs[i] - meanA;
    const db = ys[i] - meanB;
    num += da * db;
    denA += da * da;
    denB += db * db;
  }
  const den = Math.sqrt(denA * denB);
  return den > 0 ? num / den : 0;
}

function dayKey(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toISOString().slice(0, 10);
}

/** Daily net PnL time series per strategy (last 30 days). */
export function dailyPnlByStrategy(trades: TradeRow[]): Map<string, Map<string, number>> {
  const byStrategy = new Map<string, Map<string, number>>();
  for (const t of trades) {
    if (!t.strategy_id || !Number.isFinite(t.net_pnl)) continue;
    const day = dayKey(t.closed_at);
    if (!day) continue;
    const bucket = byStrategy.get(t.strategy_id) ?? new Map<string, number>();
    bucket.set(day, (bucket.get(day) ?? 0) + t.net_pnl);
    byStrategy.set(t.strategy_id, bucket);
  }
  return byStrategy;
}

export function buildCorrelationMatrix(
  trades: TradeRow[],
  maxStrategies = 12,
): { labels: string[]; matrix: number[][] } {
  const byStrategy = dailyPnlByStrategy(trades);
  const totals = [...byStrategy.entries()]
    .map(([id, days]) => ({ id, abs: [...days.values()].reduce((s, v) => s + Math.abs(v), 0) }))
    .sort((a, b) => b.abs - a.abs)
    .slice(0, maxStrategies);

  const labels = totals.map((t) => t.id);
  const allDays = [...new Set(trades.map((t) => dayKey(t.closed_at)).filter(Boolean))].sort();
  const series = labels.map((id) => {
    const days = byStrategy.get(id) ?? new Map();
    return allDays.map((d) => days.get(d) ?? 0);
  });

  const matrix = series.map((row, i) =>
    series.map((col, j) => (i === j ? 1 : pearson(row, col))),
  );

  return { labels, matrix };
}

export function aggregateByRegime(trades: TradeRow[]): Array<{
  regime: string;
  trades: number;
  net_pnl: number;
  win_rate: number;
}> {
  const map = new Map<string, { trades: number; wins: number; net: number }>();
  for (const t of trades) {
    const regime = (t.regime_at_entry ?? "UNKNOWN").trim() || "UNKNOWN";
    const bucket = map.get(regime) ?? { trades: 0, wins: 0, net: 0 };
    bucket.trades += 1;
    bucket.net += t.net_pnl;
    if (t.net_pnl > 0) bucket.wins += 1;
    map.set(regime, bucket);
  }
  return [...map.entries()]
    .map(([regime, b]) => ({
      regime,
      trades: b.trades,
      net_pnl: b.net,
      win_rate: b.trades > 0 ? b.wins / b.trades : 0,
    }))
    .sort((a, b) => b.net_pnl - a.net_pnl);
}

export type RiskMetricsResult = {
  sharpeRatio: number | null;
  sortinoRatio: number | null;
  calmarRatio: number | null;
  maxDrawdownPct: number;
  maxDrawdownDurationDays: number;
  annualizedReturn: number | null;
};

export function computePortfolioRiskMetrics(
  trades: TradeRow[],
  startingEquity = 1_000_000,
): RiskMetricsResult {
  if (trades.length === 0) {
    return {
      sharpeRatio: null,
      sortinoRatio: null,
      calmarRatio: null,
      maxDrawdownPct: 0,
      maxDrawdownDurationDays: 0,
      annualizedReturn: null,
    };
  }

  const sorted = [...trades].sort(
    (a, b) => new Date(a.closed_at).getTime() - new Date(b.closed_at).getTime(),
  );
  const pnls = sorted.map((t) => t.net_pnl);
  const mean = pnls.reduce((s, p) => s + p, 0) / pnls.length;
  const variance = pnls.reduce((s, p) => s + (p - mean) ** 2, 0) / Math.max(1, pnls.length - 1);
  const std = Math.sqrt(variance);
  const downside = pnls.filter((p) => p < 0);
  const downStd = downside.length > 1
    ? Math.sqrt(downside.reduce((s, p) => s + p ** 2, 0) / (downside.length - 1))
    : 0;

  let equity = startingEquity;
  let peak = startingEquity;
  let maxDd = 0;
  let maxDdDays = 0;
  let ddStart: number | null = null;

  for (const t of sorted) {
    equity += t.net_pnl;
    if (equity > peak) {
      peak = equity;
      ddStart = null;
    } else {
      const dd = peak > 0 ? (peak - equity) / peak : 0;
      if (dd > maxDd) maxDd = dd;
      if (ddStart === null) ddStart = new Date(t.closed_at).getTime();
      else {
        const days = (new Date(t.closed_at).getTime() - ddStart) / 86_400_000;
        if (days > maxDdDays) maxDdDays = Math.ceil(days);
      }
    }
  }

  const net = pnls.reduce((s, p) => s + p, 0);
  const spanMs =
    new Date(sorted[sorted.length - 1].closed_at).getTime() -
    new Date(sorted[0].closed_at).getTime();
  const spanYears = Math.max(0.01, spanMs / (365.25 * 86_400_000));
  const annualized = net / spanYears;
  const maxDdUsd = maxDd * startingEquity;

  return {
    sharpeRatio: std > 0 ? (mean / std) * Math.sqrt(pnls.length) : null,
    sortinoRatio: downStd > 0 ? (mean / downStd) * Math.sqrt(pnls.length) : null,
    calmarRatio: maxDdUsd > 0 ? annualized / maxDdUsd : null,
    maxDrawdownPct: maxDd * 100,
    maxDrawdownDurationDays: maxDdDays,
    annualizedReturn: spanYears > 0 ? (net / startingEquity) * (1 / spanYears) * 100 : null,
  };
}
