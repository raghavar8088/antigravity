export type TerminalAlertSeverity = "INFO" | "WARNING" | "CRITICAL";
export type TerminalSide = "LONG" | "SHORT";
export type StrategyHealth = "ACTIVE" | "WATCHLIST" | "DISABLED";

export type TerminalCandle = {
  time: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
};

export type OrderBookLevel = {
  price: number;
  size: number;
  total: number;
};

export type TerminalPosition = {
  id: string;
  symbol: string;
  side: TerminalSide;
  strategy: string;
  entryPrice: number;
  markPrice: number;
  liquidationPrice: number;
  sizeBtc: number;
  unrealizedPnl: number;
  returnPct: number;
  fundingRate: number;
  fundingPnl: number;
  marginUsd: number;
};

export type TerminalRisk = {
  var95Usd: number;
  var99Usd: number;
  cvar95Usd: number;
  heatPct: number;
  drawdownPct: number;
  netExposureUsd: number;
  grossExposureUsd: number;
  marginUsagePct: number;
  longExposureUsd: number;
  shortExposureUsd: number;
  fundingPaidUsd: number;
  fundingReceivedUsd: number;
};

export type StrategyResearchRow = {
  id: number;
  name: string;
  family: string;
  health: StrategyHealth;
  /** Per-strategy Sharpe — null when Mongo strategy_scores has no Sharpe field. */
  sharpe: number | null;
  expectancy: number;
  maxDrawdownPct: number;
  oosProfitFactor: number | null;
  promotionScore: number;
};

export type AnalyticsSnapshot = {
  equityCurve: { time: string; equity: number; btcBenchmark: number }[];
  rollingSharpe30d: number | null;
  rollingSharpe90d: number | null;
  profitFactorTrend: number | null;
  winRatePct: number | null;
  feeDragUsd: number | null;
  rMultipleBuckets: { bucket: string; count: number }[];
};

export type JournalTrade = {
  id: string;
  strategy: string;
  family: string;
  side: "BUY" | "SELL";
  entryTime: string;
  exitTime: string;
  entryPrice: number;
  exitPrice: number;
  netPnl: number;
  rMultiple: number;
  setupTag: string;
  exitReason: string;
  holdingMinutes: number;
};

export type TerminalAlert = {
  id: string;
  severity: TerminalAlertSeverity;
  title: string;
  message: string;
  createdAt: string;
  acknowledged?: boolean;
};

export type TerminalSnapshot = {
  connected: boolean;
  loading: boolean;
  price: number;
  priceChange24hPct: number;
  spreadBps: number;
  fundingRate: number;
  regime: string;
  candles: TerminalCandle[];
  bids: OrderBookLevel[];
  asks: OrderBookLevel[];
  positions: TerminalPosition[];
  risk: TerminalRisk;
  strategies: StrategyResearchRow[];
  analytics: AnalyticsSnapshot;
  journal: JournalTrade[];
  alerts: TerminalAlert[];
  updatedAt: string;
};
