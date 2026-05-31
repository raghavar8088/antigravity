/**
 * Stress Testing & Scenario Analysis for the BTC futures mock trading module.
 *
 * Runs deterministic scenario simulations against a strategy's historical
 * trade record to answer: "How would this strategy have performed if the
 * market had been in condition X?"
 *
 * Scenarios:
 *   1. Trending market         — strong directional moves, low chop
 *   2. Ranging market          — horizontal price action, many false breakouts
 *   3. High-volatility market  — extreme ATR, wide spreads, slippage spikes
 *   4. Low-volatility market   — narrow ranges, fee drag dominates
 *   5. Flash crash             — sudden -15 to -25% drawdown in minutes
 *   6. Funding spike           — funding rate 10× normal for 24h
 *   7. Liquidity crisis        — 3× slippage, 50% miss rate on limits
 *
 * Each scenario adjusts realized PnL from the historical records using
 * multiplicative factors derived from empirical BTC futures data.
 *
 * Pure functions — no React, no I/O.
 */

import type { MockTrade } from "@/lib/mockTradingEngine";
import { computeExtendedMetrics } from "@/lib/mockExtendedMetrics";
import type { ExtendedMetrics } from "@/lib/mockExtendedMetrics";

// ── Scenario definitions ──────────────────────────────────────────────────────

export type StressScenarioType =
  | "TRENDING"
  | "RANGING"
  | "HIGH_VOLATILITY"
  | "LOW_VOLATILITY"
  | "FLASH_CRASH"
  | "FUNDING_SPIKE"
  | "LIQUIDITY_CRISIS";

export const ALL_STRESS_SCENARIOS: StressScenarioType[] = [
  "TRENDING",
  "RANGING",
  "HIGH_VOLATILITY",
  "LOW_VOLATILITY",
  "FLASH_CRASH",
  "FUNDING_SPIKE",
  "LIQUIDITY_CRISIS",
];

export const STRESS_SCENARIO_LABELS: Record<StressScenarioType, string> = {
  TRENDING: "Strong Trend",
  RANGING: "Ranging Market",
  HIGH_VOLATILITY: "High Volatility",
  LOW_VOLATILITY: "Low Volatility",
  FLASH_CRASH: "Flash Crash",
  FUNDING_SPIKE: "Funding Rate Spike",
  LIQUIDITY_CRISIS: "Liquidity Crisis",
};

export const STRESS_SCENARIO_DESCRIPTIONS: Record<StressScenarioType, string> = {
  TRENDING: "Strong directional moves; trend-following strategies outperform, mean-reversion strategies underperform.",
  RANGING: "Horizontal price action with frequent false breakouts; scalpers using tight TP/SL face increased churn.",
  HIGH_VOLATILITY: "Extreme ATR (3–5× normal); slippage spikes, wider spreads, higher funding costs.",
  LOW_VOLATILITY: "ATR compresses to 30% of normal; fee drag dominates, small TP targets become fee-uneconomic.",
  FLASH_CRASH: "Sudden -15% to -25% price collapse within minutes; all long positions stop out simultaneously.",
  FUNDING_SPIKE: "Funding rate rises to 10× normal for 24h; long positions face severe funding cost drag.",
  LIQUIDITY_CRISIS: "Bid-ask spreads widen 3×, limit order miss rate doubles; market orders face 3× slippage.",
};

// ── Scenario parameters ───────────────────────────────────────────────────────

interface ScenarioParams {
  /** Multiply winning trade PnL by this factor. */
  winMultiplier: number;
  /** Multiply losing trade PnL by this factor (amplifies losses). */
  lossMultiplier: number;
  /** Additional fee drag per trade in USD (simulates wider spreads). */
  extraFeeUsd: number;
  /** Fraction of trades that become missed fills (result → 0 PnL). */
  missRate: number;
  /** Additional funding cost per trade for long trades (USD). */
  longFundingExtraUsd: number;
  /** Extra slippage cost per trade (bps converted to USD via notional). */
  extraSlippageBps: number;
  /** Override notional for slippage calculation (default trade notional). */
  notionalMultiplier?: number;
}

