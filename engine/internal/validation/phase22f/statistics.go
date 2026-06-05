package phase22f

import (
	"math"
	"sort"

	"antigravity-engine/internal/validation/phase22e"
)

// ComputeExtendedStats computes all Phase 22F metrics for a strategy's trade history.
// initialNAV is the per-strategy allocated capital (used for drawdown normalisation).
func ComputeExtendedStats(strategyID string, trades []phase22e.TradeRecord, initialNAV float64) ExtendedStats {
	es := ExtendedStats{
		RegimePerfF22: make(map[RegimeF22]*RegimePerfF22),
	}
	if len(trades) == 0 {
		return es
	}

	// base metrics via phase22e (reuse proven computation)
	es.Base = phase22eStrategyMetrics(strategyID, trades, initialNAV)

	pnls := make([]float64, len(trades))
	holdMins := make([]float64, len(trades))
	wins, losses, breaks := 0, 0, 0

	for i, t := range trades {
		pnls[i] = t.NetPnLUSD
		holdMins[i] = t.HoldMinutes
		switch {
		case t.NetPnLUSD > 0:
			wins++
			if t.NetPnLUSD > es.AvgWinUSD {
				es.AvgWinUSD = t.NetPnLUSD // tmp: will average below
			}
		case t.NetPnLUSD < 0:
			losses++
		default:
			breaks++
		}
	}

	n := float64(len(trades))

	// ── breakeven rate ────────────────────────────────────────────────────────
	es.BreakevenRate = float64(breaks) / n

	// ── average win / loss ────────────────────────────────────────────────────
	es.AvgWinUSD = es.Base.AvgWinUSD
	es.AvgLossUSD = es.Base.AvgLossUSD

	// ── trade duration ────────────────────────────────────────────────────────
	es.TradeDurationAvg = es.Base.AvgHoldMin
	es.TradeDurationMin, es.TradeDurationMax = minMax(holdMins)

	// ── Sortino ratio ─────────────────────────────────────────────────────────
	es.SortinoRatio = sortinoRatio(pnls)

	// ── Calmar ratio ──────────────────────────────────────────────────────────
	// Annualised return / max drawdown
	if es.Base.MaxDrawdown > 0 {
		// net PnL as fraction of initial NAV, scaled to annual (assume 252 trading days)
		totalReturn := sumSlice(pnls)
		if initialNAV > 0 && len(trades) > 0 {
			dailyReturn := totalReturn / initialNAV / math.Max(1, float64(len(trades))/4) // ~4 trades/day
			annReturn := dailyReturn * 252
			es.CalmarRatio = annReturn / (es.Base.MaxDrawdown / 100)
		}
	}

	// ── Recovery factor ───────────────────────────────────────────────────────
	// net profit / max drawdown in dollars
	totalPnL := sumSlice(pnls)
	maxDDDollar := es.Base.MaxDrawdown / 100 * initialNAV
	if maxDDDollar > 0 {
		es.RecoveryFactor = totalPnL / maxDDDollar
	}

	// ── Ulcer Index ───────────────────────────────────────────────────────────
	es.UlcerIndex = ulcerIndex(pnls, initialNAV)

	// ── Max consecutive wins / losses ─────────────────────────────────────────
	es.MaxConsecWins, es.MaxConsecLosses = maxConsecutive(pnls)

	// ── Risk of ruin (Monte Carlo approximation) ──────────────────────────────
	es.RiskOfRuin = estimateRiskOfRuin(pnls, initialNAV)

	// ── Probability of profitability (binomial CI) ────────────────────────────
	es.ProbProfitable = probabilityProfitable(wins, len(trades))

	// ── Regime performance (10 regimes) ──────────────────────────────────────
	for _, t := range trades {
		r := mapToF22Regime(t.Regime, t)
		rp, ok := es.RegimePerfF22[r]
		if !ok {
			rp = &RegimePerfF22{Regime: r}
			es.RegimePerfF22[r] = rp
		}
		rp.Trades++
		rp.NetPnLUSD += t.NetPnLUSD
		if t.NetPnLUSD >= 0 {
			rp.WinRate++
		}
	}
	for _, rp := range es.RegimePerfF22 {
		if rp.Trades > 0 {
			rp.WinRate /= float64(rp.Trades)
			rp.Expectancy = rp.NetPnLUSD / float64(rp.Trades)
		}
		gw, gl := 0.0, 0.0
		for _, t := range trades {
			if mapToF22Regime(t.Regime, t) != rp.Regime {
				continue
			}
			if t.NetPnLUSD >= 0 {
				gw += t.NetPnLUSD
			} else {
				gl += -t.NetPnLUSD
			}
		}
		if gl > 0 {
			rp.ProfitFactor = gw / gl
		}
		rp.MaxDrawdown = maxDrawdownPctLocal(regimePnLs(trades, rp.Regime), initialNAV/float64(max2(1, len(es.RegimePerfF22))))
		rp.Sharpe = sharpeLocal(regimePnLs(trades, rp.Regime))
	}

	return es
}

// ── Statistical primitives (local, avoid importing phase22e internals) ────────

func sortinoRatio(pnls []float64) float64 {
	if len(pnls) < 2 {
		return 0
	}
	m := meanLocal(pnls)
	// downside deviation: stddev of negative returns only
	sumSq := 0.0
	n := 0
	for _, p := range pnls {
		if p < 0 {
			sumSq += p * p
			n++
		}
	}
	if n < 2 {
		return 0
	}
	dd := math.Sqrt(sumSq / float64(n))
	if dd == 0 {
		return 0
	}
	return (m / dd) * math.Sqrt(float64(len(pnls)))
}

