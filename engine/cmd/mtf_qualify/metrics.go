package main

import (
	"math"
	"sort"
)

// Metrics is the full statistical picture of one population of trades.
//
// Deliberately more than win rate. This desk has been ranked on win rate before
// and it selected a roster that lost money at a 63% hit rate, because the
// losers were larger than the winners. Every field below exists to make some
// specific way of being wrong visible.
type Metrics struct {
	Trades   int     `json:"trades"`
	Wins     int     `json:"wins"`
	Losses   int     `json:"losses"`
	WinRate  float64 `json:"win_rate"`
	GrossPct float64 `json:"gross_pct"`
	NetPct   float64 `json:"net_pct"`
	FeesPct  float64 `json:"fees_pct"`
	FeeDrag  float64 `json:"fee_drag"`

	// Expectancy is the average NET return per trade. The single number that
	// decides whether trading this at all is better than not.
	Expectancy   float64 `json:"expectancy"`
	AvgR         float64 `json:"avg_r"`
	ProfitFactor float64 `json:"profit_factor"`

	AvgWin      float64 `json:"avg_win"`
	AvgLoss     float64 `json:"avg_loss"`
	LargestWin  float64 `json:"largest_win"`
	LargestLoss float64 `json:"largest_loss"`

	MaxDrawdownPct float64 `json:"max_drawdown_pct"`
	Sharpe         float64 `json:"sharpe"`
	Sortino        float64 `json:"sortino"`

	MaxConsecWins   int `json:"max_consec_wins"`
	MaxConsecLosses int `json:"max_consec_losses"`

	ExitTP  int `json:"exit_tp"`
	ExitSL  int `json:"exit_sl"`
	ExitTTL int `json:"exit_ttl"`

	// BreakevenWinRate is the hit rate this population would need to be flat,
	// given the average win and average loss it actually produced.
	//
	// The most useful number on the record: it converts "is 22% good?" into a
	// question with an answer. A 1:6 strategy needs roughly 1 in 7 to survive
	// costs, and knowing that is the difference between a bad strategy and a
	// misread one.
	BreakevenWinRate float64 `json:"breakeven_win_rate"`
}

// ComputeMetrics summarises a trade population.
func ComputeMetrics(trades []Trade) Metrics {
	m := Metrics{Trades: len(trades)}
	if len(trades) == 0 {
		return m
	}
	// Chronological, or the drawdown and streak figures describe a sequence
	// that never happened.
	ord := append([]Trade(nil), trades...)
	sort.Slice(ord, func(i, j int) bool { return ord[i].ClosedAt.Before(ord[j].ClosedAt) })

	var sumWin, sumLoss, sumR float64
	var equity, peak, maxDD float64
	var consecW, consecL int
	rets := make([]float64, 0, len(ord))

	for _, t := range ord {
		m.GrossPct += t.GrossPct
		m.NetPct += t.NetPct
		m.FeesPct += roundTripCostPct
		sumR += t.RMultiple
		rets = append(rets, t.NetPct)

		switch t.Reason {
		case "TP":
			m.ExitTP++
		case "SL":
			m.ExitSL++
		default:
			m.ExitTTL++
		}

		if t.NetPct > 0 {
			m.Wins++
			sumWin += t.NetPct
			consecW++
			consecL = 0
			if consecW > m.MaxConsecWins {
				m.MaxConsecWins = consecW
			}
			if t.NetPct > m.LargestWin {
				m.LargestWin = t.NetPct
			}
		} else {
			m.Losses++
			sumLoss += -t.NetPct
			consecL++
			consecW = 0
			if consecL > m.MaxConsecLosses {
				m.MaxConsecLosses = consecL
			}
			if t.NetPct < m.LargestLoss {
				m.LargestLoss = t.NetPct
			}
		}

		// Additive equity in percentage points. Compounding would make the
		// drawdown depend on the order of identical trades AND on the starting
		// balance, which is a property of the account rather than the strategy.
		equity += t.NetPct
		if equity > peak {
			peak = equity
		}
		if dd := peak - equity; dd > maxDD {
			maxDD = dd
		}
	}

	n := float64(len(ord))
	m.WinRate = float64(m.Wins) / n * 100
	m.Expectancy = m.NetPct / n
	m.AvgR = sumR / n
	m.MaxDrawdownPct = maxDD
	if m.Wins > 0 {
		m.AvgWin = sumWin / float64(m.Wins)
	}
	if m.Losses > 0 {
		m.AvgLoss = sumLoss / float64(m.Losses)
	}
	if sumLoss > 0 {
		m.ProfitFactor = sumWin / sumLoss
	} else if sumWin > 0 {
		// No losing trade yet. Reported as 0 with a sentinel meaning rather
		// than as 999: a huge profit factor on a handful of trades reads as the
		// best strategy on the board, and it is the least evidenced one.
		m.ProfitFactor = 0
	}
	if m.GrossPct > 0 {
		m.FeeDrag = m.FeesPct / m.GrossPct * 100
	}
	if m.AvgWin+m.AvgLoss > 0 {
		m.BreakevenWinRate = m.AvgLoss / (m.AvgWin + m.AvgLoss) * 100
	}

	mean := m.Expectancy
	var variance, downside float64
	for _, r := range rets {
		d := r - mean
		variance += d * d
		if r < 0 {
			downside += r * r
		}
	}
	if len(rets) > 1 {
		sd := math.Sqrt(variance / float64(len(rets)-1))
		if sd > 0 {
			// Per-trade Sharpe, NOT annualised. Annualising requires a trade
			// frequency, and these strategies span 1m to 1d — one number
			// scaled by nine different frequencies would not be comparable
			// across the very rows the leaderboard exists to compare.
			m.Sharpe = mean / sd
		}
		if dsd := math.Sqrt(downside / float64(len(rets))); dsd > 0 {
			m.Sortino = mean / dsd
		}
	}
	return m
}
