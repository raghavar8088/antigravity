import { coerceEpochMs } from "@/lib/chartTime";
import type { MarketRegime } from "@/lib/marketRegimeClassifier";
import type { MockAccountState, MockTrade } from "@/lib/mockTradingEngine";
import type { StrategyScore } from "@/lib/strategyScoringEngine";

export interface ResearchChartPoint {
  timestamp: number;
  value: number;
}

export interface EquityAnalyticsSnapshot {
  timestamp: number;
  equity: number;
  realizedPnl: number;
  unrealizedPnl: number;
  cumulativeNetPnl: number;
  drawdownPct: number;
  dailyPnl: number;
  regime?: MarketRegime | null;
}

export interface FamilyAnalyticsRow {
  family: string;
  trades: number;
  openTrades: number;
  closedTrades: number;
  netPnl: number;
  winRate: number;
  profitFactor: number;
  longTrades: number;
  shortTrades: number;
  exposure: number;
  bestRegime: string;
  worstRegime: string;
}

export interface StrategyCorrelationRow {
  strategyId: number;
  strategyName: string;
  correlations: Record<number, number>;
}

export interface ExposureAnalytics {
  totalExposure: number;
  longExposure: number;
  shortExposure: number;
  longConcentrationPct: number;
  shortConcentrationPct: number;
  familyExposure: Array<{ family: string; exposure: number; pct: number }>;
  openTrades: number;
}

export interface BiasAnalytics {
  longTrades: number;
  shortTrades: number;
  longWinRate: number;
  shortWinRate: number;
  longPnl: number;
  shortPnl: number;
  longBiasPct: number;
  shortBiasPct: number;
}

export interface ResearchWarning {
  severity: "INFO" | "WARNING" | "CRITICAL";
  code: string;
  message: string;
}

export interface AdvancedResearchAnalytics {
  familyRows: FamilyAnalyticsRow[];
  correlationRows: StrategyCorrelationRow[];
  exposure: ExposureAnalytics;
  bias: BiasAnalytics;
  warnings: ResearchWarning[];
}

const DAY_MS = 86_400_000;

function closedTrades(trades: readonly MockTrade[]): MockTrade[] {
  return trades.filter((trade) => trade.status === "CLOSED" && trade.closedAt != null);
}

function pnlForTrade(trade: MockTrade): number {
  return trade.status === "OPEN" ? trade.unrealizedPnl : trade.realizedPnl;
}

function familyForTrade(trade: MockTrade): string {
  return trade.strategyFamily ?? (trade.researchPack ? "Research" : "Core");
}

function winRate(trades: readonly MockTrade[]): number {
  const closed = closedTrades(trades);
  if (closed.length === 0) return 0;
  return closed.filter((trade) => trade.realizedPnl > 0).length / closed.length;
}

function profitFactor(trades: readonly MockTrade[]): number {
  const closed = closedTrades(trades);
  const wins = closed.filter((trade) => trade.realizedPnl > 0).reduce((sum, trade) => sum + trade.realizedPnl, 0);
  const losses = Math.abs(closed.filter((trade) => trade.realizedPnl < 0).reduce((sum, trade) => sum + trade.realizedPnl, 0));
  return losses > 0 ? wins / losses : wins > 0 ? 999 : 0;
}

export function computeDailyPnlPoints(trades: readonly MockTrade[]): ResearchChartPoint[] {
  const buckets = new Map<number, number>();
  for (const trade of closedTrades(trades)) {
    const closedAt = coerceEpochMs(trade.closedAt);
    if (closedAt <= 0) continue;
    const dayTs = Math.floor(closedAt / DAY_MS) * DAY_MS;
    buckets.set(dayTs, (buckets.get(dayTs) ?? 0) + trade.realizedPnl);
  }
  return [...buckets.entries()]
    .sort((a, b) => a[0] - b[0])
    .map(([timestamp, value]) => ({ timestamp, value }));
}

export function computeCumulativeNetPnlPoints(trades: readonly MockTrade[]): ResearchChartPoint[] {
  let cumulative = 0;
  return closedTrades(trades)
    .map((trade) => ({ trade, closedAt: coerceEpochMs(trade.closedAt) }))
    .filter((row) => row.closedAt > 0)
    .sort((a, b) => a.closedAt - b.closedAt)
    .map(({ trade, closedAt }) => {
      cumulative += trade.realizedPnl;
      return { timestamp: closedAt, value: cumulative };
    });
}

