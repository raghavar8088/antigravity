package v2

func DefaultRiskLimits(equityUSD float64) RiskLimits {
	if equityUSD <= 0 {
		equityUSD = 100_000
	}
	return RiskLimits{
		MaxRiskPerTradePct:       2,
		MaxPortfolioHeatPct:      6,
		ReduceHeatPct:            4,
		BlockHeatPct:             6,
		ForceReduceHeatPct:       8,
		MaxNetExposurePct:        150,
		MaxGrossExposurePct:      250,
		MaxFamilyAllocationPct:   30,
		MaxRegimeExposurePct:     45,
		MaxStrategyAllocationPct: 20,
		MaxClusterAllocationPct:  35,
		MaxCorrelation:           0.80,
		MaxVaRPct:                6,
		MaxCVaRPct:               9,
		MaxLeverage:              5,
		MaxDrawdownPct:           10,
		MinRiskScore:             70,
	}
}

func normalizeTradeRequest(req TradeRequest) TradeRequest {
	if req.Symbol == "" {
		req.Symbol = "BTCUSD"
	}
	if req.Strategy == "" {
		req.Strategy = "UNSPECIFIED"
	}
	if req.Family == "" {
		req.Family = FamilyReserve
	}
	if req.Side == "" {
		req.Side = SideLong
	}
	if req.RequestedLeverage <= 0 {
		req.RequestedLeverage = 1
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = nowUTC()
	}
	return req
}

func normalizeMarket(market MarketState) MarketState {
	if market.Regime == "" {
		market.Regime = RegimeUnknown
	}
	if market.LiquidityScore <= 0 {
		market.LiquidityScore = 0.65
	}
	return market
}

func normalizeAccount(account AccountState) AccountState {
	if account.EquityUSD <= 0 {
		account.EquityUSD = 100_000
	}
	if account.HighWatermarkUSD <= 0 {
		account.HighWatermarkUSD = account.EquityUSD
	}
	if account.EquityUSD > account.HighWatermarkUSD {
		account.HighWatermarkUSD = account.EquityUSD
	}
	return account
}
