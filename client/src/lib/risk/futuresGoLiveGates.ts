/**
 * futuresGoLiveGates.ts
 * Evaluates paper desk evidence against go-live gates from
 * LIVE_TRADING_PHASE.md §3. Read-only — never enables live trading.
 */

import type { PaperTradeDbRow } from "../portfolio/paperTradesTypes";
import type { HealthCheckResult } from "../trading/futuresStrategyDiagnostics";
import type { ReadinessReport } from "./futuresProductionReadiness";
import {
  computeSessionTradingMetrics,
  isProbeOrBootstrapTrade,
} from "../analytics/futuresSessionMetrics";

export interface GoLiveGate {
  id: string;
  label: string;
  pass: boolean;
  value: string;
  required: string;
  severity: "BLOCKER" | "WARN" | "INFO";
  category: "SAMPLE" | "PERFORMANCE" | "RISK" | "OPS" | "REPLAY";
}

export type GoLiveRecommendation =
  | "NOT_READY"
  | "COLLECT_MORE_DATA"
  | "REVIEW_WARNINGS"
  | "PAPER_READY";

export interface GoLiveGateReport {
  gates: GoLiveGate[];
  blockers: GoLiveGate[];
  warnings: GoLiveGate[];
  allBlockersPass: boolean;
  score: number;
  totalProduction: number;
  daysOfData: number;
  computedAt: number;
  recommendation: GoLiveRecommendation;
}

export interface GoLiveGateInputs {
  trades: PaperTradeDbRow[];
  health: HealthCheckResult | null;
  readiness: ReadinessReport | null;
  replaySignFlipRate?: number | null;
  shadowIntentCount?: number;
  nowMs?: number;
}

/** Paper-ready minimum (PR-15 harness); full live per LIVE_TRADING_PHASE.md uses 200 / 30d. */
const MIN_TRADES_PAPER = 50;
const MIN_TRADES_FULL_LIVE = 200;
const MIN_DAYS_PAPER = 7;
const MIN_DAYS_FULL_LIVE = 30;
const MIN_EXPECTANCY = 0;
const MIN_PROFIT_FACTOR_PAPER = 1.0;
const MIN_PROFIT_FACTOR_FULL = 1.1;
const MAX_FEE_PCT = 0.5;
const MAX_FEE_PCT_STRICT = 0.35;
const MAX_DRAWDOWN_PCT = 25;
const MIN_WIN_RATE = 0.35;
const MAX_REPLAY_SIGN_FLIP_RATE = 0.15;

const SESSION_BASE = 1000;

function productionTrades(trades: PaperTradeDbRow[]): PaperTradeDbRow[] {
  return trades.filter(
    (t) =>
      !isProbeOrBootstrapTrade({ strategy_name: t.strategy_name }) &&
      Boolean(t.closed_at),
  );
}

function daysOfDataFromTrades(prod: PaperTradeDbRow[], nowMs: number): number {
  if (!prod.length) return 0;
  let earliest = Infinity;
  for (const t of prod) {
    const o = t.opened_at ? new Date(t.opened_at).getTime() : NaN;
    if (Number.isFinite(o)) earliest = Math.min(earliest, o);
  }
  if (!Number.isFinite(earliest)) return 0;
  return Math.max(0, (nowMs - earliest) / 86_400_000);
}

function profitFactorFromTrades(prod: PaperTradeDbRow[]): number {
  let sumWins = 0;
  let sumLosses = 0;
  for (const t of prod) {
    const net = t.net_pnl ?? 0;
    if (net > 0) sumWins += net;
    else sumLosses += Math.abs(net);
  }
  if (sumLosses < 1e-12) return sumWins > 0 ? Infinity : 0;
  return sumWins / sumLosses;
}

function maxDrawdownPctFromTrades(prod: PaperTradeDbRow[]): number {
  if (!prod.length) return 0;
  const sorted = [...prod].sort(
    (a, b) =>
      new Date(a.closed_at!).getTime() - new Date(b.closed_at!).getTime(),
  );
  let equity = SESSION_BASE;
  let peak = SESSION_BASE;
  let maxDd = 0;
  for (const t of sorted) {
    equity += t.net_pnl ?? 0;
    peak = Math.max(peak, equity);
    if (peak > 0) {
      maxDd = Math.max(maxDd, ((peak - equity) / peak) * 100);
    }
  }
  return maxDd;
}

