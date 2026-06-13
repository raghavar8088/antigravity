/**
 * Analytics Report Generator.
 *
 * Reads NDJSON trade/equity data from the local storage layer and
 * produces summary reports (daily, weekly, monthly) as JSON files.
 * Reports are written atomically to data/reports/<period>/.
 *
 * Designed to be called by the backup API or a cron-style trigger.
 */

import path from "path";
import {
  getDataRoot,
  readNdjson,
  atomicWriteJson,
  listFiles,
} from "../utils/localStorageService";

// ── Types ──────────────────────────────────────────────────────────────────

interface FlatTrade {
  trade_id: string;
  strategy_id: number;
  strategy_name: string;
  strategy_family: string;
  side: string;
  status: string;
  opened_at: number;
  closed_at: number | null;
  entry_price: number;
  close_price: number | null;
  realized_pnl: number;
  fees: number;
  funding_costs: number;
  confidence_score: number;
  exit_reason: string | null;
  regime_at_entry: string | null;
}

interface EquityPoint {
  timestamp: number;
  equity: number;
  realized_pnl: number;
  unrealized_pnl: number;
  drawdown_pct: number;
  regime?: string;
}

export interface DailyPnlReport {
  type: "daily_pnl";
  date: string;
  generatedAt: string;
  totalTrades: number;
  closedTrades: number;
  winningTrades: number;
  losingTrades: number;
  netPnl: number;
  grossWins: number;
  grossLosses: number;
  winRate: number;
  profitFactor: number;
  totalFees: number;
  totalFundingCosts: number;
  bestTrade: { id: string; pnl: number; strategy: string } | null;
  worstTrade: { id: string; pnl: number; strategy: string } | null;
  byFamily: Record<string, { trades: number; netPnl: number; winRate: number }>;
  byRegime: Record<string, { trades: number; netPnl: number }>;
}

export interface WeeklyPerformanceReport {
  type: "weekly_performance";
  weekStart: string;
  weekEnd: string;
  generatedAt: string;
  totalTrades: number;
  netPnl: number;
  winRate: number;
  profitFactor: number;
  maxDrawdownPct: number;
  sharpeEstimate: number;
  topStrategies: { strategyId: number; name: string; netPnl: number; trades: number }[];
  dailyBreakdown: Record<string, { pnl: number; trades: number }>;
}

export interface MonthlyStrategyReport {
  type: "monthly_strategy";
  month: string;
  generatedAt: string;
  totalTrades: number;
  netPnl: number;
  winRate: number;
  profitFactor: number;
  bestFamily: string;
  worstFamily: string;
  byStrategy: {
    strategyId: number;
    name: string;
    trades: number;
    netPnl: number;
    winRate: number;
    profitFactor: number;
  }[];
  regimeBreakdown: Record<string, { trades: number; netPnl: number; winRate: number }>;
}

// ── Helpers ────────────────────────────────────────────────────────────────

async function loadTradesForRange(fromMs: number, toMs: number): Promise<FlatTrade[]> {
  const files = await listFiles("mockTrades");
  const ndjsonFiles = files.filter((f) => f.endsWith(".ndjson"));
  const all: FlatTrade[] = [];
  for (const f of ndjsonFiles) {
    const records = await readNdjson<FlatTrade>(f);
    all.push(...records.filter((r) => {
      const ts = r.closed_at ?? r.opened_at;
      return ts >= fromMs && ts <= toMs && r.status === "CLOSED";
    }));
  }
  return all;
}

async function loadEquityForRange(fromMs: number, toMs: number): Promise<EquityPoint[]> {
  const files = await listFiles("equity");
  const ndjsonFiles = files.filter((f) => f.endsWith("curve.ndjson"));
  const all: EquityPoint[] = [];
  for (const f of ndjsonFiles) {
    const records = await readNdjson<EquityPoint>(f);
    all.push(...records.filter((r) => r.timestamp >= fromMs && r.timestamp <= toMs));
  }
  return all.sort((a, b) => a.timestamp - b.timestamp);
}

function groupBy<T>(items: T[], key: (item: T) => string): Record<string, T[]> {
  const groups: Record<string, T[]> = {};
  for (const item of items) {
    const k = key(item);
    (groups[k] ??= []).push(item);
  }
  return groups;
}

function calcProfitFactor(trades: FlatTrade[]): number {
  const wins = trades.filter((t) => t.realized_pnl > 0).reduce((s, t) => s + t.realized_pnl, 0);
  const losses = Math.abs(trades.filter((t) => t.realized_pnl < 0).reduce((s, t) => s + t.realized_pnl, 0));
  return losses === 0 ? (wins > 0 ? 999 : 1) : wins / losses;
}

