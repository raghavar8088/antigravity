"use client";

import type { MockTrade, MockAccountState } from "./mockTradingEngine";

/**
 * Institutional Risk Engine
 * Advanced risk metrics and stress testing.
 */

export interface RiskMetrics {
  var95: number; // 95% Value at Risk
  expectedShortfall: number;
  riskConcentration: Record<string, number>; // by family
  leverageRatio: number;
}

export function computeRiskMetrics(trades: readonly MockTrade[], account: MockAccountState): RiskMetrics {
  const closedPnls = trades.filter(t => t.status === "CLOSED").map(t => t.realizedPnl).sort((a, b) => a - b);
  
  // 95% VaR (Value at Risk)
  const varIndex = Math.floor(closedPnls.length * 0.05);
  const var95 = closedPnls.length > 0 ? Math.abs(closedPnls[varIndex]) : 0;

  // Expected Shortfall (Average loss beyond VaR)
  const shortfallPnls = closedPnls.slice(0, varIndex + 1);
  const expectedShortfall = shortfallPnls.length > 0 
    ? Math.abs(shortfallPnls.reduce((sum, p) => sum + p, 0) / shortfallPnls.length)
    : 0;

  // Risk Concentration by Family
  const concentration: Record<string, number> = {};
  const openTrades = trades.filter(t => t.status === "OPEN");
  const totalNotional = openTrades.reduce((sum, t) => sum + t.notional, 0);

  for (const trade of openTrades) {
    const family = trade.strategyFamily ?? "Unknown";
    concentration[family] = (concentration[family] ?? 0) + (trade.notional / totalNotional);
  }

  return {
    var95,
    expectedShortfall,
    riskConcentration: concentration,
    leverageRatio: account.equity > 0 ? account.exposure / account.equity : 0,
  };
}

/**
 * Stress Test
 * Simulates extreme price moves and calculates potential equity impact.
 */
export function runStressTest(trades: readonly MockTrade[], account: MockAccountState, movePct: number) {
  const openTrades = trades.filter(t => t.status === "OPEN");
  let totalImpact = 0;

  for (const trade of openTrades) {
    const sideMult = trade.side === "BUY" ? 1 : -1;
    const impact = trade.notional * (movePct / 100) * sideMult;
    totalImpact += impact;
  }

  return {
    movePct,
    equityImpact: totalImpact,
    newEquity: account.equity + totalImpact,
    isLiquidated: (account.equity + totalImpact) <= (account.marginUsed * 0.5), // 50% maintenance margin
  };
}
