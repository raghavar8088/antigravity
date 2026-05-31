import type { TerminalSnapshot } from "./terminalTypes";

const now = Date.now();

function candle(i: number) {
  const base = 104_200 + Math.sin(i / 4) * 520 + i * 18;
  return {
    time: now - (90 - i) * 60_000,
    open: base - 45,
    high: base + 130,
    low: base - 150,
    close: base + Math.sin(i / 2) * 90,
    volume: 18 + (i % 8) * 3.2,
  };
}

export const initialTerminalSnapshot: TerminalSnapshot = {
  connected: true,
  price: 105_842.5,
  priceChange24hPct: 1.42,
  spreadBps: 1.8,
  fundingRate: 0.00018,
  regime: "Trending Bull",
  candles: Array.from({ length: 90 }, (_, i) => candle(i)),
  bids: Array.from({ length: 18 }, (_, i) => ({
    price: 105_842.5 - (i + 1) * 4.5,
    size: 2.4 + i * 0.31,
    total: 2.4 + i * 1.84,
  })),
  asks: Array.from({ length: 18 }, (_, i) => ({
    price: 105_842.5 + (i + 1) * 4.5,
    size: 2.1 + i * 0.28,
    total: 2.1 + i * 1.72,
  })),
  positions: [
    {
      id: "BTC-PERP-1042",
      symbol: "BTCUSD",
      side: "LONG",
      strategy: "Funding Mean Reversion Alpha",
      entryPrice: 104_920,
      markPrice: 105_842.5,
      liquidationPrice: 98_610,
      sizeBtc: 0.18,
      unrealizedPnl: 166.05,
      returnPct: 0.88,
      fundingRate: 0.00018,
      fundingPnl: -3.12,
      marginUsd: 1_889.6,
    },
    {
      id: "BTC-PERP-1048",
      symbol: "BTCUSD",
      side: "SHORT",
      strategy: "CVD Divergence Alpha",
      entryPrice: 106_210,
      markPrice: 105_842.5,
      liquidationPrice: 112_880,
      sizeBtc: 0.11,
      unrealizedPnl: 40.42,
      returnPct: 0.35,
      fundingRate: -0.00018,
      fundingPnl: 1.52,
      marginUsd: 1_164.27,
    },
  ],
  risk: {
    var95Usd: 1_840,
    var99Usd: 2_760,
    cvar95Usd: 2_380,
    heatPct: 3.7,
    drawdownPct: 1.4,
    netExposureUsd: 7_210,
    grossExposureUsd: 30_720,
    marginUsagePct: 18.6,
    longExposureUsd: 19_052,
    shortExposureUsd: 11_668,
    fundingPaidUsd: 82.14,
    fundingReceivedUsd: 37.92,
  },
  strategies: [
    { id: 201, name: "Funding Mean Reversion Alpha", family: "Funding", health: "ACTIVE", sharpe: 2.18, expectancy: 32.4, maxDrawdownPct: 4.1, oosProfitFactor: 1.64, promotionScore: 86 },
    { id: 202, name: "CVD Divergence Alpha", family: "Order Flow", health: "ACTIVE", sharpe: 1.92, expectancy: 24.7, maxDrawdownPct: 5.2, oosProfitFactor: 1.42, promotionScore: 79 },
    { id: 203, name: "Liquidity Sweep Alpha", family: "Smart Money", health: "WATCHLIST", sharpe: 1.26, expectancy: 12.1, maxDrawdownPct: 7.8, oosProfitFactor: 1.09, promotionScore: 63 },
    { id: 204, name: "Volume Profile LVN Breakout", family: "Volume Profile", health: "ACTIVE", sharpe: 1.71, expectancy: 19.6, maxDrawdownPct: 5.9, oosProfitFactor: 1.31, promotionScore: 73 },
    { id: 205, name: "MSS Continuation Alpha", family: "MSS", health: "DISABLED", sharpe: 0.72, expectancy: -4.3, maxDrawdownPct: 11.5, oosProfitFactor: 0.84, promotionScore: 38 },
  ],
  analytics: {
    equityCurve: [
      { time: "09:15", equity: 100_000, btcBenchmark: 100_000 },
      { time: "10:15", equity: 100_380, btcBenchmark: 100_220 },
      { time: "11:15", equity: 100_210, btcBenchmark: 99_940 },
      { time: "12:15", equity: 100_780, btcBenchmark: 100_520 },
      { time: "13:15", equity: 101_120, btcBenchmark: 100_840 },
      { time: "14:15", equity: 101_330, btcBenchmark: 100_910 },
    ],
    rollingSharpe30d: 1.84,
    rollingSharpe90d: 1.42,
    profitFactorTrend: 1.58,
    winRatePct: 54.2,
    feeDragUsd: 214.8,
    rMultipleBuckets: [
      { bucket: "<-1R", count: 4 },
      { bucket: "-1R..0", count: 18 },
      { bucket: "0..1R", count: 26 },
      { bucket: "1R..2R", count: 19 },
      { bucket: ">2R", count: 7 },
    ],
  },
  journal: [
    { id: "T-8831", strategy: "Funding Mean Reversion Alpha", family: "Funding", side: "BUY", entryTime: "10:12", exitTime: "10:27", entryPrice: 104_820, exitPrice: 105_180, netPnl: 64.8, rMultiple: 1.7, setupTag: "funding-zscore", exitReason: "TAKE_PROFIT", holdingMinutes: 15 },
    { id: "T-8838", strategy: "Liquidity Sweep Alpha", family: "Smart Money", side: "SELL", entryTime: "11:04", exitTime: "11:18", entryPrice: 105_620, exitPrice: 105_440, netPnl: 27.9, rMultiple: 0.9, setupTag: "equal-high-sweep", exitReason: "TRAILING_STOP", holdingMinutes: 14 },
    { id: "T-8842", strategy: "CVD Divergence Alpha", family: "Order Flow", side: "BUY", entryTime: "12:36", exitTime: "12:44", entryPrice: 105_240, exitPrice: 105_120, netPnl: -19.4, rMultiple: -0.7, setupTag: "bullish-absorption", exitReason: "STOP_LOSS", holdingMinutes: 8 },
  ],
  alerts: [
    { id: "A-1", severity: "CRITICAL", title: "Drawdown Guard", message: "Heat remains below block threshold; monitor if BTC volatility expands.", createdAt: "14:22" },
    { id: "A-2", severity: "WARNING", title: "Funding Spike", message: "Funding rate above session median, long carry drag increasing.", createdAt: "14:18" },
    { id: "A-3", severity: "INFO", title: "Trade Opened", message: "Funding Mean Reversion Alpha opened BTCUSD long.", createdAt: "14:05" },
  ],
  updatedAt: new Date(now).toISOString(),
};
