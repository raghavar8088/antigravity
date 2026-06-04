package pms

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PMSMetrics holds all Prometheus gauges and counters for the PMS subsystem.
type PMSMetrics struct {
	// Portfolio NAV
	PortfolioNAV *prometheus.GaugeVec // labels: portfolio_id, portfolio_type

	// Allocation
	AllocationPct   *prometheus.GaugeVec // labels: portfolio_id, strategy_id
	AllocationDrift *prometheus.GaugeVec // labels: portfolio_id, strategy_id
	CashReservePct  *prometheus.GaugeVec // labels: portfolio_id

	// Capital utilisation
	CapitalUtilisationPct *prometheus.GaugeVec // labels: portfolio_id

	// Risk budget
	PortfolioHeatPct        *prometheus.GaugeVec // labels: portfolio_id
	RiskBudgetUtilisation   *prometheus.GaugeVec // labels: portfolio_id, metric
	DrawdownPct             *prometheus.GaugeVec // labels: portfolio_id

	// Exposure
	GrossExposurePct        *prometheus.GaugeVec // labels: portfolio_id
	NetExposurePct          *prometheus.GaugeVec // labels: portfolio_id
	ExposureConcentration   *prometheus.GaugeVec // labels: portfolio_id, dimension

	// Strategy budget events
	StrategyAutoscaleTotal  *prometheus.CounterVec // labels: portfolio_id, direction
	StrategyDisabledTotal   *prometheus.CounterVec // labels: portfolio_id
	StrategyPromotedTotal   *prometheus.CounterVec // labels: portfolio_id

	// Optimizer
	OptimizerRunTotal    prometheus.Counter
	OptimizerSharpe      *prometheus.GaugeVec // labels: portfolio_id

	// Account NAV
	AccountNAV *prometheus.GaugeVec // labels: account_id, account_type

	// Performance
	PortfolioSharpe   *prometheus.GaugeVec // labels: portfolio_id
	PortfolioSortino  *prometheus.GaugeVec // labels: portfolio_id
	PortfolioCalmar   *prometheus.GaugeVec // labels: portfolio_id
	PortfolioReturnPct *prometheus.GaugeVec // labels: portfolio_id, period
}

