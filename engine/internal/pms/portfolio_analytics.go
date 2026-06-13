package pms

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// PeriodReturn holds the P&L for a discrete time window.
type PeriodReturn struct {
	Period    string    // "DAILY" | "WEEKLY" | "MONTHLY" | "YTD" | "ALL"
	StartDate time.Time
	EndDate   time.Time
	ReturnPct float64
	PnLUSD    float64
	Trades    int
}

// PortfolioPerformance is the full analytics record for one portfolio.
type PortfolioPerformance struct {
	PortfolioID string
	ComputedAt  time.Time

	// NAV
	InitialNAV float64
	CurrentNAV float64
	HWMUSD     float64 // high-water mark

	// Returns
	TotalReturnPct      float64
	AnnualisedReturnPct float64

	// Risk-adjusted returns
	SharpeRatio  float64 // vs risk-free rate
	SortinoRatio float64 // vs downside vol
	CalmarRatio  float64 // annualised return / max drawdown
	OmegaRatio   float64 // gains / losses

	// Drawdown
	MaxDrawdownPct  float64
	CurrentDDPct    float64
	RecoveryFactor  float64 // total return / max drawdown

	// Trade statistics
	TotalTrades    int
	WinRate        float64
	ProfitFactor   float64 // gross wins / gross losses
	AvgWinUSD      float64
	AvgLossUSD     float64
	AvgHoldMinutes float64

	// Exposure
	AvgGrossExpPct float64
	AvgNetExpPct   float64

	// Alpha / Beta (vs BTC benchmark)
	Alpha float64
	Beta  float64

	// Period breakdowns
	Daily   PeriodReturn
	Weekly  PeriodReturn
	Monthly PeriodReturn
	YTD     PeriodReturn
	AllTime PeriodReturn
}

// TradeRecord is the minimal analytics-side trade record.
type TradeRecord struct {
	TradeID      string
	StrategyID   string
	Symbol       string
	Side         string
	EntryPrice   float64
	ExitPrice    float64
	Quantity     float64
	GrossPnLUSD  float64
	NetPnLUSD    float64
	FeesUSD      float64
	HoldMinutes  float64
	EntryTime    time.Time
	ExitTime     time.Time
}

// AnalyticsEngine computes portfolio performance metrics from trade records.
type AnalyticsEngine struct{}

// NewAnalyticsEngine creates an analytics engine.
func NewAnalyticsEngine() *AnalyticsEngine { return &AnalyticsEngine{} }