function calcMaxDrawdown(equity: EquityPoint[]): number {
  let peak = -Infinity;
  let maxDd = 0;
  for (const p of equity) {
    if (p.equity > peak) peak = p.equity;
    const dd = peak > 0 ? (peak - p.equity) / peak : 0;
    if (dd > maxDd) maxDd = dd;
  }
  return maxDd * 100;
}

function simpleSharpEstimate(trades: FlatTrade[]): number {
  if (trades.length < 3) return 0;
  const byDay: Record<string, number> = {};
  for (const t of trades) {
    const day = new Date(t.closed_at ?? t.opened_at).toISOString().slice(0, 10);
    byDay[day] = (byDay[day] ?? 0) + t.realized_pnl;
  }
  const returns = Object.values(byDay);
  if (returns.length < 2) return 0;
  const mean = returns.reduce((s, v) => s + v, 0) / returns.length;
  const std = Math.sqrt(returns.reduce((s, v) => s + (v - mean) ** 2, 0) / returns.length);
  return std === 0 ? 0 : (mean / std) * Math.sqrt(365);
}

// ── Daily Report ───────────────────────────────────────────────────────────

export async function generateDailyReport(dateStr?: string): Promise<DailyPnlReport> {
  const date = dateStr ?? new Date().toISOString().slice(0, 10);
  const dayStart = new Date(date + "T00:00:00Z").getTime();
  const dayEnd = dayStart + 86400000 - 1;

  const trades = await loadTradesForRange(dayStart, dayEnd);
  const winning = trades.filter((t) => t.realized_pnl > 0);
  const losing = trades.filter((t) => t.realized_pnl < 0);
  const netPnl = trades.reduce((s, t) => s + t.realized_pnl, 0);
  const grossWins = winning.reduce((s, t) => s + t.realized_pnl, 0);
  const grossLosses = Math.abs(losing.reduce((s, t) => s + t.realized_pnl, 0));

  const byFamilyRaw = groupBy(trades, (t) => t.strategy_family);
  const byFamily: DailyPnlReport["byFamily"] = {};
  for (const [fam, ts] of Object.entries(byFamilyRaw)) {
    const wins = ts.filter((t) => t.realized_pnl > 0);
    byFamily[fam] = { trades: ts.length, netPnl: ts.reduce((s, t) => s + t.realized_pnl, 0), winRate: ts.length ? wins.length / ts.length : 0 };
  }

  const byRegimeRaw = groupBy(trades, (t) => t.regime_at_entry ?? "UNKNOWN");
  const byRegime: DailyPnlReport["byRegime"] = {};
  for (const [reg, ts] of Object.entries(byRegimeRaw)) {
    byRegime[reg] = { trades: ts.length, netPnl: ts.reduce((s, t) => s + t.realized_pnl, 0) };
  }

  const sorted = [...trades].sort((a, b) => b.realized_pnl - a.realized_pnl);
  const best = sorted[0] ? { id: sorted[0].trade_id, pnl: sorted[0].realized_pnl, strategy: sorted[0].strategy_name } : null;
  const worst = sorted[sorted.length - 1]?.realized_pnl < 0
    ? { id: sorted[sorted.length - 1].trade_id, pnl: sorted[sorted.length - 1].realized_pnl, strategy: sorted[sorted.length - 1].strategy_name }
    : null;

  const report: DailyPnlReport = {
    type: "daily_pnl",
    date,
    generatedAt: new Date().toISOString(),
    totalTrades: trades.length,
    closedTrades: trades.length,
    winningTrades: winning.length,
    losingTrades: losing.length,
    netPnl,
    grossWins,
    grossLosses,
    winRate: trades.length ? winning.length / trades.length : 0,
    profitFactor: calcProfitFactor(trades),
    totalFees: trades.reduce((s, t) => s + (t.fees ?? 0), 0),
    totalFundingCosts: trades.reduce((s, t) => s + (t.funding_costs ?? 0), 0),
    bestTrade: best,
    worstTrade: worst,
    byFamily,
    byRegime,
  };

  await atomicWriteJson(
    path.join(getDataRoot(), "reports", "daily", `${date}-pnl-report.json`),
    report,
  );
  return report;
}

// ── Weekly Report ──────────────────────────────────────────────────────────

