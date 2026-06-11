import type {
  AnalyticsSnapshot,
  JournalTrade,
  StrategyHealth,
  StrategyResearchRow,
  TerminalAlert,
  TerminalDelta,
  TerminalPosition,
  TerminalRisk,
} from "./terminalTypes";

export type PaperDeskSnapshotPayload = {
  server_time?: string;
  state?: Record<string, unknown>;
  open_positions?: Array<Record<string, unknown>>;
  recent_trades?: Array<Record<string, unknown>>;
  health_summary?: Record<string, unknown>;
  portfolio?: Record<string, unknown>;
};

export type StrategyIntelRow = {
  strategy_id: string;
  status: string;
  enabled: boolean;
  total_pnl: number;
  expectancy: number;
  profit_factor: number;
  win_rate: number;
  max_drawdown: number;
  sample_size: number;
  evidence_score: number;
  allocation_tier: string;
  health_reasons?: string[];
};

export type BtcPricePayload = {
  price?: number;
  change24h?: number;
};

export type EquityCurvePayload = {
  curve?: Array<{ snapped_at?: string; ts?: string; equity?: number; drawdown_pct?: number }>;
};

function num(v: unknown, fallback = 0): number {
  const n = Number(v);
  return Number.isFinite(n) ? n : fallback;
}

function healthFromStatus(status: string): StrategyHealth {
  if (status === "HEALTHY") return "ACTIVE";
  if (status === "WARNING" || status === "INSUFFICIENT_DATA") return "WATCHLIST";
  return "DISABLED";
}

function strategyFamily(id: string): string {
  const prefix = id.split(/[_\d]/)[0] ?? id;
  return prefix || "Unknown";
}

export function mapPositions(rows: Array<Record<string, unknown>> | undefined): TerminalPosition[] {
  if (!rows?.length) return [];
  return rows.map((p) => ({
    id: String(p.position_id ?? p._id ?? ""),
    symbol: String(p.symbol ?? "BTCUSD"),
    side: (String(p.side ?? "LONG").toUpperCase() === "SHORT" ? "SHORT" : "LONG") as "LONG" | "SHORT",
    strategy: String(p.strategy_id ?? p.strategy ?? ""),
    entryPrice: num(p.entry_price),
    markPrice: num(p.mark_price, num(p.entry_price)),
    liquidationPrice: num(p.liquidation_price),
    sizeBtc: num(p.size_btc, num(p.size)),
    unrealizedPnl: num(p.unrealized_pnl),
    returnPct: num(p.return_pct),
    fundingRate: num(p.funding_rate),
    fundingPnl: num(p.funding_pnl),
    marginUsd: num(p.margin_usd),
  }));
}

export function mapJournal(rows: Array<Record<string, unknown>> | undefined): JournalTrade[] {
  if (!rows?.length) return [];
  return rows.slice(0, 50).map((t) => ({
    id: String(t.trade_id ?? t.client_trade_id ?? t._id ?? ""),
    strategy: String(t.strategy_id ?? t.strategy ?? ""),
    family: String(t.family ?? strategyFamily(String(t.strategy_id ?? ""))),
    side: (String(t.side ?? "BUY").toUpperCase() === "SELL" ? "SELL" : "BUY") as "BUY" | "SELL",
    entryTime: String(t.entry_at ?? t.entry_time ?? t.opened_at ?? ""),
    exitTime: String(t.exit_at ?? t.exit_time ?? t.closed_at ?? ""),
    entryPrice: num(t.entry_price),
    exitPrice: num(t.exit_price),
    netPnl: num(t.net_pnl),
    rMultiple: num(t.r_multiple),
    setupTag: String(t.setup_tag ?? t.exit_reason ?? ""),
    exitReason: String(t.exit_reason ?? ""),
    holdingMinutes: num(t.holding_minutes),
  }));
}

export function mapStrategies(rows: StrategyIntelRow[] | undefined): StrategyResearchRow[] {
  if (!rows?.length) return [];
  return rows.map((s, index) => ({
    id: index + 1,
    name: s.strategy_id,
    family: strategyFamily(s.strategy_id),
    health: healthFromStatus(s.status),
    sharpe: s.evidence_score / 50,
    expectancy: s.expectancy,
    maxDrawdownPct: Math.abs(s.max_drawdown) * 100,
    oosProfitFactor: s.profit_factor,
    promotionScore: s.evidence_score,
  }));
}