// Compute calculates the full PortfolioPerformance from a slice of trades and
// a daily equity series. equitySeries is a slice of (date, NAV) pairs sorted by date.
func (a *AnalyticsEngine) Compute(
	portfolioID string,
	initialNAV float64,
	trades []TradeRecord,
	dailyReturns []float64, // daily return fractions e.g. [0.01, -0.005, ...]
	riskFreeAnnual float64,
	benchmarkReturns []float64, // optional benchmark daily returns
) PortfolioPerformance {
	now := time.Now().UTC()
	perf := PortfolioPerformance{
		PortfolioID: portfolioID,
		ComputedAt:  now,
		InitialNAV:  initialNAV,
		TotalTrades: len(trades),
	}

	if len(trades) == 0 && len(dailyReturns) == 0 {
		perf.CurrentNAV = initialNAV
		return perf
	}

	// ── NAV and total return ──────────────────────────────────────────────────
	currentNAV := initialNAV
	for _, r := range dailyReturns {
		currentNAV *= (1 + r)
	}
	perf.CurrentNAV = currentNAV
	perf.HWMUSD = computeHWM(initialNAV, dailyReturns)
	if initialNAV > 0 {
		perf.TotalReturnPct = (currentNAV - initialNAV) / initialNAV * 100
	}

	// Annualised return: CAGR
	n := len(dailyReturns)
	if n > 0 && initialNAV > 0 {
		years := float64(n) / 252.0
		if years > 0 {
			perf.AnnualisedReturnPct = (math.Pow(currentNAV/initialNAV, 1.0/years) - 1.0) * 100
		}
	}

	// ── Drawdown ──────────────────────────────────────────────────────────────
	perf.MaxDrawdownPct, perf.CurrentDDPct = computeDrawdowns(initialNAV, dailyReturns)
	if perf.MaxDrawdownPct > 0 {
		perf.RecoveryFactor = perf.TotalReturnPct / perf.MaxDrawdownPct
		perf.CalmarRatio = perf.AnnualisedReturnPct / perf.MaxDrawdownPct
	}

	// ── Risk-adjusted returns ─────────────────────────────────────────────────
	rfDaily := riskFreeAnnual / 252.0
	if len(dailyReturns) >= 2 {
		vol := stddevF(dailyReturns)
		excess := meanF(dailyReturns) - rfDaily
		if vol > 0 {
			perf.SharpeRatio = excess / vol * math.Sqrt(252)
		}
		downVol := downsideVol(dailyReturns, rfDaily)
		if downVol > 0 {
			perf.SortinoRatio = excess / downVol * math.Sqrt(252)
		}
		perf.OmegaRatio = omegaRatio(dailyReturns, rfDaily)
	}

	// ── Alpha / Beta ──────────────────────────────────────────────────────────
	if len(benchmarkReturns) >= 2 && len(dailyReturns) >= 2 {
		perf.Beta, perf.Alpha = computeAlphaBeta(dailyReturns, benchmarkReturns, rfDaily)
	}

	// ── Trade statistics ──────────────────────────────────────────────────────
	if len(trades) > 0 {
		perf.WinRate, perf.ProfitFactor, perf.AvgWinUSD, perf.AvgLossUSD, perf.AvgHoldMinutes =
			computeTradeStats(trades)
	}

	// ── Period returns ────────────────────────────────────────────────────────
	perf.Daily = computePeriodReturn("DAILY", trades, dailyReturns, 1, now)
	perf.Weekly = computePeriodReturn("WEEKLY", trades, dailyReturns, 5, now)
	perf.Monthly = computePeriodReturn("MONTHLY", trades, dailyReturns, 21, now)
	perf.YTD = computeYTD(trades, dailyReturns, initialNAV, now)
	perf.AllTime = computeAllTime(trades, perf.TotalReturnPct, initialNAV, currentNAV-initialNAV, now)

	return perf
}

// Report produces a formatted single-line summary.
func (p PortfolioPerformance) Report() string {
	return fmt.Sprintf(
		"portfolio=%s NAV=$%.0f return=%.1f%% ann=%.1f%% sharpe=%.2f sortino=%.2f calmar=%.2f maxDD=%.1f%% trades=%d wr=%.1f%%",
		p.PortfolioID, p.CurrentNAV, p.TotalReturnPct, p.AnnualisedReturnPct,
		p.SharpeRatio, p.SortinoRatio, p.CalmarRatio, p.MaxDrawdownPct,
		p.TotalTrades, p.WinRate*100,
	)
}

// ── Statistical helpers ───────────────────────────────────────────────────────

func computeHWM(initial float64, returns []float64) float64 {
	hwm := initial
	nav := initial
	for _, r := range returns {
		nav *= (1 + r)
		if nav > hwm {
			hwm = nav
		}
	}
	return hwm
}

func computeDrawdowns(initial float64, returns []float64) (maxDD, currentDD float64) {
	if len(returns) == 0 {
		return 0, 0
	}
	hwm := initial
	nav := initial
	for _, r := range returns {
		nav *= (1 + r)
		if nav > hwm {
			hwm = nav
		}
		if hwm > 0 {
			dd := (hwm - nav) / hwm * 100
			if dd > maxDD {
				maxDD = dd
			}
			currentDD = dd
		}
	}
	return maxDD, currentDD
}

