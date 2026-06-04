package pms

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"antigravity-engine/internal/ledger"
)

// AllocationMethod determines how capital is distributed across strategies.
type AllocationMethod string

const (
	AllocationFixed            AllocationMethod = "FIXED"
	AllocationDynamic          AllocationMethod = "DYNAMIC"
	AllocationPerfWeighted     AllocationMethod = "PERF_WEIGHTED"
	AllocationSharpeWeighted   AllocationMethod = "SHARPE_WEIGHTED"
	AllocationVolAdjusted      AllocationMethod = "VOL_ADJUSTED"
	AllocationDrawdownAdjusted AllocationMethod = "DRAWDOWN_ADJUSTED"
	AllocationKelly            AllocationMethod = "KELLY"
)

// StrategyMetrics provides the performance data required to compute
// dynamic allocations. Populated from the performance analytics engine.
type StrategyMetrics struct {
	StrategyID   string
	StrategyName string
	Family       string

	// Performance
	TotalTrades  int
	WinRate      float64 // 0–1
	ProfitFactor float64
	SharpeRatio  float64
	SortinoRatio float64
	CalmarRatio  float64

	// Risk
	MaxDrawdownPct  float64 // positive value, e.g. 5.2 means 5.2%
	VolatilityPct   float64 // annualised volatility as percentage
	CurrentDDPct    float64 // current drawdown from HWM

	// Kelly
	AvgWinUSD  float64
	AvgLossUSD float64 // positive

	// Capital context
	CurrentAllocPct float64
	CurrentBudgetUSD float64
	TotalEquityUSD   float64
}

// AllocationRequest specifies the full set of strategies to be allocated across.
type AllocationRequest struct {
	PortfolioID    string
	TotalCapitalUSD float64
	Method         AllocationMethod
	// FixedWeights maps strategyID → desired percentage (used for FIXED method).
	// Must sum to ≤ 100.0.
	FixedWeights map[string]float64
	Strategies   []StrategyMetrics
	// CashReservePct is the minimum cash buffer kept unallocated (default 5%).
	CashReservePct float64
	// MaxSingleAllocPct caps any one strategy's allocation (default 25%).
	MaxSingleAllocPct float64
}

// AllocationResult holds the computed capital distribution.
type AllocationResult struct {
	PortfolioID    string
	TotalCapitalUSD float64
	Method         AllocationMethod
	Weights        []StrategyWeight
	CashReserveUSD  float64
	AllocatedPct   float64
	ComputedAt     time.Time
}

// StrategyWeight is a single strategy's computed allocation.
type StrategyWeight struct {
	StrategyID   string
	StrategyName string
	AllocPct     float64
	AllocUSD     float64
	Rationale    string
}

// AllocationEngine computes capital distributions across strategies and
// commits changes to the portfolio aggregate via the ledger.
type AllocationEngine struct {
	mu      sync.Mutex
	manager *PortfolioManager
	store   ledger.Store
}

// NewAllocationEngine creates an AllocationEngine.
func NewAllocationEngine(manager *PortfolioManager, store ledger.Store) *AllocationEngine {
	return &AllocationEngine{
		manager: manager,
		store:   store,
	}
}

