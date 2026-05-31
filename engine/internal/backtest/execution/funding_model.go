package execution

import "time"

type FundingRate struct {
	Rate      float64
	Timestamp time.Time
}

type FundingAttribution struct {
	FundingPaidUSD     float64
	FundingReceivedUSD float64
	FundingPnLUSD      float64
	FundingDragPct     float64
}

type FundingModel struct {
	Interval time.Duration
	Rates    []FundingRate
}

func NewFundingModel(rates []FundingRate) FundingModel {
	return FundingModel{Interval: 8 * time.Hour, Rates: rates}
}

func (m FundingModel) Apply(side OrderSide, notionalUSD float64, entry, exit time.Time) FundingAttribution {
	interval := m.Interval
	if interval <= 0 {
		interval = 8 * time.Hour
	}
	var out FundingAttribution
	for _, r := range m.Rates {
		if r.Timestamp.Before(entry) || r.Timestamp.After(exit) {
			continue
		}
		cashflow := notionalUSD * r.Rate
		if side == SideBuy {
			if cashflow > 0 {
				out.FundingPaidUSD += cashflow
			} else {
				out.FundingReceivedUSD += -cashflow
			}
		} else {
			if cashflow > 0 {
				out.FundingReceivedUSD += cashflow
			} else {
				out.FundingPaidUSD += -cashflow
			}
		}
	}
	out.FundingPnLUSD = out.FundingReceivedUSD - out.FundingPaidUSD
	if notionalUSD > 0 {
		out.FundingDragPct = -out.FundingPnLUSD / notionalUSD * 100
	}
	return out
}
