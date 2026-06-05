package phase24

import (
	"math"

	"antigravity-engine/internal/validation/phase23b"
)

const (
	annualTradingPeriods = 252.0 // assumed trades/year for annualisation
	riskFreeAnnual       = 0.05  // 5% annual risk-free rate
)

// ComputeExtendedMetrics builds a complete ExtendedMetrics from a slice of
// certified trades and the corresponding equity curve from the replay.
func ComputeExtendedMetrics(stratName, family, alphaSource string, tf Timeframe,
	trades []phase23b.CertifiedTrade, equityCurve []float64,
	initialCapital float64, tradingDays int, cagrPct float64) ExtendedMetrics {

	m := ExtendedMetrics{
		StrategyName: stratName,
		Family:       family,
		AlphaSource:  alphaSource,
		Timeframe:    tf,
		CAGR:         cagrPct,
	}
	if len(trades) == 0 {
		return m
	}

	m.TotalTrades = len(trades)
	m.StatSig = m.TotalTrades >= 200

	wins, losses := 0, 0
	sumWin, sumLoss := 0.0, 0.0
	largestWin, largestLoss := 0.0, 0.0
	totalHold := 0.0
	var netPnLs []float64
	peak := 0.0
	equity := 0.0
	maxDDPct := 0.0
	maxDDUSD := 0.0
	peakUSD := 0.0

	for _, t := range trades {
		netPnLs = append(netPnLs, t.NetPnLUSD)
		equity += t.NetPnLUSD
		if equity > peakUSD {
			peakUSD = equity
		}
		ddUSD := peakUSD - equity
		if ddUSD > maxDDUSD {
			maxDDUSD = ddUSD
		}
		_ = peak
		// percentage drawdown against initial capital for comparability
		ddPct := 0.0
		if peakUSD+initialCapital > 0 {
			ddPct = ddUSD / (peakUSD + initialCapital) * 100
		}
		if ddPct > maxDDPct {
			maxDDPct = ddPct
		}
		totalHold += t.HoldMinutes
		if t.NetPnLUSD > 0 {
			wins++
			sumWin += t.NetPnLUSD
			if t.NetPnLUSD > largestWin {
				largestWin = t.NetPnLUSD
			}
		} else {
			losses++
			absLoss := math.Abs(t.NetPnLUSD)
			sumLoss += absLoss
			if absLoss > largestLoss {
				largestLoss = absLoss
			}
		}
	}

	n := float64(m.TotalTrades)
	m.WinRate = float64(wins) / n
	m.LossRate = float64(losses) / n
	m.GrossProfit = sumWin
	m.GrossLoss = sumLoss
	m.NetProfit = sumWin - sumLoss
	if sumLoss > 0 {
		m.ProfitFactor = sumWin / sumLoss
	}
	m.Expectancy = m.NetProfit / n
	m.AvgWin = 0
	if wins > 0 {
		m.AvgWin = sumWin / float64(wins)
	}
	m.AvgLoss = 0
	if losses > 0 {
		m.AvgLoss = sumLoss / float64(losses)
	}
	m.LargestWin = largestWin
	m.LargestLoss = largestLoss
	m.MaxDrawdown = maxDDPct
	m.AvgHoldMinutes = totalHold / n

	// Sharpe & Sortino
	m.Sharpe = sharpe24(netPnLs)
	m.Sortino = sortino24(netPnLs)

	// Calmar
	m.Calmar = calmar(cagrPct, maxDDPct)

	// Ulcer Index
	if len(equityCurve) > 0 {
		m.Ulcer = ulcerIndex(equityCurve)
	}

	// Recovery Factor
	m.RecoveryFactor = recoveryFactor(m.NetProfit, maxDDUSD)

	// Return/Drawdown
	if maxDDPct > 0 {
		totalReturnPct := m.NetProfit / initialCapital * 100
		m.ReturnDrawdown = totalReturnPct / maxDDPct
	}

	// Risk of Ruin
	m.RiskOfRuin = ror24(m.WinRate, m.ProfitFactor)

	// Consecutive wins/losses
	m.MaxConWins, m.MaxConLosses = maxConsecutive(trades)

	// Execution quality score (0–100)
	m.ExecQualityScore = execQualityScore(trades)

	// Stability score (0–100): composite of PF, Sharpe, WinRate
	m.StabilityScore = stabilityScore(m.ProfitFactor, m.Sharpe, m.WinRate, m.MaxDrawdown)

	return m
}