// Allocate computes the allocation for the given request and applies it
// to the portfolio, emitting one AllocationChanged event per strategy.
func (e *AllocationEngine) Allocate(ctx context.Context, req AllocationRequest) (AllocationResult, error) {
	if req.CashReservePct == 0 {
		req.CashReservePct = 5.0
	}
	if req.MaxSingleAllocPct == 0 {
		req.MaxSingleAllocPct = 25.0
	}
	if len(req.Strategies) == 0 {
		return AllocationResult{}, fmt.Errorf("pms: allocation requires at least one strategy")
	}

	var weights []StrategyWeight
	var err error

	switch req.Method {
	case AllocationFixed:
		weights, err = e.computeFixed(req)
	case AllocationSharpeWeighted:
		weights, err = e.computeSharpeWeighted(req)
	case AllocationVolAdjusted:
		weights, err = e.computeVolAdjusted(req)
	case AllocationDrawdownAdjusted:
		weights, err = e.computeDrawdownAdjusted(req)
	case AllocationKelly:
		weights, err = e.computeKelly(req)
	case AllocationPerfWeighted:
		weights, err = e.computePerfWeighted(req)
	default: // AllocationDynamic
		weights, err = e.computeDynamic(req)
	}
	if err != nil {
		return AllocationResult{}, err
	}

	// Apply cash reserve constraint
	weights = e.applyCashReserve(weights, req.CashReservePct)
	// Apply single-strategy cap
	weights = e.applyMaxCap(weights, req.MaxSingleAllocPct)
	// Normalise to 100 - cashReserve
	weights = normaliseWeights(weights, 100.0-req.CashReservePct)

	// Compute USD values
	availableUSD := req.TotalCapitalUSD * (100.0 - req.CashReservePct) / 100.0
	allocPctTotal := 0.0
	for i := range weights {
		weights[i].AllocUSD = availableUSD * weights[i].AllocPct / 100.0
		allocPctTotal += weights[i].AllocPct
	}

	result := AllocationResult{
		PortfolioID:     req.PortfolioID,
		TotalCapitalUSD: req.TotalCapitalUSD,
		Method:          req.Method,
		Weights:         weights,
		CashReserveUSD:  req.TotalCapitalUSD * req.CashReservePct / 100.0,
		AllocatedPct:    allocPctTotal,
		ComputedAt:      time.Now().UTC(),
	}

	// Persist allocation changes via events
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.persistAllocation(ctx, req.PortfolioID, result); err != nil {
		return result, fmt.Errorf("pms: persist allocation: %w", err)
	}
	return result, nil
}

// ── Allocation computation methods ───────────────────────────────────────────

func (e *AllocationEngine) computeFixed(req AllocationRequest) ([]StrategyWeight, error) {
	if len(req.FixedWeights) == 0 {
		return nil, fmt.Errorf("pms: FIXED allocation requires FixedWeights map")
	}
	total := 0.0
	for _, pct := range req.FixedWeights {
		total += pct
	}
	if total > 100.0 {
		return nil, fmt.Errorf("pms: FIXED weights sum %.1f%% exceeds 100%%", total)
	}
	weights := make([]StrategyWeight, 0, len(req.FixedWeights))
	idxByID := make(map[string]StrategyMetrics)
	for _, s := range req.Strategies {
		idxByID[s.StrategyID] = s
	}
	for stratID, pct := range req.FixedWeights {
		s := idxByID[stratID]
		weights = append(weights, StrategyWeight{
			StrategyID:   stratID,
			StrategyName: s.StrategyName,
			AllocPct:     pct,
			Rationale:    fmt.Sprintf("fixed %.1f%%", pct),
		})
	}
	return weights, nil
}

func (e *AllocationEngine) computeSharpeWeighted(req AllocationRequest) ([]StrategyWeight, error) {
	weights := make([]StrategyWeight, 0, len(req.Strategies))
	sharpeSum := 0.0
	for _, s := range req.Strategies {
		if s.SharpeRatio > 0 {
			sharpeSum += s.SharpeRatio
		}
	}
	if sharpeSum == 0 {
		return e.computeEqual(req)
	}
	for _, s := range req.Strategies {
		if s.SharpeRatio <= 0 {
			continue
		}
		pct := s.SharpeRatio / sharpeSum * (100.0 - req.CashReservePct)
		weights = append(weights, StrategyWeight{
			StrategyID:   s.StrategyID,
			StrategyName: s.StrategyName,
			AllocPct:     pct,
			Rationale:    fmt.Sprintf("sharpe=%.2f / total_sharpe=%.2f", s.SharpeRatio, sharpeSum),
		})
	}
	return weights, nil
}

func (e *AllocationEngine) computeVolAdjusted(req AllocationRequest) ([]StrategyWeight, error) {
	// Inverse-volatility weighting: lower vol → higher weight.
	weights := make([]StrategyWeight, 0, len(req.Strategies))
	invVolSum := 0.0
	for _, s := range req.Strategies {
		if s.VolatilityPct > 0 {
			invVolSum += 1.0 / s.VolatilityPct
		}
	}
	if invVolSum == 0 {
		return e.computeEqual(req)
	}
	for _, s := range req.Strategies {
		if s.VolatilityPct <= 0 {
			continue
		}
		pct := (1.0 / s.VolatilityPct) / invVolSum * (100.0 - req.CashReservePct)
		weights = append(weights, StrategyWeight{
			StrategyID:   s.StrategyID,
			StrategyName: s.StrategyName,
			AllocPct:     pct,
			Rationale:    fmt.Sprintf("inv_vol=1/%.2f%%", s.VolatilityPct),
		})
	}
	return weights, nil
}

