package omsv3

import "antigravity-engine/internal/ledger"

// RiskProjectionV2 is the full granular read model for the risk engine's history.
// It extends the basic RiskProjection in projections.go with per-violation-type
// breakdowns, exposure history, and P&L reconstruction.
//
// Built deterministically from RISK + POSITION events via BuildRiskProjectionV2.
// This is the projection the compliance dashboard and post-trade audit use.
type RiskProjectionV2 struct {
	// Check summary
	TotalChecks  int     `json:"total_checks"`
	Approved     int     `json:"approved"`
	Blocked      int     `json:"blocked"`
	ApprovalRate float64 `json:"approval_rate"`

	// Violation breakdown
	TotalViolations         int `json:"total_violations"`
	ExposureViolations      int `json:"exposure_violations"`
	DrawdownViolations      int `json:"drawdown_violations"`
	DailyLossViolations     int `json:"daily_loss_violations"`
	MarginViolations        int `json:"margin_violations"`
	LeverageViolations      int `json:"leverage_violations"`
	ConcentrationViolations int `json:"concentration_violations"`
	CorrelationViolations   int `json:"correlation_violations"`
	PortfolioHeatBreaches   int `json:"portfolio_heat_breaches"`
	VaRBreaches             int `json:"var_breaches"`
	CVaRBreaches            int `json:"cvar_breaches"`

	// Kill switch
	KillSwitchHits    int  `json:"kill_switch_hits"`
	KillSwitchActive  bool `json:"kill_switch_active"`

	// Exposure (from last approved check)
	CurrentExposureBTC  float64 `json:"current_exposure_btc"`
	PeakExposureBTC     float64 `json:"peak_exposure_btc"`

	// PnL (from position close events)
	RealisedPnLUSD   float64 `json:"realised_pnl_usd"`
	TotalLossUSD     float64 `json:"total_loss_usd"`
	HighWatermarkUSD float64 `json:"high_watermark_usd"`
	MaxDrawdownUSD   float64 `json:"max_drawdown_usd"`
}

// BuildRiskProjectionV2 performs a single-pass scan of all account events and
// returns the full risk read model. O(n) where n = total event count.
func BuildRiskProjectionV2(events []ledger.Event) RiskProjectionV2 {
	var proj RiskProjectionV2
	equity := 0.0

	for _, e := range events {
		switch e.AggregateType {
		case ledger.AggregateRisk:
			applyRiskV2Event(&proj, e)

		case ledger.AggregatePosition:
			// Accumulate realised PnL from position close events.
			if e.EventType == ledger.EventPositionClosed || e.EventType == ledger.EventPositionLiquidated {
				var payload ledger.PositionClosedPayload
				if unmarshalSilent(e.Payload, &payload) {
					proj.RealisedPnLUSD += payload.NetPnLUSD
					equity += payload.NetPnLUSD
					if payload.NetPnLUSD < 0 {
						proj.TotalLossUSD += -payload.NetPnLUSD
					}
					if equity > proj.HighWatermarkUSD {
						proj.HighWatermarkUSD = equity
					}
					drawdown := proj.HighWatermarkUSD - equity
					if drawdown > proj.MaxDrawdownUSD {
						proj.MaxDrawdownUSD = drawdown
					}
				}
			}
		}
	}

	if proj.TotalChecks > 0 {
		proj.ApprovalRate = float64(proj.Approved) / float64(proj.TotalChecks)
	}
	return proj
}

func applyRiskV2Event(proj *RiskProjectionV2, e ledger.Event) {
	switch e.EventType {
	case ledger.EventRiskApproved:
		proj.TotalChecks++
		proj.Approved++
		var payload ledger.RiskCheckPayload
		if unmarshalSilent(e.Payload, &payload) {
			proj.CurrentExposureBTC = payload.ProposedExposureBTC
			if absF(payload.ProposedExposureBTC) > proj.PeakExposureBTC {
				proj.PeakExposureBTC = absF(payload.ProposedExposureBTC)
			}
		}

	case ledger.EventRiskBlocked:
		proj.TotalChecks++
		proj.Blocked++

	case ledger.EventRiskViolation:
		proj.TotalViolations++

	case ledger.EventExposureLimitExceeded:
		proj.TotalViolations++
		proj.ExposureViolations++

	case ledger.EventMaxDrawdownBreached:
		proj.TotalViolations++
		proj.DrawdownViolations++

	case ledger.EventRiskDailyLossLimitExceeded:
		proj.TotalViolations++
		proj.DailyLossViolations++

	case ledger.EventRiskMarginViolation:
		proj.TotalViolations++
		proj.MarginViolations++

	case ledger.EventRiskLeverageViolation:
		proj.TotalViolations++
		proj.LeverageViolations++

	case ledger.EventRiskConcentrationViolation:
		proj.TotalViolations++
		proj.ConcentrationViolations++

	case ledger.EventRiskCorrelationViolation:
		proj.TotalViolations++
		proj.CorrelationViolations++

	case ledger.EventPortfolioHeatExceeded:
		proj.TotalViolations++
		proj.PortfolioHeatBreaches++

	case ledger.EventVaRBreach:
		proj.TotalViolations++
		proj.VaRBreaches++

	case ledger.EventCVaRBreach:
		proj.TotalViolations++
		proj.CVaRBreaches++

	case ledger.EventKillSwitchTriggered:
		proj.KillSwitchHits++
		proj.KillSwitchActive = true

	case ledger.EventKillSwitchReleased:
		proj.KillSwitchActive = false
	}
}

func absF(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
