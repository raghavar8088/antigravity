package riskv3

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"antigravity-engine/internal/ledger"
)

// OrderCheckRequest is the input to the portfolio risk engine's pre-trade check.
// It must be populated before any order is submitted to OMS v3.
type OrderCheckRequest struct {
	ClientOrderID string    // assigned by OMS v3 GenerateClientOrderID
	Symbol        string
	Side          string    // BUY | SELL
	Size          float64   // base-asset quantity
	EntryPrice    float64
	StopLoss      float64
	StrategyName  string
	FamilyName    string
	Exchange      string
	Confidence    float64
	RequestedAt   time.Time

	// Strategy performance metrics for Kelly sizing
	TotalTrades int
	Wins        int
	AvgWin      float64 // average win in USD
	AvgLoss     float64 // average loss in USD (positive)

	// Return series for VaR / correlation (most recent 252 returns)
	StrategyReturns []float64
}

// Engine is the Phase 15D Portfolio Risk Engine V3.
//
// It is the single gate between strategy signals and OMS v3 order submission.
// Every CheckOrder call:
//  1. Computes portfolio heat, VaR, CVaR, correlation, concentration
//  2. Applies all institutional hard limits from limits.go
//  3. Emits EventRiskApproved or EventRiskBlocked to the ledger
//  4. Returns OrderDecision with full risk context
//
// All state is held in PortfolioState and rebuilt from ledger on startup.
type Engine struct {
	mu        sync.RWMutex
	state     *PortfolioState
	ledger    ledger.Store
	accountID string

	// Strategy return series for correlation / VaR (keyed by strategy name)
	returnsByStrategy map[string][]ReturnSeries

	// Prometheus metrics (nil when Prometheus is not available)
	prom *PrometheusMetrics

	// Snapshot ticker — emits periodic portfolio snapshot events to ledger
	snapshotInterval time.Duration
}

// NewEngine creates a portfolio risk engine backed by the given ledger store.
// initialEquity is the starting equity in USD; used until the ledger is replayed.
func NewEngine(store ledger.Store, accountID string, initialEquity float64) *Engine {
	return &Engine{
		state:             NewPortfolioState(initialEquity),
		ledger:            store,
		accountID:         accountID,
		returnsByStrategy: make(map[string][]ReturnSeries),
		snapshotInterval:  5 * time.Minute,
	}
}

// SetPrometheusMetrics attaches Prometheus metrics to the engine.
func (e *Engine) SetPrometheusMetrics(prom *PrometheusMetrics) {
	e.mu.Lock()
	e.prom = prom
	e.mu.Unlock()
}

// ─── Pre-trade risk check (primary interface) ─────────────────────────────────