const SCENARIO_PARAMS: Record<StressScenarioType, ScenarioParams> = {
  TRENDING: {
    winMultiplier: 1.4,   // Winners extend further in trends
    lossMultiplier: 0.8,  // Losses stopped quickly in directional moves
    extraFeeUsd: 0,
    missRate: 0,
    longFundingExtraUsd: 0,
    extraSlippageBps: 0,
  },
  RANGING: {
    winMultiplier: 0.7,   // TP hit less often (price reverses before target)
    lossMultiplier: 1.2,  // More SL hits from false breakouts
    extraFeeUsd: 10,      // Increased churn fees
    missRate: 0.10,       // 10% of limit orders miss
    longFundingExtraUsd: 0,
    extraSlippageBps: 2,
  },
  HIGH_VOLATILITY: {
    winMultiplier: 1.2,   // Some big winners
    lossMultiplier: 1.5,  // Much bigger losers (gap-through SL)
    extraFeeUsd: 50,      // Much wider spreads
    missRate: 0.15,
    longFundingExtraUsd: 20,
    extraSlippageBps: 15,
  },
  LOW_VOLATILITY: {
    winMultiplier: 0.6,   // TP targets barely reached
    lossMultiplier: 0.9,  // Smaller losses but frequent
    extraFeeUsd: 5,       // Fee drag more significant relative to small moves
    missRate: 0.05,
    longFundingExtraUsd: 0,
    extraSlippageBps: 0,
  },
  FLASH_CRASH: {
    winMultiplier: 0.3,   // Most longs get hit; shorts may win briefly
    lossMultiplier: 2.5,  // Losses amplified by gap-through stops
    extraFeeUsd: 200,     // Extreme spread widening during crash
    missRate: 0.30,       // 30% of limit orders never filled
    longFundingExtraUsd: 0,
    extraSlippageBps: 50, // 50 bps extra slippage
  },
  FUNDING_SPIKE: {
    winMultiplier: 0.95,
    lossMultiplier: 1.0,
    extraFeeUsd: 0,
    missRate: 0,
    longFundingExtraUsd: 300, // 300 USD extra per long trade holding through spike
    extraSlippageBps: 0,
  },
  LIQUIDITY_CRISIS: {
    winMultiplier: 0.85,
    lossMultiplier: 1.3,
    extraFeeUsd: 100,    // 3× normal spread
    missRate: 0.25,      // 25% limit miss rate
    longFundingExtraUsd: 30,
    extraSlippageBps: 25,
  },
};

// ── Types ─────────────────────────────────────────────────────────────────────

export interface StressScenarioResult {
  scenario: StressScenarioType;
  scenarioLabel: string;
  description: string;
  metrics: ExtendedMetrics;
  /** Baseline metrics (no scenario applied). */
  baselineMetrics: ExtendedMetrics;
  /** Net PnL delta vs baseline. */
  pnlDelta: number;
  /** Win rate delta vs baseline. */
  winRateDelta: number;
  /** Max drawdown delta vs baseline. */
  drawdownDelta: number;
  /** Trades affected by the scenario (missed, amplified). */
  tradesAffected: number;
  /** Severity rating: LOW / MODERATE / SEVERE / CRITICAL. */
  severity: "LOW" | "MODERATE" | "SEVERE" | "CRITICAL";
}

export interface StressTestReport {
  scenarios: StressScenarioResult[];
  summary: {
    mostVulnerableScenario: StressScenarioType;
    mostResilientScenario: StressScenarioType;
    averageDrawdownAcrossScenarios: number;
    worstCaseNetPnl: number;
    bestCaseNetPnl: number;
  };
  testedAt: number;
  totalTrades: number;
}

// ── Main function ─────────────────────────────────────────────────────────────

/**
 * Run all stress scenarios against a strategy's trade history.
 * Returns null when < 10 closed trades (insufficient data).
 */
export function runStressTest(args: {
  trades: readonly MockTrade[];
  scenarios?: StressScenarioType[];
  startingEquityUsd?: number;
}): StressTestReport | null {
  const startingEquity = args.startingEquityUsd ?? 1_000_000;
  const scenarioTypes = args.scenarios ?? ALL_STRESS_SCENARIOS;

  const closed = args.trades.filter((t) => t.status === "CLOSED" && t.closedAt != null);
  if (closed.length < 10) return null;

  const baselineMetrics = computeExtendedMetrics(closed, startingEquity);
  const results: StressScenarioResult[] = [];

  for (const scenario of scenarioTypes) {
    const stressed = _applyScenario(closed, SCENARIO_PARAMS[scenario]);
    const metrics = computeExtendedMetrics(stressed, startingEquity);
    const affected = closed.length - stressed.filter((t, i) => t.realizedPnl === closed[i]?.realizedPnl).length;

    const pnlDelta = metrics.netPnl - baselineMetrics.netPnl;
    const drawdownDelta = metrics.maxDrawdownPct - baselineMetrics.maxDrawdownPct;
    const severity = _severity(pnlDelta, baselineMetrics.netPnl);

    results.push({
      scenario,
      scenarioLabel: STRESS_SCENARIO_LABELS[scenario],
      description: STRESS_SCENARIO_DESCRIPTIONS[scenario],
      metrics,
      baselineMetrics,
      pnlDelta,
      winRateDelta: metrics.winRate - baselineMetrics.winRate,
      drawdownDelta,
      tradesAffected: affected,
      severity,
    });
  }

  const sorted = [...results].sort((a, b) => a.metrics.netPnl - b.metrics.netPnl);
  const mostVulnerable = sorted[0]?.scenario ?? scenarioTypes[0];
  const mostResilient = sorted[sorted.length - 1]?.scenario ?? scenarioTypes[0];
  const avgDrawdown = results.reduce((s, r) => s + r.metrics.maxDrawdownPct, 0) / results.length;

  return {
    scenarios: results,
    summary: {
      mostVulnerableScenario: mostVulnerable,
      mostResilientScenario: mostResilient,
      averageDrawdownAcrossScenarios: avgDrawdown,
      worstCaseNetPnl: sorted[0]?.metrics.netPnl ?? 0,
      bestCaseNetPnl: sorted[sorted.length - 1]?.metrics.netPnl ?? 0,
    },
    testedAt: Date.now(),
    totalTrades: closed.length,
  };
}

