package phase27

import (
	"fmt"
	"sort"
)

// BuildChampionship ranks alpha families by a composite of PF, Sharpe, and Expectancy.
// Returns ranked AlphaChampionRecord slice.
func BuildChampionship(perfs []AlphaFamilyPerf) []AlphaChampionRecord {
	// Score each family
	type scored struct {
		perf  AlphaFamilyPerf
		score float64
	}

	var items []scored
	for _, p := range perfs {
		composite := p.ProfitFactor*0.4 + p.Sharpe*0.4 + clampExpectancy(p.Expectancy)*0.2
		items = append(items, scored{perf: p, score: composite})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].score > items[j].score
	})

	records := make([]AlphaChampionRecord, 0, len(items))
	for rank, item := range items {
		p := item.perf
		verdict := classifyChampionship(p)
		evidence := fmt.Sprintf(
			"Family=%s | Trades=%d | PF=%.3f | Sharpe=%.3f | Expectancy=%.4f | NetPnL=$%.2f | CompositeScore=%.4f",
			p.Family, p.Trades, p.ProfitFactor, p.Sharpe, p.Expectancy, p.NetPnLUSD, item.score,
		)
		records = append(records, AlphaChampionRecord{
			Rank:     rank + 1,
			Family:   p.Family,
			Verdict:  verdict,
			Evidence: evidence,
			Perf:     p,
		})
	}
	return records
}

func classifyChampionship(p AlphaFamilyPerf) string {
	switch {
	case p.ProfitFactor >= 1.5 && p.Sharpe >= 2.0 && p.Expectancy > 0:
		return AlphaChampion
	case p.ProfitFactor >= 1.3 && p.Sharpe >= 1.5:
		return AlphaStrong
	case p.ProfitFactor >= 1.2 && p.Sharpe >= 1.0:
		return AlphaModerate
	case p.ProfitFactor >= 1.0:
		return AlphaWeak
	case p.NetPnLUSD < 0 && p.Trades >= 30:
		return AlphaEliminated
	default:
		return AlphaRetirementCandidate
	}
}

func clampExpectancy(e float64) float64 {
	if e < 0 {
		return 0
	}
	if e > 1000 {
		return 1000
	}
	return e
}
