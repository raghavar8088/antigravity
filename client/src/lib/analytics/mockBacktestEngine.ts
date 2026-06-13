"use client";

import type { MockTrade } from "../trading/mockTradingEngine";

/**
 * Monte Carlo Simulation
 * Randomly shuffles trade outcomes to simulate equity curve variations.
 */
export function runMonteCarlo(trades: readonly MockTrade[], iterations = 1000, sampleSize = 100) {
  const closedTrades = trades.filter(t => t.status === "CLOSED");
  if (closedTrades.length === 0) return null;

  const results: number[][] = [];
  for (let i = 0; i < iterations; i++) {
    let equity = 0;
    const curve: number[] = [0];
    for (let j = 0; j < sampleSize; j++) {
      const randomTrade = closedTrades[Math.floor(Math.random() * closedTrades.length)];
      equity += randomTrade.realizedPnl;
      curve.push(equity);
    }
    results.push(curve);
  }

  // Calculate percentiles
  const finalEquities = results.map(r => r[r.length - 1]).sort((a, b) => a - b);
  return {
    results,
    p10: finalEquities[Math.floor(iterations * 0.1)],
    p50: finalEquities[Math.floor(iterations * 0.5)],
    p90: finalEquities[Math.floor(iterations * 0.9)],
  };
}

/**
 * Walk-Forward Validation
 * Splits data into training and testing sets to validate strategy robustness.
 */
export function runWalkForward(trades: readonly MockTrade[], folds = 5) {
  const closedTrades = [...trades]
    .filter(t => t.status === "CLOSED")
    .sort((a, b) => (a.closedAt ?? 0) - (b.closedAt ?? 0));

  if (closedTrades.length < folds * 2) return null;

  const foldSize = Math.floor(closedTrades.length / folds);
  const results: { trainPnl: number; testPnl: number }[] = [];

  for (let i = 0; i < folds - 1; i++) {
    const train = closedTrades.slice(0, (i + 1) * foldSize);
    const test = closedTrades.slice((i + 1) * foldSize, (i + 2) * foldSize);

    results.push({
      trainPnl: train.reduce((sum, t) => sum + t.realizedPnl, 0),
      testPnl: test.reduce((sum, t) => sum + t.realizedPnl, 0),
    });
  }

  return results;
}
