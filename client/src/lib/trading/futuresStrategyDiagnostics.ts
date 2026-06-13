/**
 * futuresStrategyDiagnostics.ts
 * Pure functions — no side effects, fully testable.
 * Aggregates closed trade history into per-strategy performance rows.
 */

import type { BTCFuturesTrade } from "./btcFuturesTrade.types";
import { isProbeOrBootstrapTrade } from "../analytics/futuresSessionMetrics";
import type { PaperTradeDbRow } from "../portfolio/paperTradesTypes";

// ─── Core types ───────────────────────────────────────────────────────────────

export interface StrategyDiagnosticRow {
  strategyId:        number;
  strategyName:      string;
  templateFamily:    string;
  totalTrades:       number;
  wins:              number;
  losses:            number;
  winRate:           number;           // 0–1
  totalNetPnl:       number;
  avgNetPnl:         number;           // expectancy per trade
  avgWin:            number;
  avgLoss:           number;
  profitFactor:      number;           // sum(wins) / |sum(losses)|, Infinity if no losses
  totalFees:         number;
  feePctOfAbsGross:  number;           // sum(fees) / sum(|gross|) — fraction (not %)
  avgHoldMinutes:    number;
  exitReasonCounts:  Record<string, number>; // { SL: 3, TP: 1, TIME: 2 ... }
  slCount:           number;
  tpCount:           number;
  timeCount:         number;
  trailCount:        number;
  profitLockCount:   number;
  worstTrade:        number;
  bestTrade:         number;
  lastTradeAt:       string | null;
  isProbe:           boolean;          // exclude from production metrics
}

export interface DiagnosticSummary {
  rows:               StrategyDiagnosticRow[];
  totalProduction:    number;          // non-probe trade count
  topByExpectancy:    StrategyDiagnosticRow[];  // top 5 by avgNetPnl
  bottomByExpectancy: StrategyDiagnosticRow[];  // bottom 5 by avgNetPnl
  highFeeStrategies:  StrategyDiagnosticRow[];  // feePctOfAbsGross > 0.5
  slDominatedStrats:  StrategyDiagnosticRow[];  // slCount / total > 0.6, min 3 trades
  computedAt:         number;          // Date.now()
}

