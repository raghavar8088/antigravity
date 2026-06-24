package scalpers

// buildFamily4MLProxy returns S60-S69 (Family 4 — Machine Learning Signal
// Proxies). All implement explicit deterministic rules that replicate what
// published ML-feature-importance research identifies as predictive — there
// is NO model inference or AI/LLM call anywhere in this family at runtime.
// All 10 strategies are registered at rollout phase 99, which exceeds any
// real STRATEGY_ROLLOUT_PHASE value (1-5), so they ALWAYS evaluate but NEVER
// auto-promote to live trading — see rollout_phase.go. Promotion only
// happens via the explicit ShadowPromoter.Promote() mechanism.
func buildFamily4MLProxy() []RegistryEntry {
	return []RegistryEntry{
		{
			Strategy:     withRolloutPhase(&FeatureConfluenceScore{}, 99),
			Name:         "Feature_Confluence_Score",
			Description:  "S60: sums 8 explicit bull/bear/neutral votes (RSI, MACD histogram, volume ratio, ATR trend, price-vs-EMA21, BB position, CVD trend, funding sign) replicating top RF-feature-importance splits from crypto price-prediction research; total score >=+5/-5 required",
			Regimes:      []Regime{RegimeTrending, RegimeRanging},
			Timeframes:   []string{"15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&RegimeProbabilityComposite{}, 99),
			Name:         "Regime_Probability_Composite",
			Description:  "S61: 5-feature majority-vote regime classifier (Hamilton 1989 HMM-inspired) cross-checked against ctx.Regime; only trades when both independent regime estimates agree, then applies trend-following or mean-reversion logic per the agreed regime",
			Regimes:      []Regime{RegimeTrending, RegimeRanging, RegimeVolatile},
			Timeframes:   []string{"15m", "1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&GradientBoostingFeatureSignal{}, 99),
			Name:         "Gradient_Boosting_Feature_Signal",
			Description:  "S62: Krauss/Do/Huck (2017) multi-horizon ROC+momentum features at 5m/15m/1h; requires unanimous cross-horizon agreement (strong signal), skips on partial agreement (weak signal)",
			Regimes:      []Regime{RegimeTrending},
			Timeframes:   []string{"5m", "15m", "1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&AutocorrelationPattern{}, 99),
			Name:         "Autocorrelation_Pattern",
			Description:  "S63: Biais et al. (2016) — lag-1 1m autocorrelation >0.3 fires momentum continuation; lag-5 5m autocorrelation <-0.3 fires mean-reversion; momentum path takes priority if both fire",
			Regimes:      []Regime{RegimeTrending, RegimeRanging},
			Timeframes:   []string{"1m", "5m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&ShannonEntropyRegime{}, 99),
			Name:         "Shannon_Entropy_Regime",
			Description:  "S64: Lo & MacKinlay (1999) — only trades EMA9/EMA21 trend direction when 15m Shannon entropy (period 30, bins 8) is below a fixed 0.6 threshold proxy for low-entropy/ordered regime (documented tercile-distribution simplification)",
			Regimes:      []Regime{RegimeTrending, RegimeRanging},
			Timeframes:   []string{"15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&CrossCorrelationLeadLag{}, 99),
			Name:         "Cross_Correlation_Lead_Lag",
			Description:  "S65: BTC-leads-altcoins lead-lag literature — boosts confidence on a base BTC EMA9/EMA21 continuation signal when BTC moved >0.3% over the last 5x1m bars without ETH confirmation yet (qualitative single-point ETH check, requires ETHPricePopulated)",
			Regimes:      []Regime{RegimeTrending},
			Timeframes:   []string{"1m", "5m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&WaveletTrendSignal{}, 99),
			Name:         "Wavelet_Trend_Signal",
			Description:  "S66: Gallegati & Semmler (2014) — HaarWavelet (levels=3) trend estimate on 15m must be directional vs a longer EMA AND have low noise relative to ATR (good signal-to-noise) before trend entry",
			Regimes:      []Regime{RegimeTrending},
			Timeframes:   []string{"15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&RecurrenceQuantificationSignal{}, 99),
			Name:         "Recurrence_Quantification_Signal",
			Description:  "S67: Webber & Marwan (2015) RQA-inspired — compares current 20-bar 15m feature vector (price+volume change) via Euclidean distance to available historical non-overlapping windows in the buffer; signals if closest matches were unanimously followed by a consistent directional move (reduced lookback vs literature's 200-window ideal, documented constraint)",
			Regimes:      []Regime{RegimeTrending, RegimeRanging},
			Timeframes:   []string{"15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&ExcessReturnMomentum{}, 99),
			Name:         "Excess_Return_Momentum",
			Description:  "S68: Cong, Tang, Wang & Yang (2021) Crypto Factors — requires 15m/1h/4h excess returns (vs each timeframe's own trailing mean, substituted for a literal 30-day mean) to be all-positive or all-negative AND accelerating in magnitude across horizons",
			Regimes:      []Regime{RegimeTrending},
			Timeframes:   []string{"15m", "1h", "4h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&StructuralBreakDetection{}, 99),
			Name:         "Structural_Break_Detection",
			Description:  "S69: Page (1954) CUSUM / Lopez de Prado (2018) — CUSUM(period=20, threshold=2.0 std dev) break on 15m, 3-bar confirmed, trades in break direction with SL tightened to exactly 1.0xATR rather than wider",
			Regimes:      []Regime{RegimeTrending, RegimeRanging, RegimeVolatile},
			Timeframes:   []string{"15m"},
			MaxPositions: 1,
		},
	}
}
