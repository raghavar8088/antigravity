/** Closed futures paper trade (shared by engine, UI, Supabase sync). */

export type FuturesTradeSide = "LONG" | "SHORT";

export type FuturesTradeExitReason =
  | "TP"
  | "SL"
  | "TIME"
  | "TRAIL"
  | "BREAKEVEN"
  | "LIQUIDATION_RISK"
  | "PROFIT_LOCK";

export interface BTCFuturesTrade {
  id: string;
  symbol: string;
  strategyId: number;
  strategyName: string;
  side: FuturesTradeSide;
  entryPrice: number;
  exitPrice: number;
  contracts: number;
  notional: number;
  marginUsed: number;
  realizedPnl: number;
  fees: number;
  netPnl: number;
  netPnlPct: number;
  priceMovePct: number;
  fundingCosts: number;
  lastFundingAppliedAt?: number;
  fundingSinceOpenMs?: number;
  openedAt: string;
  closedAt: string;
  exitReason: FuturesTradeExitReason;
  liquidationPrice: number;
  liquidationDistancePct: number;
  /** Stable id for Supabase upsert / sync queue (set at close). */
  clientTradeId?: string;
  /**
   * Signal feature contributions at entry — used for offline weight calibration.
   * Populated when the strategy has a btcFtTemplate; omitted on simple signal-key strats.
   * Schema: [{ reason: "HTF bull stack", pts: 18 }, ...]
   */
  signalContributions?: Array<{ reason: string; pts: number }>;
}