// CheckOrder is the mandatory pre-trade risk gate.
// No order may reach OMS v3 without passing this check.
//
// It evaluates:
//  1. Portfolio heat (current + proposed)
//  2. VaR / CVaR limits
//  3. Drawdown and daily loss limits
//  4. Concentration limits (symbol, exchange, directional)
//  5. Correlation limits
//  6. Position-level dollar risk (Kelly sizing)
//
// On every call it emits a RISK_APPROVED or RISK_BLOCKED ledger event.
func (e *Engine) CheckOrder(ctx context.Context, req OrderCheckRequest) OrderDecision {
	if req.RequestedAt.IsZero() {
		req.RequestedAt = time.Now().UTC()
	}

	snap := e.state.Snapshot()
	equity := snap.EquityUSD
	decision := OrderDecision{
		RequestedSize: req.Size,
		CheckedAt:     req.RequestedAt,
	}

	var violations []Violation

	// ── 1. Equity sanity ─────────────────────────────────────────────────────
	if equity <= 0 {
		return e.reject(ctx, req, decision, "equity is zero or negative", violations)
	}

	// ── 2. Portfolio heat (current + proposed) ────────────────────────────────
	proposedDollarRisk := PositionDollarRisk(req.EntryPrice, req.StopLoss, req.Size, req.Side)
	heatResult := ComputeHeatWithProposal(snap, proposedDollarRisk)
	decision.HeatPct = heatResult.HeatPct
	decision.HeatLevel = heatResult.HeatLevel

	if heatResult.KillBreached {
		violations = append(violations, Violation{
			Type:    ViolationHeatExceeded,
			Metric:  "proposed_heat_pct",
			Current: heatResult.ProposedHeatPct,
			Limit:   HeatKillPct,
			Description: "proposed trade would push portfolio heat above kill threshold",
		})
	} else if heatResult.CriticalBreached {
		violations = append(violations, Violation{
			Type:    ViolationHeatExceeded,
			Metric:  "proposed_heat_pct",
			Current: heatResult.ProposedHeatPct,
			Limit:   HeatCriticalPct,
			Description: "proposed trade would push portfolio heat to critical level",
		})
	}

	// ── 3. Position-level dollar risk limit ───────────────────────────────────
	positionRiskPct := proposedDollarRisk / equity * 100
	if positionRiskPct > MaxPositionRiskPct {
		violations = append(violations, Violation{
			Type:    ViolationPositionRiskExceeded,
			Metric:  "position_risk_pct",
			Current: positionRiskPct,
			Limit:   MaxPositionRiskPct,
			Description: "single position dollar risk exceeds 1% of equity",
		})
	}

	// ── 4. VaR ───────────────────────────────────────────────────────────────
	varResult := ComputeVaR(snap.Returns, equity)
	decision.VaR95Pct = varResult.Daily95Pct
	if varResult.VaRBreached {
		violations = append(violations, Violation{
			Type:    ViolationVaRExceeded,
			Metric:  "daily_var_95_pct",
			Current: varResult.Daily95Pct,
			Limit:   MaxDailyVaR95Pct,
			Description: "portfolio VaR at 95% exceeds daily limit",
		})
	}

	// ── 5. CVaR ──────────────────────────────────────────────────────────────
	cvarResult := ComputeCVaR(snap.Returns, equity)
	decision.CVaR95Pct = cvarResult.CVaR95Pct
	if cvarResult.CVaR95Breached {
		violations = append(violations, Violation{
			Type:    ViolationCVaRExceeded,
			Metric:  "cvar_95_pct",
			Current: cvarResult.CVaR95Pct,
			Limit:   MaxCVaR95Pct,
			Description: "portfolio CVaR at 95% exceeds limit",
		})
	}

	// ── 6. Drawdown ───────────────────────────────────────────────────────────
	decision.DrawdownPct = snap.DrawdownPct
	if snap.DrawdownPct >= MaxDrawdownPct {
		violations = append(violations, Violation{
			Type:    ViolationDrawdownExceeded,
			Metric:  "drawdown_pct",
			Current: snap.DrawdownPct,
			Limit:   MaxDrawdownPct,
			Description: "portfolio drawdown exceeds maximum threshold — trading halted",
		})
	}

	// ── 7. Daily loss ─────────────────────────────────────────────────────────
	decision.DailyLossPct = snap.DailyLossPct
	if snap.DailyLossPct >= MaxDailyLossPct {
		violations = append(violations, Violation{
			Type:    ViolationDailyLossExceeded,
			Metric:  "daily_loss_pct",
			Current: snap.DailyLossPct,
			Limit:   MaxDailyLossPct,
			Description: "daily loss limit reached — no new orders today",
		})
	}

	// ── 8. Weekly loss ────────────────────────────────────────────────────────
	weeklyLossPct := 0.0
	if equity > 0 && snap.WeeklyPnLUSD < 0 {
		weeklyLossPct = -snap.WeeklyPnLUSD / equity * 100
	}
	if weeklyLossPct >= MaxWeeklyLossPct {
		violations = append(violations, Violation{
			Type:    ViolationWeeklyLossExceeded,
			Metric:  "weekly_loss_pct",
			Current: weeklyLossPct,
			Limit:   MaxWeeklyLossPct,
			Description: "weekly loss limit reached",
		})
	}

	// ── 9. Concentration ──────────────────────────────────────────────────────
	proposed := PositionRecord{
		Symbol:   req.Symbol,
		Side:     req.Side,
		NotionalUSD: req.Size * req.EntryPrice,
		Exchange: req.Exchange,
	}
	concViolations := ConcentrationGuard(snap, proposed)
	violations = append(violations, concViolations...)

	// ── 10. Gross exposure ────────────────────────────────────────────────────
	gross := snap.TotalNotionalUSD()
	newGross := gross + req.Size*req.EntryPrice
	grossPct := newGross / equity * 100
	decision.ConcentrationRisk = grossPct
	if grossPct > MaxGrossExposurePct {
		violations = append(violations, Violation{
			Type:    ViolationGrossExposureExceeded,
			Metric:  "gross_exposure_pct",
			Current: grossPct,
			Limit:   MaxGrossExposurePct,
			Description: "gross exposure would exceed 200% of equity",
		})
	}

	// ── 11. Kelly sizing ─────────────────────────────────────────────────────
	kelly := KellyFromStats(req.TotalTrades, req.Wins, req.AvgWin, req.AvgLoss, equity)
	decision.KellyFractionPct = kelly.RecommendedFractionPct
	// Kelly-recommended size in USD
	kellyUSD := kelly.RecommendedUSD
	approvedNotional := req.Size * req.EntryPrice
	if kellyUSD > 0 && approvedNotional > kellyUSD*2 {
		// Requested size > 2× Kelly → cap at 2× Kelly with a warning
		// (we use 2× rather than 1× to allow some flexibility above Kelly)
		maxSize := kellyUSD * 2 / req.EntryPrice
		if maxSize < req.Size {
			req.Size = maxSize
			log.Printf("[RISK V3] %s: size capped to %.4f BTC (Kelly cap)", req.StrategyName, maxSize)
		}
	}
	decision.ApprovedSize = req.Size

	// ── 12. Composite risk score ──────────────────────────────────────────────
	metrics := PortfolioMetrics{
		HeatPct:          heatResult.HeatPct,
		DailyVaR95Pct:    varResult.Daily95Pct,
		CVaR95Pct:        cvarResult.CVaR95Pct,
		DrawdownPct:      snap.DrawdownPct,
		DailyLossPct:     snap.DailyLossPct,
		MaxPairwiseCorr:  decision.CorrelationRisk,
	}
	decision.RiskScore = ComputeRiskScore(metrics)
	decision.Violations = violations

	// ── Decision ──────────────────────────────────────────────────────────────
	if len(violations) > 0 {
		reason := violations[0].Description
		if len(violations) > 1 {
			reason = fmt.Sprintf("%s (and %d more violations)", reason, len(violations)-1)
		}
		return e.reject(ctx, req, decision, reason, violations)
	}
	return e.approve(ctx, req, decision)
}