func (e *AllocationEngine) computeDrawdownAdjusted(req AllocationRequest) ([]StrategyWeight, error) {
	// Weight strategies inversely proportional to max drawdown.
	weights := make([]StrategyWeight, 0, len(req.Strategies))
	invDDSum := 0.0
	for _, s := range req.Strategies {
		if s.MaxDrawdownPct > 0 {
			invDDSum += 1.0 / s.MaxDrawdownPct
		}
	}
	if invDDSum == 0 {
		return e.computeEqual(req)
	}
	for _, s := range req.Strategies {
		if s.MaxDrawdownPct <= 0 {
			continue
		}
		pct := (1.0 / s.MaxDrawdownPct) / invDDSum * (100.0 - req.CashReservePct)
		weights = append(weights, StrategyWeight{
			StrategyID:   s.StrategyID,
			StrategyName: s.StrategyName,
			AllocPct:     pct,
			Rationale:    fmt.Sprintf("inv_dd=1/%.2f%%", s.MaxDrawdownPct),
		})
	}
	return weights, nil
}

func (e *AllocationEngine) computeKelly(req AllocationRequest) ([]StrategyWeight, error) {
	// Kelly fraction f* = (p*b - q) / b, where p=win_rate, q=1-p, b=avg_win/avg_loss.
	// We use a half-Kelly cap for institutional safety.
	weights := make([]StrategyWeight, 0, len(req.Strategies))
	kellySum := 0.0
	kellys := make([]float64, len(req.Strategies))
	for i, s := range req.Strategies {
		if s.AvgLossUSD <= 0 || s.AvgWinUSD <= 0 || s.WinRate <= 0 {
			continue
		}
		b := s.AvgWinUSD / s.AvgLossUSD
		p := s.WinRate
		q := 1.0 - p
		full := (p*b - q) / b
		half := math.Max(0, full*0.5) // half-Kelly
		kellys[i] = half
		kellySum += half
	}
	if kellySum == 0 {
		return e.computeEqual(req)
	}
	for i, s := range req.Strategies {
		if kellys[i] == 0 {
			continue
		}
		pct := kellys[i] / kellySum * (100.0 - req.CashReservePct)
		weights = append(weights, StrategyWeight{
			StrategyID:   s.StrategyID,
			StrategyName: s.StrategyName,
			AllocPct:     pct,
			Rationale:    fmt.Sprintf("half_kelly=%.3f", kellys[i]),
		})
	}
	return weights, nil
}

func (e *AllocationEngine) computePerfWeighted(req AllocationRequest) ([]StrategyWeight, error) {
	// Composite score = Sharpe * WinRate * ProfitFactor / (MaxDD + 1).
	weights := make([]StrategyWeight, 0, len(req.Strategies))
	scores := make([]float64, len(req.Strategies))
	total := 0.0
	for i, s := range req.Strategies {
		score := s.SharpeRatio * s.WinRate * s.ProfitFactor / (s.MaxDrawdownPct + 1.0)
		if score < 0 {
			score = 0
		}
		scores[i] = score
		total += score
	}
	if total == 0 {
		return e.computeEqual(req)
	}
	for i, s := range req.Strategies {
		if scores[i] == 0 {
			continue
		}
		pct := scores[i] / total * (100.0 - req.CashReservePct)
		weights = append(weights, StrategyWeight{
			StrategyID:   s.StrategyID,
			StrategyName: s.StrategyName,
			AllocPct:     pct,
			Rationale:    fmt.Sprintf("perf_score=%.4f", scores[i]),
		})
	}
	return weights, nil
}

// computeDynamic selects the best method per strategy based on data availability.
func (e *AllocationEngine) computeDynamic(req AllocationRequest) ([]StrategyWeight, error) {
	// Primary: Sharpe weighted; fallback to equal if data sparse.
	w, err := e.computeSharpeWeighted(req)
	if err != nil || len(w) == 0 {
		return e.computeEqual(req)
	}
	return w, nil
}

