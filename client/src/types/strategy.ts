/** TypeScript interfaces for the Strategy Operations Console (Phase 4). */

export type StrategyStatus = "ACTIVE" | "PAUSED" | "DISQUALIFIED";
export type AlphaTier = "A" | "B" | "C" | "UNRANKED";
export type SessionType = "SESSION_GATED" | "24_7";
export type WinnersOnlyGate = "PASS" | "FAIL" | "PENDING_EVALUATION";

export interface StrategyRow {
  id: string;
  name: string;
  family: string;
  exchange: "BINANCE" | "DELTA" | "COINBASE";
  market: "FUTURES" | "OPTIONS" | "SPOT";
  status: StrategyStatus;
  alphaTier: AlphaTier;
  sessionType: SessionType;
  winnersOnlyGate: WinnersOnlyGate;
  winRatePct: number;
  sharpeRatio: number;
  maxDrawdownPct: number;
  dailyPnl: number;
  sessionPnl: number;
  openPositions: number;
  signalsToday: number;
  lastSignalAt: string | null;
  intradayHistory: number[];   // 7-day daily P&L for sparkline
  winnersOnlyFailReason?: string;
}

export interface StrategyDetail extends StrategyRow {
  sortinoRatio: number;
  calmarRatio: number;
  avgTradeDurationMs: number;
  bestTradePnl: number;
  worstTradePnl: number;
  equityCurve7d: number[];
  equityCurve30d: number[];
  recentTrades: StrategyTrade[];
  recentSignals: StrategySignal[];
  betaToBTC: number;
  betaToBTC: number;
  correlationBucket: string;
  capitalAllocated: number;
  capitalCurrency: "INR" | "USDT";
}

export interface StrategyTrade {
  tradeId: string;
  entryAt: string;
  exitAt: string;
  symbol: string;
  direction: "LONG" | "SHORT";
  entryPrice: number;
  exitPrice: number;
  quantity: number;
  pnl: number;
  returnPct: number;
  exitReason: string;
}

export interface StrategySignal {
  signalId: string;
  generatedAt: string;
  symbol: string;
  action: "ENTER_LONG" | "ENTER_SHORT" | "EXIT" | "SKIP";
  indicator: string;
  explanation: string;
}

export interface AlphaTierSummary {
  tier: AlphaTier;
  count: number;
  capitalAllocated: number;
  currency: "INR" | "USDT";
}

export interface StrategyOverride {
  action: "WINNERS_ONLY_OVERRIDE";
  strategyId: string;
  adminUser: string;
  timestamp: string;
  reason: string;
}
