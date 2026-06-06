package phase27

import (
	"fmt"
	"math"
	"time"

	"antigravity-engine/internal/validation/phase25"
)

// BuildOverlapAnalysis computes pairwise trade and return correlations.
func BuildOverlapAnalysis(perfs []AlphaFamilyPerf, records []AlphaAttributionRecord) []OverlapRecord {
	if len(perfs) < 2 {
		return nil
	}

	// Build verdict tier map for redundancy check
	verdictMap := make(map[phase25.AlphaFamily]string)
	for _, c := range BuildChampionship(perfs) {
		verdictMap[c.Family] = c.Verdict
	}

	var result []OverlapRecord

	for i := 0; i < len(perfs); i++ {
		for j := i + 1; j < len(perfs); j++ {
			famA := perfs[i].Family
			famB := perfs[j].Family

			tradeCorr := computeTradeOverlap(famA, famB, records)
			retCorr := computeReturnCorrelation(famA, famB, records)

			redundant := retCorr > 0.85 && verdictMap[famA] == verdictMap[famB]

			evidence := fmt.Sprintf(
				"%s vs %s | TradeOverlap=%.2f%% | ReturnCorr=%.4f | Redundant=%v",
				famA, famB, tradeCorr, retCorr, redundant,
			)

			result = append(result, OverlapRecord{
				FamilyA:          famA,
				FamilyB:          famB,
				TradeCorrelation: tradeCorr,
				ReturnCorrelation: retCorr,
				Redundant:        redundant,
				Evidence:         evidence,
			})
		}
	}

	return result
}

// computeTradeOverlap estimates % of 1-hour windows where both families are active simultaneously.
func computeTradeOverlap(famA, famB phase25.AlphaFamily, records []AlphaAttributionRecord) float64 {
	// Build hourly activity sets
	hoursA := make(map[int64]bool)
	hoursB := make(map[int64]bool)

	epoch := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, r := range records {
		h := int64(r.EntryTime.Sub(epoch).Hours())
		if r.AlphaFamily == famA {
			hoursA[h] = true
		} else if r.AlphaFamily == famB {
			hoursB[h] = true
		}
	}

	if len(hoursA) == 0 || len(hoursB) == 0 {
		return 0
	}

	overlap := 0
	for h := range hoursA {
		if hoursB[h] {
			overlap++
		}
	}

	// Overlap relative to the smaller set
	denom := len(hoursA)
	if len(hoursB) < denom {
		denom = len(hoursB)
	}
	return float64(overlap) / float64(denom) * 100
}

// computeReturnCorrelation computes Pearson r of daily PnL series for two families.
func computeReturnCorrelation(famA, famB phase25.AlphaFamily, records []AlphaAttributionRecord) float64 {
	dailyA := buildDailyPnL(famA, records)
	dailyB := buildDailyPnL(famB, records)

	// Find common days
	var xa, xb []float64
	for day, pnlA := range dailyA {
		if pnlB, ok := dailyB[day]; ok {
			xa = append(xa, pnlA)
			xb = append(xb, pnlB)
		}
	}

	return pearsonR(xa, xb)
}

func buildDailyPnL(fam phase25.AlphaFamily, records []AlphaAttributionRecord) map[string]float64 {
	daily := make(map[string]float64)
	for _, r := range records {
		if r.AlphaFamily != fam {
			continue
		}
		day := r.EntryTime.Format("2006-01-02")
		daily[day] += r.NetPnLUSD
	}
	return daily
}

// pearsonR computes Pearson correlation coefficient.
func pearsonR(x, y []float64) float64 {
	n := len(x)
	if n < 2 || n != len(y) {
		return 0
	}

	var mx, my float64
	for i := 0; i < n; i++ {
		mx += x[i]
		my += y[i]
	}
	mx /= float64(n)
	my /= float64(n)

	var num, sx, sy float64
	for i := 0; i < n; i++ {
		dx := x[i] - mx
		dy := y[i] - my
		num += dx * dy
		sx += dx * dx
		sy += dy * dy
	}

	denom := math.Sqrt(sx * sy)
	if denom == 0 {
		return 0
	}
	return num / denom
}
