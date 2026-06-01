package v3

import (
	"fmt"
	"math"
	"time"
)

// StressScenario defines a discrete stress event injection.
type StressScenario struct {
	Type            StressScenarioType
	Name            string
	Duration        time.Duration
	PriceDropPct    float64 // for flash crash: percentage drop
	FundingRatePct  float64 // for funding shock: rate per period
	SpreadMultiplier float64
	DepthMultiplier  float64
	OrdersBlocked   bool    // for exchange outage
	Description     string
}

var scenarioCatalog = map[StressScenarioType]StressScenario{
	StressFlashCrash: {
		Type:             StressFlashCrash,
		Name:             "Flash Crash",
		Duration:         5 * time.Minute,
		PriceDropPct:     -15,
		SpreadMultiplier: 12,
		DepthMultiplier:  0.05,
		Description:      "15% price drop in 5 minutes, spread widens 12×, 95% depth evaporation",
	},
	StressExchangeOutage: {
		Type:          StressExchangeOutage,
		Name:          "Exchange Outage",
		Duration:      30 * time.Minute,
		OrdersBlocked: true,
		Description:   "Exchange unreachable for 30 minutes — all pending orders rejected",
	},
	StressLiquidityCollapse: {
		Type:             StressLiquidityCollapse,
		Name:             "Liquidity Collapse",
		Duration:         20 * time.Minute,
		SpreadMultiplier: 20,
		DepthMultiplier:  0.03,
		Description:      "Book depth collapses to 3%, spreads widen 20×",
	},
	StressFundingShock: {
		Type:            StressFundingShock,
		Name:            "Funding Shock",
		Duration:        24 * time.Hour,
		FundingRatePct:  0.3, // 0.3% per 8h period
		Description:     "Funding rate spikes to 0.3% per 8h for 24h",
	},
	StressExtremeVol: {
		Type:             StressExtremeVol,
		Name:             "Extreme Volatility",
		Duration:         4 * time.Hour,
		SpreadMultiplier: 5,
		DepthMultiplier:  0.20,
		Description:      "Annualized vol > 400%, spreads 5×, depth 20%",
	},
	StressLiqCascade: {
		Type:             StressLiqCascade,
		Name:             "Liquidation Cascade",
		Duration:         3 * time.Hour,
		PriceDropPct:     -12,
		SpreadMultiplier: 8,
		DepthMultiplier:  0.08,
		Description:      "Sequential forced liquidations — 12% drop, 8× spreads",
	},
}

// ActiveStressEvent is a stress scenario active during a time window.
type ActiveStressEvent struct {
	Scenario  StressScenario
	StartTime time.Time
	EndTime   time.Time
}

func (e *ActiveStressEvent) IsActive(ts time.Time) bool {
	return !ts.Before(e.StartTime) && ts.Before(e.EndTime)
}

// StressEngine manages stress event injection during a backtest replay.
type StressEngine struct {
	events []*ActiveStressEvent
}

// NewStressEngine creates a stress engine with the given scenario types.
// Events are staggered 2 hours apart from startTime.
func NewStressEngine(types []StressScenarioType, startTime time.Time) *StressEngine {
	engine := &StressEngine{}
	offset := time.Duration(0)
	for _, st := range types {
		sc, ok := scenarioCatalog[st]
		if !ok {
			continue
		}
		start := startTime.Add(offset)
		engine.events = append(engine.events, &ActiveStressEvent{
			Scenario:  sc,
			StartTime: start,
			EndTime:   start.Add(sc.Duration),
		})
		offset += sc.Duration + 2*time.Hour
	}
	return engine
}

// AddAt schedules a specific scenario at an exact start time.
func (s *StressEngine) AddAt(scenarioType StressScenarioType, startTime time.Time) error {
	sc, ok := scenarioCatalog[scenarioType]
	if !ok {
		return fmt.Errorf("unknown stress scenario: %s", scenarioType)
	}
	s.events = append(s.events, &ActiveStressEvent{
		Scenario:  sc,
		StartTime: startTime,
		EndTime:   startTime.Add(sc.Duration),
	})
	return nil
}

// ActiveAt returns all stress events active at the given timestamp.
func (s *StressEngine) ActiveAt(ts time.Time) []StressScenario {
	result := make([]StressScenario, 0)
	for _, e := range s.events {
		if e.IsActive(ts) {
			result = append(result, e.Scenario)
		}
	}
	return result
}

// IsOrderBlocked returns true if any active scenario blocks order submission.
func (s *StressEngine) IsOrderBlocked(ts time.Time) bool {
	for _, sc := range s.ActiveAt(ts) {
		if sc.OrdersBlocked {
			return true
		}
	}
	return false
}

