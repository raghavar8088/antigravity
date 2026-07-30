// Package hunt scores the strategy hunt: every registered strategy runs on real
// Delta data with its own $1,000 account, and the ones that genuinely grow it
// become candidates for real money.
//
// Two things are kept deliberately separate, because conflating them is how a
// search over ~900 strategies funds a coin flip:
//
//   - The LEADERBOARD ranks by capital growth. It draws attention.
//   - The GATE decides eligibility. It authorises capital.
//
// With ~900 concurrent accounts, roughly half will be profitable by chance and
// the top handful will look outstanding. This codebase has already measured that
// twice: all 100 scalp strategies failed offline qualification 0/400, and two
// fee-honest sweeps lost on 17/17 and 66/66. So growth is a screen, never a
// promotion criterion, and ExpectedByChance reports how many of the survivors
// noise alone would have produced.
//
// Everything here is a pure function over closed trades. The desks already
// expose their trade history, so no engine's balance path is modified to make
// this work — the accounts are derived, which also makes them reproducible.
package hunt

import (
	"math"
	"sort"
	"time"
)

// DefaultStartingCapital is the per-strategy stake for the hunt.
const DefaultStartingCapital = 1000.0

// Trade is one closed paper trade, normalised across the three desks.
type Trade struct {
	Strategy string
	Symbol   string // "" for desks that trade a single underlying
	// NetPnL is AFTER fees — the only figure the gate reads. GrossPnL and Fees
	// are carried separately so fee drag is visible: an edge that exists only
	// before fees is not an edge, and this codebase has shipped that mistake.
	NetPnL   float64
	GrossPnL float64
	Fees     float64
	ClosedAt time.Time
}

// Key identifies one independent hypothesis. Scalp runs strategy x symbol, so a
// strategy that works on BTC and fails on DOGE is two results, not an average.
func (t Trade) Key() string {
	if t.Symbol == "" {
		return t.Strategy
	}
	return t.Strategy + "|" + t.Symbol
}

// Account is one strategy's $1,000 experiment.
type Account struct {
	Key      string `json:"key"`
	Strategy string `json:"strategy"`
	Symbol   string `json:"symbol,omitempty"`
	Desk     string `json:"desk"`

	StartingCapital float64 `json:"startingCapital"`
	Capital         float64 `json:"capital"`
	GrowthPct       float64 `json:"growthPct"`

	Trades int `json:"trades"`
	Wins   int `json:"wins"`
	Losses int `json:"losses"`

	NetPnL   float64 `json:"netPnl"`
	GrossPnL float64 `json:"grossPnl"`
	Fees     float64 `json:"fees"`
	// FeeDragPct is fees as a share of gross profit. Above 100% means the
	// strategy made money before costs and lost it to them.
	FeeDragPct float64 `json:"feeDragPct"`

	WinRate      float64 `json:"winRate"`
	ProfitFactor float64 `json:"profitFactor"`
	Expectancy   float64 `json:"expectancy"`
	MaxDrawdown  float64 `json:"maxDrawdown"`

	FirstTrade time.Time `json:"firstTrade"`
	LastTrade  time.Time `json:"lastTrade"`
	// DaysLive is the span of the record, not calendar age. A strategy that
	// traded twice a month apart has 30 days of span and 2 trades; the gate
	// requires both.
	DaysLive float64 `json:"daysLive"`

	// FirstHalfNet / SecondHalfNet split the record at its midpoint in TIME.
	// A strategy carried by one lucky streak fails this even when its total is
	// strongly positive.
	FirstHalfNet  float64 `json:"firstHalfNet"`
	SecondHalfNet float64 `json:"secondHalfNet"`
}

// BuildAccounts derives one account per independent hypothesis.
//
// Trades are sorted by close time before the equity walk, so max drawdown and
// the half-split are computed on the real sequence rather than on whatever
// order the caller happened to supply.
func BuildAccounts(desk string, trades []Trade, startingCapital float64) []Account {
	return BuildAccountsWithRoster(desk, trades, nil, startingCapital)
}

