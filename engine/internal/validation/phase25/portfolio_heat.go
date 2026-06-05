package phase25

import (
	"fmt"
	"math"
	"sort"
)

// BuildPortfolioHeat measures portfolio-level risk concentration, alpha clustering,
// directional exposure, and strategy correlation from live trades.
func BuildPortfolioHeat(trades []LiveTrade, escalation []CapEscalationRecord) PortfolioHeat {
	heat := PortfolioHeat{
		AlphaConcentration: make(map[AlphaFamily]float64),
	}

	if len(trades) == 0 {
		return heat
	}

	// Capital allocation by strategy
	capByStrat := make(map[string]float64, len(escalation))
	totalCap := 0.0
	for _, e := range escalation {
		capByStrat[e.StrategyName] = e.RecommendedCapUSD
		totalCap += e.RecommendedCapUSD
	}

	// Alpha family allocation concentration
	alphaCapByFamily := make(map[AlphaFamily]float64)
	for _, e := range escalation {
		// Infer family from live trades
		family := FamilyMomentum
		for _, t := range trades {
			if t.StrategyName == e.StrategyName {
				family = t.AlphaFamily
				break
			}
		}
		alphaCapByFamily[family] += e.RecommendedCapUSD
	}
	if totalCap > 0 {
		for f, cap := range alphaCapByFamily {
			heat.AlphaConcentration[f] = cap / totalCap * 100
		}
	}

	// Capital concentration (top 1, top 3)
	capValues := make([]float64, 0, len(capByStrat))
	for _, c := range capByStrat {
		capValues = append(capValues, c)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(capValues)))

	if totalCap > 0 && len(capValues) > 0 {
		heat.CapConcentrationTop1 = capValues[0] / totalCap * 100
		top3 := 0.0
		for i := 0; i < 3 && i < len(capValues); i++ {
			top3 += capValues[i]
		}
		heat.CapConcentrationTop3 = top3 / totalCap * 100
	}

	// Direction exposure
	longPnL, shortPnL := 0.0, 0.0
	for _, t := range trades {
		if t.Direction == "LONG" {
			longPnL += math.Abs(t.NetPnLUSD)
		} else {
			shortPnL += math.Abs(t.NetPnLUSD)
		}
	}
	total := longPnL + shortPnL
	if total > 0 {
		heat.DirectionLong = longPnL / total * 100
		heat.DirectionShort = shortPnL / total * 100
	}

	// Strategy correlations (returns-based, top pairs)
	heat.StrategyCorrelations = computeStrategyCorrelations(trades)

	// Alpha correlations
	heat.AlphaCorrelations = computeAlphaCorrelations(trades)

	// Detect violations
	heat.Violations = detectViolations(heat)

	// Overall heat score (0–100)
	heat.TotalHeat = computeHeatScore(heat)

	// Mitigations
	heat.MitigationActions = buildMitigations(heat.Violations)

	return heat
}

func computeStrategyCorrelations(trades []LiveTrade) []CorrelationPair {
	pnlByStrat := make(map[string][]float64)
	for _, t := range trades {
		pnlByStrat[t.StrategyName] = append(pnlByStrat[t.StrategyName], t.NetPnLUSD)
	}

	strats := make([]string, 0, len(pnlByStrat))
	for s := range pnlByStrat {
		strats = append(strats, s)
	}
	sort.Strings(strats)

	var pairs []CorrelationPair
	for i := 0; i < len(strats); i++ {
		for j := i + 1; j < len(strats); j++ {
			corr := correlation(pnlByStrat[strats[i]], pnlByStrat[strats[j]])
			if math.Abs(corr) > 0.60 {
				pairs = append(pairs, CorrelationPair{A: strats[i], B: strats[j], Correlation: corr})
			}
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		return math.Abs(pairs[i].Correlation) > math.Abs(pairs[j].Correlation)
	})
	if len(pairs) > 20 {
		pairs = pairs[:20]
	}
	return pairs
}

func computeAlphaCorrelations(trades []LiveTrade) []CorrelationPair {
	pnlByFamily := make(map[AlphaFamily][]float64)
	for _, t := range trades {
		pnlByFamily[t.AlphaFamily] = append(pnlByFamily[t.AlphaFamily], t.NetPnLUSD)
	}

	families := make([]AlphaFamily, 0, len(pnlByFamily))
	for f := range pnlByFamily {
		families = append(families, f)
	}
	sort.Slice(families, func(i, j int) bool { return string(families[i]) < string(families[j]) })

	var pairs []CorrelationPair
	for i := 0; i < len(families); i++ {
		for j := i + 1; j < len(families); j++ {
			corr := correlation(pnlByFamily[families[i]], pnlByFamily[families[j]])
			if math.Abs(corr) > 0.50 {
				pairs = append(pairs, CorrelationPair{
					A:           string(families[i]),
					B:           string(families[j]),
					Correlation: corr,
				})
			}
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		return math.Abs(pairs[i].Correlation) > math.Abs(pairs[j].Correlation)
	})
	return pairs
}