func (e *AllocationEngine) computeEqual(req AllocationRequest) ([]StrategyWeight, error) {
	n := len(req.Strategies)
	if n == 0 {
		return nil, fmt.Errorf("pms: no strategies for equal allocation")
	}
	equalPct := (100.0 - req.CashReservePct) / float64(n)
	weights := make([]StrategyWeight, n)
	for i, s := range req.Strategies {
		weights[i] = StrategyWeight{
			StrategyID:   s.StrategyID,
			StrategyName: s.StrategyName,
			AllocPct:     equalPct,
			Rationale:    fmt.Sprintf("equal 1/%d", n),
		}
	}
	return weights, nil
}

// ── Constraint helpers ────────────────────────────────────────────────────────

func (e *AllocationEngine) applyCashReserve(weights []StrategyWeight, cashPct float64) []StrategyWeight {
	maxDeployable := 100.0 - cashPct
	for i := range weights {
		if weights[i].AllocPct > maxDeployable {
			weights[i].AllocPct = maxDeployable
		}
	}
	return weights
}

func (e *AllocationEngine) applyMaxCap(weights []StrategyWeight, maxPct float64) []StrategyWeight {
	for i := range weights {
		if weights[i].AllocPct > maxPct {
			weights[i].AllocPct = maxPct
		}
	}
	return weights
}

func normaliseWeights(weights []StrategyWeight, targetTotal float64) []StrategyWeight {
	total := 0.0
	for _, w := range weights {
		total += w.AllocPct
	}
	if total == 0 || targetTotal == 0 {
		return weights
	}
	scale := targetTotal / total
	for i := range weights {
		weights[i].AllocPct = math.Round(weights[i].AllocPct*scale*100) / 100
	}
	return weights
}

// ── Persistence ───────────────────────────────────────────────────────────────

func (e *AllocationEngine) persistAllocation(ctx context.Context, portfolioID string, result AllocationResult) error {
	p, err := e.manager.Get(portfolioID)
	if err != nil {
		return err
	}
	snap := p.Snapshot()

	// Sort for deterministic event ordering.
	sort.Slice(result.Weights, func(i, j int) bool {
		return result.Weights[i].StrategyID < result.Weights[j].StrategyID
	})

	for _, w := range result.Weights {
		existing, exists := snap.Allocations[w.StrategyID]

		if !exists {
			// New allocation
			allocID := fmt.Sprintf("alloc:%s:%s:%d", portfolioID, w.StrategyID, result.ComputedAt.UnixNano())
			payload := AllocationCreatedPayload{
				AllocationID: allocID,
				PortfolioID:  portfolioID,
				StrategyID:   w.StrategyID,
				StrategyName: w.StrategyName,
				Method:       string(result.Method),
				AllocPct:     w.AllocPct,
				AllocUSD:     w.AllocUSD,
			}
			ev, err := ledger.NewEvent(ledger.NewEventInput{
				AggregateType: AggregatePortfolio,
				AggregateID:   portfolioID,
				EventType:     EventAllocationCreated,
				AccountID:     portfolioID,
				StrategyID:    w.StrategyID,
				Payload:       payload,
				Source:        "pms.allocation",
			})
			if err != nil {
				return err
			}
			if err := p.ApplyEvent(ev); err != nil {
				return err
			}
			if e.store != nil {
				e.store.Append(ctx, ev) //nolint:errcheck
			}
		} else if math.Abs(existing.AllocPct-w.AllocPct) > 0.01 {
			// Changed allocation
			payload := AllocationChangedPayload{
				AllocationID: existing.AllocationID,
				PortfolioID:  portfolioID,
				StrategyID:   w.StrategyID,
				PreviousPct:  existing.AllocPct,
				NewPct:       w.AllocPct,
				PreviousUSD:  existing.AllocUSD,
				NewUSD:       w.AllocUSD,
				Reason:       fmt.Sprintf("rebalance via %s", result.Method),
			}
			ev, err := ledger.NewEvent(ledger.NewEventInput{
				AggregateType: AggregatePortfolio,
				AggregateID:   portfolioID,
				EventType:     EventAllocationChanged,
				AccountID:     portfolioID,
				StrategyID:    w.StrategyID,
				Payload:       payload,
				Source:        "pms.allocation",
			})
			if err != nil {
				return err
			}
			if err := p.ApplyEvent(ev); err != nil {
				return err
			}
			if e.store != nil {
				e.store.Append(ctx, ev) //nolint:errcheck
			}
		}
	}
	return nil
}
