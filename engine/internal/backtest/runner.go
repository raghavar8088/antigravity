package backtest

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"time"

	"antigravity-engine/internal/marketdata"
	scalers "antigravity-engine/internal/strategy/scalpers"
	v3 "antigravity-engine/internal/backtest/v3"
	btExecution "antigravity-engine/internal/backtest/execution"
)

// StrategyResult holds the backtest outcome for a single strategy.
type StrategyResult struct {
	StrategyName   string
	Symbol         string
	From, To       time.Time
	TotalTrades    int
	WinRate        float64
	Sharpe         float64
	MaxDrawdownPct float64
	ProfitFactor   float64
	TotalReturnPct float64
	AvgWinPct      float64
	AvgLossPct     float64

	// Promotion eligibility (filled by EvaluatePromotion)
	MeetsPromotion bool
	DemoteReason   string

	// Raw v3 output for deep analysis
	V3Result v3.V3Result
}

// RunConfig controls a single-strategy backtest run.
type RunConfig struct {
	Symbol      string
	Regime      scalers.Regime
	SizeBTC     float64
	V3Config    v3.Config
}

// DefaultRunConfig returns sensible defaults for a batch backtest.
// Monte Carlo is disabled (Paths=-1) to prevent OOM on small instances —
// 5000 paths × 50 strategies exceeds Lightsail 600MB limit.
func DefaultRunConfig(symbol string) RunConfig {
	cfg := v3.DefaultConfig()
	cfg.WarmupBars = 60
	cfg.MaxOpenPositions = 1
	cfg.AllowShorts = true
	cfg.MonteCarloConfig.Paths = 1                       // minimal — 5000 paths OOMs Lightsail
	cfg.MonteCarloConfig.BootstrapWithReplacement = false // no resampling needed
	// Backtest cost model: maker pricing + tighter spread assumption
	cfg.CommissionTier = v3.TierMaker
	cfg.SpreadBaseBps = 0.5  // 0.05% half-spread vs 0.15% institutional default
	cfg.MaxTradesPerDay = 5  // prevent overtrading — max 5 entries per strategy per day
	cfg.CapitalFloorPct = 0.5 // halt strategy if equity drops below 50% of initial capital
	cfg.OHLCVMode = true     // disables OHLCV-artifact vol inflation in spread model
	cfg.MaxHoldDuration = 7 * 24 * time.Hour // swing trades need days to reach TP
	return RunConfig{
		Symbol:   symbol,
		Regime:   scalers.RegimeTrending,
		SizeBTC:  0.1,
		V3Config: cfg,
	}
}

// Runner orchestrates a backtest for one strategy against a dataset.
// cb is a shared, read-only ContextBuilder — safe for concurrent callers.
type Runner struct {
	ds   marketdata.MTFDataset
	cfg  RunConfig
	cb   *ContextBuilder
	tick []marketdata.Tick
	fr   []btExecution.FundingRate
}

// NewRunner creates a Runner with a pre-built shared ContextBuilder.
// Pass ticks and funding rates pre-computed once for the whole batch.
func NewRunner(ds marketdata.MTFDataset, cfg RunConfig, cb *ContextBuilder, ticks []marketdata.Tick, fr []btExecution.FundingRate) *Runner {
	return &Runner{ds: ds, cfg: cfg, cb: cb, tick: ticks, fr: fr}
}

// Run executes the backtest and returns a StrategyResult.
func (r *Runner) Run(entry scalers.RegistryEntry) (StrategyResult, error) {
	if len(r.tick) == 0 {
		return StrategyResult{}, fmt.Errorf("no ticks in dataset for %s", r.cfg.Symbol)
	}

	regime := r.cfg.Regime
	if len(entry.Regimes) > 0 {
		regime = entry.Regimes[0]
	}

	adapter := NewScalerV3Adapter(entry.Strategy, r.cb, regime, r.cfg.Symbol, r.cfg.SizeBTC)

	// HARD CONSTRAINT: always pass nil bridge — never the production OMSBridge.
	result, err := v3.NewEngine(r.cfg.V3Config, r.fr, nil).Run(v3.V3Input{
		Config:   r.cfg.V3Config,
		Ticks:    r.tick,
		Strategy: adapter,
	})
	if err != nil {
		return StrategyResult{}, fmt.Errorf("v3 engine: %w", err)
	}

	return buildStrategyResult(entry.Name, r.cfg.Symbol, r.ds.From, r.ds.To, result), nil
}

// ── BatchRunner ───────────────────────────────────────────────────────────────

// BatchRunner runs all provided strategies in sequence and collects results.
type BatchRunner struct {
	ds      marketdata.MTFDataset
	cfg     RunConfig
	entries []scalers.RegistryEntry
}

// NewBatchRunner creates a BatchRunner for a list of strategies.
func NewBatchRunner(ds marketdata.MTFDataset, cfg RunConfig, entries []scalers.RegistryEntry) *BatchRunner {
	return &BatchRunner{ds: ds, cfg: cfg, entries: entries}
}

