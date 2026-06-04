package phase22e

import (
	"math"
	"sort"
	"time"
)

// computePortfolioMetrics derives all portfolio-level statistics from trades.
// initialNAV is the portfolio starting value used to normalise drawdown.
func computePortfolioMetrics(trades []TradeRecord, initialNAV ...float64) PortfolioMetrics {
	m := PortfolioMetrics{
		TotalTrades: len(trades),
		RegimePerf:  make(map[Regime]*RegimePerformance),
	}
	if len(trades) == 0 {
		return m
	}

	m.Checkpoint500 = m.TotalTrades >= MinTrades500
	m.Checkpoint1000 = m.TotalTrades >= MinTrades1000

	wins, losses := 0, 0
	holdSum := 0.0
	netPnLs := make([]float64, 0, len(trades))

	for _, t := range trades {
		m.TotalFeesUSD += t.FeesUSD
		m.NetPnLUSD += t.NetPnLUSD
		holdSum += t.HoldMinutes
		netPnLs = append(netPnLs, t.NetPnLUSD)

		if t.NetPnLUSD >= 0 {
			wins++
			m.GrossWinUSD += t.NetPnLUSD
		} else {
			losses++
			m.GrossLossUSD += -t.NetPnLUSD
		}

		// regime breakdown
		rp, ok := m.RegimePerf[t.Regime]
		if !ok {
			rp = &RegimePerformance{Regime: t.Regime}
			m.RegimePerf[t.Regime] = rp
		}
		rp.Trades++
		rp.NetPnLUSD += t.NetPnLUSD
		if t.NetPnLUSD >= 0 {
			rp.WinRate++ // count wins; normalise after
		}
	}

	n := float64(m.TotalTrades)
	m.WinRate = float64(wins) / n
	m.AvgHoldMin = holdSum / n
	if wins > 0 {
		m.AvgWinUSD = m.GrossWinUSD / float64(wins)
	}
	if losses > 0 {
		m.AvgLossUSD = m.GrossLossUSD / float64(losses)
		m.ProfitFactor = m.GrossWinUSD / m.GrossLossUSD
	}
	m.Expectancy = m.NetPnLUSD / n

	// finalise regime rates
	for _, rp := range m.RegimePerf {
		if rp.Trades > 0 {
			rp.WinRate /= float64(rp.Trades)
			rp.Expectancy = rp.NetPnLUSD / float64(rp.Trades)
			grossW, grossL := 0.0, 0.0
			for _, t := range trades {
				if t.Regime != rp.Regime {
					continue
				}
				if t.NetPnLUSD >= 0 {
					grossW += t.NetPnLUSD
				} else {
					grossL += -t.NetPnLUSD
				}
			}
			if grossL > 0 {
				rp.ProfitFactor = grossW / grossL
			}
		}
	}

	// Sharpe on per-trade returns (proxy; true Sharpe uses daily equity)
	if len(netPnLs) >= 2 {
		meanR := mean(netPnLs)
		stdR := stddev(netPnLs)
		if stdR > 0 {
			// annualise assuming ~252 trading days, ~4 trades/day average
			m.Sharpe = (meanR / stdR) * math.Sqrt(float64(len(netPnLs)))
		}
	}

	// max drawdown on cumulative PnL series, normalised by initial NAV
	startNAV := 0.0
	if len(initialNAV) > 0 {
		startNAV = initialNAV[0]
	}
	m.MaxDrawdown = maxDrawdownPct(netPnLs, startNAV)

	// statistical significance (binomial z-test on WR ≠ 0.5)
	if m.TotalTrades >= 30 {
		p0 := 0.50
		observed := float64(wins) / n
		se := math.Sqrt(p0 * (1 - p0) / n)
		if se > 0 {
			z := math.Abs((observed - p0) / se)
			m.StatSig = z > 1.96 // p < 0.05 two-tailed
		}
	}

	// distribution metrics
	m.ReturnSkew = skewness(netPnLs)
	m.ReturnKurtosis = kurtosis(netPnLs)
	m.VaR95USD, m.CVaR95USD = varCVar(netPnLs, 0.05)

	// outlier analysis — top/bottom 5% contribution
	sorted := make([]float64, len(netPnLs))
	copy(sorted, netPnLs)
	sort.Float64s(sorted)
	topN := max1(1, len(sorted)/20)
	top5sum := 0.0
	for i := len(sorted) - topN; i < len(sorted); i++ {
		top5sum += sorted[i]
	}
	if m.GrossWinUSD > 0 {
		m.OutlierWinPct = top5sum / m.GrossWinUSD * 100
	}
	bot5sum := 0.0
	for i := 0; i < topN; i++ {
		bot5sum += -sorted[i]
	}
	if m.GrossLossUSD > 0 {
		m.OutlierLossPct = bot5sum / m.GrossLossUSD * 100
	}

	// monthly returns
	m.MonthlyReturns = computeMonthlyReturns(trades)
	m.ConsecPositiveMonths = longestConsecPositive(m.MonthlyReturns)

	return m
}