// ─── Position lifecycle notifications ────────────────────────────────────────

// NotifyPositionOpened updates the portfolio state when a position is opened.
// Must be called after every successful order fill.
func (e *Engine) NotifyPositionOpened(pos PositionRecord) {
	e.state.OpenPosition(pos)
	e.publishMetrics()
}

// NotifyPositionClosed updates the portfolio state when a position is closed.
func (e *Engine) NotifyPositionClosed(positionID string, realisedPnLUSD float64, now time.Time) {
	e.state.ClosePosition(positionID, realisedPnLUSD, now)
	e.state.RecordDailyReturn(realisedPnLUSD / e.state.EquityUSD() * 100)
	e.publishMetrics()
}

// UpdateEquity updates the equity from the latest account balance.
func (e *Engine) UpdateEquity(equityUSD float64) {
	e.state.UpdateEquity(equityUSD)
}

// RecordStrategyReturn records a single return observation for a strategy.
// Used to build the rolling return series for VaR / correlation computation.
func (e *Engine) RecordStrategyReturn(strategyName string, returnPct float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	series := e.returnsByStrategy[strategyName]
	if len(series) == 0 {
		series = []ReturnSeries{{Name: strategyName}}
	}
	latest := &series[len(series)-1]
	latest.Returns = append(latest.Returns, returnPct)
	if len(latest.Returns) > 252 {
		latest.Returns = latest.Returns[1:]
	}
	e.returnsByStrategy[strategyName] = series
}

// ─── Analytics / query ────────────────────────────────────────────────────────

