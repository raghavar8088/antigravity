package quality

import "sort"

type Inputs struct {
	Funding       float64
	CVD           float64
	Delta         float64
	Liquidity     float64
	OrderBlock    float64
	FVG           float64
	MSS           float64
	VolumeProfile float64
	Session       float64
}

type Component struct {
	Name   string
	Score  int
	Weight int
}

type Result struct {
	Score      int
	Category   string
	Components []Component
}

func Score(in Inputs) Result {
	components := []Component{
		comp("Funding", in.Funding, 15),
		comp("CVD", in.CVD, 20),
		comp("Delta", in.Delta, 10),
		comp("Liquidity", in.Liquidity, 15),
		comp("OrderBlock", in.OrderBlock, 10),
		comp("FVG", in.FVG, 10),
		comp("MSS", in.MSS, 10),
		comp("VolumeProfile", in.VolumeProfile, 5),
		comp("Session", in.Session, 5),
	}
	total := 0
	for _, c := range components {
		total += c.Score
	}
	return Result{Score: total, Category: Category(total), Components: components}
}

func Category(score int) string {
	switch {
	case score >= 90:
		return "Elite"
	case score >= 80:
		return "Institutional"
	case score >= 70:
		return "Tradable"
	case score >= 60:
		return "Watchlist"
	default:
		return "Reject"
	}
}

func MandatoryPass(score int) bool {
	return score >= 70
}

func comp(name string, value float64, weight int) Component {
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}
	return Component{Name: name, Score: int(value*float64(weight) + 0.5), Weight: weight}
}

func TopContributors(result Result, n int) []Component {
	cp := append([]Component(nil), result.Components...)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Score > cp[j].Score })
	if n > 0 && len(cp) > n {
		cp = cp[:n]
	}
	return cp
}