export interface HealthCheckResult {
  window:            number;
  expectancy:        number;
  expectancyPass:    boolean;  // > 0
  winRate:           number;
  winRatePass:       boolean;  // > 0.35
  feePctOfAbsGross:  number;
  feePass:           boolean;  // < 0.5
  profitFactor:      number;
  pfPass:            boolean;  // > 1.0
  tpHits:            number;
  tpHitPass:         boolean;  // >= 3 in window
  slCount:           number;
  timeCount:         number;
  overallPass:       boolean;  // all 5 pass
  grade:             "A" | "B" | "C" | "F";
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function holdMinutesFromRow(trade: PaperTradeDbRow): number {
  if (!trade.closed_at || !trade.opened_at) return 0;
  return (
    new Date(trade.closed_at).getTime() - new Date(trade.opened_at).getTime()
  ) / 60_000;
}

// ─── Per-strategy aggregation ─────────────────────────────────────────────────

export function computeStrategyDiagnostics(
  trades: PaperTradeDbRow[],
): DiagnosticSummary {
  const groups = new Map<number, PaperTradeDbRow[]>();

  for (const t of trades) {
    const sid = t.strategy_id ?? -1;
    if (!groups.has(sid)) groups.set(sid, []);
    groups.get(sid)!.push(t);
  }

  const rows: StrategyDiagnosticRow[] = [];

  for (const [strategyId, stratTrades] of groups) {
    const name   = stratTrades[0]?.strategy_name ?? `strategy_${strategyId}`;
    const family = stratTrades[0]?.template_family ?? "unknown";
    const probe  = isProbeOrBootstrapTrade({ strategy_name: name });

    const exitReasonCounts: Record<string, number> = {};
    for (const t of stratTrades) {
      const r = t.exit_reason ?? "UNKNOWN";
      exitReasonCounts[r] = (exitReasonCounts[r] ?? 0) + 1;
    }

    const wins   = stratTrades.filter((t) => (t.net_pnl ?? 0) > 0);
    const losses = stratTrades.filter((t) => (t.net_pnl ?? 0) <= 0);

    const totalNetPnl   = stratTrades.reduce((s, t) => s + (t.net_pnl   ?? 0), 0);
    const totalFees     = stratTrades.reduce((s, t) => s + (t.fees       ?? 0), 0);
    const totalAbsGross = stratTrades.reduce((s, t) => s + Math.abs(t.gross_pnl ?? 0), 0);
    const sumWins       = wins.reduce((s, t)   => s + (t.net_pnl ?? 0), 0);
    const sumLosses     = Math.abs(losses.reduce((s, t) => s + (t.net_pnl ?? 0), 0));

    const pnlList  = stratTrades.map((t) => t.net_pnl ?? 0);
    const holdList = stratTrades.map(holdMinutesFromRow);

    const sortedClosed = [...stratTrades]
      .filter((t) => t.closed_at)
      .sort(
        (a, b) =>
          new Date(b.closed_at).getTime() - new Date(a.closed_at).getTime(),
      );

    rows.push({
      strategyId,
      strategyName:     name,
      templateFamily:   family,
      totalTrades:      stratTrades.length,
      wins:             wins.length,
      losses:           losses.length,
      winRate:          stratTrades.length ? wins.length / stratTrades.length : 0,
      totalNetPnl,
      avgNetPnl:        stratTrades.length ? totalNetPnl / stratTrades.length : 0,
      avgWin:           wins.length   ? sumWins / wins.length          : 0,
      avgLoss:          losses.length ? -(sumLosses / losses.length)   : 0,
      profitFactor:     sumLosses > 0 ? sumWins / sumLosses : sumWins > 0 ? Infinity : 0,
      totalFees,
      feePctOfAbsGross: totalAbsGross > 0 ? totalFees / totalAbsGross  : 0,
      avgHoldMinutes:   holdList.length ? holdList.reduce((a, b) => a + b, 0) / holdList.length : 0,
      exitReasonCounts,
      slCount:          exitReasonCounts["SL"]          ?? 0,
      tpCount:          exitReasonCounts["TP"]          ?? 0,
      timeCount:        exitReasonCounts["TIME"]        ?? 0,
      trailCount:       exitReasonCounts["TRAIL"]       ?? 0,
      profitLockCount:  exitReasonCounts["PROFIT_LOCK"] ?? 0,
      worstTrade:       pnlList.length ? Math.min(...pnlList) : 0,
      bestTrade:        pnlList.length ? Math.max(...pnlList) : 0,
      lastTradeAt:      sortedClosed[0]?.closed_at ?? null,
      isProbe:          probe,
    });
  }

  const prodRows = rows.filter((r) => !r.isProbe);
  const sorted   = [...prodRows].sort((a, b) => b.avgNetPnl - a.avgNetPnl);

  return {
    rows,
    totalProduction:    prodRows.reduce((s, r) => s + r.totalTrades, 0),
    topByExpectancy:    sorted.slice(0, 5),
    bottomByExpectancy: sorted.slice(-5).reverse(),
    highFeeStrategies:  prodRows.filter((r) => r.feePctOfAbsGross > 0.5),
    slDominatedStrats:  prodRows.filter(
      (r) => r.totalTrades >= 3 && r.slCount / r.totalTrades > 0.6,
    ),
    computedAt: Date.now(),
  };
}

// ─── 20-trade rolling health check ───────────────────────────────────────────

export function computeRollingHealthCheck(
  trades: PaperTradeDbRow[],
  windowN = 20,
): HealthCheckResult {
  const prod = trades
    .filter((t) => !isProbeOrBootstrapTrade({ strategy_name: t.strategy_name }) && t.closed_at)
    .sort(
      (a, b) =>
        new Date(b.closed_at).getTime() - new Date(a.closed_at).getTime(),
    )
    .slice(0, windowN);

  const n        = prod.length;
  const wins     = prod.filter((t) => (t.net_pnl ?? 0) > 0);
  const sumNet   = prod.reduce((s, t) => s + (t.net_pnl   ?? 0), 0);
  const sumFees  = prod.reduce((s, t) => s + (t.fees       ?? 0), 0);
  const sumAbsGross = prod.reduce((s, t) => s + Math.abs(t.gross_pnl ?? 0), 0);
  const sumWins  = wins.reduce((s, t) => s + (t.net_pnl ?? 0), 0);
  const sumLosses = Math.abs(
    prod
      .filter((t) => (t.net_pnl ?? 0) <= 0)
      .reduce((s, t) => s + (t.net_pnl ?? 0), 0),
  );

  const expectancy       = n ? sumNet / n : 0;
  const winRate          = n ? wins.length / n : 0;
  const feePctOfAbsGross = sumAbsGross > 0 ? sumFees / sumAbsGross : 0;
  const profitFactor     = sumLosses > 0 ? sumWins / sumLosses : sumWins > 0 ? Infinity : 0;
  const tpHits           = prod.filter((t) => t.exit_reason === "TP").length;
  const slCount          = prod.filter((t) => t.exit_reason === "SL").length;
  const timeCount        = prod.filter((t) => t.exit_reason === "TIME").length;

  const expectancyPass = expectancy       > 0;
  const winRatePass    = winRate          > 0.35;
  const feePass        = feePctOfAbsGross < 0.5;
  const pfPass         = profitFactor     > 1.0;
  const tpHitPass      = tpHits           >= 3;

  const passCount = [expectancyPass, winRatePass, feePass, pfPass, tpHitPass].filter(Boolean).length;
  const overallPass = passCount === 5;

  const grade: HealthCheckResult["grade"] =
    passCount === 5 ? "A" :
    passCount >= 4  ? "B" :
    passCount >= 3  ? "C" : "F";

  return {
    window: n,
    expectancy,       expectancyPass,
    winRate,          winRatePass,
    feePctOfAbsGross, feePass,
    profitFactor,     pfPass,
    tpHits,           tpHitPass,
    slCount,
    timeCount,
    overallPass,
    grade,
  };
}

// ─── Engine-trade adapters ────────────────────────────────────────────────────
// `BTCFuturesTrade` (camelCase, in-memory) → `PaperTradeDbRow` (snake_case, DB).
// Allows calling the pure functions directly from calculateStats() without async.

function engineTradeToPaperRow(t: BTCFuturesTrade): PaperTradeDbRow {
  return {
    id:              t.id,
    created_at:      t.openedAt,
    account_key:     "",
    client_trade_id: t.clientTradeId ?? t.id,
    opened_at:       t.openedAt,
    closed_at:       t.closedAt,
    symbol:          t.symbol,
    strategy_id:     t.strategyId,
    strategy_name:   t.strategyName,
    side:            t.side,
    entry_price:     t.entryPrice,
    exit_price:      t.exitPrice,
    contracts:       t.contracts,
    notional:        t.notional,
    margin_used:     t.marginUsed,
    gross_pnl:       t.realizedPnl,
    fees:            t.fees,
    funding_costs:   t.fundingCosts,
    net_pnl:         t.netPnl,
    exit_reason:     t.exitReason,
    payload:         null,
    template_family: null,
  };
}

export function computeStrategyDiagnosticsFromEngineTrades(
  trades: BTCFuturesTrade[],
): DiagnosticSummary {
  return computeStrategyDiagnostics(trades.map(engineTradeToPaperRow));
}

export function computeRollingHealthCheckFromEngineTrades(
  trades: BTCFuturesTrade[],
  windowN = 20,
): HealthCheckResult {
  return computeRollingHealthCheck(trades.map(engineTradeToPaperRow), windowN);
}