// NewPMSMetrics registers all PMS metrics with Prometheus.
// Safe to call multiple times — uses promauto which panics on duplicate registration
// only if the metric was registered with a different help string.
func NewPMSMetrics() *PMSMetrics {
	return &PMSMetrics{
		PortfolioNAV: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pms",
			Name:      "portfolio_nav_usd",
			Help:      "Current Net Asset Value of the portfolio in USD.",
		}, []string{"portfolio_id", "portfolio_type"}),

		AllocationPct: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pms",
			Name:      "allocation_pct",
			Help:      "Current allocation percentage per strategy.",
		}, []string{"portfolio_id", "strategy_id"}),

		AllocationDrift: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pms",
			Name:      "allocation_drift_pct",
			Help:      "Drift from target allocation (current - target) percentage.",
		}, []string{"portfolio_id", "strategy_id"}),

		CashReservePct: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pms",
			Name:      "cash_reserve_pct",
			Help:      "Percentage of portfolio held as cash reserve.",
		}, []string{"portfolio_id"}),

		CapitalUtilisationPct: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pms",
			Name:      "capital_utilisation_pct",
			Help:      "Percentage of portfolio capital currently allocated to strategies.",
		}, []string{"portfolio_id"}),

		PortfolioHeatPct: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pms",
			Name:      "portfolio_heat_pct",
			Help:      "Current portfolio heat (dollar risk at risk / equity * 100).",
		}, []string{"portfolio_id"}),

		RiskBudgetUtilisation: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pms",
			Name:      "risk_budget_utilisation",
			Help:      "Risk budget utilisation ratio (0-1) per metric.",
		}, []string{"portfolio_id", "metric"}),

		DrawdownPct: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pms",
			Name:      "drawdown_pct",
			Help:      "Current portfolio drawdown from HWM in percentage.",
		}, []string{"portfolio_id"}),

		GrossExposurePct: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pms",
			Name:      "gross_exposure_pct",
			Help:      "Gross notional exposure as percentage of NAV.",
		}, []string{"portfolio_id"}),

		NetExposurePct: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pms",
			Name:      "net_exposure_pct",
			Help:      "Net notional exposure (long - short) as percentage of NAV.",
		}, []string{"portfolio_id"}),

		ExposureConcentration: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pms",
			Name:      "exposure_concentration_pct",
			Help:      "Highest single-dimension concentration as percentage of gross exposure.",
		}, []string{"portfolio_id", "dimension"}),

		StrategyAutoscaleTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "pms",
			Name:      "strategy_autoscale_total",
			Help:      "Total number of automatic strategy allocation scale events.",
		}, []string{"portfolio_id", "direction"}),

		StrategyDisabledTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "pms",
			Name:      "strategy_auto_disabled_total",
			Help:      "Total number of strategies auto-disabled by the budget engine.",
		}, []string{"portfolio_id"}),

		StrategyPromotedTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: "pms",
			Name:      "strategy_auto_promoted_total",
			Help:      "Total number of strategies auto-promoted by the budget engine.",
		}, []string{"portfolio_id"}),

		OptimizerRunTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: "pms",
			Name:      "optimizer_run_total",
			Help:      "Total number of portfolio optimisation runs.",
		}),

		OptimizerSharpe: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pms",
			Name:      "optimizer_expected_sharpe",
			Help:      "Expected Sharpe ratio from the latest optimisation recommendation.",
		}, []string{"portfolio_id"}),

		AccountNAV: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pms",
			Name:      "account_nav_usd",
			Help:      "Current NAV of a managed trading account in USD.",
		}, []string{"account_id", "account_type"}),

		PortfolioSharpe: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pms",
			Name:      "portfolio_sharpe_ratio",
			Help:      "Annualised Sharpe ratio for the portfolio.",
		}, []string{"portfolio_id"}),

		PortfolioSortino: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pms",
			Name:      "portfolio_sortino_ratio",
			Help:      "Annualised Sortino ratio for the portfolio.",
		}, []string{"portfolio_id"}),

		PortfolioCalmar: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pms",
			Name:      "portfolio_calmar_ratio",
			Help:      "Calmar ratio (annualised return / max drawdown) for the portfolio.",
		}, []string{"portfolio_id"}),

		PortfolioReturnPct: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "pms",
			Name:      "portfolio_return_pct",
			Help:      "Portfolio return percentage for a given period.",
		}, []string{"portfolio_id", "period"}),
	}
}

// ── MetricsPublisher pushes live metrics to Prometheus ────────────────────────

// MetricsPublisher periodically publishes PMS metrics to Prometheus.
type MetricsPublisher struct {
	mu          sync.Mutex
	metrics     *PMSMetrics
	manager     *PortfolioManager
	accounts    *AccountManager
	exposure    *ExposureAggregationEngine
	riskBudget  *PortfolioRiskBudget
	interval    time.Duration
}

// NewMetricsPublisher creates a publisher that refreshes metrics every interval.
func NewMetricsPublisher(
	metrics *PMSMetrics,
	manager *PortfolioManager,
	accounts *AccountManager,
	exposure *ExposureAggregationEngine,
	riskBudget *PortfolioRiskBudget,
	interval time.Duration,
) *MetricsPublisher {
	if interval == 0 {
		interval = 15 * time.Second
	}
	return &MetricsPublisher{
		metrics:    metrics,
		manager:    manager,
		accounts:   accounts,
		exposure:   exposure,
		riskBudget: riskBudget,
		interval:   interval,
	}
}

