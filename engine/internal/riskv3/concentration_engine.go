package riskv3

import "math"

// ConcentrationResult contains the concentration analysis of the current portfolio.
type ConcentrationResult struct {
	// Concentration by dimension (% of total notional)
	BySymbol   map[string]float64 `json:"by_symbol"`   // symbol → % of gross notional
	ByStrategy map[string]float64 `json:"by_strategy"` // strategy → %
	ByExchange map[string]float64 `json:"by_exchange"`  // exchange → %
	ByFamily   map[string]float64 `json:"by_family"`    // family → %

	// Peak concentration in each dimension
	MaxSymbolPct   float64 `json:"max_symbol_pct"`
	MaxStrategyPct float64 `json:"max_strategy_pct"`
	MaxExchangePct float64 `json:"max_exchange_pct"`

	// Directional exposure
	LongNotionalPct  float64 `json:"long_notional_pct"`  // long notional / gross * 100
	ShortNotionalPct float64 `json:"short_notional_pct"` // short notional / gross * 100

	// Concentration score 0–100 (higher = more concentrated = worse)
	ConcentrationScore float64 `json:"concentration_score"`

	// Violations
	Violations []Violation `json:"violations,omitempty"`
}

// AnalyseConcentration computes the concentration metrics for the current snapshot.
// The concentration score is a weighted composite of the worst single-dimension
// concentration, penalising directional imbalance.
func AnalyseConcentration(snapshot PortfolioSnapshot) ConcentrationResult {
	gross := snapshot.TotalNotionalUSD()
	result := ConcentrationResult{
		BySymbol:   make(map[string]float64),
		ByStrategy: make(map[string]float64),
		ByExchange: make(map[string]float64),
		ByFamily:   make(map[string]float64),
	}
	if gross <= 0 {
		return result
	}

	// Accumulate notional per dimension
	var longNotional, shortNotional float64
	for _, pos := range snapshot.Positions {
		pct := pos.NotionalUSD / gross * 100
		result.BySymbol[pos.Symbol] += pct
		result.ByStrategy[pos.StrategyName] += pct
		result.ByExchange[pos.Exchange] += pct
		result.ByFamily[pos.FamilyName] += pct

		if pos.Side == "BUY" {
			longNotional += pos.NotionalUSD
		} else {
			shortNotional += pos.NotionalUSD
		}
	}

	result.LongNotionalPct = longNotional / gross * 100
	result.ShortNotionalPct = shortNotional / gross * 100

	// Peak concentrations
	result.MaxSymbolPct = maxMapValue(result.BySymbol)
	result.MaxStrategyPct = maxMapValue(result.ByStrategy)
	result.MaxExchangePct = maxMapValue(result.ByExchange)

	// Concentration score (0–100; higher = worse)
	result.ConcentrationScore = computeConcentrationScore(
		result.MaxSymbolPct,
		result.MaxStrategyPct,
		result.MaxExchangePct,
		math.Abs(result.LongNotionalPct-result.ShortNotionalPct),
	)

	// Detect violations
	result.Violations = detectConcentrationViolations(result, snapshot.EquityUSD)

	return result
}