func sharpe24(pnls []float64) float64 {
	if len(pnls) < 2 {
		return 0
	}
	mean := stat_mean(pnls)
	std := stat_std(pnls)
	if std == 0 {
		return 0
	}
	rfPerTrade := riskFreeAnnual / annualTradingPeriods
	return (mean - rfPerTrade) / std * math.Sqrt(annualTradingPeriods)
}

func sortino24(pnls []float64) float64 {
	if len(pnls) < 2 {
		return 0
	}
	mean := stat_mean(pnls)
	downVar := 0.0
	for _, p := range pnls {
		if p < 0 {
			downVar += p * p
		}
	}
	downVar /= float64(len(pnls))
	downStd := math.Sqrt(downVar)
	if downStd == 0 {
		return 0
	}
	return mean / downStd * math.Sqrt(annualTradingPeriods)
}

func ror24(wr, pf float64) float64 {
	if wr <= 0 || wr >= 1 || pf <= 0 {
		return 1.0
	}
	lr := 1.0 - wr
	ratio := lr / wr
	if ratio >= 1 {
		return 1.0
	}
	ror := math.Pow(ratio, 20)
	if math.IsNaN(ror) || math.IsInf(ror, 0) {
		return 0
	}
	return ror
}

func execQualityScore(trades []phase23b.CertifiedTrade) float64 {
	if len(trades) == 0 {
		return 0
	}
	// Score based on TP hit rate vs SL rate
	tp, sl, timeout := 0, 0, 0
	for _, t := range trades {
		switch t.ExitReason {
		case phase23b.ExitTP:
			tp++
		case phase23b.ExitSL:
			sl++
		default:
			timeout++
		}
	}
	n := float64(len(trades))
	tpRate := float64(tp) / n
	slRate := float64(sl) / n
	// Quality = TP rate penalised by high timeout rate
	timeoutPenalty := float64(timeout) / n * 0.5
	score := (tpRate*0.7 + (1-slRate)*0.3 - timeoutPenalty) * 100
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func stabilityScore(pf, sharpe, wr, maxDD float64) float64 {
	pfScore := normClamp24(pf, 1.0, 2.5)
	sharpeScore := normClamp24(sharpe, 0, 3.0)
	wrScore := normClamp24(wr, 0.4, 0.7)
	ddScore := normClamp24(20.0-maxDD, 0, 20)
	return (pfScore*0.35 + sharpeScore*0.3 + wrScore*0.2 + ddScore*0.15) * 100
}

func normClamp24(v, minV, maxV float64) float64 {
	if maxV <= minV {
		return 0
	}
	n := (v - minV) / (maxV - minV)
	if n < 0 {
		return 0
	}
	if n > 1 {
		return 1
	}
	return n
}

// compositeScore24 produces a single ranking score for a strategy.
func compositeScore24(m ExtendedMetrics, wfScore, mcScore float64) float64 {
	pfScore := normClamp24(m.ProfitFactor, 1.0, 2.5)
	sharpeScore := normClamp24(m.Sharpe, 0, 3.0)
	expectScore := normClamp24(m.Expectancy, 0, 500)
	ddScore := normClamp24(20.0-m.MaxDrawdown, 0, 20)
	rorScore := normClamp24(1.0-m.RiskOfRuin, 0, 1.0)
	wfNorm := wfScore / 100.0
	mcNorm := mcScore / 100.0

	return (pfScore*0.25 + sharpeScore*0.20 + expectScore*0.15 +
		ddScore*0.12 + rorScore*0.10 + wfNorm*0.10 + mcNorm*0.08)
}

// stat_mean returns the arithmetic mean of a float64 slice.
func stat_mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

// stat_std returns the sample standard deviation.
func stat_std(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	mean := stat_mean(xs)
	v := 0.0
	for _, x := range xs {
		d := x - mean
		v += d * d
	}
	return math.Sqrt(v / float64(len(xs)-1))
}