export function computeStrategyComparisonSeries(
  trades: readonly MockTrade[],
  limit = 5,
): Array<{ id: number; name: string; family: string; points: ResearchChartPoint[] }> {
  const grouped = new Map<number, MockTrade[]>();
  for (const trade of closedTrades(trades)) {
    const bucket = grouped.get(trade.strategyId) ?? [];
    bucket.push(trade);
    grouped.set(trade.strategyId, bucket);
  }

  return [...grouped.entries()]
    .map(([id, strategyTrades]) => ({
      id,
      name: strategyTrades[0]?.strategyName ?? `Strategy ${id}`,
      family: familyForTrade(strategyTrades[0]),
      netPnl: strategyTrades.reduce((sum, trade) => sum + trade.realizedPnl, 0),
      trades: strategyTrades,
    }))
    .sort((a, b) => Math.abs(b.netPnl) - Math.abs(a.netPnl))
    .slice(0, limit)
    .map((item) => {
      let cumulative = 0;
      const points = item.trades
        .sort((a, b) => (a.closedAt ?? 0) - (b.closedAt ?? 0))
        .map((trade) => {
          cumulative += trade.realizedPnl;
          return { timestamp: trade.closedAt ?? trade.openedAt, value: cumulative };
        });
      return { id: item.id, name: item.name, family: item.family, points };
    });
}

export function computeFamilyComparisonSeries(
  trades: readonly MockTrade[],
  limit = 6,
): Array<{ id: string; name: string; points: ResearchChartPoint[] }> {
  const grouped = new Map<string, MockTrade[]>();
  for (const trade of closedTrades(trades)) {
    const family = familyForTrade(trade);
    const bucket = grouped.get(family) ?? [];
    bucket.push(trade);
    grouped.set(family, bucket);
  }

  return [...grouped.entries()]
    .map(([family, familyTrades]) => ({
      family,
      netPnl: familyTrades.reduce((sum, trade) => sum + trade.realizedPnl, 0),
      trades: familyTrades,
    }))
    .sort((a, b) => Math.abs(b.netPnl) - Math.abs(a.netPnl))
    .slice(0, limit)
    .map((item) => {
      let cumulative = 0;
      const points = item.trades
        .sort((a, b) => (a.closedAt ?? 0) - (b.closedAt ?? 0))
        .map((trade) => {
          cumulative += trade.realizedPnl;
          return { timestamp: trade.closedAt ?? trade.openedAt, value: cumulative };
        });
      return { id: item.family, name: item.family, points };
    });
}

export function createEquitySnapshot(args: {
  account: MockAccountState;
  trades: readonly MockTrade[];
  regime?: MarketRegime | null;
  timestamp?: number;
}): EquityAnalyticsSnapshot {
  const timestamp = args.timestamp ?? Date.now();
  const dayStart = Math.floor(timestamp / DAY_MS) * DAY_MS;
  const dailyPnl = closedTrades(args.trades)
    .filter((trade) => (trade.closedAt ?? 0) >= dayStart)
    .reduce((sum, trade) => sum + trade.realizedPnl, 0);
  return {
    timestamp,
    equity: args.account.equity,
    realizedPnl: args.account.realizedPnl,
    unrealizedPnl: args.account.unrealizedPnl,
    cumulativeNetPnl: args.account.realizedPnl + args.account.unrealizedPnl,
    drawdownPct: args.account.maxDrawdownPct,
    dailyPnl,
    regime: args.regime,
  };
}