// RunAll runs all strategies in parallel (worker pool = NumCPU) and returns results.
// Errors are embedded in DemoteReason. onProgress is called after each strategy completes.
func (b *BatchRunner) RunAll(onProgress func(done, total int, name string)) []StrategyResult {
	total := len(b.entries)
	results := make([]StrategyResult, total)

	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}
	// Cap at 4 to avoid OOM on small Lightsail instances (1.5 CPU / 600 MB limit)
	if workers > 4 {
		workers = 4
	}

	type work struct {
		idx   int
		entry scalers.RegistryEntry
	}

	jobs := make(chan work, total)
	for i, e := range b.entries {
		jobs <- work{i, e}
	}
	close(jobs)

	// Build shared read-only resources once — avoids 4× memory copies.
	cb := NewContextBuilder(b.ds)
	ticks := DatasetToTicks(b.ds)
	fr := toFundingRates(b.ds)

	var mu sync.Mutex
	done := 0
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := NewRunner(b.ds, b.cfg, cb, ticks, fr)
			for job := range jobs {
				sr, err := r.Run(job.entry)
				if err != nil {
					sr = StrategyResult{
						StrategyName: job.entry.Name,
						Symbol:       b.cfg.Symbol,
						From:         b.ds.From,
						To:           b.ds.To,
						DemoteReason: fmt.Sprintf("run error: %v", err),
					}
				}
				mu.Lock()
				results[job.idx] = sr
				done++
				d := done
				mu.Unlock()
				if onProgress != nil {
					onProgress(d, total, job.entry.Name)
				}
			}
		}()
	}

	wg.Wait()
	if onProgress != nil {
		onProgress(total, total, "")
	}
	return results
}

// ── helpers ──────────────────────────────────────────────────────────────────

func buildStrategyResult(name, symbol string, from, to time.Time, r v3.V3Result) StrategyResult {
	trades := r.Trades
	n := len(trades)
	if n == 0 {
		return StrategyResult{
			StrategyName: name, Symbol: symbol, From: from, To: to,
			V3Result: r,
		}
	}

	wins := 0
	totalWinPct := 0.0
	totalLossPct := 0.0
	lossCount := 0
	grossProfit := 0.0
	grossLoss := 0.0

	returns := make([]float64, 0, n)
	peak := r.InitialCapitalUSD
	maxDD := 0.0
	equity := r.InitialCapitalUSD

	for _, t := range trades {
		pnl := t.NetPnL
		retPct := 0.0
		if equity > 0 {
			retPct = pnl / equity * 100
		}
		returns = append(returns, retPct)
		equity += pnl
		if equity > peak {
			peak = equity
		}
		dd := (peak - equity) / peak * 100
		if dd > maxDD {
			maxDD = dd
		}
		if pnl > 0 {
			wins++
			grossProfit += pnl
			totalWinPct += retPct
		} else {
			grossLoss += math.Abs(pnl)
			totalLossPct += math.Abs(retPct)
			lossCount++
		}
	}

	winRate := float64(wins) / float64(n)
	avgWin := 0.0
	avgLoss := 0.0
	if wins > 0 {
		avgWin = totalWinPct / float64(wins)
	}
	if lossCount > 0 {
		avgLoss = totalLossPct / float64(lossCount)
	}
	pf := 0.0
	if grossLoss > 0 {
		pf = grossProfit / grossLoss
	}

	totalReturn := (r.FinalCapitalUSD - r.InitialCapitalUSD) / r.InitialCapitalUSD * 100
	sharpe := calcSharpe(returns)

	return StrategyResult{
		StrategyName:   name,
		Symbol:         symbol,
		From:           from,
		To:             to,
		TotalTrades:    n,
		WinRate:        winRate,
		Sharpe:         sharpe,
		MaxDrawdownPct: maxDD,
		ProfitFactor:   pf,
		TotalReturnPct: totalReturn,
		AvgWinPct:      avgWin,
		AvgLossPct:     avgLoss,
		V3Result:       r,
	}
}

func calcSharpe(returns []float64) float64 {
	n := len(returns)
	if n < 2 {
		return 0
	}
	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(n)
	variance := 0.0
	for _, r := range returns {
		d := r - mean
		variance += d * d
	}
	variance /= float64(n - 1)
	std := math.Sqrt(variance)
	if std == 0 {
		return 0
	}
	// annualise assuming 15m bars ≈ 35040 bars/year
	return mean / std * math.Sqrt(35040)
}

func toFundingRates(ds marketdata.MTFDataset) []btExecution.FundingRate {
	out := make([]btExecution.FundingRate, len(ds.FundingRates))
	for i, f := range ds.FundingRates {
		out[i] = btExecution.FundingRate{
			Timestamp: f.Time,
			Rate:      f.Rate,
		}
	}
	return out
}
