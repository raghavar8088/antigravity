"use client";

import type { MockTrade, MockTradeAnalytics } from "../trading/mockTradingEngine";
import type { StrategyScore } from "./strategyScoringEngine";

/**
 * AI Quant Research Agent
 * Provides AI-assisted strategy analysis and discovery.
 */

export interface AIResearchReport {
  summary: string;
  topStrategies: string[];
  riskAssessment: string;
  recommendations: string[];
}

export class QuantAIAgent {
  public async generateReport(analytics: MockTradeAnalytics, scores: StrategyScore[]): Promise<AIResearchReport> {
    // In a real implementation, this would call an LLM API (like Gemini or GPT)
    // with the analytics data to generate a detailed report.
    
    const topStrats = scores.slice(0, 3).map(s => `#${s.strategyId} ${s.strategyName} (Score: ${s.overallScore.toFixed(1)})`);
    
    return {
      summary: `The portfolio is currently showing ${analytics.totalPnl > 0 ? "positive" : "negative"} momentum with a win rate of ${(analytics.winRate * 100).toFixed(1)}%.`,
      topStrategies: topStrats,
      riskAssessment: analytics.sharpeRatio && analytics.sharpeRatio > 1.5 ? "Risk-adjusted returns are excellent." : "Risk-adjusted returns are sub-optimal; consider reducing exposure.",
      recommendations: [
        "Focus on strategies with high composite scores and sufficient sample size.",
        "Review highly correlated strategies to avoid concentration risk.",
        "Consider adjusting sizing mode to 'risk_pct_equity' for better drawdown management."
      ],
    };
  }

  public async suggestOptimizations(strategyId: number, trades: MockTrade[]): Promise<string[]> {
    const strategyTrades = trades.filter(t => t.strategyId === strategyId);
    if (strategyTrades.length === 0) return ["Insufficient data for optimization."];

    const avgWin = strategyTrades.filter(t => t.realizedPnl > 0).reduce((sum, t) => sum + t.realizedPnl, 0) / strategyTrades.length;
    const avgLoss = Math.abs(strategyTrades.filter(t => t.realizedPnl < 0).reduce((sum, t) => sum + t.realizedPnl, 0)) / strategyTrades.length;

    if (avgWin < avgLoss) {
      return ["Increase take-profit percentage or tighten stop-loss to improve risk/reward ratio."];
    }

    return ["Strategy is performing well; monitor for regime drift."];
  }
}

export const quantAI = new QuantAIAgent();