export function computeFamilyAnalytics(trades: readonly MockTrade[]): FamilyAnalyticsRow[] {
  const grouped = new Map<string, MockTrade[]>();
  for (const trade of trades) {
    const family = familyForTrade(trade);
    const bucket = grouped.get(family) ?? [];
    bucket.push(trade);
    grouped.set(family, bucket);
  }

  return [...grouped.entries()]
    .map(([family, familyTrades]) => {
      const byRegime = new Map<string, number>();
      for (const trade of closedTrades(familyTrades)) {
        const regime = trade.regimeAtEntry ?? "UNKNOWN";
        byRegime.set(regime, (byRegime.get(regime) ?? 0) + trade.realizedPnl);
      }
      const regimes = [...byRegime.entries()].sort((a, b) => b[1] - a[1]);
      const closed = closedTrades(familyTrades);
      return {
        family,
        trades: familyTrades.length,
        openTrades: familyTrades.filter((trade) => trade.status === "OPEN").length,
        closedTrades: closed.length,
        netPnl: familyTrades.reduce((sum, trade) => sum + pnlForTrade(trade), 0),
        winRate: winRate(familyTrades),
        profitFactor: profitFactor(familyTrades),
        longTrades: familyTrades.filter((trade) => trade.side === "BUY").length,
        shortTrades: familyTrades.filter((trade) => trade.side === "SELL").length,
        exposure: familyTrades
          .filter((trade) => trade.status === "OPEN")
          .reduce((sum, trade) => sum + trade.notional, 0),
        bestRegime: regimes[0]?.[0] ?? "UNKNOWN",
        worstRegime: regimes[regimes.length - 1]?.[0] ?? "UNKNOWN",
      };
    })
    .sort((a, b) => b.netPnl - a.netPnl);
}

function pearson(a: readonly number[], b: readonly number[]): number {
  const n = Math.min(a.length, b.length);
  if (n < 3) return 0;
  const ax = a.slice(-n);
  const bx = b.slice(-n);
  const meanA = ax.reduce((sum, value) => sum + value, 0) / n;
  const meanB = bx.reduce((sum, value) => sum + value, 0) / n;
  let numerator = 0;
  let denomA = 0;
  let denomB = 0;
  for (let i = 0; i < n; i++) {
    const da = ax[i] - meanA;
    const db = bx[i] - meanB;
    numerator += da * db;
    denomA += da * da;
    denomB += db * db;
  }
  const denominator = Math.sqrt(denomA * denomB);
  return denominator > 0 ? numerator / denominator : 0;
}

export function computeStrategyCorrelationRows(
  trades: readonly MockTrade[],
  limit = 8,
): StrategyCorrelationRow[] {
  const series = computeStrategyComparisonSeries(trades, limit).map((item) => ({
    strategyId: item.id,
    strategyName: item.name,
    values: item.points.map((point) => point.value),
  }));

  return series.map((row) => ({
    strategyId: row.strategyId,
    strategyName: row.strategyName,
    correlations: Object.fromEntries(
      series.map((other) => [other.strategyId, row.strategyId === other.strategyId ? 1 : pearson(row.values, other.values)]),
    ),
  }));
}

export function computeExposureAnalytics(trades: readonly MockTrade[]): ExposureAnalytics {
  const open = trades.filter((trade) => trade.status === "OPEN");
  const longExposure = open.filter((trade) => trade.side === "BUY").reduce((sum, trade) => sum + trade.notional, 0);
  const shortExposure = open.filter((trade) => trade.side === "SELL").reduce((sum, trade) => sum + trade.notional, 0);
  const totalExposure = longExposure + shortExposure;
  const byFamily = new Map<string, number>();
  for (const trade of open) {
    const family = familyForTrade(trade);
    byFamily.set(family, (byFamily.get(family) ?? 0) + trade.notional);
  }
  return {
    totalExposure,
    longExposure,
    shortExposure,
    longConcentrationPct: totalExposure > 0 ? longExposure / totalExposure : 0,
    shortConcentrationPct: totalExposure > 0 ? shortExposure / totalExposure : 0,
    familyExposure: [...byFamily.entries()]
      .map(([family, exposure]) => ({ family, exposure, pct: totalExposure > 0 ? exposure / totalExposure : 0 }))
      .sort((a, b) => b.exposure - a.exposure),
    openTrades: open.length,
  };
}

export function computeBiasAnalytics(trades: readonly MockTrade[]): BiasAnalytics {
  const closed = closedTrades(trades);
  const longs = closed.filter((trade) => trade.side === "BUY");
  const shorts = closed.filter((trade) => trade.side === "SELL");
  const longPnl = longs.reduce((sum, trade) => sum + trade.realizedPnl, 0);
  const shortPnl = shorts.reduce((sum, trade) => sum + trade.realizedPnl, 0);
  const total = longs.length + shorts.length;
  return {
    longTrades: longs.length,
    shortTrades: shorts.length,
    longWinRate: longs.length > 0 ? longs.filter((trade) => trade.realizedPnl > 0).length / longs.length : 0,
    shortWinRate: shorts.length > 0 ? shorts.filter((trade) => trade.realizedPnl > 0).length / shorts.length : 0,
    longPnl,
    shortPnl,
    longBiasPct: total > 0 ? longs.length / total : 0,
    shortBiasPct: total > 0 ? shorts.length / total : 0,
  };
}

