package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ─────────────────────────────────────────────
// Business / Portfolio Metrics
// ─────────────────────────────────────────────

var (
	// PortfolioAUM is the current Assets Under Management in USD.
	PortfolioAUM = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "portfolio",
		Name:      "aum_usd",
		Help:      "Current portfolio Assets Under Management in USD.",
	}, []string{"account"})

	// PortfolioPnL is the current unrealised + realised PnL in USD.
	PortfolioPnL = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "portfolio",
		Name:      "pnl_usd",
		Help:      "Current portfolio PnL (unrealised + realised) in USD.",
	}, []string{"account", "type"})

	// PortfolioDrawdown is the current drawdown from the high-water mark.
	PortfolioDrawdown = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "portfolio",
		Name:      "drawdown_pct",
		Help:      "Current drawdown from high-water mark as a percentage.",
	}, []string{"account"})

	// DailyPnL is today's realised PnL in USD.
	DailyPnL = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "portfolio",
		Name:      "daily_pnl_usd",
		Help:      "Today's realised PnL in USD.",
	}, []string{"account"})

	// PortfolioExposure is total notional exposure in USD.
	PortfolioExposure = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "portfolio",
		Name:      "exposure_usd",
		Help:      "Total notional portfolio exposure in USD.",
	}, []string{"account", "direction"})

	// OpenPositions is the current count of open positions.
	OpenPositions = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "portfolio",
		Name:      "open_positions",
		Help:      "Current number of open positions.",
	}, []string{"account", "exchange"})

	// HighWaterMark is the all-time peak AUM.
	HighWaterMark = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "portfolio",
		Name:      "high_water_mark_usd",
		Help:      "All-time peak AUM (high-water mark) in USD.",
	}, []string{"account"})
)

// ─────────────────────────────────────────────
// Strategy Performance Metrics
// ─────────────────────────────────────────────

var (
	// StrategyPnL is the per-strategy realised PnL.
	StrategyPnL = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "strategy_perf",
		Name:      "pnl_usd",
		Help:      "Per-strategy realised PnL in USD.",
	}, []string{"account", "strategy"})

	// StrategyWins counts winning trades per strategy.
	StrategyWins = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "strategy_perf",
		Name:      "wins_total",
		Help:      "Total winning trades per strategy.",
	}, []string{"account", "strategy"})

	// StrategyLosses counts losing trades per strategy.
	StrategyLosses = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "strategy_perf",
		Name:      "losses_total",
		Help:      "Total losing trades per strategy.",
	}, []string{"account", "strategy"})

	// SharpeRatio is the rolling 30-day Sharpe ratio per account.
	SharpeRatio = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "strategy_perf",
		Name:      "sharpe_ratio",
		Help:      "Rolling 30-day Sharpe ratio (annualised). Target: >1.5.",
	}, []string{"account"})

	// SortinoRatio is the rolling 30-day Sortino ratio per account.
	SortinoRatio = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "strategy_perf",
		Name:      "sortino_ratio",
		Help:      "Rolling 30-day Sortino ratio (annualised).",
	}, []string{"account"})

	// ProfitFactor is gross profit / gross loss over rolling 30 days.
	ProfitFactor = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "strategy_perf",
		Name:      "profit_factor",
		Help:      "Gross profit / gross loss (rolling 30 days). Target: >1.3.",
	}, []string{"account"})

	// CapitalAllocated is the current capital allocation per strategy family.
	CapitalAllocated = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "strategy_perf",
		Name:      "capital_allocated_usd",
		Help:      "Current capital allocated to each strategy family in USD.",
	}, []string{"account", "family"})

	// RiskUtilisation is the percentage of risk budget consumed.
	RiskUtilisation = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "strategy_perf",
		Name:      "risk_utilisation_pct",
		Help:      "Risk budget utilisation as a percentage of maximum allowed.",
	}, []string{"account"})
)

// ─────────────────────────────────────────────
// Execution Cost Metrics
// ─────────────────────────────────────────────

var (
	// FundingCostDaily is the daily funding fee paid in USD.
	FundingCostDaily = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "costs",
		Name:      "funding_usd_daily",
		Help:      "Daily funding fees paid in USD per exchange.",
	}, []string{"account", "exchange"})

	// SlippageCostTotal is the cumulative slippage cost in USD.
	SlippageCostTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "costs",
		Name:      "slippage_usd_total",
		Help:      "Cumulative slippage cost in USD per exchange/symbol.",
	}, []string{"account", "exchange", "symbol"})

	// CommissionCostTotal is the cumulative commission paid in USD.
	CommissionCostTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "costs",
		Name:      "commission_usd_total",
		Help:      "Cumulative commission paid in USD per exchange.",
	}, []string{"account", "exchange"})

	// TradingVolume24h is the rolling 24-hour trading volume in USD.
	TradingVolume24h = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "costs",
		Name:      "volume_24h_usd",
		Help:      "Rolling 24-hour trading volume in USD per exchange.",
	}, []string{"account", "exchange"})
)