// ConcentrationGuard checks whether a proposed trade would push any concentration
// limit above its threshold. Concentration is measured as a fraction of portfolio
// equity (not gross notional) so that the first position on an empty book does not
// trigger a false violation.
func ConcentrationGuard(snapshot PortfolioSnapshot, proposed PositionRecord) []Violation {
	equity := snapshot.EquityUSD
	if equity <= 0 {
		return nil
	}

	var violations []Violation

	// Check symbol concentration: (existing symbol notional + proposed) / equity
	currentSymbolNotional := 0.0
	for _, pos := range snapshot.Positions {
		if pos.Symbol == proposed.Symbol {
			currentSymbolNotional += pos.NotionalUSD
		}
	}
	newSymbolPct := (currentSymbolNotional + proposed.NotionalUSD) / equity * 100
	if newSymbolPct > MaxAssetConcentrationPct {
		violations = append(violations, Violation{
			Type:        ViolationConcentrationExceeded,
			Metric:      "symbol_concentration",
			Current:     newSymbolPct,
			Limit:       MaxAssetConcentrationPct,
			Description: "asset concentration would exceed limit after proposed trade",
		})
	}

	// Check exchange concentration: (existing exchange notional + proposed) / equity
	currentExchangeNotional := 0.0
	for _, pos := range snapshot.Positions {
		if pos.Exchange == proposed.Exchange {
			currentExchangeNotional += pos.NotionalUSD
		}
	}
	newExchangePct := (currentExchangeNotional + proposed.NotionalUSD) / equity * 100
	if newExchangePct > MaxExchangeConcentrationPct {
		violations = append(violations, Violation{
			Type:        ViolationExchangeExceeded,
			Metric:      "exchange_concentration",
			Current:     newExchangePct,
			Limit:       MaxExchangeConcentrationPct,
			Description: "exchange concentration would exceed limit after proposed trade",
		})
	}

	// Check directional exposure: (long or short notional including proposal) / equity
	longNotional := 0.0
	shortNotional := 0.0
	for _, pos := range snapshot.Positions {
		if pos.Side == "BUY" {
			longNotional += pos.NotionalUSD
		} else {
			shortNotional += pos.NotionalUSD
		}
	}
	if proposed.Side == "BUY" {
		longNotional += proposed.NotionalUSD
	} else {
		shortNotional += proposed.NotionalUSD
	}
	longPct := longNotional / equity * 100
	if longPct > MaxDirectionalExposurePct {
		violations = append(violations, Violation{
			Type:        ViolationConcentrationExceeded,
			Metric:      "directional_exposure_long",
			Current:     longPct,
			Limit:       MaxDirectionalExposurePct,
			Description: "directional (long) exposure would exceed limit",
		})
	}
	shortPct := shortNotional / equity * 100
	if shortPct > MaxDirectionalExposurePct {
		violations = append(violations, Violation{
			Type:        ViolationConcentrationExceeded,
			Metric:      "directional_exposure_short",
			Current:     shortPct,
			Limit:       MaxDirectionalExposurePct,
			Description: "directional (short) exposure would exceed limit",
		})
	}

	return violations
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

// computeConcentrationScore returns a score from 0–100 (higher = more concentrated).
//
// Weights:
//   - Symbol concentration:   40% (largest single asset)
//   - Exchange concentration: 30% (platform risk)
//   - Strategy concentration: 20% (single-strategy overweight)
//   - Directional imbalance:  10% (long vs short)
func computeConcentrationScore(maxSymbol, maxStrategy, maxExchange, directionalGap float64) float64 {
	// Each dimension capped at the limit before scoring
	symbolScore := math.Min(maxSymbol/MaxAssetConcentrationPct, 1.0) * 40
	exchangeScore := math.Min(maxExchange/MaxExchangeConcentrationPct, 1.0) * 30
	strategyScore := math.Min(maxStrategy/MaxStrategyRiskPct, 1.0) * 20
	directionScore := math.Min(directionalGap/MaxDirectionalExposurePct, 1.0) * 10
	return symbolScore + exchangeScore + strategyScore + directionScore
}

// detectConcentrationViolations identifies which limits are breached.
func detectConcentrationViolations(result ConcentrationResult, _ float64) []Violation {
	var violations []Violation
	if result.MaxSymbolPct > MaxAssetConcentrationPct {
		violations = append(violations, Violation{
			Type:        ViolationConcentrationExceeded,
			Metric:      "max_symbol_concentration",
			Current:     result.MaxSymbolPct,
			Limit:       MaxAssetConcentrationPct,
			Description: "single asset concentration exceeds limit",
		})
	}
	if result.MaxExchangePct > MaxExchangeConcentrationPct {
		violations = append(violations, Violation{
			Type:        ViolationExchangeExceeded,
			Metric:      "max_exchange_concentration",
			Current:     result.MaxExchangePct,
			Limit:       MaxExchangeConcentrationPct,
			Description: "single exchange concentration exceeds limit",
		})
	}
	if result.LongNotionalPct > MaxDirectionalExposurePct {
		violations = append(violations, Violation{
			Type:        ViolationConcentrationExceeded,
			Metric:      "directional_long",
			Current:     result.LongNotionalPct,
			Limit:       MaxDirectionalExposurePct,
			Description: "long directional exposure exceeds limit",
		})
	}
	if result.ShortNotionalPct > MaxDirectionalExposurePct {
		violations = append(violations, Violation{
			Type:        ViolationConcentrationExceeded,
			Metric:      "directional_short",
			Current:     result.ShortNotionalPct,
			Limit:       MaxDirectionalExposurePct,
			Description: "short directional exposure exceeds limit",
		})
	}
	return violations
}

func maxMapValue(m map[string]float64) float64 {
	max := 0.0
	for _, v := range m {
		if v > max {
			max = v
		}
	}
	return max
}
