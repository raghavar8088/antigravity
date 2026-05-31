package risk

import "math"

type FundingRiskMetrics struct {
	FundingExposureUSD       float64
	ProjectedFundingCostUSD  float64
	LongFundingLiabilityUSD  float64
	ShortFundingLiabilityUSD float64
}

func FundingRisk(positions map[string]RiskPosition, fundingIntervalHours float64) FundingRiskMetrics {
	if fundingIntervalHours <= 0 {
		fundingIntervalHours = 8
	}
	var m FundingRiskMetrics
	for _, p := range positions {
		cost := math.Abs(p.NotionalUSD * p.FundingRate)
		m.FundingExposureUSD += p.NotionalUSD
		m.ProjectedFundingCostUSD += cost
		if p.Side == SideLong {
			m.LongFundingLiabilityUSD += cost
		} else {
			m.ShortFundingLiabilityUSD += cost
		}
	}
	return m
}

func CheckFundingRisk(order RiskOrder, limits RiskLimits) string {
	if limits.MaxFundingCostPct <= 0 || order.NotionalUSD <= 0 {
		return ""
	}
	costPct := math.Abs(order.FundingRate) * 100
	if costPct > limits.MaxFundingCostPct {
		return "funding cost limit breached"
	}
	return ""
}
