package risk

type ExposureMetrics struct {
	LongExposureUSD  float64
	ShortExposureUSD float64
	NetExposureUSD   float64
	GrossExposureUSD float64
	Leverage         float64
}

func PortfolioExposure(positions map[string]RiskPosition, equityUSD float64) ExposureMetrics {
	var longExposure, shortExposure float64
	for _, p := range positions {
		notional := p.NotionalUSD
		if notional <= 0 {
			notional = p.EntryPrice * p.SizeBTC
		}
		if p.Side == SideShort {
			shortExposure += notional
		} else {
			longExposure += notional
		}
	}
	gross := longExposure + shortExposure
	leverage := 0.0
	if equityUSD > 0 {
		leverage = gross / equityUSD
	}
	return ExposureMetrics{
		LongExposureUSD:  longExposure,
		ShortExposureUSD: shortExposure,
		NetExposureUSD:   longExposure - shortExposure,
		GrossExposureUSD: gross,
		Leverage:         leverage,
	}
}