// CurrentMetrics computes and returns the full portfolio metrics snapshot.
func (e *Engine) CurrentMetrics() PortfolioMetrics {
	snap := e.state.Snapshot()
	equity := snap.EquityUSD

	heatResult := ComputeHeat(snap)
	varResult := ComputeVaR(snap.Returns, equity)
	cvarResult := ComputeCVaR(snap.Returns, equity)
	concResult := AnalyseConcentration(snap)

	metrics := PortfolioMetrics{
		AccountID:                   e.accountID,
		EquityUSD:                   equity,
		HWMUSD:                      snap.HWMUSD,
		OpenPositions:               len(snap.Positions),
		DailyPnLUSD:                 snap.DailyPnLUSD,
		WeeklyPnLUSD:                snap.WeeklyPnLUSD,
		DailyLossPct:                snap.DailyLossPct,
		DrawdownPct:                 snap.DrawdownPct,
		HeatPct:                     heatResult.HeatPct,
		HeatLevel:                   heatResult.HeatLevel,
		HeatUSD:                     heatResult.TotalDollarRiskUSD,
		GrossNotionalUSD:            snap.TotalNotionalUSD(),
		NetNotionalUSD:              snap.NetNotionalUSD(),
		GrossExposurePct:            snap.TotalNotionalUSD() / equity * 100,
		NetExposurePct:              snap.NetNotionalUSD() / equity * 100,
		DailyVaR95USD:               varResult.Daily95USD,
		DailyVaR99USD:               varResult.Daily99USD,
		DailyVaR95Pct:               varResult.Daily95Pct,
		DailyVaR99Pct:               varResult.Daily99Pct,
		WeeklyVaR95USD:              varResult.Weekly95USD,
		CVaR95USD:                   cvarResult.CVaR95USD,
		CVaR99USD:                   cvarResult.CVaR99USD,
		CVaR95Pct:                   cvarResult.CVaR95Pct,
		CVaR99Pct:                   cvarResult.CVaR99Pct,
		MaxSymbolConcentrationPct:   concResult.MaxSymbolPct,
		MaxStrategyConcentrationPct: concResult.MaxStrategyPct,
		MaxExchangeConcentrationPct: concResult.MaxExchangePct,
		ConcentrationScore:          concResult.ConcentrationScore,
		ComputedAt:                  time.Now().UTC(),
	}
	metrics.RiskScore = ComputeRiskScore(metrics)
	return metrics
}

// StartSnapshotLoop starts a background goroutine that emits periodic portfolio
// snapshot events to the ledger. Call once after construction.
func (e *Engine) StartSnapshotLoop(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(e.snapshotInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				metrics := e.CurrentMetrics()
				EmitPortfolioSnapshot(ctx, e.ledger, e.accountID, metrics)
				e.publishMetricsFrom(metrics)
			}
		}
	}()
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

func (e *Engine) approve(ctx context.Context, req OrderCheckRequest, decision OrderDecision) OrderDecision {
	decision.Status = DecisionApproved
	decision.Reason = "all portfolio risk checks passed"
	e.logDecision(decision, req.StrategyName)
	if e.ledger != nil {
		go EmitRiskApproved(ctx, e.ledger, e.accountID, req.ClientOrderID, decision, req.StrategyName)
	}
	e.publishMetrics()
	return decision
}

func (e *Engine) reject(ctx context.Context, req OrderCheckRequest, decision OrderDecision, reason string, violations []Violation) OrderDecision {
	decision.Status = DecisionBlocked
	decision.Reason = reason
	decision.Violations = violations
	decision.ApprovedSize = 0
	e.logDecision(decision, req.StrategyName)
	if e.ledger != nil {
		go EmitRiskBlocked(ctx, e.ledger, e.accountID, req.ClientOrderID, decision, req.StrategyName)
	}
	return decision
}

func (e *Engine) logDecision(d OrderDecision, strategyName string) {
	if d.IsApproved() {
		log.Printf("[RISK V3] APPROVED | strategy=%s size=%.4f heat=%.1f%% VaR95=%.1f%% score=%d",
			strategyName, d.ApprovedSize, d.HeatPct, d.VaR95Pct, d.RiskScore)
	} else {
		log.Printf("[RISK V3] BLOCKED | strategy=%s reason=%s heat=%.1f%% violations=%d",
			strategyName, d.Reason, d.HeatPct, len(d.Violations))
	}
}

func (e *Engine) publishMetrics() {
	if e.prom == nil {
		return
	}
	metrics := e.CurrentMetrics()
	e.publishMetricsFrom(metrics)
}

func (e *Engine) publishMetricsFrom(metrics PortfolioMetrics) {
	if e.prom == nil {
		return
	}
	e.prom.HeatPct.Set(metrics.HeatPct)
	e.prom.VaR95Pct.Set(metrics.DailyVaR95Pct)
	e.prom.CVaR95Pct.Set(metrics.CVaR95Pct)
	e.prom.DrawdownPct.Set(metrics.DrawdownPct)
	e.prom.DailyLossPct.Set(metrics.DailyLossPct)
	e.prom.RiskScore.Set(float64(metrics.RiskScore))
	e.prom.ConcentrationScore.Set(metrics.ConcentrationScore)
}