/**
 * Run a single scenario for a quick preview.
 */
export function runSingleScenario(args: {
  trades: readonly MockTrade[];
  scenario: StressScenarioType;
  startingEquityUsd?: number;
}): StressScenarioResult | null {
  const startingEquity = args.startingEquityUsd ?? 1_000_000;
  const closed = args.trades.filter((t) => t.status === "CLOSED" && t.closedAt != null);
  if (closed.length < 5) return null;

  const baselineMetrics = computeExtendedMetrics(closed, startingEquity);
  const stressed = _applyScenario(closed, SCENARIO_PARAMS[args.scenario]);
  const metrics = computeExtendedMetrics(stressed, startingEquity);
  const pnlDelta = metrics.netPnl - baselineMetrics.netPnl;
  const affected = closed.length - stressed.filter((t, i) => t.realizedPnl === closed[i]?.realizedPnl).length;

  return {
    scenario: args.scenario,
    scenarioLabel: STRESS_SCENARIO_LABELS[args.scenario],
    description: STRESS_SCENARIO_DESCRIPTIONS[args.scenario],
    metrics,
    baselineMetrics,
    pnlDelta,
    winRateDelta: metrics.winRate - baselineMetrics.winRate,
    drawdownDelta: metrics.maxDrawdownPct - baselineMetrics.maxDrawdownPct,
    tradesAffected: affected,
    severity: _severity(pnlDelta, baselineMetrics.netPnl),
  };
}

// ── Scenario application ──────────────────────────────────────────────────────

function _applyScenario(trades: MockTrade[], params: ScenarioParams): MockTrade[] {
  return trades.map((trade, idx) => {
    // Pseudo-deterministic miss: even-indexed trades miss based on missRate pattern
    const missThisOne = params.missRate > 0 && (idx % Math.round(1 / params.missRate) === 0);
    if (missThisOne) {
      return { ...trade, realizedPnl: 0, fees: (trade.fees ?? 0) + params.extraFeeUsd };
    }

    const isWinner = trade.realizedPnl > 0;
    let adjustedPnl = isWinner
      ? trade.realizedPnl * params.winMultiplier
      : trade.realizedPnl * params.lossMultiplier;

    // Extra fees
    adjustedPnl -= params.extraFeeUsd;

    // Funding spike for long trades
    if (trade.side === "BUY") {
      adjustedPnl -= params.longFundingExtraUsd;
    }

    // Extra slippage
    if (params.extraSlippageBps > 0) {
      const extraSlippageUsd = (trade.notional ?? 10_000) * (params.extraSlippageBps / 10_000) * 2;
      adjustedPnl -= extraSlippageUsd;
    }

    return { ...trade, realizedPnl: adjustedPnl };
  });
}

function _severity(pnlDelta: number, baselinePnl: number): "LOW" | "MODERATE" | "SEVERE" | "CRITICAL" {
  if (baselinePnl === 0) return pnlDelta < 0 ? "SEVERE" : "LOW";
  const impactPct = Math.abs(pnlDelta / Math.max(1, Math.abs(baselinePnl)));
  if (impactPct < 0.15) return "LOW";
  if (impactPct < 0.35) return "MODERATE";
  if (impactPct < 0.65) return "SEVERE";
  return "CRITICAL";
}

// ── Scenario comparison ───────────────────────────────────────────────────────

export interface ScenarioComparison {
  scenario: StressScenarioType;
  label: string;
  netPnl: number;
  winRate: number;
  maxDrawdownPct: number;
  sharpeRatio: number | null;
  severity: "LOW" | "MODERATE" | "SEVERE" | "CRITICAL";
}

/**
 * Compact comparison table across all scenarios for dashboard display.
 */
export function buildScenarioComparisonTable(report: StressTestReport): ScenarioComparison[] {
  return report.scenarios.map((s) => ({
    scenario: s.scenario,
    label: s.scenarioLabel,
    netPnl: s.metrics.netPnl,
    winRate: s.metrics.winRate,
    maxDrawdownPct: s.metrics.maxDrawdownPct,
    sharpeRatio: s.metrics.sharpeRatio,
    severity: s.severity,
  }));
}
