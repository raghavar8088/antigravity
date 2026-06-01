package v3

import (
	btExecution "antigravity-engine/internal/backtest/execution"
)

// SimMode controls execution realism level.
type SimMode string

const (
	SimModeFast          SimMode = "FAST"          // minimal friction, fast iteration
	SimModeInstitutional SimMode = "INSTITUTIONAL"  // full microstructure realism
	SimModeStress        SimMode = "STRESS"         // institutional + stress overlays
)

// Config is the master configuration for V3 backtest runs.
type Config struct {
	Mode             SimMode
	Exchange         Exchange
	InitialCapitalUSD float64
	RiskPerTradePct  float64
	LatencyTier      btExecution.LatencyTier
	LatencyJitter    bool   // sample latency from log-normal, not fixed
	WarmupBars       int
	AllowShorts      bool
	MaxOpenPositions int

	// Commission
	CommissionTier VolumeTier // 30-day trading volume tier

	// Liquidity
	LiquidityScenario LiquidityScenario

	// Stress (only used in SimModeStress)
	StressScenarios []StressScenarioType

	// Monte Carlo
	MonteCarloConfig MonteCarloV3Config

	// Validation
	ValidationConfig ValidationConfig

	// OMS v3 integration — when true, all fills transit through ledger
	EnableOMSBridge bool
	AccountID       string
}

func DefaultConfig() Config {
	return Config{
		Mode:              SimModeInstitutional,
		Exchange:          ExchangeBinance,
		InitialCapitalUSD: 1_000_000,
		RiskPerTradePct:   0.5,
		LatencyTier:       btExecution.Latency50ms,
		LatencyJitter:     true,
		WarmupBars:        50,
		AllowShorts:       true,
		MaxOpenPositions:  10,
		CommissionTier:    TierStandard,
		LiquidityScenario: ScenarioNormal,
		MonteCarloConfig: MonteCarloV3Config{
			Paths:          5000,
			Seed:           42,
			UseStudentT:    true,
			DegreesOfFreedom: 4,
			BootstrapWithReplacement: true,
		},
		ValidationConfig: ValidationConfig{
			MaxSimErrorPct: 10.0,
		},
	}
}