// computeStrategyMetrics computes per-strategy statistics.
// initialNAV is the per-strategy starting capital used to normalise drawdown.
func computeStrategyMetrics(strat string, trades []TradeRecord, initialNAV ...float64) StrategyMetrics {
	m := StrategyMetrics{
		RegimePerf: make(map[Regime]*RegimePerformance),
	}
	if len(trades) == 0 {
		return m
	}

	// set identity from first trade
	m.StrategyID = trades[0].StrategyID
	m.StrategyName = trades[0].StrategyName
	m.Family = trades[0].Family
	m.TotalTrades = len(trades)
	m.StatSig = m.TotalTrades >= MinStrategyTrades

	wins, losses := 0, 0
	netPnLs := make([]float64, 0, len(trades))
	holdSum := 0.0

	for _, t := range trades {
		holdSum += t.HoldMinutes
		netPnLs = append(netPnLs, t.NetPnLUSD)
		if t.NetPnLUSD >= 0 {
			wins++
			m.GrossWin += t.NetPnLUSD
			if t.NetPnLUSD > m.LargestWinUSD {
				m.LargestWinUSD = t.NetPnLUSD
			}
		} else {
			losses++
			m.GrossLoss += -t.NetPnLUSD
			if -t.NetPnLUSD > m.LargestLossUSD {
				m.LargestLossUSD = -t.NetPnLUSD
			}
		}

		// regime
		rp, ok := m.RegimePerf[t.Regime]
		if !ok {
			rp = &RegimePerformance{Regime: t.Regime}
			m.RegimePerf[t.Regime] = rp
		}
		rp.Trades++
		rp.NetPnLUSD += t.NetPnLUSD
		if t.NetPnLUSD >= 0 {
			rp.WinRate++
		}
	}

	n := float64(m.TotalTrades)
	m.WinRate = float64(wins) / n
	m.AvgHoldMin = holdSum / n
	if wins > 0 {
		m.AvgWinUSD = m.GrossWin / float64(wins)
	}
	if losses > 0 {
		m.AvgLossUSD = m.GrossLoss / float64(losses)
		m.ProfitFactor = m.GrossWin / m.GrossLoss
	}
	m.Expectancy = (m.GrossWin - m.GrossLoss) / n

	if len(netPnLs) >= 2 {
		stdR := stddev(netPnLs)
		if stdR > 0 {
			m.Sharpe = (mean(netPnLs) / stdR) * math.Sqrt(float64(len(netPnLs)))
		}
	}
	stratNAV := 0.0
	if len(initialNAV) > 0 {
		stratNAV = initialNAV[0]
	}
	m.MaxDrawdown = maxDrawdownPct(netPnLs, stratNAV)

	// tail ratio
	m.TailRatioWL = tailRatio(netPnLs)

	// Kelly
	if m.WinRate > 0 && m.AvgLossUSD > 0 && m.AvgWinUSD > 0 {
		m.KellyFraction = m.WinRate - (1-m.WinRate)*(m.AvgLossUSD/m.AvgWinUSD)
		if m.KellyFraction < 0 {
			m.KellyFraction = 0
		}
		m.HalfKelly = m.KellyFraction / 2
	}

	// finalise regime rates
	for _, rp := range m.RegimePerf {
		if rp.Trades > 0 {
			rp.WinRate /= float64(rp.Trades)
			rp.Expectancy = rp.NetPnLUSD / float64(rp.Trades)
			gw, gl := 0.0, 0.0
			for _, t := range trades {
				if t.Regime != rp.Regime {
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
		}
	}

	// live vs backtest split
	livePF, backPF, liveWR, backWR := liveVsBacktestPF(trades)
	m.LivePF, m.BacktestPF = livePF, backPF
	m.LiveWR, m.BacktestWR = liveWR, backWR
	if backPF > 0 {
		m.PFDegradation = (livePF - backPF) / backPF
	}
	m.LiveVsBacktestOK = m.PFDegradation >= -MaxLiveVsBacktestDeg

	return m
}

// ── Statistical primitives ────────────────────────────────────────────────────

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func stddev(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := mean(xs)
	s := 0.0
	for _, x := range xs {
		d := x - m
		s += d * d
	}
	return math.Sqrt(s / float64(len(xs)-1))
}

func skewness(xs []float64) float64 {
	n := float64(len(xs))
	if n < 3 {
		return 0
	}
	m, s := mean(xs), stddev(xs)
	if s == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range xs {
		sum += math.Pow((x-m)/s, 3)
	}
	return sum * n / ((n - 1) * (n - 2))
}

func kurtosis(xs []float64) float64 {
	n := float64(len(xs))
	if n < 4 {
		return 0
	}
	m, s := mean(xs), stddev(xs)
	if s == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range xs {
		sum += math.Pow((x-m)/s, 4)
	}
	k := sum * n * (n + 1) / ((n - 1) * (n - 2) * (n - 3))
	k -= 3 * (n - 1) * (n - 1) / ((n - 2) * (n - 3))
	return k
}

func varCVar(xs []float64, alpha float64) (varVal, cvarVal float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	sorted := make([]float64, len(xs))
	copy(sorted, xs)
	sort.Float64s(sorted)
	idx := int(math.Ceil(alpha*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	varVal = -sorted[idx]
	sum := 0.0
	cnt := 0
	for i := 0; i <= idx; i++ {
		sum += sorted[i]
		cnt++
	}
	if cnt > 0 {
		cvarVal = -sum / float64(cnt)
	}
	return varVal, cvarVal
}

func tailRatio(xs []float64) float64 {
	if len(xs) < 20 {
		return 0
	}
	sorted := make([]float64, len(xs))
	copy(sorted, xs)
	sort.Float64s(sorted)
	n := len(sorted)
	p95Win := sorted[int(math.Floor(0.95*float64(n)))]
	p05Loss := -sorted[int(math.Floor(0.05*float64(n)))]
	if p05Loss <= 0 {
		return 0
	}
	if p95Win < 0 {
		return 0
	}
	return p95Win / p05Loss
}

// maxDrawdownPct computes peak-to-trough drawdown as % of starting NAV.
// nav is the initial portfolio value (e.g. 1_000_000 for a $1M paper account).
// Falls back to P&L-relative computation when nav == 0.
func maxDrawdownPct(netPnLs []float64, nav ...float64) float64 {
	if len(netPnLs) == 0 {
		return 0
	}
	startNAV := 0.0
	if len(nav) > 0 && nav[0] > 0 {
		startNAV = nav[0]
	}
	peak, cum, maxDD := startNAV, startNAV, 0.0
	for _, p := range netPnLs {
		cum += p
		if cum > peak {
			peak = cum
		}
		base := peak
		if base <= 0 {
			base = startNAV
		}
		if base > 0 {
			dd := (peak - cum) / base * 100
			if dd > maxDD {
				maxDD = dd
			}
		}
	}
	return maxDD
}

func liveVsBacktestPF(trades []TradeRecord) (livePF, backtestPF, liveWR, backtestWR float64) {
	var lgw, lgl, bgw, bgl float64
	lw, ll, bw, bl := 0, 0, 0, 0
	for _, t := range trades {
		if t.IsLive {
			if t.NetPnLUSD >= 0 {
				lw++
				lgw += t.NetPnLUSD
			} else {
				ll++
				lgl += -t.NetPnLUSD
			}
		} else {
			if t.NetPnLUSD >= 0 {
				bw++
				bgw += t.NetPnLUSD
			} else {
				bl++
				bgl += -t.NetPnLUSD
			}
		}
	}
	if lgl > 0 {
		livePF = lgw / lgl
	}
	if bgl > 0 {
		backtestPF = bgw / bgl
	}
	if lw+ll > 0 {
		liveWR = float64(lw) / float64(lw+ll)
	}
	if bw+bl > 0 {
		backtestWR = float64(bw) / float64(bw+bl)
	}
	return livePF, backtestPF, liveWR, backtestWR
}

func computeMonthlyReturns(trades []TradeRecord) []MonthlyReturn {
	type key struct{ y int; m time.Month }
	buckets := make(map[key]*MonthlyReturn)
	for _, t := range trades {
		k := key{t.ExitTime.Year(), t.ExitTime.Month()}
		mr, ok := buckets[k]
		if !ok {
			mr = &MonthlyReturn{Year: k.y, Month: k.m}
			buckets[k] = mr
		}
		mr.NetPnL += t.NetPnLUSD
		mr.Trades++
	}
	out := make([]MonthlyReturn, 0, len(buckets))
	for _, mr := range buckets {
		mr.Positive = mr.NetPnL > 0
		out = append(out, *mr)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Year != out[j].Year {
			return out[i].Year < out[j].Year
		}
		return out[i].Month < out[j].Month
	})
	return out
}

func longestConsecPositive(months []MonthlyReturn) int {
	best, cur := 0, 0
	for _, m := range months {
		if m.Positive {
			cur++
			if cur > best {
				best = cur
			}
		} else {
			cur = 0
		}
	}
	return best
}

func max1(a, b int) int {
	if a > b {
		return a
	}
	return b
}