func detectViolations(h PortfolioHeat) []HeatViolation {
	var v []HeatViolation

	if h.CapConcentrationTop1 > 40 {
		v = append(v, HeatViolation{
			Type:       "CONCENTRATION",
			Description: fmt.Sprintf("Top strategy holds %.1f%% of capital (>40%% threshold)", h.CapConcentrationTop1),
			Severity:   "HIGH",
			Mitigation: "Redistribute capital — cap single strategy at 25%",
		})
	}
	if h.CapConcentrationTop3 > 70 {
		v = append(v, HeatViolation{
			Type:       "CONCENTRATION",
			Description: fmt.Sprintf("Top-3 strategies hold %.1f%% of capital (>70%% threshold)", h.CapConcentrationTop3),
			Severity:   "MEDIUM",
			Mitigation: "Broaden allocation across more strategies",
		})
	}
	for family, pct := range h.AlphaConcentration {
		if pct > 50 {
			v = append(v, HeatViolation{
				Type:       "ALPHA_CLUSTER",
				Description: fmt.Sprintf("Alpha family '%s' holds %.1f%% of capital (>50%% threshold)", family, pct),
				Severity:   "HIGH",
				Mitigation: "Diversify across uncorrelated alpha families",
			})
		}
	}
	if h.DirectionLong > 80 || h.DirectionShort > 80 {
		dir := "LONG"
		pct := h.DirectionLong
		if h.DirectionShort > 80 {
			dir = "SHORT"
			pct = h.DirectionShort
		}
		v = append(v, HeatViolation{
			Type:       "DIRECTION",
			Description: fmt.Sprintf("Portfolio is %.1f%% %s-biased (>80%% threshold)", pct, dir),
			Severity:   "MEDIUM",
			Mitigation: "Add strategies with opposite directional bias",
		})
	}
	for _, pair := range h.StrategyCorrelations {
		if pair.Correlation > 0.85 {
			v = append(v, HeatViolation{
				Type:       "CORRELATION",
				Description: fmt.Sprintf("Strategies %s / %s correlation=%.2f (>0.85)", pair.A, pair.B, pair.Correlation),
				Severity:   "HIGH",
				Mitigation: "Remove one of the two highly-correlated strategies",
			})
		}
	}
	return v
}

func computeHeatScore(h PortfolioHeat) float64 {
	score := 0.0
	if h.CapConcentrationTop1 > 40 {
		score += 30
	} else if h.CapConcentrationTop1 > 25 {
		score += 15
	}
	if h.CapConcentrationTop3 > 70 {
		score += 20
	} else if h.CapConcentrationTop3 > 55 {
		score += 10
	}
	dirBias := math.Abs(h.DirectionLong-50) / 50 * 20
	score += dirBias
	for _, pair := range h.StrategyCorrelations {
		if pair.Correlation > 0.80 {
			score += 5
		}
	}
	if score > 100 {
		score = 100
	}
	return score
}

func buildMitigations(violations []HeatViolation) []string {
	if len(violations) == 0 {
		return []string{"Portfolio heat within acceptable bounds — no action required"}
	}
	seen := make(map[string]bool)
	var actions []string
	for _, v := range violations {
		if !seen[v.Mitigation] {
			seen[v.Mitigation] = true
			actions = append(actions, fmt.Sprintf("[%s] %s", v.Severity, v.Mitigation))
		}
	}
	return actions
}

// correlation computes Pearson r between two float64 slices (min length pads shorter).
func correlation(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n < 2 {
		return 0
	}
	meanA, meanB := 0.0, 0.0
	for i := 0; i < n; i++ {
		meanA += a[i]
		meanB += b[i]
	}
	meanA /= float64(n)
	meanB /= float64(n)

	num, varA, varB := 0.0, 0.0, 0.0
	for i := 0; i < n; i++ {
		da := a[i] - meanA
		db := b[i] - meanB
		num += da * db
		varA += da * da
		varB += db * db
	}
	denom := math.Sqrt(varA * varB)
	if denom == 0 {
		return 0
	}
	return num / denom
}
