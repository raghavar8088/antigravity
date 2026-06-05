package phase23b

import (
	"fmt"
	"log"
	"time"

	"antigravity-engine/internal/strategy"
	"antigravity-engine/internal/validation/phase22e"
)

// DefaultReplayConfig returns production-grade replay configuration.
func DefaultReplayConfig() ReplayConfig {
	to := time.Now().UTC().Truncate(time.Minute)
	from := to.AddDate(-2, 0, 0) // 24 months of real data
	return ReplayConfig{
		Symbol:         "BTCUSDT",
		From:           from,
		To:             to,
		InitialCapital: 1_000_000.0,
		PositionCapPct: DefaultPositionCapPct,
		TakerFeeBps:    TakerFeeBps,
		MakerFeeBps:    MakerFeeBps,
		SlippageBps:    SlippageBps,
		MaxHoldMins:    DefaultMaxHoldMins,
		MinConfidence:  DefaultMinConfidence,
		MaxStrategies:  0, // 0 = all
	}
}

// Pipeline23B orchestrates the complete Phase 23B validation:
//
//  23B.1  Data source audit
//  23B.2  Synthetic removal documentation
//  23B.3  Real execution integration (via Strategy.OnCandle)
//  23B.4  Historical replay engine
//  23B.5  Real execution cost model
//  23B.6  Certified trade database
//  23B.7  Walk-forward validation
//  23B.8  Monte Carlo validation
//  23B.9  Regime performance analysis
//  23B.10 Capital certification
type Pipeline23B struct {
	Config  ReplayConfig
	loader  *BinanceFuturesLoader
}

// NewPipeline23B creates a pipeline with the given configuration.
func NewPipeline23B(cfg ReplayConfig) *Pipeline23B {
	return &Pipeline23B{
		Config: cfg,
		loader: NewBinanceFuturesLoader(),
	}
}