// CompositeMultipliers returns the worst-case combined spread and depth multipliers
// across all active stress scenarios.
func (s *StressEngine) CompositeMultipliers(ts time.Time) (spreadMult, depthMult float64) {
	spreadMult = 1.0
	depthMult = 1.0
	for _, sc := range s.ActiveAt(ts) {
		if sc.SpreadMultiplier > spreadMult {
			spreadMult = sc.SpreadMultiplier
		}
		if sc.DepthMultiplier > 0 && sc.DepthMultiplier < depthMult {
			depthMult = sc.DepthMultiplier
		}
	}
	return
}

// PriceAdjustment returns the cumulative price impact from active crash scenarios.
func (s *StressEngine) PriceAdjustment(ts time.Time) float64 {
	adj := 0.0
	for _, sc := range s.ActiveAt(ts) {
		adj += sc.PriceDropPct
	}
	return adj
}

// StrategyStressResult measures how a strategy performs under a stress scenario.
type StrategyStressResult struct {
	Scenario         StressScenario
	TradesBlocked    int
	TradesPartial    int
	TradesCompleted  int
	PnLDuringStress  float64
	MaxDrawdownPct   float64
	SurvivalScore    float64 // 0–100
	Description      string
}

// StressReport aggregates results across all stress scenarios.
type StressReport struct {
	ScenarioResults     []StrategyStressResult
	PortfolioSurvival   float64 // 0–100
	WorstScenario       string
	WorstPnL            float64
	WorstDrawdownPct    float64
	TotalBlockedTrades  int
}

// BuildStressReport computes a stress report from trade results under stress windows.
func BuildStressReport(trades []V3Trade, events []*ActiveStressEvent) StressReport {
	report := StressReport{}
	if len(events) == 0 || len(trades) == 0 {
		report.PortfolioSurvival = 100
		return report
	}

	byScenario := make(map[string]*StrategyStressResult)
	for _, ev := range events {
		key := string(ev.Scenario.Type)
		if _, ok := byScenario[key]; !ok {
			byScenario[key] = &StrategyStressResult{
				Scenario:    ev.Scenario,
				Description: ev.Scenario.Description,
			}
		}
	}

	worstPnL := 0.0
	worstDD := 0.0

	for _, t := range trades {
		for _, ev := range events {
			if ev.IsActive(t.OpenedAt) || ev.IsActive(t.ClosedAt) {
				key := string(ev.Scenario.Type)
				r := byScenario[key]
				r.PnLDuringStress += t.NetPnL
				r.TradesCompleted++
				if t.NetPnL < worstPnL {
					worstPnL = t.NetPnL
				}
			}
		}
	}

	totalSurvival := 0.0
	count := 0
	for _, r := range byScenario {
		dd := maxDrawdownFromPnL(tradePnLsDuringEvent(trades, events, r.Scenario.Type))
		r.MaxDrawdownPct = dd
		survival := math.Max(0, 100-(dd*2)-(math.Abs(r.PnLDuringStress)/10_000))
		r.SurvivalScore = math.Min(100, survival)
		totalSurvival += r.SurvivalScore
		count++
		if dd > worstDD {
			worstDD = dd
			report.WorstScenario = r.Scenario.Name
		}
		report.ScenarioResults = append(report.ScenarioResults, *r)
	}

	if count > 0 {
		report.PortfolioSurvival = totalSurvival / float64(count)
	}
	report.WorstPnL = worstPnL
	report.WorstDrawdownPct = worstDD
	return report
}

func tradePnLsDuringEvent(trades []V3Trade, events []*ActiveStressEvent, stype StressScenarioType) []float64 {
	var pnls []float64
	for _, ev := range events {
		if StressScenarioType(ev.Scenario.Type) != stype {
			continue
		}
		for _, t := range trades {
			if ev.IsActive(t.OpenedAt) {
				pnls = append(pnls, t.NetPnL)
			}
		}
	}
	return pnls
}

func maxDrawdownFromPnL(pnls []float64) float64 {
	if len(pnls) == 0 {
		return 0
	}
	peak, equity, maxDD := pnls[0], pnls[0], 0.0
	for _, p := range pnls[1:] {
		equity += p
		if equity > peak {
			peak = equity
		}
		dd := peak - equity
		if dd > maxDD {
			maxDD = dd
		}
	}
	// For all-loss sequences peak may equal initial PnL (negative); report as absolute USD drawdown pct.
	base := math.Abs(peak)
	if base > 0 {
		return maxDD / base * 100
	}
	return maxDD
}