func meanF(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func stddevF(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := meanF(xs)
	s := 0.0
	for _, x := range xs {
		d := x - m
		s += d * d
	}
	return math.Sqrt(s / float64(len(xs)-1))
}

func downsideVol(returns []float64, threshold float64) float64 {
	if len(returns) < 2 {
		return 0
	}
	s := 0.0
	count := 0
	for _, r := range returns {
		if r < threshold {
			d := r - threshold
			s += d * d
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return math.Sqrt(s / float64(count))
}

func omegaRatio(returns []float64, threshold float64) float64 {
	gains, losses := 0.0, 0.0
	for _, r := range returns {
		if r > threshold {
			gains += r - threshold
		} else {
			losses += threshold - r
		}
	}
	if losses == 0 {
		return math.Inf(1)
	}
	return gains / losses
}

func computeAlphaBeta(strat, bench []float64, rf float64) (beta, alpha float64) {
	n := len(strat)
	if len(bench) < n {
		n = len(bench)
	}
	if n < 2 {
		return 0, 0
	}
	mS, mB := meanF(strat[:n]), meanF(bench[:n])
	cov, varB := 0.0, 0.0
	for i := 0; i < n; i++ {
		cov += (strat[i] - mS) * (bench[i] - mB)
		varB += (bench[i] - mB) * (bench[i] - mB)
	}
	if varB == 0 {
		return 0, 0
	}
	beta = cov / varB
	alpha = (mS - rf - beta*(mB-rf)) * 252 * 100 // annualised %
	return beta, alpha
}

func computeTradeStats(trades []TradeRecord) (winRate, profitFactor, avgWin, avgLoss, avgHold float64) {
	wins, losses := 0, 0
	grossWin, grossLoss := 0.0, 0.0
	holdSum := 0.0
	for _, t := range trades {
		holdSum += t.HoldMinutes
		if t.NetPnLUSD >= 0 {
			wins++
			grossWin += t.NetPnLUSD
		} else {
			losses++
			grossLoss += -t.NetPnLUSD
		}
	}
	n := len(trades)
	if n > 0 {
		winRate = float64(wins) / float64(n)
		avgHold = holdSum / float64(n)
	}
	if wins > 0 {
		avgWin = grossWin / float64(wins)
	}
	if losses > 0 {
		avgLoss = grossLoss / float64(losses)
		profitFactor = grossWin / grossLoss
	}
	return winRate, profitFactor, avgWin, avgLoss, avgHold
}

func computePeriodReturn(period string, trades []TradeRecord, dailyReturns []float64, lookback int, now time.Time) PeriodReturn {
	n := len(dailyReturns)
	if n == 0 {
		return PeriodReturn{Period: period}
	}
	start := n - lookback
	if start < 0 {
		start = 0
	}
	returns := dailyReturns[start:]
	cum := 1.0
	for _, r := range returns {
		cum *= (1 + r)
	}
	retPct := (cum - 1) * 100
	startDate := now.AddDate(0, 0, -lookback)
	pnl := 0.0
	tradeCount := 0
	for _, t := range trades {
		if t.ExitTime.After(startDate) {
			pnl += t.NetPnLUSD
			tradeCount++
		}
	}
	return PeriodReturn{
		Period:    period,
		StartDate: startDate,
		EndDate:   now,
		ReturnPct: retPct,
		PnLUSD:    pnl,
		Trades:    tradeCount,
	}
}

func computeYTD(trades []TradeRecord, dailyReturns []float64, initialNAV float64, now time.Time) PeriodReturn {
	ytdStart := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	daysIntoYear := int(now.Sub(ytdStart).Hours() / 24)
	return computePeriodReturn("YTD", trades, dailyReturns, daysIntoYear, now)
}

func computeAllTime(trades []TradeRecord, totalRetPct, initialNAV, totalPnL float64, now time.Time) PeriodReturn {
	_ = initialNAV
	sort.Slice(trades, func(i, j int) bool { return trades[i].EntryTime.Before(trades[j].EntryTime) })
	startDate := now
	if len(trades) > 0 {
		startDate = trades[0].EntryTime
	}
	return PeriodReturn{
		Period:    "ALL",
		StartDate: startDate,
		EndDate:   now,
		ReturnPct: totalRetPct,
		PnLUSD:    totalPnL,
		Trades:    len(trades),
	}
}
