package riskv3

import (
	"encoding/json"
	"time"

	"antigravity-engine/internal/ledger"
)

// ─── Dashboard CQRS projections ───────────────────────────────────────────────
// All projections are built in a single pass over the event log.
// Time complexity: O(n) where n = total event count.
// They are rebuilt from scratch on every query — no caching.
// For production: maintain as materialised views updated by event streaming.

// PortfolioHeatProjection tracks portfolio heat over time.
type PortfolioHeatProjection struct {
	Current   float64     `json:"current_heat_pct"`
	Level     HeatLevel   `json:"level"`
	Peak      float64     `json:"peak_heat_pct"`
	History   []HeatPoint `json:"history"`   // last 100 snapshots
	Breaches  int         `json:"breaches"`  // times heat exceeded HeatWarningPct
	UpdatedAt time.Time   `json:"updated_at"`
}

// HeatPoint is one time-stamped heat observation in the history.
type HeatPoint struct {
	HeatPct   float64   `json:"heat_pct"`
	Level     HeatLevel `json:"level"`
	Timestamp time.Time `json:"timestamp"`
}

// VaRProjection tracks VaR metrics over time.
type VaRProjection struct {
	Current95Pct float64    `json:"current_var_95_pct"`
	Current99Pct float64    `json:"current_var_99_pct"`
	Peak95Pct    float64    `json:"peak_var_95_pct"`
	Breaches95   int        `json:"breaches_95"` // times VaR95 > MaxDailyVaR95Pct
	History      []VaRPoint `json:"history"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// VaRPoint is one time-stamped VaR observation.
type VaRPoint struct {
	VaR95Pct  float64   `json:"var_95_pct"`
	VaR99Pct  float64   `json:"var_99_pct"`
	Timestamp time.Time `json:"timestamp"`
}

// CVaRProjection tracks Expected Shortfall over time.
type CVaRProjection struct {
	Current95Pct float64     `json:"current_cvar_95_pct"`
	Current99Pct float64     `json:"current_cvar_99_pct"`
	Peak95Pct    float64     `json:"peak_cvar_95_pct"`
	Breaches95   int         `json:"breaches_95"`
	History      []CVaRPoint `json:"history"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// CVaRPoint is one time-stamped CVaR observation.
type CVaRPoint struct {
	CVaR95Pct float64   `json:"cvar_95_pct"`
	CVaR99Pct float64   `json:"cvar_99_pct"`
	Timestamp time.Time `json:"timestamp"`
}

// ExposureProjectionV3 tracks portfolio exposure over time.
type ExposureProjectionV3 struct {
	GrossExposurePct float64   `json:"gross_exposure_pct"`
	NetExposurePct   float64   `json:"net_exposure_pct"`
	OpenPositions    int       `json:"open_positions"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// CorrelationProjection tracks the maximum pairwise correlation.
type CorrelationProjection struct {
	MaxPairwiseCorr float64   `json:"max_pairwise_corr"`
	Breaches        int       `json:"breaches"` // times corr > MaxCorrelationCoeff
	UpdatedAt       time.Time `json:"updated_at"`
}

// ConcentrationProjection tracks the concentration score.
type ConcentrationProjection struct {
	MaxSymbolPct   float64   `json:"max_symbol_concentration_pct"`
	MaxExchangePct float64   `json:"max_exchange_concentration_pct"`
	Score          float64   `json:"concentration_score"`
	Breaches       int       `json:"breaches"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// PortfolioRiskSummary bundles all projections for the dashboard endpoint.
type PortfolioRiskSummary struct {
	Heat          PortfolioHeatProjection `json:"heat"`
	VaR           VaRProjection           `json:"var"`
	CVaR          CVaRProjection          `json:"cvar"`
	Exposure      ExposureProjectionV3    `json:"exposure"`
	Correlation   CorrelationProjection   `json:"correlation"`
	Concentration ConcentrationProjection `json:"concentration"`

	// Risk decision statistics (from RISK aggregate events)
	TotalChecks   int64   `json:"total_checks"`
	ApprovalRate  float64 `json:"approval_rate"`
	ViolationRate float64 `json:"violation_rate"`

	// Current composite score
	RiskScore int    `json:"risk_score"`
	ComputedAt time.Time `json:"computed_at"`
}

// BuildPortfolioRiskSummary scans all events for an account in a single pass
// and returns the complete portfolio risk dashboard projection.
func BuildPortfolioRiskSummary(events []ledger.Event) PortfolioRiskSummary {
	heat := &PortfolioHeatProjection{}
	varProj := &VaRProjection{}
	cvarProj := &CVaRProjection{}
	exposure := &ExposureProjectionV3{}
	corrProj := &CorrelationProjection{}
	concProj := &ConcentrationProjection{}

	var totalChecks, approved, violations int64
	var latestRiskScore int

	const maxHistory = 100

	for _, event := range events {
		if event.AggregateType != ledger.AggregateRisk {
			continue
		}

		switch event.EventType {
		case ledger.EventRiskApproved:
			totalChecks++
			approved++
			latestRiskScore = applyRiskCheckToProjections(event, heat, varProj, cvarProj, exposure, corrProj, concProj, maxHistory)

		case ledger.EventRiskBlocked, ledger.EventRiskViolation:
			totalChecks++
			violations++
			applyRiskCheckToProjections(event, heat, varProj, cvarProj, exposure, corrProj, concProj, maxHistory)

		case ledger.EventPortfolioHeatExceeded, ledger.EventVaRBreach,
			ledger.EventCVaRBreach, ledger.EventMaxDrawdownBreached,
			ledger.EventExposureLimitExceeded:
			violations++

		case ledger.EventRiskCheckStarted:
			// Portfolio snapshot event emitted by EmitPortfolioSnapshot
			applyPortfolioSnapshot(event, heat, varProj, cvarProj, exposure, corrProj, concProj, maxHistory)
		}
	}

	approvalRate := 0.0
	violationRate := 0.0
	if totalChecks > 0 {
		approvalRate = float64(approved) / float64(totalChecks) * 100
		violationRate = float64(violations) / float64(totalChecks) * 100
	}

	return PortfolioRiskSummary{
		Heat:          *heat,
		VaR:           *varProj,
		CVaR:          *cvarProj,
		Exposure:      *exposure,
		Correlation:   *corrProj,
		Concentration: *concProj,
		TotalChecks:   totalChecks,
		ApprovalRate:  approvalRate,
		ViolationRate: violationRate,
		RiskScore:     latestRiskScore,
		ComputedAt:    time.Now().UTC(),
	}
}

// ─── Internal projection appliers ─────────────────────────────────────────────

// applyRiskCheckToProjections updates all projections from a risk-check event
// and returns the latest composite risk score recorded in the payload.
func applyRiskCheckToProjections(
	event ledger.Event,
	heat *PortfolioHeatProjection,
	varProj *VaRProjection,
	cvarProj *CVaRProjection,
	exposure *ExposureProjectionV3,
	corrProj *CorrelationProjection,
	concProj *ConcentrationProjection,
	maxHistory int,
) int {
	var payload RiskCheckPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return 0
	}
	ts := event.CreatedAt

	// Heat
	heat.Current = payload.HeatPct
	heat.Level = ClassifyHeat(payload.HeatPct)
	if payload.HeatPct > heat.Peak {
		heat.Peak = payload.HeatPct
	}
	if payload.HeatPct >= HeatWarningPct {
		heat.Breaches++
	}
	heat.History = appendHeatPoint(heat.History, HeatPoint{payload.HeatPct, heat.Level, ts}, maxHistory)
	heat.UpdatedAt = ts

	// VaR
	varProj.Current95Pct = payload.VaR95Pct
	if payload.VaR95Pct > varProj.Peak95Pct {
		varProj.Peak95Pct = payload.VaR95Pct
	}
	if payload.VaR95Pct > MaxDailyVaR95Pct {
		varProj.Breaches95++
	}
	varProj.History = appendVaRPoint(varProj.History, VaRPoint{payload.VaR95Pct, 0, ts}, maxHistory)
	varProj.UpdatedAt = ts

	// CVaR
	cvarProj.Current95Pct = payload.CVaR95Pct
	if payload.CVaR95Pct > cvarProj.Peak95Pct {
		cvarProj.Peak95Pct = payload.CVaR95Pct
	}
	if payload.CVaR95Pct > MaxCVaR95Pct {
		cvarProj.Breaches95++
	}
	cvarProj.History = appendCVaRPoint(cvarProj.History, CVaRPoint{payload.CVaR95Pct, 0, ts}, maxHistory)
	cvarProj.UpdatedAt = ts

	// Correlation
	corrProj.MaxPairwiseCorr = payload.CorrelationRisk
	if payload.CorrelationRisk > MaxCorrelationCoeff {
		corrProj.Breaches++
	}
	corrProj.UpdatedAt = ts

	// Concentration
	concProj.Score = payload.ConcentrationScore
	if payload.ConcentrationScore > 80 {
		concProj.Breaches++
	}
	concProj.UpdatedAt = ts

	return payload.RiskScore
}

func applyPortfolioSnapshot(
	event ledger.Event,
	heat *PortfolioHeatProjection,
	varProj *VaRProjection,
	cvarProj *CVaRProjection,
	exposure *ExposureProjectionV3,
	corrProj *CorrelationProjection,
	concProj *ConcentrationProjection,
	maxHistory int,
) {
	var payload PortfolioSnapshotPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return
	}
	ts := event.CreatedAt

	heat.Current = payload.HeatPct
	heat.Level = ClassifyHeat(payload.HeatPct)
	if payload.HeatPct > heat.Peak {
		heat.Peak = payload.HeatPct
	}
	heat.History = appendHeatPoint(heat.History, HeatPoint{payload.HeatPct, heat.Level, ts}, maxHistory)
	heat.UpdatedAt = ts

	varProj.Current95Pct = payload.DailyVaR95Pct
	varProj.History = appendVaRPoint(varProj.History, VaRPoint{payload.DailyVaR95Pct, 0, ts}, maxHistory)
	varProj.UpdatedAt = ts

	cvarProj.Current95Pct = payload.CVaR95Pct
	cvarProj.History = appendCVaRPoint(cvarProj.History, CVaRPoint{payload.CVaR95Pct, 0, ts}, maxHistory)
	cvarProj.UpdatedAt = ts

	exposure.GrossExposurePct = payload.GrossExposurePct
	exposure.OpenPositions = payload.OpenPositions
	exposure.UpdatedAt = ts

	corrProj.MaxPairwiseCorr = payload.MaxCorrelation
	corrProj.UpdatedAt = ts

	concProj.MaxSymbolPct = payload.MaxSymbolConc
	concProj.UpdatedAt = ts
}

// ─── Append helpers with length cap ──────────────────────────────────────────

func appendHeatPoint(s []HeatPoint, p HeatPoint, max int) []HeatPoint {
	s = append(s, p)
	if len(s) > max {
		s = s[1:]
	}
	return s
}

func appendVaRPoint(s []VaRPoint, p VaRPoint, max int) []VaRPoint {
	s = append(s, p)
	if len(s) > max {
		s = s[1:]
	}
	return s
}

func appendCVaRPoint(s []CVaRPoint, p CVaRPoint, max int) []CVaRPoint {
	s = append(s, p)
	if len(s) > max {
		s = s[1:]
	}
	return s
}