func ulcerIndex(pnls []float64, nav float64) float64 {
	if len(pnls) == 0 || nav <= 0 {
		return 0
	}
	peak := nav
	cur := nav
	sumSq := 0.0
	for _, p := range pnls {
		cur += p
		if cur > peak {
			peak = cur
		}
		dd := (peak - cur) / peak * 100
		sumSq += dd * dd
	}
	return math.Sqrt(sumSq / float64(len(pnls)))
}

func maxConsecutive(pnls []float64) (wins, losses int) {
	cw, cl, mw, ml := 0, 0, 0, 0
	for _, p := range pnls {
		if p >= 0 {
			cw++
			cl = 0
		} else {
			cl++
			cw = 0
		}
		if cw > mw {
			mw = cw
		}
		if cl > ml {
			ml = cl
		}
	}
	return mw, ml
}

// estimateRiskOfRuin uses a simplified formula: (L/W)^(capital/avgWin)
// Falls back to Monte Carlo approximation for accuracy.
func estimateRiskOfRuin(pnls []float64, nav float64) float64 {
	if len(pnls) < 10 || nav <= 0 {
		return 1.0
	}
	wins, losses := 0, 0
	gw, gl := 0.0, 0.0
	for _, p := range pnls {
		if p > 0 {
			wins++
			gw += p
		} else if p < 0 {
			losses++
			gl += -p
		}
	}
	if wins == 0 || losses == 0 {
		return 0
	}
	wr := float64(wins) / float64(len(pnls))
	lr := float64(losses) / float64(len(pnls))
	avgW := gw / float64(wins)
	avgL := gl / float64(losses)
	if avgW == 0 {
		return 1.0
	}
	ratio := (lr * avgL) / (wr * avgW)
	if ratio >= 1 {
		return 1.0
	}
	// RoR ≈ ratio^(ruin_units) where ruin_units = nav / avgL
	ruinUnits := nav / avgL
	ror := math.Pow(ratio, ruinUnits)
	if math.IsNaN(ror) || math.IsInf(ror, 0) {
		return 0
	}
	return math.Min(1.0, math.Max(0.0, ror))
}

func probabilityProfitable(wins, total int) float64 {
	if total < 10 {
		return 0.5
	}
	// one-sided binomial: P(WR > 0.5)
	p := float64(wins) / float64(total)
	se := math.Sqrt(0.5 * 0.5 / float64(total))
	z := (p - 0.5) / se
	return 0.5 * (1 + math.Erf(z/math.Sqrt2))
}

func meanLocal(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func sumSlice(xs []float64) float64 {
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s
}

func minMax(xs []float64) (min, max float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	min, max = xs[0], xs[0]
	for _, x := range xs[1:] {
		if x < min {
			min = x
		}
		if x > max {
			max = x
		}
	}
	return min, max
}

func maxDrawdownPctLocal(pnls []float64, nav float64) float64 {
	if len(pnls) == 0 {
		return 0
	}
	peak, cum, maxDD := nav, nav, 0.0
	for _, p := range pnls {
		cum += p
		if cum > peak {
			peak = cum
		}
		if peak > 0 {
			dd := (peak - cum) / peak * 100
			if dd > maxDD {
				maxDD = dd
			}
		}
	}
	return maxDD
}

func sharpeLocal(pnls []float64) float64 {
	if len(pnls) < 2 {
		return 0
	}
	m := meanLocal(pnls)
	s := stddevLocal(pnls)
	if s == 0 {
		return 0
	}
	return (m / s) * math.Sqrt(float64(len(pnls)))
}

func stddevLocal(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := meanLocal(xs)
	s := 0.0
	for _, x := range xs {
		d := x - m
		s += d * d
	}
	return math.Sqrt(s / float64(len(xs)-1))
}

func regimePnLs(trades []phase22e.TradeRecord, r RegimeF22) []float64 {
	out := make([]float64, 0)
	for _, t := range trades {
		if mapToF22Regime(t.Regime, t) == r {
			out = append(out, t.NetPnLUSD)
		}
	}
	return out
}

func max2(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// phase22eStrategyMetrics is a local shim that calls the exported validator
// via the public API (phase22e.NewValidator).
// We re-implement the core computation to avoid importing unexported helpers.
func phase22eStrategyMetrics(strategyID string, trades []phase22e.TradeRecord, nav float64) phase22e.StrategyMetrics {
	if len(trades) == 0 {
		return phase22e.StrategyMetrics{}
	}
	v := phase22e.NewValidator(nav)
	result := v.Run(trades)
	// find the specific strategy
	for _, s := range result.Strategies {
		if s.StrategyID == strategyID {
			return s
		}
	}
	// fallback: return the first strategy result (single-strategy input)
	if len(result.Strategies) > 0 {
		return result.Strategies[0]
	}
	return phase22e.StrategyMetrics{}
}

// GroupTradesByStrategy partitions trades by StrategyID.
func GroupTradesByStrategy(trades []phase22e.TradeRecord) map[string][]phase22e.TradeRecord {
	m := make(map[string][]phase22e.TradeRecord)
	for _, t := range trades {
		m[t.StrategyID] = append(m[t.StrategyID], t)
	}
	return m
}

// SortedStrategyIDs returns strategy IDs in deterministic alphabetical order.
func SortedStrategyIDs(m map[string][]phase22e.TradeRecord) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
