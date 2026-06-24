package scalpers

// buildFamily5DerivativesMacro returns S70-S79 (Family 5 — Derivatives,
// On-Chain & Macro). Several of these strategies depend on data feeds whose
// MarketContext fields exist but whose live fetchers are not yet wired into
// the orchestrator (see each file's "Data dependency" header note) — those
// strategies correctly return NoSignal until an operator wires the fetcher,
// at which point they activate with no further code changes. All 10
// strategies are registered at rollout phase 99, which exceeds any real
// STRATEGY_ROLLOUT_PHASE value (1-5), so they ALWAYS evaluate but NEVER
// auto-promote to live trading — see rollout_phase.go. Promotion only
// happens via the explicit ShadowPromoter.Promote() mechanism.
func buildFamily5DerivativesMacro() []RegistryEntry {
	return []RegistryEntry{
		{
			Strategy:     withRolloutPhase(&OptionsSkewDirectionSignal{}, 99),
			Name:         "Options_Skew_Direction_Signal",
			Description:  "S70: synthetic options-skew proxy from DVOL vs RealizedVol asymmetry combined with 3-bar confirmed price direction (no options chain feed wired — PROXY)",
			Regimes:      []Regime{RegimeTrending, RegimeRanging},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&GammaExposureMagnet{}, 99),
			Name:         "Gamma_Exposure_Magnet",
			Description:  "S71: notional GEX proxy from OI concentration near round-number price levels, gated behind gexFeedAvailable=false (no strike-level OI feed — PENDING, always NoSignal until wired)",
			Regimes:      []Regime{RegimeTrending, RegimeRanging},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&PerpetualFundingBasisComposite{}, 99),
			Name:         "Perpetual_Funding_Basis_Composite",
			Description:  "S72: combines funding level + trend + cumulative-paid proxy (3 legs not combined by S8/S15) for max-crowding fade (FundingRate/FundingHistory live — DONE)",
			Regimes:      []Regime{RegimeTrending, RegimeRanging},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&LongShortRatioContrarian{}, 99),
			Name:         "Long_Short_Ratio_Contrarian",
			Description:  "S73: fades extreme Binance top-trader long/short account ratio confirmed by funding agreement (positioning feed not yet wired — PENDING)",
			Regimes:      []Regime{RegimeTrending, RegimeRanging},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&TakerBuySellVolumeRatio{}, 99),
			Name:         "Taker_Buy_Sell_Volume_Ratio",
			Description:  "S74: follows sustained (3+ reading) Binance taker buy/sell volume ratio imbalance (positioning feed not yet wired — PENDING)",
			Regimes:      []Regime{RegimeTrending, RegimeRanging},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&OpenInterestVelocity{}, 99),
			Name:         "Open_Interest_Velocity",
			Description:  "S75: OI rate-of-change over OIHistory's 4-reading window, follows fresh momentum or fades fast unwinds, confirmed by 15m EMA9/EMA21 (OIHistory live — DONE)",
			Regimes:      []Regime{RegimeTrending, RegimeRanging},
			Timeframes:   []string{"15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&ExchangeNetflowSignal{}, 99),
			Name:         "Exchange_Netflow_Signal",
			Description:  "S76: proxies exchange netflow via OI build + price range stability (no on-chain netflow feed wired — PROXY)",
			Regimes:      []Regime{RegimeTrending, RegimeRanging},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&MinerHashRateMomentum{}, 99),
			Name:         "Miner_Hash_Rate_Momentum",
			Description:  "S77: 30d hash rate trend macro bias confirmed by 1h EMA9/EMA21 (hash rate feed not yet wired — PENDING)",
			Regimes:      []Regime{RegimeTrending, RegimeRanging},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&StablecoinSupplyFlow{}, 99),
			Name:         "Stablecoin_Supply_Flow",
			Description:  "S78: conviction modifier — boosts confidence of base EMA9/EMA21 trend signal when stablecoin 7d supply growth is positive/above-trend (feed not yet wired — PENDING, base trend still fires unboosted)",
			Regimes:      []Regime{RegimeTrending, RegimeRanging},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&FearGreedContrarian{}, 99),
			Name:         "Fear_Greed_Contrarian",
			Description:  "S79: fades 3-day-sustained Extreme Fear (<20) or Extreme Greed (>80) on alternative.me Fear & Greed Index (feed not yet wired — PENDING)",
			Regimes:      []Regime{RegimeTrending, RegimeRanging},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
	}
}