export function mapRisk(
  state: Record<string, unknown> | undefined,
  portfolio: Record<string, unknown> | undefined,
  markPrice: number,
): TerminalRisk {
  const exposure = (portfolio?.exposure ?? {}) as Record<string, unknown>;
  const drawdown = (portfolio?.drawdown ?? {}) as Record<string, unknown>;
  const grossBtc = num(exposure.gross_exposure_btc, num(state?.total_exposure_btc));
  const netBtc = num(exposure.net_exposure_btc);
  const exposureUsd = num(exposure.exposure_usd, grossBtc * markPrice);
  const longUsd = num(exposure.long_exposure_btc) * markPrice;
  const shortUsd = num(exposure.short_exposure_btc) * markPrice;
  const equity = num(state?.equity, num(state?.balance));
  const marginUsage = equity > 0 ? Math.min(100, (exposureUsd / equity) * 100) : 0;

  return {
    var95Usd: num(portfolio?.var_95_usd),
    var99Usd: num(portfolio?.var_99_usd),
    cvar95Usd: num(portfolio?.cvar_95_usd),
    heatPct: marginUsage,
    drawdownPct: num(drawdown.current_drawdown, num(state?.current_drawdown)) * 100,
    netExposureUsd: netBtc * markPrice,
    grossExposureUsd: exposureUsd,
    marginUsagePct: marginUsage,
    longExposureUsd: longUsd,
    shortExposureUsd: shortUsd,
    fundingPaidUsd: num(state?.total_fees),
    fundingReceivedUsd: 0,
  };
}

export function mapAnalytics(
  state: Record<string, unknown> | undefined,
  portfolio: Record<string, unknown> | undefined,
  equityPayload: EquityCurvePayload | undefined,
): AnalyticsSnapshot {
  const curve = equityPayload?.curve ?? [];
  const equityCurve = curve.slice(-12).map((pt) => ({
    time: String(pt.snapped_at ?? pt.ts ?? "").slice(11, 16) || "—",
    equity: num(pt.equity),
    btcBenchmark: num(pt.equity),
  }));

  return {
    equityCurve,
    rollingSharpe30d: num(portfolio?.sharpe),
    rollingSharpe90d: num(portfolio?.sharpe),
    profitFactorTrend: num(portfolio?.profit_factor, num(state?.profit_factor)),
    winRatePct: num(state?.win_rate, num(portfolio?.win_rate)) * 100,
    feeDragUsd: num(state?.total_fees, num(portfolio?.total_fees)),
    rMultipleBuckets: [],
  };
}

export function mapAlerts(
  healthSummary: Record<string, unknown> | undefined,
  state: Record<string, unknown> | undefined,
): TerminalAlert[] {
  const alerts: TerminalAlert[] = [];
  const critical = num(healthSummary?.critical);
  const warning = num(healthSummary?.warning);
  const dd = num(state?.current_drawdown);

  if (critical > 0) {
    alerts.push({
      id: "health-critical",
      severity: "CRITICAL",
      title: "Strategy Health Critical",
      message: `${critical} strategies in CRITICAL state — review retirement candidates.`,
      createdAt: new Date().toISOString(),
    });
  }
  if (warning > 0) {
    alerts.push({
      id: "health-warning",
      severity: "WARNING",
      title: "Strategy Health Warning",
      message: `${warning} strategies on watchlist.`,
      createdAt: new Date().toISOString(),
    });
  }
  if (dd < -0.03) {
    alerts.push({
      id: "drawdown-critical",
      severity: "CRITICAL",
      title: "Drawdown Limit Breach",
      message: `Current drawdown ${(dd * 100).toFixed(2)}% exceeds -3% operator threshold.`,
      createdAt: String(state?.snapped_at ?? new Date().toISOString()),
    });
  }

  return alerts;
}

export function mapSnapshotToTerminalDelta(input: {
  snapshot: PaperDeskSnapshotPayload;
  strategies?: StrategyIntelRow[];
  btcPrice?: BtcPricePayload;
  equity?: EquityCurvePayload;
}): TerminalDelta {
  const { snapshot, strategies, btcPrice, equity } = input;
  const state = snapshot.state;
  const portfolio = snapshot.portfolio;
  const markPrice = num(btcPrice?.price);

  const delta: TerminalDelta = {
    connected: false,
    loading: false,
    price: markPrice,
    priceChange24hPct: num(btcPrice?.change24h),
    spreadBps: 0,
    fundingRate: 0,
    regime: portfolio?.regime ? String(portfolio.regime) : "",
    candles: [],
    bids: [],
    asks: [],
    positions: mapPositions(snapshot.open_positions),
    risk: mapRisk(state, portfolio, markPrice),
    strategies: mapStrategies(strategies),
    analytics: mapAnalytics(state, portfolio, equity),
    journal: mapJournal(snapshot.recent_trades),
    alerts: mapAlerts(snapshot.health_summary, state),
    updatedAt: snapshot.server_time ?? new Date().toISOString(),
  };

  if (!delta.positions?.length && snapshot.open_positions?.length === 0) {
    delta.positions = [];
  }

  return delta;
}