// BuildAccountsWithRoster derives accounts and ALSO emits a funded, zero-trade
// account for every roster entry that has not closed a trade yet.
//
// Deriving accounts from trades alone made a freshly funded strategy invisible
// until its first close — the strategies an operator most wants to see waiting
// at the line. It also understated the hunt: 100 strategies at $1,000 is
// $100,000 deployed whether or not they have traded.
func BuildAccountsWithRoster(desk string, trades []Trade, roster []RosterEntry, startingCapital float64) []Account {
	if startingCapital <= 0 {
		startingCapital = DefaultStartingCapital
	}

	byKey := map[string][]Trade{}
	for _, t := range trades {
		byKey[t.Key()] = append(byKey[t.Key()], t)
	}

	out := make([]Account, 0, len(byKey)+len(roster))
	for key, ts := range byKey {
		sort.Slice(ts, func(i, j int) bool { return ts[i].ClosedAt.Before(ts[j].ClosedAt) })
		out = append(out, buildOne(desk, key, ts, startingCapital))
	}

	// Funded but not yet traded: full stake, zero everything else. Shown rather
	// than hidden so the roster and the leaderboard agree on how many strategies
	// are running.
	for _, r := range roster {
		if _, traded := byKey[r.Key()]; traded {
			continue
		}
		out = append(out, Account{
			Key: r.Key(), Desk: desk,
			Strategy:        r.Strategy,
			Symbol:          r.Symbol,
			StartingCapital: startingCapital,
			Capital:         startingCapital,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func buildOne(desk, key string, ts []Trade, start float64) Account {
	a := Account{
		Key: key, Desk: desk,
		Strategy:        ts[0].Strategy,
		Symbol:          ts[0].Symbol,
		StartingCapital: start,
		Capital:         start,
	}

	var grossWins, grossLosses float64
	equity := start
	peak := start

	// Half-split boundary by TIME, not trade count: a strategy that traded
	// heavily in one week and idled the next should not have its "halves"
	// silently redefined by activity.
	first, last := ts[0].ClosedAt, ts[len(ts)-1].ClosedAt
	mid := first.Add(last.Sub(first) / 2)

	for _, t := range ts {
		a.Trades++
		a.NetPnL += t.NetPnL
		a.GrossPnL += t.GrossPnL
		a.Fees += t.Fees

		if t.NetPnL > 0 {
			a.Wins++
			grossWins += t.NetPnL
		} else if t.NetPnL < 0 {
			a.Losses++
			grossLosses += -t.NetPnL
		}

		if t.ClosedAt.After(mid) {
			a.SecondHalfNet += t.NetPnL
		} else {
			a.FirstHalfNet += t.NetPnL
		}

		equity += t.NetPnL
		if equity > peak {
			peak = equity
		}
		if peak > 0 {
			if dd := (peak - equity) / peak; dd > a.MaxDrawdown {
				a.MaxDrawdown = dd
			}
		}
	}

	a.Capital = equity
	if start > 0 {
		a.GrowthPct = (a.Capital - start) / start * 100
	}
	if a.Trades > 0 {
		a.WinRate = float64(a.Wins) / float64(a.Trades) * 100
		a.Expectancy = a.NetPnL / float64(a.Trades)
	}
	if grossLosses > 0 {
		a.ProfitFactor = grossWins / grossLosses
	} else if grossWins > 0 {
		// No losing trade yet. Reporting +Inf would top every sort on a
		// two-trade sample, so leave it 0 and let the gate's trade minimum
		// decide — an unbeaten record is not evidence, it is a small sample.
		a.ProfitFactor = 0
	}
	if a.GrossPnL > 0 {
		a.FeeDragPct = a.Fees / a.GrossPnL * 100
	}

	a.FirstTrade, a.LastTrade = first, last
	a.DaysLive = last.Sub(first).Hours() / 24
	return a
}

// Leaderboard ranks accounts by capital growth — the ranking that was asked for.
// It answers "who grew fastest", which is NOT the same question as "who earned
// real money", and callers must read Verdict before funding anything.
func Leaderboard(accounts []Account) []Account {
	out := append([]Account(nil), accounts...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].GrowthPct != out[j].GrowthPct {
			return out[i].GrowthPct > out[j].GrowthPct
		}
		// Tie-break on sample size: between two equal returns, prefer the one
		// with more evidence behind it.
		return out[i].Trades > out[j].Trades
	})
	return out
}

// TotalCapital is what the whole hunt is running on, for the desk header.
func TotalCapital(accounts []Account) (deployed, current float64) {
	for _, a := range accounts {
		deployed += a.StartingCapital
		current += a.Capital
	}
	return deployed, current
}

func round2(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return math.Round(v*100) / 100
}