// Publish pushes the current PMS state to Prometheus immediately.
func (p *MetricsPublisher) Publish() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Portfolio metrics
	for _, snap := range p.manager.All() {
		pid := snap.PortfolioID
		ptype := strings.ToLower(string(snap.Type))
		p.metrics.PortfolioNAV.WithLabelValues(pid, ptype).Set(snap.CurrentNAV)
		p.metrics.CashReservePct.WithLabelValues(pid).Set(100.0 - snap.AllocatedPct)
		p.metrics.CapitalUtilisationPct.WithLabelValues(pid).Set(snap.AllocatedPct)

		for stratID, alloc := range snap.Allocations {
			p.metrics.AllocationPct.WithLabelValues(pid, stratID).Set(alloc.AllocPct)
		}

		// Risk budget metrics
		if rSnap, ok := p.riskBudget.Snapshot(pid); ok {
			p.metrics.PortfolioHeatPct.WithLabelValues(pid).Set(rSnap.TotalHeatPct)
			p.metrics.DrawdownPct.WithLabelValues(pid).Set(rSnap.DrawdownPct)
			p.metrics.RiskBudgetUtilisation.WithLabelValues(pid, "heat").Set(rSnap.HeatUtilisation)
			p.metrics.RiskBudgetUtilisation.WithLabelValues(pid, "drawdown").Set(rSnap.DrawdownUtilisation)
		}

		// Exposure metrics
		expSnap := p.exposure.Snapshot(pid)
		p.metrics.GrossExposurePct.WithLabelValues(pid).Set(expSnap.GrossExpPct)
		p.metrics.NetExposurePct.WithLabelValues(pid).Set(expSnap.NetExpPct)
		p.metrics.ExposureConcentration.WithLabelValues(pid, "symbol").Set(expSnap.MaxSymbolConcentrationPct)
		p.metrics.ExposureConcentration.WithLabelValues(pid, "exchange").Set(expSnap.MaxExchangeConcentrationPct)
		p.metrics.ExposureConcentration.WithLabelValues(pid, "strategy").Set(expSnap.MaxStrategyConcentrationPct)
	}

	// Account metrics
	for _, acc := range p.accounts.AllSnapshots() {
		p.metrics.AccountNAV.WithLabelValues(acc.AccountID, strings.ToLower(string(acc.Type))).Set(acc.CurrentNAV)
	}
}

// PublishPerformance publishes performance analytics metrics for a portfolio.
func (p *MetricsPublisher) PublishPerformance(perf PortfolioPerformance) {
	pid := perf.PortfolioID
	p.metrics.PortfolioSharpe.WithLabelValues(pid).Set(perf.SharpeRatio)
	p.metrics.PortfolioSortino.WithLabelValues(pid).Set(perf.SortinoRatio)
	p.metrics.PortfolioCalmar.WithLabelValues(pid).Set(perf.CalmarRatio)
	p.metrics.PortfolioReturnPct.WithLabelValues(pid, "total").Set(perf.TotalReturnPct)
	p.metrics.PortfolioReturnPct.WithLabelValues(pid, "daily").Set(perf.Daily.ReturnPct)
	p.metrics.PortfolioReturnPct.WithLabelValues(pid, "weekly").Set(perf.Weekly.ReturnPct)
	p.metrics.PortfolioReturnPct.WithLabelValues(pid, "monthly").Set(perf.Monthly.ReturnPct)
	p.metrics.PortfolioReturnPct.WithLabelValues(pid, "ytd").Set(perf.YTD.ReturnPct)
}

// GrafanaDashboardJSON returns the minimal Grafana dashboard JSON for PMS panels.
// Embed this in your Grafana provisioning config.
func GrafanaDashboardJSON() string {
	panels := []string{
		grafanaGauge("Portfolio NAV (USD)", `pms_portfolio_nav_usd`, "portfolio_id"),
		grafanaGauge("Capital Utilisation (%)", `pms_capital_utilisation_pct`, "portfolio_id"),
		grafanaGauge("Portfolio Heat (%)", `pms_portfolio_heat_pct`, "portfolio_id"),
		grafanaGauge("Gross Exposure (%)", `pms_gross_exposure_pct`, "portfolio_id"),
		grafanaGauge("Net Exposure (%)", `pms_net_exposure_pct`, "portfolio_id"),
		grafanaGauge("Drawdown (%)", `pms_drawdown_pct`, "portfolio_id"),
		grafanaGauge("Sharpe Ratio", `pms_portfolio_sharpe_ratio`, "portfolio_id"),
		grafanaGauge("Sortino Ratio", `pms_portfolio_sortino_ratio`, "portfolio_id"),
		grafanaGauge("Calmar Ratio", `pms_portfolio_calmar_ratio`, "portfolio_id"),
	}
	log.Printf("[PMS METRICS] Grafana dashboard panels: %d", len(panels))
	return fmt.Sprintf(`{"title":"PMS - Institutional Portfolio Management","panels":[%s]}`,
		strings.Join(panels, ","))
}

func grafanaGauge(title, metric, labelKey string) string {
	return fmt.Sprintf(`{"type":"gauge","title":%q,"targets":[{"expr":%q,"legendFormat":"{{%s}}"}]}`,
		title, metric+`{`+labelKey+`=~"$portfolio"}`, labelKey)
}
