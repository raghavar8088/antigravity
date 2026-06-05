package phase25

import (
	"sort"
)

// BuildAlphaSurvival aggregates live trades by alpha family and produces a
// ranked survival analysis for all 11 alpha engine categories.
func BuildAlphaSurvival(trades []LiveTrade, tradingDays float64) []AlphaSurvivalRecord {
	byFamily := make(map[AlphaFamily][]LiveTrade)
	for _, t := range trades {
		byFamily[t.AlphaFamily] = append(byFamily[t.AlphaFamily], t)
	}

	// Ensure all families are represented (even with 0 trades)
	for _, f := range AllAlphaFamilies {
		if _, ok := byFamily[f]; !ok {
			byFamily[f] = nil
		}
	}

	records := make([]AlphaSurvivalRecord, 0, len(AllAlphaFamilies))
	for _, family := range AllAlphaFamilies {
		familyTrades := byFamily[family]
		rec := buildFamilyRecord(family, familyTrades, tradingDays)
		records = append(records, rec)
	}

	// Rank by composite: PF 40% + Sharpe 30% + Expectancy 20% + ExecQuality 10%
	for i := range records {
		records[i].ExecQualityScore = execQualityScore(byFamily[records[i].Family])
	}

	sort.Slice(records, func(i, j int) bool {
		si := alphaComposite(records[i])
		sj := alphaComposite(records[j])
		return si > sj
	})
	for i := range records {
		records[i].Rank = i + 1
		records[i].Verdict = alphaVerdict(records[i], i)
	}
	return records
}

func buildFamilyRecord(family AlphaFamily, trades []LiveTrade, tradingDays float64) AlphaSurvivalRecord {
	r := AlphaSurvivalRecord{Family: family}
	if len(trades) == 0 {
		r.Verdict = "NO_TRADES"
		return r
	}

	r.Trades = len(trades)
	wins, sumWin, sumLoss := 0, 0.0, 0.0
	for _, t := range trades {
		r.NetPnLUSD += t.NetPnLUSD
		if t.IsWin {
			wins++
			sumWin += t.NetPnLUSD
		} else {
			sumLoss += -t.NetPnLUSD
		}
	}
	r.WinRate = float64(wins) / float64(len(trades))
	if sumLoss > 0 {
		r.ProfitFactor = sumWin / sumLoss
	}

	pnls := make([]float64, len(trades))
	for i, t := range trades {
		pnls[i] = t.NetPnLUSD
	}
	r.Sharpe = sharpe25(pnls)
	r.Sortino = sortino25(pnls)
	r.MaxDD = maxDD25(pnls)
	r.Expectancy = r.NetPnLUSD / float64(len(trades))

	// Average win/loss for RoR
	avgWin, avgLoss := 0.0, 0.0
	if wins > 0 {
		avgWin = sumWin / float64(wins)
	}
	if lossCount := len(trades) - wins; lossCount > 0 {
		avgLoss = sumLoss / float64(lossCount)
	}
	r.RiskOfRuin = riskOfRuin25(r.WinRate, avgWin, avgLoss)

	// Capital efficiency: return per unit of max drawdown risk
	if r.MaxDD > 0 {
		r.CapitalEfficiency = r.NetPnLUSD / r.MaxDD
	}
	return r
}

func execQualityScore(trades []LiveTrade) float64 {
	if len(trades) == 0 {
		return 50
	}
	totalLatency := 0.0
	for _, t := range trades {
		totalLatency += t.LatencySignalSubmitMs + t.LatencySubmitAckMs + t.LatencyAckFillMs
	}
	avgMs := totalLatency / float64(len(trades))
	// Score: 100 if avg < 3ms, degrades linearly to 50 at 20ms
	score := 100 - (avgMs-3)/17*50
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score
}

func alphaComposite(r AlphaSurvivalRecord) float64 {
	return r.ProfitFactor*0.40 + r.Sharpe*0.30 + (r.Expectancy/1000)*0.20 + r.ExecQualityScore/100*0.10
}

func alphaVerdict(r AlphaSurvivalRecord, rank int) string {
	if r.Trades == 0 {
		return "NO_TRADES"
	}
	if rank == 0 {
		return "CHAMPION"
	}
	if r.ProfitFactor >= 1.40 && r.Sharpe >= 1.50 {
		return "STRONG"
	}
	if r.ProfitFactor >= 1.20 && r.Sharpe >= 1.00 {
		return "MODERATE"
	}
	if r.ProfitFactor >= 1.05 {
		return "WEAK"
	}
	return "RETIREMENT_CANDIDATE"
}

// ChampionAlpha returns the top-ranked alpha family.
func ChampionAlpha(records []AlphaSurvivalRecord) (AlphaFamily, string) {
	if len(records) == 0 {
		return FamilyMomentum, "no data"
	}
	c := records[0]
	return c.Family, c.Verdict
}

// WeakestAlpha returns the worst-ranked alpha family.
func WeakestAlpha(records []AlphaSurvivalRecord) (AlphaFamily, string) {
	if len(records) == 0 {
		return FamilyMomentum, "no data"
	}
	w := records[len(records)-1]
	return w.Family, w.Verdict
}