/** Full-live tier warnings — shown in UI but do not block paper-ready recommendation. */
const FULL_LIVE_WARN_IDS = new Set([
  "SAMPLE_SIZE_200",
  "SAMPLE_DAYS_30",
  "PROFIT_FACTOR_FULL",
  "FEE_RATIO_STRICT",
]);

function deriveRecommendation(
  blockers: GoLiveGate[],
  warnings: GoLiveGate[],
  totalProduction: number,
): GoLiveRecommendation {
  const blockersFail = blockers.some((g) => !g.pass);
  if (blockersFail) return "NOT_READY";
  if (totalProduction < MIN_TRADES_PAPER) return "COLLECT_MORE_DATA";
  const paperWarnings = warnings.filter((g) => !FULL_LIVE_WARN_IDS.has(g.id));
  if (paperWarnings.some((g) => !g.pass)) return "REVIEW_WARNINGS";
  return "PAPER_READY";
}

export function computeGoLiveGates(inputs: GoLiveGateInputs): GoLiveGateReport {
  const nowMs = inputs.nowMs ?? Date.now();
  const prod = productionTrades(inputs.trades);
  const totalProduction = prod.length;
  const daysOfData = daysOfDataFromTrades(prod, nowMs);

  const sessionMetrics = computeSessionTradingMetrics(
    prod.map((t) => ({
      openedAt: t.opened_at ?? "",
      closedAt: t.closed_at ?? "",
      netPnl: t.net_pnl ?? 0,
      fees: t.fees ?? 0,
      realizedPnl: t.gross_pnl ?? 0,
      strategyName: t.strategy_name ?? "",
    })),
    nowMs,
  );

  const expectancy = sessionMetrics.expectancyPerTrade;
  const feePctFraction = sessionMetrics.feePctOfAbsGross / 100;
  const winRate =
    totalProduction > 0
      ? prod.filter((t) => (t.net_pnl ?? 0) > 0).length / totalProduction
      : 0;
  const profitFactor = profitFactorFromTrades(prod);
  const maxDrawdownPct = maxDrawdownPctFromTrades(prod);

  const gates: GoLiveGate[] = [];

  const add = (
    id: string,
    label: string,
    pass: boolean,
    value: string,
    required: string,
    severity: GoLiveGate["severity"],
    category: GoLiveGate["category"],
  ) => {
    gates.push({ id, label, pass, value, required, severity, category });
  };

  add(
    "SAMPLE_SIZE_50",
    "Closed production trades (paper-ready)",
    totalProduction >= MIN_TRADES_PAPER,
    String(totalProduction),
    `>= ${MIN_TRADES_PAPER}`,
    "BLOCKER",
    "SAMPLE",
  );

  add(
    "SAMPLE_DAYS_7",
    "Days of paper data",
    daysOfData >= MIN_DAYS_PAPER,
    daysOfData.toFixed(1),
    `>= ${MIN_DAYS_PAPER} days`,
    "BLOCKER",
    "SAMPLE",
  );

  add(
    "SAMPLE_SIZE_200",
    "Closed trades (full live gate)",
    totalProduction >= MIN_TRADES_FULL_LIVE,
    String(totalProduction),
    `>= ${MIN_TRADES_FULL_LIVE}`,
    "WARN",
    "SAMPLE",
  );

  add(
    "SAMPLE_DAYS_30",
    "Days of data (full live gate)",
    daysOfData >= MIN_DAYS_FULL_LIVE,
    daysOfData.toFixed(1),
    `>= ${MIN_DAYS_FULL_LIVE} days`,
    "WARN",
    "SAMPLE",
  );

  add(
    "EXPECTANCY_POSITIVE",
    "Mean net PnL per trade",
    expectancy > MIN_EXPECTANCY,
    `$${expectancy.toFixed(2)}`,
    `> $${MIN_EXPECTANCY}`,
    "BLOCKER",
    "PERFORMANCE",
  );

  add(
    "WIN_RATE_MIN",
    "Win rate",
    winRate >= MIN_WIN_RATE,
    `${(winRate * 100).toFixed(1)}%`,
    `>= ${(MIN_WIN_RATE * 100).toFixed(0)}%`,
    "BLOCKER",
    "PERFORMANCE",
  );

  add(
    "PROFIT_FACTOR_MIN",
    "Profit factor",
    profitFactor >= MIN_PROFIT_FACTOR_PAPER,
    profitFactor === Infinity ? "∞" : profitFactor.toFixed(2),
    `>= ${MIN_PROFIT_FACTOR_PAPER}`,
    "BLOCKER",
    "PERFORMANCE",
  );

  add(
    "PROFIT_FACTOR_FULL",
    "Profit factor (full live)",
    profitFactor >= MIN_PROFIT_FACTOR_FULL,
    profitFactor === Infinity ? "∞" : profitFactor.toFixed(2),
    `>= ${MIN_PROFIT_FACTOR_FULL}`,
    "WARN",
    "PERFORMANCE",
  );

  add(
    "FEE_RATIO_MAX",
    "Fee / |gross|",
    feePctFraction < MAX_FEE_PCT,
    `${(feePctFraction * 100).toFixed(1)}%`,
    `< ${(MAX_FEE_PCT * 100).toFixed(0)}%`,
    "BLOCKER",
    "PERFORMANCE",
  );

  add(
    "FEE_RATIO_STRICT",
    "Fee / |gross| (strict)",
    feePctFraction < MAX_FEE_PCT_STRICT,
    `${(feePctFraction * 100).toFixed(1)}%`,
    `< ${(MAX_FEE_PCT_STRICT * 100).toFixed(0)}%`,
    "WARN",
    "PERFORMANCE",
  );

  add(
    "MAX_DRAWDOWN",
    "Peak-to-trough drawdown",
    maxDrawdownPct <= MAX_DRAWDOWN_PCT,
    `${maxDrawdownPct.toFixed(1)}%`,
    `<= ${MAX_DRAWDOWN_PCT}%`,
    "BLOCKER",
    "RISK",
  );

  const readinessPass = inputs.readiness?.productionReady === true;
  add(
    "OPS_READINESS",
    "Production readiness (runtime invariants)",
    inputs.readiness == null ? false : readinessPass,
    inputs.readiness == null ? "N/A" : readinessPass ? "READY" : "NOT READY",
    "critical checks pass",
    "BLOCKER",
    "OPS",
  );

  const healthGrade = inputs.health?.grade;
  add(
    "HEALTH_GRADE",
    "Rolling health grade not F",
    healthGrade != null && healthGrade !== "F",
    healthGrade ?? "N/A",
    "A / B / C",
    "WARN",
    "OPS",
  );

  if (inputs.replaySignFlipRate != null && Number.isFinite(inputs.replaySignFlipRate)) {
    add(
      "REPLAY_SIGN_FLIP",
      "Replay sign-flip rate",
      inputs.replaySignFlipRate <= MAX_REPLAY_SIGN_FLIP_RATE,
      `${(inputs.replaySignFlipRate * 100).toFixed(1)}%`,
      `<= ${(MAX_REPLAY_SIGN_FLIP_RATE * 100).toFixed(0)}%`,
      "WARN",
      "REPLAY",
    );
  } else {
    add(
      "REPLAY_SIGN_FLIP",
      "Replay sign-flip rate",
      false,
      "not run",
      `<= ${(MAX_REPLAY_SIGN_FLIP_RATE * 100).toFixed(0)}% (run replay:compare)`,
      "WARN",
      "REPLAY",
    );
  }

  const shadowCount = inputs.shadowIntentCount ?? 0;
  add(
    "SHADOW_INTENTS",
    "Shadow intents logged (if enabled)",
    shadowCount >= 10,
    String(shadowCount),
    ">= 10 when shadow mode on",
    "INFO",
    "OPS",
  );

  const blockers = gates.filter((g) => g.severity === "BLOCKER");
  const warnings = gates.filter((g) => g.severity === "WARN");
  const allBlockersPass = blockers.every((g) => g.pass);
  const score = gates.length ? gates.filter((g) => g.pass).length / gates.length : 0;
  const recommendation = deriveRecommendation(blockers, warnings, totalProduction);

  return {
    gates,
    blockers,
    warnings,
    allBlockersPass,
    score,
    totalProduction,
    daysOfData,
    computedAt: nowMs,
    recommendation,
  };
}