export function buildResearchWarnings(args: {
  trades: readonly MockTrade[];
  scores: readonly StrategyScore[];
  account: MockAccountState;
  exposure: ExposureAnalytics;
  correlations: readonly StrategyCorrelationRow[];
}): ResearchWarning[] {
  const warnings: ResearchWarning[] = [];
  const lowSample = args.scores.filter((score) => score.confidenceRating === "INSUFFICIENT" || score.confidenceRating === "LOW");
  if (lowSample.length > 0) {
    warnings.push({
      severity: "INFO",
      code: "INSUFFICIENT_SAMPLE_SIZE",
      message: `${lowSample.length} ranked strategies have low sample-size confidence.`,
    });
  }
  if (args.account.maxDrawdownPct > 10) {
    warnings.push({
      severity: args.account.maxDrawdownPct > 20 ? "CRITICAL" : "WARNING",
      code: "EXCESSIVE_DRAWDOWN",
      message: `Mock account drawdown is ${args.account.maxDrawdownPct.toFixed(2)}%.`,
    });
  }
  if (args.exposure.totalExposure > args.account.equity * 5) {
    warnings.push({
      severity: "WARNING",
      code: "HIGH_EXPOSURE",
      message: `Open mock exposure is ${args.exposure.totalExposure.toFixed(0)}, above 5x current equity.`,
    });
  }
  const highCorrelation = args.correlations.some((row) =>
    Object.entries(row.correlations).some(([id, value]) => Number(id) !== row.strategyId && value > 0.8),
  );
  if (highCorrelation) {
    warnings.push({
      severity: "WARNING",
      code: "CORRELATED_STRATEGIES",
      message: "Some strategy equity curves are highly correlated; review stacking risk.",
    });
  }
  return warnings;
}

export function computeWeeklyPnl(trades: readonly MockTrade[]): number {
  const now = Date.now();
  const weekStart = now - (7 * DAY_MS);
  return closedTrades(trades)
    .filter((trade) => (trade.closedAt ?? 0) >= weekStart)
    .reduce((sum, trade) => sum + trade.realizedPnl, 0);
}

export function computeMonthlyPnl(trades: readonly MockTrade[]): number {
  const now = Date.now();
  const monthStart = now - (30 * DAY_MS);
  return closedTrades(trades)
    .filter((trade) => (trade.closedAt ?? 0) >= monthStart)
    .reduce((sum, trade) => sum + trade.realizedPnl, 0);
}

export interface MonthlyHeatmapRow {
  year: number;
  months: Record<number, number>; // 0-11 -> PnL
  total: number;
}

export function computeMonthlyHeatmap(trades: readonly MockTrade[]): MonthlyHeatmapRow[] {
  const closed = closedTrades(trades);
  const heatmap: Record<number, Record<number, number>> = {};

  for (const trade of closed) {
    const date = new Date(trade.closedAt!);
    const year = date.getUTCFullYear();
    const month = date.getUTCMonth();

    if (!heatmap[year]) heatmap[year] = {};
    heatmap[year][month] = (heatmap[year][month] ?? 0) + trade.realizedPnl;
  }

  return Object.entries(heatmap)
    .map(([year, months]) => {
      const yearNum = parseInt(year);
      const monthValues = Object.values(months);
      return {
        year: yearNum,
        months,
        total: monthValues.reduce((sum, v) => sum + v, 0),
      };
    })
    .sort((a, b) => b.year - a.year);
}

export function computeAdvancedResearchAnalytics(args: {
  trades: readonly MockTrade[];
  scores: readonly StrategyScore[];
  account: MockAccountState;
}): AdvancedResearchAnalytics {
  const correlationRows = computeStrategyCorrelationRows(args.trades);
  const exposure = computeExposureAnalytics(args.trades);
  return {
    familyRows: computeFamilyAnalytics(args.trades),
    correlationRows,
    exposure,
    bias: computeBiasAnalytics(args.trades),
    warnings: buildResearchWarnings({
      trades: args.trades,
      scores: args.scores,
      account: args.account,
      exposure,
      correlations: correlationRows,
    }),
  };
}
