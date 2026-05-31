package v2

type ExposureReport struct {
	LongExposureUSD  float64
	ShortExposureUSD float64
	NetExposureUSD   float64
	GrossExposureUSD float64
	NetExposurePct   float64
	GrossExposurePct float64
	ByStrategy       map[string]float64
	ByFamily         map[StrategyFamily]float64
	ByRegime         map[MarketRegime]float64
	BySignalType     map[string]float64
	FamilyBreach     bool
	RegimeBreach     bool
}

func CalculateExposure(portfolio Portfolio, req TradeRequest, proposedSizeBTC float64) ExposureReport {
	out := ExposureReport{
		ByStrategy:   make(map[string]float64),
		ByFamily:     make(map[StrategyFamily]float64),
		ByRegime:     make(map[MarketRegime]float64),
		BySignalType: make(map[string]float64),
	}
	add := func(symbol string, side Side, notional float64, strategy string, family StrategyFamily, regime MarketRegime, signalType string) {
		if side == SideShort {
			out.ShortExposureUSD += notional
			out.NetExposureUSD -= notional
		} else {
			out.LongExposureUSD += notional
			out.NetExposureUSD += notional
		}
		out.GrossExposureUSD += notional
		out.ByStrategy[strategy] += notional
		out.ByFamily[family] += notional
		out.ByRegime[regime] += notional
		out.BySignalType[signalType] += notional
	}
	for _, pos := range portfolio.Positions {
		add(pos.Symbol, pos.Side, PositionNotionalUSD(pos), pos.Strategy, pos.Family, pos.Regime, pos.SignalType)
	}
	add(req.Symbol, req.Side, req.EntryPrice*proposedSizeBTC, req.Strategy, req.Family, RegimeUnknown, req.SignalType)
	equity := max(portfolio.Account.EquityUSD, 1)
	out.NetExposurePct = abs(out.NetExposureUSD) / equity * 100
	out.GrossExposurePct = out.GrossExposureUSD / equity * 100
	for _, v := range out.ByFamily {
		if v/equity*100 > 30 {
			out.FamilyBreach = true
		}
	}
	for _, v := range out.ByRegime {
		if v/equity*100 > 45 {
			out.RegimeBreach = true
		}
	}
	return out
}