// Run executes the full Phase 23B pipeline and returns the master result.
func (p *Pipeline23B) Run() (Phase23BResult, error) {
	result := Phase23BResult{
		GeneratedAt:       time.Now().UTC(),
		Config:            p.Config,
		TradesPerStrategy: make(map[string]int),
		TradesPerAlpha:    make(map[string]int),
		TradesPerRegime:   make(map[phase22e.Regime]int),
		Metrics:           make(map[string]Metrics23B),
	}

	// ── Phase 23B.1: Data Source Audit ────────────────────────────────────────
	log.Println("[23B.1] Loading real Binance Futures data...")
	candles, fundingCount, oiCount, err := p.loadRealData()
	if err != nil {
		return result, fmt.Errorf("23B.1 data load: %w", err)
	}
	result.DataAudit = AuditLoadedData(p.Config.Symbol, candles, fundingCount, oiCount, p.Config.From, p.Config.To)
	result.TotalCandles = len(candles)
	if len(candles) >= 2 {
		result.CoverageDays = candles[len(candles)-1].CloseTime.Sub(candles[0].OpenTime).Hours() / 24
	}
	log.Printf("[23B.1] Loaded %d candles, %.0f days coverage, quality=%s",
		len(candles), result.CoverageDays, result.DataAudit.OverallQuality)

	if !result.DataAudit.Accepted {
		return result, fmt.Errorf("data quality insufficient: %v", result.DataAudit.Issues)
	}

	// ── Phase 23B.2: Synthetic Removal ────────────────────────────────────────
	log.Println("[23B.2] Documenting synthetic component removal...")
	result.SyntheticRemoval = SyntheticRemovalAudit()
	log.Printf("[23B.2] Removed %d/%d synthetic components",
		result.SyntheticRemoval.TotalRemoved, result.SyntheticRemoval.TotalFound)

	// ── Phase 23B.3 + 23B.4: Real Execution + Replay ─────────────────────────
	log.Println("[23B.3] Loading real strategy instances from production registry...")
	entries := strategy.BuildCuratedScalpers()
	log.Printf("[23B.3] %d strategies registered", len(entries))

	log.Println("[23B.4] Running historical replay against real candle data...")
	engine := NewRealReplayEngine(p.Config)
	result.StrategyReplays = engine.RunAll(entries, candles)
	result.TotalStrategies = len(result.StrategyReplays)
	log.Printf("[23B.4] Replay complete: %d strategies", len(result.StrategyReplays))

	// ── Phase 23B.5: Cost Model ───────────────────────────────────────────────
	log.Println("[23B.5] Computing execution cost breakdowns...")
	result.CostBreakdowns = BuildCostBreakdowns(result.StrategyReplays)

	// ── Phase 23B.6: Certified Trade Database ────────────────────────────────
	log.Println("[23B.6] Building certified trade database...")
	result.CertifiedTrades = collectAllTrades(result.StrategyReplays)
	result.TotalTrades = len(result.CertifiedTrades)
	for _, t := range result.CertifiedTrades {
		result.TradesPerStrategy[t.StrategyName]++
		result.TradesPerAlpha[t.AlphaSource]++
		result.TradesPerRegime[t.Regime]++
	}
	log.Printf("[23B.6] Certified %d trades across %d strategies",
		result.TotalTrades, len(result.TradesPerStrategy))

	// Compute per-strategy metrics
	byStrategy := groupTradesByStrategy(result.StrategyReplays)
	for name, trades := range byStrategy {
		result.Metrics[name] = ComputeMetrics(name, trades)
		if result.Metrics[name].TotalTrades > 0 {
			result.ValidatedStrategies++
		}
	}

	// ── Phase 23B.7: Walk-Forward ─────────────────────────────────────────────
	log.Println("[23B.7] Running walk-forward validation...")
	result.WalkForward = RunWalkForward(result.StrategyReplays, candles)
	log.Printf("[23B.7] Walk-forward complete: %d strategy reports", len(result.WalkForward))

	// ── Phase 23B.8: Monte Carlo ──────────────────────────────────────────────
	log.Println("[23B.8] Running Monte Carlo (1000 simulations/strategy)...")
	result.MonteCarlo = RunMonteCarlo(result.StrategyReplays, p.Config.InitialCapital)
	log.Printf("[23B.8] Monte Carlo complete: %d strategies", len(result.MonteCarlo))

	// ── Phase 23B.9: Regime Analysis ─────────────────────────────────────────
	log.Println("[23B.9] Building regime performance profiles...")
	result.RegimeProfiles = BuildRegimeProfiles(result.StrategyReplays)

	// ── Phase 23B.10: Capital Certification ──────────────────────────────────
	log.Println("[23B.10] Running capital certification gates...")
	result.CapCertifications = RunCapitalCertification(result.Metrics, result.MonteCarlo, p.Config.InitialCapital)
	for _, c := range result.CapCertifications {
		if c.Tier == CapTierFailed {
			result.RetiredStrategies++
		} else if c.Tier >= CapTierLimited {
			result.CertifiedStrategies++
		}
	}
	log.Printf("[23B.10] Certifications: %d certified, %d retired",
		result.CertifiedStrategies, result.RetiredStrategies)

	return result, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (p *Pipeline23B) loadRealData() ([]OHLCVCandle, int, int, error) {
	candles, err := p.loader.LoadCandles(p.Config.Symbol, p.Config.From, p.Config.To)
	if err != nil {
		return nil, 0, 0, err
	}
	// Count enrichments
	fundingCount, oiCount := 0, 0
	for _, c := range candles {
		if c.FundingRate != 0 {
			fundingCount++
		}
		if c.OpenInterest > 0 {
			oiCount++
		}
	}
	return candles, fundingCount, oiCount, nil
}

func collectAllTrades(replays []StrategyReplayResult) []CertifiedTrade {
	total := 0
	for _, r := range replays {
		total += len(r.Trades)
	}
	all := make([]CertifiedTrade, 0, total)
	for _, r := range replays {
		all = append(all, r.Trades...)
	}
	return all
}

func groupTradesByStrategy(replays []StrategyReplayResult) map[string][]CertifiedTrade {
	m := make(map[string][]CertifiedTrade, len(replays))
	for _, r := range replays {
		m[r.StrategyName] = append(m[r.StrategyName], r.Trades...)
	}
	return m
}