export async function generateWeeklyReport(weekStartStr?: string): Promise<WeeklyPerformanceReport> {
  const now = new Date();
  // Default: last full week (Mon–Sun)
  const dayOfWeek = now.getDay() || 7; // 1=Mon … 7=Sun
  const weekStart = weekStartStr
    ? new Date(weekStartStr + "T00:00:00Z")
    : new Date(now.getTime() - (dayOfWeek - 1) * 86400000);
  weekStart.setUTCHours(0, 0, 0, 0);
  const weekEnd = new Date(weekStart.getTime() + 7 * 86400000 - 1);

  const fromMs = weekStart.getTime();
  const toMs = weekEnd.getTime();
  const trades = await loadTradesForRange(fromMs, toMs);
  const equity = await loadEquityForRange(fromMs, toMs);

  const byStratRaw = groupBy(trades, (t) => String(t.strategy_id));
  const topStrategies = Object.entries(byStratRaw)
    .map(([, ts]) => ({
      strategyId: ts[0].strategy_id,
      name: ts[0].strategy_name,
      netPnl: ts.reduce((s, t) => s + t.realized_pnl, 0),
      trades: ts.length,
    }))
    .sort((a, b) => b.netPnl - a.netPnl)
    .slice(0, 10);

  const byDayRaw = groupBy(trades, (t) => new Date(t.closed_at ?? t.opened_at).toISOString().slice(0, 10));
  const dailyBreakdown: Record<string, { pnl: number; trades: number }> = {};
  for (const [day, ts] of Object.entries(byDayRaw)) {
    dailyBreakdown[day] = { pnl: ts.reduce((s, t) => s + t.realized_pnl, 0), trades: ts.length };
  }

  const winning = trades.filter((t) => t.realized_pnl > 0);
  const report: WeeklyPerformanceReport = {
    type: "weekly_performance",
    weekStart: weekStart.toISOString().slice(0, 10),
    weekEnd: weekEnd.toISOString().slice(0, 10),
    generatedAt: new Date().toISOString(),
    totalTrades: trades.length,
    netPnl: trades.reduce((s, t) => s + t.realized_pnl, 0),
    winRate: trades.length ? winning.length / trades.length : 0,
    profitFactor: calcProfitFactor(trades),
    maxDrawdownPct: calcMaxDrawdown(equity),
    sharpeEstimate: simpleSharpEstimate(trades),
    topStrategies,
    dailyBreakdown,
  };

  await atomicWriteJson(
    path.join(getDataRoot(), "reports", "weekly", `${weekStart.toISOString().slice(0, 10)}-weekly-report.json`),
    report,
  );
  return report;
}

// ── Monthly Report ─────────────────────────────────────────────────────────

export async function generateMonthlyReport(monthStr?: string): Promise<MonthlyStrategyReport> {
  const now = new Date();
  const month = monthStr ?? `${now.getUTCFullYear()}-${String(now.getUTCMonth() + 1).padStart(2, "0")}`;
  const fromMs = new Date(month + "-01T00:00:00Z").getTime();
  const toMs = new Date(new Date(fromMs).getUTCFullYear(), new Date(fromMs).getUTCMonth() + 1, 0, 23, 59, 59, 999).getTime();

  const trades = await loadTradesForRange(fromMs, toMs);
  const winning = trades.filter((t) => t.realized_pnl > 0);

  const byStratRaw = groupBy(trades, (t) => String(t.strategy_id));
  const byStrategy = Object.entries(byStratRaw)
    .map(([, ts]) => {
      const wins = ts.filter((t) => t.realized_pnl > 0);
      return {
        strategyId: ts[0].strategy_id,
        name: ts[0].strategy_name,
        trades: ts.length,
        netPnl: ts.reduce((s, t) => s + t.realized_pnl, 0),
        winRate: ts.length ? wins.length / ts.length : 0,
        profitFactor: calcProfitFactor(ts),
      };
    })
    .sort((a, b) => b.netPnl - a.netPnl);

  const byFamilyRaw = groupBy(trades, (t) => t.strategy_family);
  const familyPnls = Object.entries(byFamilyRaw).map(([fam, ts]) => ({ fam, pnl: ts.reduce((s, t) => s + t.realized_pnl, 0) }));
  const bestFamily = familyPnls.sort((a, b) => b.pnl - a.pnl)[0]?.fam ?? "";
  const worstFamily = familyPnls[familyPnls.length - 1]?.fam ?? "";

  const byRegimeRaw = groupBy(trades, (t) => t.regime_at_entry ?? "UNKNOWN");
  const regimeBreakdown: MonthlyStrategyReport["regimeBreakdown"] = {};
  for (const [reg, ts] of Object.entries(byRegimeRaw)) {
    const wins = ts.filter((t) => t.realized_pnl > 0);
    regimeBreakdown[reg] = { trades: ts.length, netPnl: ts.reduce((s, t) => s + t.realized_pnl, 0), winRate: ts.length ? wins.length / ts.length : 0 };
  }

  const report: MonthlyStrategyReport = {
    type: "monthly_strategy",
    month,
    generatedAt: new Date().toISOString(),
    totalTrades: trades.length,
    netPnl: trades.reduce((s, t) => s + t.realized_pnl, 0),
    winRate: trades.length ? winning.length / trades.length : 0,
    profitFactor: calcProfitFactor(trades),
    bestFamily,
    worstFamily,
    byStrategy,
    regimeBreakdown,
  };

  await atomicWriteJson(
    path.join(getDataRoot(), "reports", "monthly", `${month}-strategy-report.json`),
    report,
  );
  return report;
}
